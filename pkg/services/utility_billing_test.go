package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func newTestUtilityMeter(unit models.UtilityMeterUnit, billingYearStartMonth byte) *models.UtilityMeter {
	return &models.UtilityMeter{
		MeterId:               1,
		Uid:                   1,
		Name:                  "Test Meter",
		Unit:                  unit,
		Currency:              "EUR",
		BillingYearStartMonth: billingYearStartMonth,
	}
}

func newTestUtilityReading(readingId int64, date int32, value int64) *models.UtilityReading {
	return &models.UtilityReading{
		ReadingId:   readingId,
		Uid:         1,
		MeterId:     1,
		ReadingDate: date,
		Value:       value,
	}
}

func TestCalculateUtilityBillingYears_ElectricityMatchesTheTariffOnTheBill(t *testing.T) {
	// an electricity meter at 0.3280 per kWh, a Grundpreis of 11.90 a month, and 81.00 collected
	// every month against it
	meter := newTestUtilityMeter(models.UTILITY_METER_UNIT_KWH, 1)
	tariffs := []*models.UtilityTariff{
		{
			TariffId:          1,
			MeterId:           1,
			StartDate:         20260101,
			UnitPrice:         328000,
			BaseFeeAnnual:     14280,
			MonthlyPrepayment: 8100,
		},
	}
	readings := []*models.UtilityReading{
		newTestUtilityReading(1, 20260601, 8461000),
		newTestUtilityReading(2, 20260701, 8538000),
	}

	years := CalculateUtilityBillingYears(meter, tariffs, readings)

	assert.Equal(t, 1, len(years))
	assert.Equal(t, "2026", years[0].Label)
	assert.Equal(t, 1, len(years[0].Periods))

	period := years[0].Periods[0]

	assert.Equal(t, int32(30), period.Days)
	assert.Equal(t, int64(77000), period.UsedUnits)
	// a meter read in the unit it is priced in reports no converted energy of its own
	assert.Equal(t, int64(0), period.UsedEnergy)
	// 77 kWh x 0.3280 = 25.256
	assert.Equal(t, int64(2526), period.EnergyCost)
	// 142.80 a year over 30 of its days
	assert.Equal(t, int64(1174), period.BaseFee)
	assert.Equal(t, int64(3700), period.Cost)
	// 81.00 a month over 30 days
	assert.Equal(t, int64(7989), period.Prepaid)
	assert.Equal(t, int64(4289), period.Difference)
	assert.False(t, period.PriceChanged)
	assert.False(t, period.Partial)
}

func TestCalculateUtilityBillingYears_GasIsConvertedBeforeItIsPriced(t *testing.T) {
	// a gas meter counting cubic metres at 0.10 per kWh, a Brennwert of 10 kWh/m3, a Zustandszahl
	// of 0.95 and a Grundpreis of 13.90 a month
	meter := newTestUtilityMeter(models.UTILITY_METER_UNIT_CUBIC_METRE, 1)
	tariffs := []*models.UtilityTariff{
		{
			TariffId:          1,
			MeterId:           1,
			StartDate:         20260101,
			UnitPrice:         100000,
			BaseFeeAnnual:     16680,
			CalorificValue:    100000,
			StateNumber:       9500,
			MonthlyPrepayment: 8500,
		},
	}
	readings := []*models.UtilityReading{
		newTestUtilityReading(1, 20260601, 306300),
		newTestUtilityReading(2, 20260701, 312380),
	}

	years := CalculateUtilityBillingYears(meter, tariffs, readings)

	assert.Equal(t, 1, len(years))

	period := years[0].Periods[0]

	assert.Equal(t, int64(6080), period.UsedUnits)
	// 6.08 m3 x 10 kWh/m3 x 0.95 = 57.76 kWh
	assert.Equal(t, int64(57760), period.UsedEnergy)
	// 57.76 kWh x 0.10 = 5.776
	assert.Equal(t, int64(578), period.EnergyCost)
	assert.Equal(t, int64(1371), period.BaseFee)
	assert.Equal(t, int64(1949), period.Cost)
}

func TestCalculateUtilityBillingYears_UseIsSplitOverAPriceChange(t *testing.T) {
	// the price doubles halfway through a sixty day period, so half the use is priced at each
	meter := newTestUtilityMeter(models.UTILITY_METER_UNIT_KWH, 1)
	tariffs := []*models.UtilityTariff{
		{
			TariffId:      1,
			MeterId:       1,
			StartDate:     20260101,
			UnitPrice:     100000,
			BaseFeeAnnual: 0,
		},
		{
			TariffId:      2,
			MeterId:       1,
			StartDate:     20260401,
			UnitPrice:     200000,
			BaseFeeAnnual: 0,
		},
	}
	readings := []*models.UtilityReading{
		newTestUtilityReading(1, 20260301, 1000000),
		newTestUtilityReading(2, 20260501, 1600000),
	}

	years := CalculateUtilityBillingYears(meter, tariffs, readings)
	period := years[0].Periods[0]

	assert.Equal(t, int32(61), period.Days)
	assert.Equal(t, int64(600000), period.UsedUnits)
	assert.True(t, period.PriceChanged)

	// 600 kWh over 61 days: 304.918 kWh of it in the 31 days at 0.10, and the remaining
	// 295.082 kWh in the 30 days at 0.20
	assert.Equal(t, int64(8951), period.EnergyCost)
	assert.Equal(t, int64(8951), period.Cost)
}

func TestCalculateUtilityBillingYears_IntervalIsCutWhereTheBillingYearEnds(t *testing.T) {
	// a billing year that turns on the first of June, read on the third of every month
	meter := newTestUtilityMeter(models.UTILITY_METER_UNIT_KWH, 6)
	tariffs := []*models.UtilityTariff{
		{
			TariffId:          1,
			MeterId:           1,
			StartDate:         20250101,
			UnitPrice:         1000000,
			MonthlyPrepayment: 10000,
		},
	}
	readings := []*models.UtilityReading{
		newTestUtilityReading(1, 20260503, 1000000),
		newTestUtilityReading(2, 20260603, 1031000),
	}

	years := CalculateUtilityBillingYears(meter, tariffs, readings)

	assert.Equal(t, 2, len(years))
	// newest first
	assert.Equal(t, "2026-2027", years[0].Label)
	assert.Equal(t, "2025-2026", years[1].Label)

	oldYearPeriod := years[1].Periods[0]
	newYearPeriod := years[0].Periods[0]

	// 29 of the 31 days fall in the old billing year and 2 in the new one, and the use is spread
	// over the days because there is no reading for the first of June
	assert.Equal(t, int32(29), oldYearPeriod.Days)
	assert.Equal(t, int32(2), newYearPeriod.Days)
	assert.Equal(t, int64(29000), oldYearPeriod.UsedUnits)
	assert.Equal(t, int64(2000), newYearPeriod.UsedUnits)
	assert.True(t, oldYearPeriod.Partial)
	assert.True(t, newYearPeriod.Partial)

	// the pieces add back up to the reading that ended the interval
	assert.Equal(t, int64(1000000), oldYearPeriod.StartValue)
	assert.Equal(t, newYearPeriod.StartValue, oldYearPeriod.EndValue)
	assert.Equal(t, int64(1031000), newYearPeriod.EndValue)
}

func TestCalculateUtilityBillingYears_ProjectsTheYearFromWhatHasBeenRead(t *testing.T) {
	// a whole month of a calendar billing year read, at 1.00 a kWh with 100.00 collected a month
	meter := newTestUtilityMeter(models.UTILITY_METER_UNIT_KWH, 1)
	tariffs := []*models.UtilityTariff{
		{
			TariffId:          1,
			MeterId:           1,
			StartDate:         20260101,
			UnitPrice:         1000000,
			MonthlyPrepayment: 10000,
		},
	}
	readings := []*models.UtilityReading{
		newTestUtilityReading(1, 20260101, 0),
		newTestUtilityReading(2, 20260201, 100000),
	}

	year := CalculateUtilityBillingYears(meter, tariffs, readings)[0]

	assert.False(t, year.Complete)
	assert.Equal(t, int32(31), year.DaysCovered)
	assert.Equal(t, int32(365), year.DaysInYear)
	assert.Equal(t, int64(10000), year.Cost)
	// 100.00 of cost over 31 days, carried across all 365 of them
	assert.Equal(t, int64(117742), year.ProjectedCost)
	// twelve collections of 100.00, whatever has been read
	assert.Equal(t, int64(120000), year.ProjectedPrepaid)
	assert.Equal(t, int64(2258), year.ProjectedDifference)
}

func TestCalculateUtilityBillingYears_AYearReadToItsEndIsComplete(t *testing.T) {
	meter := newTestUtilityMeter(models.UTILITY_METER_UNIT_KWH, 1)
	tariffs := []*models.UtilityTariff{
		{
			TariffId:  1,
			MeterId:   1,
			StartDate: 20260101,
			UnitPrice: 1000000,
		},
	}
	readings := []*models.UtilityReading{
		newTestUtilityReading(1, 20260101, 0),
		newTestUtilityReading(2, 20270101, 100000),
	}

	years := CalculateUtilityBillingYears(meter, tariffs, readings)

	assert.Equal(t, 1, len(years))
	assert.True(t, years[0].Complete)
	assert.Equal(t, int32(365), years[0].DaysCovered)
	assert.Equal(t, years[0].Cost, years[0].ProjectedCost)
}

func TestCalculateUtilityBillingYears_AReplacedMeterCountsAsNoUse(t *testing.T) {
	// a reading below the one before it is a meter that was swapped or rolled over, and counting it
	// as negative use would report a refund that is not owed
	meter := newTestUtilityMeter(models.UTILITY_METER_UNIT_KWH, 1)
	tariffs := []*models.UtilityTariff{
		{
			TariffId:  1,
			MeterId:   1,
			StartDate: 20260101,
			UnitPrice: 1000000,
		},
	}
	readings := []*models.UtilityReading{
		newTestUtilityReading(1, 20260101, 900000),
		newTestUtilityReading(2, 20260201, 1000),
	}

	period := CalculateUtilityBillingYears(meter, tariffs, readings)[0].Periods[0]

	assert.Equal(t, int64(0), period.UsedUnits)
	assert.Equal(t, int64(0), period.EnergyCost)
}

func TestCalculateUtilityBillingYears_ASingleReadingIsNotAPeriod(t *testing.T) {
	meter := newTestUtilityMeter(models.UTILITY_METER_UNIT_KWH, 1)
	readings := []*models.UtilityReading{
		newTestUtilityReading(1, 20260101, 900000),
	}

	assert.Equal(t, 0, len(CalculateUtilityBillingYears(meter, nil, readings)))
}

func TestCalculateUtilityBillingYears_UseIsCountedWithNoTariffOnFile(t *testing.T) {
	// what a meter has used is a fact about the meter, and is still worth showing before anybody
	// has said what it costs
	meter := newTestUtilityMeter(models.UTILITY_METER_UNIT_KWH, 1)
	readings := []*models.UtilityReading{
		newTestUtilityReading(1, 20260101, 0),
		newTestUtilityReading(2, 20260201, 100000),
	}

	period := CalculateUtilityBillingYears(meter, nil, readings)[0].Periods[0]

	assert.Equal(t, int64(100000), period.UsedUnits)
	assert.Equal(t, int64(0), period.Cost)
	assert.Equal(t, int64(0), period.Prepaid)
}

func TestUtilityMulDiv_RoundsHalfAwayFromZero(t *testing.T) {
	assert.Equal(t, int64(3), utilityMulDiv(5, 1, 2))
	assert.Equal(t, int64(-3), utilityMulDiv(-5, 1, 2))
	assert.Equal(t, int64(1), utilityMulDiv(4, 1, 3))
	assert.Equal(t, int64(0), utilityMulDiv(1, 1, 3))
	assert.Equal(t, int64(0), utilityMulDiv(0, 1, 3))
	assert.Equal(t, int64(0), utilityMulDiv(1, 1, 0))
}

func TestUtilityMulDiv_SurvivesNumbersThatWouldOverflowAnInt64(t *testing.T) {
	// the largest reading this program accepts, at the largest price, is past what an int64 holds
	// before it is divided back down
	largestEnergy := models.MaximumUtilityReadingValue * models.UtilityReadingValueScale
	largestPrice := models.MaximumUtilityUnitPrice * models.UtilityUnitPriceScale

	assert.Equal(t, int64(10000000000000000), utilityMulDiv(largestEnergy, largestPrice, utilityCostDivisor))
}
