package finance

import (
	"math"
	"math/big"
	"strings"

	"golang.org/x/text/currency"
)

const (
	fxDecimalRadix                   = 10
	fxConcatenatedRateFractionDigits = 4
	fxLocalizedAmountGroupSize       = 3
)

// fxEvidence is the syntax-independent FX data used by transfer matching.
type fxEvidence struct {
	namespace       string
	reference       string
	baseCurrency    string
	quoteCurrency   string
	rateCoefficient string
	rateScale       int
}

func extractFXEvidence(description string) (fxEvidence, bool) {
	cursor := 0
	if !consumeFXLiteral(description, &cursor, "FX") {
		return fxEvidence{}, false
	}
	reference, ok := consumeFXDigits(description, &cursor)
	if !ok || !consumeFXLiteral(description, &cursor, " ") {
		return fxEvidence{}, false
	}
	baseCurrency, ok := consumeFXCurrency(description, &cursor)
	if !ok || !consumeFXLiteral(description, &cursor, "/") {
		return fxEvidence{}, false
	}
	quoteCurrency, ok := consumeFXCurrency(description, &cursor)
	if !ok || !consumeFXLiteral(description, &cursor, " ") || baseCurrency == quoteCurrency {
		return fxEvidence{}, false
	}
	if !isRecognizedCurrency(baseCurrency) || !isRecognizedCurrency(quoteCurrency) {
		return fxEvidence{}, false
	}
	rate, hasAmounts, ok := consumeFXRate(description, &cursor)
	if !ok {
		return fxEvidence{}, false
	}
	if hasAmounts && (!consumeFXLocalizedAmount(description, &cursor, false) ||
		!consumeFXLiteral(description, &cursor, " ") ||
		!consumeFXCurrencyLiteral(description, &cursor, baseCurrency) ||
		!consumeFXLiteral(description, &cursor, " ") ||
		!consumeFXLocalizedAmount(description, &cursor, true) ||
		!consumeFXLiteral(description, &cursor, " ") ||
		!consumeFXCurrencyLiteral(description, &cursor, quoteCurrency)) {
		return fxEvidence{}, false
	}
	if cursor != len(description) {
		return fxEvidence{}, false
	}
	coefficient, scale, ok := normalizeFXRate(rate)
	if !ok {
		return fxEvidence{}, false
	}
	return fxEvidence{
		namespace:       "FX",
		reference:       reference,
		baseCurrency:    baseCurrency,
		quoteCurrency:   quoteCurrency,
		rateCoefficient: coefficient,
		rateScale:       scale,
	}, true
}

func consumeFXLiteral(description string, cursor *int, literal string) bool {
	if !strings.HasPrefix(description[*cursor:], literal) {
		return false
	}
	*cursor += len(literal)
	return true
}

func consumeFXDigits(description string, cursor *int) (string, bool) {
	start := *cursor
	for *cursor < len(description) && description[*cursor] >= '0' && description[*cursor] <= '9' {
		*cursor++
	}
	return description[start:*cursor], *cursor > start
}

func consumeFXCurrency(description string, cursor *int) (string, bool) {
	if *cursor+3 > len(description) {
		return "", false
	}
	code := description[*cursor : *cursor+3]
	for index := range len(code) {
		if code[index] < 'A' || code[index] > 'Z' {
			return "", false
		}
	}
	*cursor += len(code)
	return code, true
}

func consumeFXCurrencyLiteral(description string, cursor *int, currency string) bool {
	return consumeFXLiteral(description, cursor, currency)
}

func consumeFXRate(description string, cursor *int) (string, bool, bool) {
	start := *cursor
	_, hasInteger := consumeFXDigits(description, cursor)
	if !hasInteger {
		return "", false, false
	}
	if *cursor == len(description) {
		return description[start:*cursor], false, true
	}
	if description[*cursor] != '.' && description[*cursor] != ',' {
		return "", false, false
	}
	*cursor++
	fractionStart := *cursor
	_, hasFraction := consumeFXDigits(description, cursor)
	if !hasFraction {
		return "", false, false
	}
	if *cursor == len(description) {
		return description[start:*cursor], false, true
	}
	if fractionStart+fxConcatenatedRateFractionDigits >= len(description) ||
		description[fractionStart+fxConcatenatedRateFractionDigits] < '0' ||
		description[fractionStart+fxConcatenatedRateFractionDigits] > '9' {
		return "", false, false
	}
	*cursor = fractionStart + fxConcatenatedRateFractionDigits
	return description[start:*cursor], true, true
}

func consumeFXLocalizedAmount(description string, cursor *int, signed bool) bool {
	if signed {
		if !consumeFXLiteral(description, cursor, "-") {
			return false
		}
	}
	start := *cursor
	_, hasInteger := consumeFXDigits(description, cursor)
	if !hasInteger {
		return false
	}
	if strings.HasPrefix(description[*cursor:], "\u00a0") {
		if *cursor-start > fxLocalizedAmountGroupSize {
			return false
		}
		for strings.HasPrefix(description[*cursor:], "\u00a0") {
			*cursor += len("\u00a0")
			groupStart := *cursor
			_, hasGroup := consumeFXDigits(description, cursor)
			if !hasGroup || *cursor-groupStart != fxLocalizedAmountGroupSize {
				return false
			}
		}
	}
	if !consumeFXLiteral(description, cursor, ",") {
		return false
	}
	fractionStart := *cursor
	_, hasFraction := consumeFXDigits(description, cursor)
	return hasFraction && *cursor-fractionStart == 2
}

func fxConversionMatches(
	evidence fxEvidence,
	firstCurrency string,
	firstAmountMinor int64,
	secondCurrency string,
	secondAmountMinor int64,
) bool {
	if evidence.baseCurrency == evidence.quoteCurrency ||
		!isRecognizedCurrency(evidence.baseCurrency) ||
		!isRecognizedCurrency(evidence.quoteCurrency) ||
		evidence.rateScale < 0 ||
		(firstAmountMinor < 0) == (secondAmountMinor < 0) {
		return false
	}
	firstMagnitude, firstOK := positiveMinorMagnitude(firstAmountMinor)
	secondMagnitude, secondOK := positiveMinorMagnitude(secondAmountMinor)
	if !firstOK || !secondOK {
		return false
	}
	var baseMagnitude, quoteMagnitude *big.Int
	switch {
	case firstCurrency == evidence.baseCurrency && secondCurrency == evidence.quoteCurrency:
		baseMagnitude, quoteMagnitude = firstMagnitude, secondMagnitude
	case firstCurrency == evidence.quoteCurrency && secondCurrency == evidence.baseCurrency:
		baseMagnitude, quoteMagnitude = secondMagnitude, firstMagnitude
	default:
		return false
	}
	rateCoefficient, ok := positiveDecimalInteger(evidence.rateCoefficient)
	if !ok {
		return false
	}
	baseUnit, baseOK := parseCurrency(evidence.baseCurrency)
	quoteUnit, quoteOK := parseCurrency(evidence.quoteCurrency)
	if !baseOK || !quoteOK {
		return false
	}
	baseScale, _ := currency.Standard.Rounding(baseUnit)
	quoteScale, _ := currency.Standard.Rounding(quoteUnit)
	if baseScale < 0 || quoteScale < 0 {
		return false
	}
	numerator := new(big.Int).Mul(baseMagnitude, rateCoefficient)
	numerator.Mul(numerator, powerOfTen(quoteScale))
	denominator := new(big.Int).Mul(powerOfTen(evidence.rateScale), powerOfTen(baseScale))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	if new(big.Int).Lsh(remainder, 1).Cmp(denominator) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return quotient.Cmp(quoteMagnitude) == 0
}

func normalizeFXRate(rate string) (string, int, bool) {
	integerPart, fractionalPart, hasDecimal := strings.Cut(rate, ".")
	if !hasDecimal {
		integerPart, fractionalPart, _ = strings.Cut(rate, ",")
	}
	if strings.ContainsAny(integerPart, ".,") || strings.ContainsAny(fractionalPart, ".,") {
		return "", 0, false
	}
	coefficient := integerPart + fractionalPart
	coefficient = strings.TrimLeft(coefficient, "0")
	if coefficient == "" {
		return "", 0, false
	}
	scale := len(fractionalPart)
	for scale > 0 && coefficient[len(coefficient)-1] == '0' {
		coefficient = coefficient[:len(coefficient)-1]
		scale--
	}
	return coefficient, scale, true
}

func isRecognizedCurrency(code string) bool {
	_, ok := parseCurrency(code)
	return ok
}

func parseCurrency(code string) (currency.Unit, bool) {
	unit, err := currency.ParseISO(code)
	if err != nil || unit == currency.XXX {
		return currency.XXX, false
	}
	return unit, true
}

func positiveMinorMagnitude(amount int64) (*big.Int, bool) {
	if amount == 0 || amount == math.MinInt64 {
		return nil, false
	}
	value := big.NewInt(amount)
	return value.Abs(value), true
}

func positiveDecimalInteger(value string) (*big.Int, bool) {
	if value == "" || strings.Trim(value, "0123456789") != "" {
		return nil, false
	}
	integer, ok := new(big.Int).SetString(value, fxDecimalRadix)
	if !ok || integer.Sign() <= 0 {
		return nil, false
	}
	return integer, true
}

func powerOfTen(scale int) *big.Int {
	return new(big.Int).Exp(big.NewInt(fxDecimalRadix), big.NewInt(int64(scale)), nil)
}
