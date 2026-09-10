package finance

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFXEvidence(t *testing.T) {
	makeEvidence := func(rateCoefficient string, rateScale int) fxEvidence {
		return fxEvidence{
			namespace:       "FX",
			reference:       "123",
			baseCurrency:    "USD",
			quoteCurrency:   "PLN",
			rateCoefficient: rateCoefficient,
			rateScale:       rateScale,
		}
	}

	t.Run("extracts complete syntax and normalizes equivalent decimal rates", func(t *testing.T) {
		reference := "123456789"
		description := "FX" + reference + " USD/PLN 4.12500"

		actual, ok := extractFXEvidence(description)

		require.True(t, ok)
		expected := fxEvidence{
			namespace:       "FX",
			reference:       reference,
			baseCurrency:    "USD",
			quoteCurrency:   "PLN",
			rateCoefficient: "4125",
			rateScale:       3,
		}
		assert.Equal(t, expected, actual)
		comma, commaOK := extractFXEvidence("FX" + reference + " USD/PLN 4,12500")
		require.True(t, commaOK)
		assert.Equal(t, expected.rateCoefficient, comma.rateCoefficient)
		assert.Equal(t, expected.rateScale, comma.rateScale)
	})

	t.Run("rejects unknown malformed invalid and nonpositive descriptions", func(t *testing.T) {
		descriptions := []string{
			"not an FX description",
			"FX123 USD/PLN",
			"FX123 USD/PLN .125",
			"FX123 USD/PLN 4.",
			"FX123 USD/PLN 4,125.0",
			"FX123 USD/PLN -4.125",
			"FX123 USD/PLN 0",
			"FX123 USD/PLN 0.000",
			"FX123 USD/USD 1.0",
			"FX123 ZZZ/PLN 1.0",
			"FX123 usd/PLN 1.0",
		}
		for _, description := range descriptions {
			_, ok := extractFXEvidence(description)
			assert.False(t, ok, description)
		}
	})

	t.Run("matches exact conversions regardless of the debited currency", func(t *testing.T) {
		evidence := makeEvidence("4125", 3)

		assert.True(t, fxConversionMatches(evidence, "USD", -10_000, "PLN", 41_250))
		assert.True(t, fxConversionMatches(evidence, "PLN", -41_250, "USD", 10_000))
		assert.False(t, fxConversionMatches(evidence, "USD", -10_000, "PLN", 41_249))
		assert.False(t, fxConversionMatches(evidence, "USD", 10_000, "PLN", 41_250))
	})

	t.Run("rounds quote minor units upward at halfway and uses currency scales", func(t *testing.T) {
		belowHalf := makeEvidence("449", 2)
		halfway := makeEvidence("45", 1)
		jpyToKuwaitiDinar := fxEvidence{
			namespace: "FX", reference: "456", baseCurrency: "JPY", quoteCurrency: "KWD",
			rateCoefficient: "12345", rateScale: 4,
		}

		assert.True(t, fxConversionMatches(belowHalf, "USD", -1, "PLN", 4))
		assert.False(t, fxConversionMatches(belowHalf, "USD", -1, "PLN", 5))
		assert.True(t, fxConversionMatches(halfway, "USD", -1, "PLN", 5))
		assert.False(t, fxConversionMatches(halfway, "USD", -1, "PLN", 4))
		assert.True(t, fxConversionMatches(jpyToKuwaitiDinar, "JPY", 2, "KWD", -2_469))
	})

	t.Run("fails closed for invalid evidence and safely handles large values", func(t *testing.T) {
		unknownCurrency := makeEvidence("1", 0)
		unknownCurrency.baseCurrency = "ZZZ"
		equalCurrencies := makeEvidence("1", 0)
		equalCurrencies.quoteCurrency = "USD"
		zeroRate := makeEvidence("0", 0)
		largeRate := makeEvidence("100000000000000000001", 20)

		assert.False(t, fxConversionMatches(unknownCurrency, "ZZZ", -1, "PLN", 1))
		assert.False(t, fxConversionMatches(equalCurrencies, "USD", -1, "USD", 1))
		assert.False(t, fxConversionMatches(zeroRate, "USD", -1, "PLN", 1))
		assert.False(t, fxConversionMatches(largeRate, "USD", -math.MaxInt64, "PLN", math.MaxInt64-1))
		assert.True(t, fxConversionMatches(largeRate, "USD", math.MaxInt64, "PLN", -math.MaxInt64))
	})
}
