package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/errs"
)

func TestCalculateCryptoHoldingValue_RoundsToMinorUnits(t *testing.T) {
	// 0.3421 x 43250.75 = 14796.081575, which is 14796.08 to the cent
	actualValue, err := CalculateCryptoHoldingValue("0.34210000", "43250.75")

	assert.Nil(t, err)
	assert.Equal(t, int64(1479608), actualValue)

	// the half-up boundary
	actualValue, err = CalculateCryptoHoldingValue("1", "0.005")

	assert.Nil(t, err)
	assert.Equal(t, int64(1), actualValue)

	actualValue, err = CalculateCryptoHoldingValue("1", "0.004")

	assert.Nil(t, err)
	assert.Equal(t, int64(0), actualValue)
}

func TestCalculateCryptoHoldingValue_LargeQuantityOfACheapCoin(t *testing.T) {
	// a billion tokens at a hundredth of a cent each, which is the case a fixed int64 scale
	// cannot represent: a billion tokens in units of 1e-8 already overflows
	actualValue, err := CalculateCryptoHoldingValue("1000000000", "0.00002341")

	assert.Nil(t, err)
	assert.Equal(t, int64(2341000), actualValue)
}

func TestCalculateCryptoHoldingValue_EighteenDecimalPlaces(t *testing.T) {
	actualValue, err := CalculateCryptoHoldingValue("0.123456789012345678", "2500")

	assert.Nil(t, err)
	assert.Equal(t, int64(30864), actualValue)
}

func TestCalculateCryptoHoldingValue_WorthLessThanACentIsNotAnError(t *testing.T) {
	actualValue, err := CalculateCryptoHoldingValue("0.00000001", "43250.75")

	assert.Nil(t, err)
	assert.Equal(t, int64(0), actualValue)
}

func TestCalculateCryptoHoldingValue_PriceInExponentNotation(t *testing.T) {
	// prices are not user input, so a data source printing a small price as an exponent is read
	// rather than rejected
	actualValue, err := CalculateCryptoHoldingValue("1000000000", "2.341e-5")

	assert.Nil(t, err)
	assert.Equal(t, int64(2341000), actualValue)
}

func TestCalculateCryptoHoldingValue_ZeroAmount(t *testing.T) {
	actualValue, err := CalculateCryptoHoldingValue("0", "43250.75")

	assert.Nil(t, err)
	assert.Equal(t, int64(0), actualValue)
}

func TestValidateCryptoAmount_InvalidAmount(t *testing.T) {
	invalidAmounts := []string{
		"",
		"-0.5",
		"1e10",
		"1,5",
		"0x1",
		" 1",
		"1 ",
		"1.2.3",
		".5",
		"0.1234567890123456789", // 19 fraction digits
	}

	for i := 0; i < len(invalidAmounts); i++ {
		err := ValidateCryptoAmount(invalidAmounts[i])
		assert.EqualError(t, err, errs.ErrInvalidCryptoAmount.Message, "amount \"%s\" should be rejected", invalidAmounts[i])
	}
}

func TestValidateCryptoAmount_ValidAmount(t *testing.T) {
	validAmounts := []string{
		"0",
		"1",
		"0.5",
		"0.00000001",
		"1000000000",
		"0.123456789012345678",
	}

	for i := 0; i < len(validAmounts); i++ {
		err := ValidateCryptoAmount(validAmounts[i])
		assert.Nil(t, err, "amount \"%s\" should be accepted", validAmounts[i])
	}
}

func TestCalculateWeightedPriceChange_WeightsByValue(t *testing.T) {
	// one percent of the portfolio moving forty percent moves the portfolio by 0.4 percent, and
	// not by the twenty an unweighted mean would report
	values := []int64{100, 9900}
	priceChanges := []string{"40", "0"}

	assert.Equal(t, "0.40", CalculateWeightedPriceChange(values, priceChanges))
}

func TestCalculateWeightedPriceChange_NegativeChanges(t *testing.T) {
	values := []int64{5000, 5000}
	priceChanges := []string{"-4.5", "1.5"}

	assert.Equal(t, "-1.50", CalculateWeightedPriceChange(values, priceChanges))
}

func TestCalculateWeightedPriceChange_SkipsCoinsWithoutAChange(t *testing.T) {
	values := []int64{5000, 5000}
	priceChanges := []string{"2", ""}

	assert.Equal(t, "2.00", CalculateWeightedPriceChange(values, priceChanges))
}

func TestCalculateWeightedPriceChange_NothingToWeigh(t *testing.T) {
	assert.Equal(t, "", CalculateWeightedPriceChange([]int64{}, []string{}))
	assert.Equal(t, "", CalculateWeightedPriceChange([]int64{0}, []string{"5"}))
	assert.Equal(t, "", CalculateWeightedPriceChange([]int64{100}, []string{"5", "6"}))
}
