package finance

import (
	"math"
	"math/big"
	"regexp"
	"strings"

	"golang.org/x/text/currency"
)

const fxDecimalRadix = 10

var fxDescriptionPattern = regexp.MustCompile(`^FX([0-9]+) ([A-Z]{3})/([A-Z]{3}) ([0-9]+(?:[,.][0-9]+)?)$`)

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
	match := fxDescriptionPattern.FindStringSubmatch(description)
	if match == nil || match[2] == match[3] {
		return fxEvidence{}, false
	}
	if !isRecognizedCurrency(match[2]) || !isRecognizedCurrency(match[3]) {
		return fxEvidence{}, false
	}
	coefficient, scale, ok := normalizeFXRate(match[4])
	if !ok {
		return fxEvidence{}, false
	}
	return fxEvidence{
		namespace:       "FX",
		reference:       match[1],
		baseCurrency:    match[2],
		quoteCurrency:   match[3],
		rateCoefficient: coefficient,
		rateScale:       scale,
	}, true
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
