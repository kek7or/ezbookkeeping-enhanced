package services

import (
	"math/big"
	"regexp"

	"github.com/mayswind/ezbookkeeping/pkg/errs"
)

// cryptoAmountPattern is what a holding amount may look like. Quantities are plain decimals with
// at most 18 fraction digits, the most an ERC-20 token uses. Exponent notation, thousands
// separators, commas as the decimal mark and negative quantities are all rejected, so that what
// is stored is always exactly what the user typed.
var cryptoAmountPattern = regexp.MustCompile(`^\d{1,24}(\.\d{1,18})?$`)

// cryptoMinorUnitFactor is the scale every value is reported in, matching utils.ParseAmount
var cryptoMinorUnitFactor = big.NewRat(100, 1)

// ValidateCryptoAmount returns whether a holding amount is a quantity this application can store
func ValidateCryptoAmount(amount string) error {
	if !cryptoAmountPattern.MatchString(amount) {
		return errs.ErrInvalidCryptoAmount
	}

	return nil
}

// parseCryptoAmount returns the exact rational value of a holding amount
func parseCryptoAmount(amount string) (*big.Rat, error) {
	if err := ValidateCryptoAmount(amount); err != nil {
		return nil, err
	}

	value, ok := new(big.Rat).SetString(amount)

	if !ok {
		return nil, errs.ErrInvalidCryptoAmount
	}

	return value, nil
}

// parseCryptoPrice returns the exact rational value of a price as the data source printed it
//
// Prices are parsed leniently, unlike amounts: they are not user input, and a data source is free
// to print a small price in exponent notation.
func parseCryptoPrice(price string) (*big.Rat, bool) {
	if price == "" {
		return nil, false
	}

	value, ok := new(big.Rat).SetString(price)

	if !ok || value.Sign() < 0 {
		return nil, false
	}

	return value, true
}

// CalculateCryptoHoldingValue returns what a quantity of a coin is worth, in minor units of the
// currency the price is quoted in
//
// The multiplication is exact and only the final result is rounded, half-up, so a value never
// drifts the way it would if the price were held in a float64.
func CalculateCryptoHoldingValue(amount string, price string) (int64, error) {
	parsedAmount, err := parseCryptoAmount(amount)

	if err != nil {
		return 0, err
	}

	parsedPrice, ok := parseCryptoPrice(price)

	if !ok {
		return 0, errs.ErrInvalidCryptoAmount
	}

	value := new(big.Rat).Mul(parsedAmount, parsedPrice)

	return ratToMinorUnits(value)
}

// ratToMinorUnits returns a non-negative rational value in minor units, rounded half-up
func ratToMinorUnits(value *big.Rat) (int64, error) {
	scaled := new(big.Rat).Mul(value, cryptoMinorUnitFactor)

	// floor(num/denom + 1/2), which is (2*num + denom) / (2*denom) in integer arithmetic
	numerator := new(big.Int).Mul(scaled.Num(), big.NewInt(2))
	numerator.Add(numerator, scaled.Denom())
	denominator := new(big.Int).Mul(scaled.Denom(), big.NewInt(2))
	rounded := new(big.Int).Div(numerator, denominator)

	if !rounded.IsInt64() {
		return 0, errs.ErrInvalidCryptoAmount
	}

	return rounded.Int64(), nil
}

// CalculateWeightedPriceChange returns the price change of a whole portfolio: the mean of the
// per-coin changes weighted by what each coin is worth
//
// The weighting is what makes the number honest. A coin that is one percent of the portfolio
// moving forty percent moves the portfolio by 0.4 percent, and an unweighted mean of the same
// two changes would report twenty.
func CalculateWeightedPriceChange(values []int64, priceChanges []string) string {
	if len(values) != len(priceChanges) {
		return ""
	}

	weightedSum := new(big.Rat)
	totalWeight := new(big.Rat)

	for i := 0; i < len(values); i++ {
		change, ok := new(big.Rat).SetString(priceChanges[i])

		if priceChanges[i] == "" || !ok || values[i] <= 0 {
			continue
		}

		weight := new(big.Rat).SetInt64(values[i])
		weightedSum = weightedSum.Add(weightedSum, new(big.Rat).Mul(weight, change))
		totalWeight = totalWeight.Add(totalWeight, weight)
	}

	if totalWeight.Sign() == 0 {
		return ""
	}

	return new(big.Rat).Quo(weightedSum, totalWeight).FloatString(2)
}
