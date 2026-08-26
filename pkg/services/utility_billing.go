package services

import (
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// utilityMinorUnitFactor is the scale every amount in this program is held in, matching
// utils.ParseAmount
const utilityMinorUnitFactor = int64(100)

// utilityCostDivisor turns a use held at the reading scale, multiplied by a price held at the unit
// price scale, back into an amount at the minor unit scale
const utilityCostDivisor = models.UtilityReadingValueScale * models.UtilityUnitPriceScale / utilityMinorUnitFactor

// utilityYearSegment is one stretch of days that belongs to a single billing year, together with
// what the meter was reading at each end of it
type utilityYearSegment struct {
	yearStart  time.Time
	start      time.Time
	end        time.Time
	days       int64
	startValue int64
	endValue   int64
	estimated  bool
	partial    bool
}

// CalculateUtilityBillingYears works out what a meter has used, what that is estimated to have cost
// and how it stands against what has been paid for it, billing year by billing year.
//
// Nothing here is a bill. The supplier bills on its own readings, its own rounding and a tax
// treatment this knows nothing about; what this produces is the number the user would otherwise
// keep in a spreadsheet, close enough to tell a refund from a demand months before the bill says so.
//
// The readings and the tariffs may arrive in any order.
func CalculateUtilityBillingYears(meter *models.UtilityMeter, tariffs []*models.UtilityTariff, readings []*models.UtilityReading) []*models.UtilityBillingYearResponse {
	if meter == nil || len(readings) < 2 {
		return make([]*models.UtilityBillingYearResponse, 0)
	}

	sortedTariffs := sortUtilityTariffs(tariffs)
	sortedReadings := sortUtilityReadings(readings)

	yearsByStart := make(map[int32]*models.UtilityBillingYearResponse)
	yearOrder := make([]int32, 0, 4)

	for i := 1; i < len(sortedReadings); i++ {
		previous := sortedReadings[i-1]
		current := sortedReadings[i]

		previousDate, err := utils.ParseFromNumericYearMonthDay(previous.ReadingDate)

		if err != nil {
			continue
		}

		currentDate, err := utils.ParseFromNumericYearMonthDay(current.ReadingDate)

		if err != nil {
			continue
		}

		for _, segment := range splitUtilityIntervalByBillingYear(meter, previous, current, previousDate, currentDate) {
			period := calculateUtilityPeriod(meter, sortedTariffs, segment)

			if period == nil {
				continue
			}

			yearStartDate := utils.FormatDateToNumericYearMonthDay(segment.yearStart)
			year, exists := yearsByStart[yearStartDate]

			if !exists {
				year = newUtilityBillingYear(segment.yearStart)
				yearsByStart[yearStartDate] = year
				yearOrder = append(yearOrder, yearStartDate)
			}

			year.Periods = append(year.Periods, period)
		}
	}

	years := make([]*models.UtilityBillingYearResponse, 0, len(yearOrder))

	for _, yearStartDate := range yearOrder {
		year := yearsByStart[yearStartDate]
		summariseUtilityBillingYear(meter, sortedTariffs, year)
		years = append(years, year)
	}

	// newest first, because the year being lived in is the one being looked at
	sort.Slice(years, func(i, j int) bool {
		return years[i].StartDate > years[j].StartDate
	})

	return years
}

// sortUtilityTariffs returns the tariffs of a meter oldest first, which is the order the search for
// the one in force on a given day walks them in
func sortUtilityTariffs(tariffs []*models.UtilityTariff) []*models.UtilityTariff {
	sorted := make([]*models.UtilityTariff, len(tariffs))
	copy(sorted, tariffs)

	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].StartDate != sorted[j].StartDate {
			return sorted[i].StartDate < sorted[j].StartDate
		}

		return sorted[i].TariffId < sorted[j].TariffId
	})

	return sorted
}

// sortUtilityReadings returns the readings of a meter oldest first
func sortUtilityReadings(readings []*models.UtilityReading) []*models.UtilityReading {
	sorted := make([]*models.UtilityReading, len(readings))
	copy(sorted, readings)

	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].ReadingDate != sorted[j].ReadingDate {
			return sorted[i].ReadingDate < sorted[j].ReadingDate
		}

		return sorted[i].ReadingId < sorted[j].ReadingId
	})

	return sorted
}

// getUtilityBillingYearStart returns the first day of the billing year the given day falls in.
//
// A supplier settles a contract on its anniversary rather than on New Year's Day, and a balance
// counted over the wrong twelve months is compared against a refund that covers other months than
// it does.
func getUtilityBillingYearStart(date time.Time, billingYearStartMonth byte) time.Time {
	startMonth := int(billingYearStartMonth)

	if startMonth < 1 || startMonth > 12 {
		startMonth = 1
	}

	year := date.Year()

	if int(date.Month()) < startMonth {
		year--
	}

	return time.Date(year, time.Month(startMonth), 1, 0, 0, 0, 0, time.UTC)
}

// splitUtilityIntervalByBillingYear cuts the days between two readings wherever a billing year ends,
// so that no part of one year's use is counted into another's.
//
// A meter read on the third of every month against a year that turns on the first leaves two days
// of the old year in every January reading. Two days is nothing; two days of a January is not, and
// the refund the year is heading for is the whole reason any of this is being counted. Where a cut
// falls, the reading at the cut is interpolated over the days on either side of it - a meter has no
// reading for a day nobody read it, and the only honest thing to do is to spread the use evenly and
// say so.
func splitUtilityIntervalByBillingYear(meter *models.UtilityMeter, previous *models.UtilityReading, current *models.UtilityReading, previousDate time.Time, currentDate time.Time) []utilityYearSegment {
	totalDays := int64(currentDate.Sub(previousDate).Hours() / 24)

	if totalDays <= 0 {
		return nil
	}

	usedUnits := current.Value - previous.Value

	// a reading below the one before it is a meter that was replaced, or one that has rolled over
	// past its last digit. Neither is use, and a negative one would report a refund that is not
	// owed, so the interval is counted as nothing used.
	if usedUnits < 0 {
		usedUnits = 0
	}

	estimated := previous.Estimated || current.Estimated
	cuts := make([]time.Time, 0, 2)
	yearStart := getUtilityBillingYearStart(previousDate, meter.BillingYearStartMonth)

	for {
		nextYearStart := yearStart.AddDate(1, 0, 0)

		if !nextYearStart.Before(currentDate) {
			break
		}

		if nextYearStart.After(previousDate) {
			cuts = append(cuts, nextYearStart)
		}

		yearStart = nextYearStart
	}

	segments := make([]utilityYearSegment, 0, len(cuts)+1)
	segmentStart := previousDate
	startValue := previous.Value
	consumedDays := int64(0)
	consumedUnits := int64(0)

	for _, cut := range cuts {
		segmentDays := int64(cut.Sub(segmentStart).Hours() / 24)
		consumedDays += segmentDays
		unitsSoFar := utilityMulDiv(usedUnits, consumedDays, totalDays)
		segmentUnits := unitsSoFar - consumedUnits

		segments = append(segments, utilityYearSegment{
			yearStart:  getUtilityBillingYearStart(segmentStart, meter.BillingYearStartMonth),
			start:      segmentStart,
			end:        cut,
			days:       segmentDays,
			startValue: startValue,
			endValue:   startValue + segmentUnits,
			estimated:  estimated,
			partial:    true,
		})

		consumedUnits = unitsSoFar
		startValue += segmentUnits
		segmentStart = cut
	}

	// the last segment takes whatever the shares handed out above did not, so that the pieces of an
	// interval always add back up to the reading that ended it
	segments = append(segments, utilityYearSegment{
		yearStart:  getUtilityBillingYearStart(segmentStart, meter.BillingYearStartMonth),
		start:      segmentStart,
		end:        currentDate,
		days:       totalDays - consumedDays,
		startValue: startValue,
		endValue:   previous.Value + usedUnits,
		estimated:  estimated,
		partial:    len(cuts) > 0,
	})

	return segments
}

// calculateUtilityPeriod prices one stretch of days at whichever tariffs covered it
func calculateUtilityPeriod(meter *models.UtilityMeter, tariffs []*models.UtilityTariff, segment utilityYearSegment) *models.UtilityPeriodResponse {
	if segment.days <= 0 {
		return nil
	}

	usedUnits := segment.endValue - segment.startValue

	if usedUnits < 0 {
		usedUnits = 0
	}

	period := &models.UtilityPeriodResponse{
		StartDate:  utils.FormatDateToNumericYearMonthDay(segment.start),
		EndDate:    utils.FormatDateToNumericYearMonthDay(segment.end),
		Days:       int32(segment.days),
		StartValue: segment.startValue,
		EndValue:   segment.endValue,
		UsedUnits:  usedUnits,
		Estimated:  segment.estimated,
		Partial:    segment.partial,
	}

	if len(tariffs) < 1 {
		return period
	}

	priceSegments := splitUtilitySegmentByTariff(tariffs, segment)
	period.PriceChanged = len(priceSegments) > 1

	consumedDays := int64(0)
	consumedUnits := int64(0)

	for _, priceSegment := range priceSegments {
		consumedDays += priceSegment.days
		unitsSoFar := utilityMulDiv(usedUnits, consumedDays, segment.days)
		segmentUnits := unitsSoFar - consumedUnits
		consumedUnits = unitsSoFar

		tariff := priceSegment.tariff
		segmentEnergy := convertUtilityUsageToEnergy(segmentUnits, tariff)

		period.UsedEnergy += segmentEnergy
		period.EnergyCost += utilityMulDiv(segmentEnergy, tariff.UnitPrice, utilityCostDivisor)
		period.BaseFee += utilityMulDiv(tariff.BaseFeeAnnual, priceSegment.days, models.UtilityDaysPerYear)
		period.Prepaid += utilityMulDiv(tariff.MonthlyPrepayment, priceSegment.days*12, models.UtilityDaysPerYear)
	}

	// a meter that is read in the unit it is priced in has no energy of its own to report, and a
	// column of kilowatt hours beside a column of cubic metres that says the same thing twice is
	// only something else to check
	if meter.Unit == models.UTILITY_METER_UNIT_KWH {
		period.UsedEnergy = 0
	}

	period.Cost = period.EnergyCost + period.BaseFee
	period.Difference = period.Prepaid - period.Cost

	return period
}

// utilityPriceSegment is one stretch of days that a single tariff covered
type utilityPriceSegment struct {
	tariff *models.UtilityTariff
	days   int64
}

// splitUtilitySegmentByTariff divides a stretch of days at every price change inside it.
//
// Use is spread evenly over the days rather than guessed at, which is wrong in the small - a cold
// week costs more than a mild one - and is the only division available when all that is known is
// two readings with a price change somewhere between them.
func splitUtilitySegmentByTariff(tariffs []*models.UtilityTariff, segment utilityYearSegment) []utilityPriceSegment {
	segments := make([]utilityPriceSegment, 0, 2)
	cursor := segment.start

	for cursor.Before(segment.end) {
		tariff := getUtilityTariffOn(tariffs, cursor)
		next := getUtilityNextTariffStartAfter(tariffs, cursor)
		end := segment.end

		if next != nil && next.Before(end) {
			end = *next
		}

		segments = append(segments, utilityPriceSegment{
			tariff: tariff,
			days:   int64(end.Sub(cursor).Hours() / 24),
		})

		cursor = end
	}

	if len(segments) < 1 {
		segments = append(segments, utilityPriceSegment{
			tariff: getUtilityTariffOn(tariffs, segment.start),
			days:   segment.days,
		})
	}

	return segments
}

// getUtilityTariffOn returns the tariff in force on a given day.
//
// A day before every tariff on file is priced at the oldest one rather than left uncosted: somebody
// entering a year of readings and the price they are paying now should see a year of numbers, not a
// blank column and no way to tell why.
func getUtilityTariffOn(tariffs []*models.UtilityTariff, date time.Time) *models.UtilityTariff {
	numericDate := utils.FormatDateToNumericYearMonthDay(date)
	result := tariffs[0]

	for _, tariff := range tariffs {
		if tariff.StartDate <= numericDate {
			result = tariff
		} else {
			break
		}
	}

	return result
}

// getUtilityNextTariffStartAfter returns the first day after the given one on which the price
// changes, or nothing when no later tariff is on file
func getUtilityNextTariffStartAfter(tariffs []*models.UtilityTariff, date time.Time) *time.Time {
	numericDate := utils.FormatDateToNumericYearMonthDay(date)

	for _, tariff := range tariffs {
		if tariff.StartDate <= numericDate {
			continue
		}

		startDate, err := utils.ParseFromNumericYearMonthDay(tariff.StartDate)

		if err != nil {
			continue
		}

		return &startDate
	}

	return nil
}

// convertUtilityUsageToEnergy turns what the dial counted into what the tariff prices.
//
// A gas meter counts volume and gas is sold by energy, so the two numbers printed on every German
// gas bill do the conversion: the Brennwert, how much energy a cubic metre of the gas actually
// delivered holds, and the Zustandszahl, which corrects the volume measured at the pressure and
// temperature the meter stands in to the volume the price is quoted against.
//
//	kilowatt hours = cubic metres * Brennwert * Zustandszahl
//
// A tariff with no Brennwert is a tariff whose price is per unit as read - an electricity meter, or
// water sold by the cubic metre - and nothing is converted.
func convertUtilityUsageToEnergy(usedUnits int64, tariff *models.UtilityTariff) int64 {
	if tariff.CalorificValue <= 0 {
		return usedUnits
	}

	energy := utilityMulDiv(usedUnits, tariff.CalorificValue, models.UtilityConversionFactorScale)

	if tariff.StateNumber > 0 {
		energy = utilityMulDiv(energy, tariff.StateNumber, models.UtilityConversionFactorScale)
	}

	return energy
}

// newUtilityBillingYear returns an empty billing year beginning on the given day
func newUtilityBillingYear(yearStart time.Time) *models.UtilityBillingYearResponse {
	yearEnd := yearStart.AddDate(1, 0, 0)
	label := fmt.Sprintf("%d", yearStart.Year())

	if yearStart.Month() != time.January {
		label = fmt.Sprintf("%d-%d", yearStart.Year(), yearStart.Year()+1)
	}

	return &models.UtilityBillingYearResponse{
		Label:      label,
		StartDate:  utils.FormatDateToNumericYearMonthDay(yearStart),
		EndDate:    utils.FormatDateToNumericYearMonthDay(yearEnd.AddDate(0, 0, -1)),
		DaysInYear: int32(yearEnd.Sub(yearStart).Hours() / 24),
		Periods:    make([]*models.UtilityPeriodResponse, 0, 12),
	}
}

// summariseUtilityBillingYear adds up the periods of a year and says where the year is heading
func summariseUtilityBillingYear(meter *models.UtilityMeter, tariffs []*models.UtilityTariff, year *models.UtilityBillingYearResponse) {
	sort.Slice(year.Periods, func(i, j int) bool {
		return year.Periods[i].StartDate < year.Periods[j].StartDate
	})

	for _, period := range year.Periods {
		year.UsedUnits += period.UsedUnits
		year.UsedEnergy += period.UsedEnergy
		year.Cost += period.Cost
		year.Prepaid += period.Prepaid
		year.Difference += period.Difference
		year.DaysCovered += period.Days
	}

	if len(year.Periods) > 0 {
		year.Complete = year.Periods[len(year.Periods)-1].EndDate >= year.EndDate
	}

	// what the whole year is on course to cost, from what the days that have been read cost. It is
	// the flattest projection there is: it assumes the rest of the year uses what the part already
	// read used, which for anything that burns in winter is only true across a whole one.
	if year.DaysCovered > 0 && year.DaysInYear > 0 {
		year.ProjectedCost = utilityMulDiv(year.Cost, int64(year.DaysInYear), int64(year.DaysCovered))
	}

	year.ProjectedPrepaid = getUtilityAnnualPrepayment(tariffs, year)
	year.ProjectedDifference = year.ProjectedPrepaid - year.ProjectedCost

	if meter.Unit == models.UTILITY_METER_UNIT_KWH {
		year.UsedEnergy = 0
	}
}

// getUtilityAnnualPrepayment is what the supplier will have collected by the end of the billing
// year: one monthly payment for each of its twelve months, at whatever it stood at in that month.
//
// It is counted by the month rather than spread over the days because that is how it is actually
// taken - twelve debits, not a daily rate - and because it has to be counted for the whole year
// including the months that have not been read yet, which have no days to spread anything over.
func getUtilityAnnualPrepayment(tariffs []*models.UtilityTariff, year *models.UtilityBillingYearResponse) int64 {
	if len(tariffs) < 1 {
		return 0
	}

	yearStart, err := utils.ParseFromNumericYearMonthDay(year.StartDate)

	if err != nil {
		return 0
	}

	total := int64(0)

	for month := 0; month < 12; month++ {
		total += getUtilityTariffOn(tariffs, yearStart.AddDate(0, month, 0)).MonthlyPrepayment
	}

	return total
}

// utilityMulDiv returns value * multiplier / divisor, rounded to the nearest whole number and away
// from zero at a half.
//
// It goes through math/big because the numbers being multiplied are each scaled by up to a million
// before they meet, and an int64 that overflows would report a refund of the wrong sign rather than
// a number that merely looks wrong.
func utilityMulDiv(value int64, multiplier int64, divisor int64) int64 {
	if value == 0 || multiplier == 0 || divisor == 0 {
		return 0
	}

	product := new(big.Int).Mul(big.NewInt(value), big.NewInt(multiplier))
	divisorValue := big.NewInt(divisor)

	quotient := new(big.Int)
	remainder := new(big.Int)
	quotient.QuoRem(product, divisorValue, remainder)

	if remainder.Sign() == 0 {
		return quotient.Int64()
	}

	// QuoRem truncates towards zero, so what is left over is rounded away from it when it is half
	// the divisor or more
	twiceRemainder := new(big.Int).Abs(remainder)
	twiceRemainder.Lsh(twiceRemainder, 1)

	if twiceRemainder.CmpAbs(divisorValue) < 0 {
		return quotient.Int64()
	}

	if (product.Sign() < 0) != (divisor < 0) {
		return quotient.Sub(quotient, big.NewInt(1)).Int64()
	}

	return quotient.Add(quotient, big.NewInt(1)).Int64()
}
