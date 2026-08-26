package models

import (
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// The scales the numbers of a meter are kept at. Each of them is an integer, because a tariff held
// as a float stops being the number the bill printed, and these numbers are checked against a bill.
const (
	// UtilityReadingValueScale is what a meter reading is multiplied by before it is stored. A gas
	// meter reads to two decimals and a water meter to three, so three decimals is what is kept.
	UtilityReadingValueScale = int64(1000)
	// UtilityUnitPriceScale is what the price of one unit is multiplied by. A German tariff is
	// quoted to four decimals of a euro (0.3280 per kWh) and sometimes to six, so millionths is kept.
	UtilityUnitPriceScale = int64(1000000)
	// UtilityConversionFactorScale is what the calorific value and the state number are multiplied
	// by. Both are printed on a gas bill to four decimals.
	UtilityConversionFactorScale = int64(10000)
	// UtilityDaysPerYear is what an annual amount is spread over to reach one day of it. A billing
	// year is not always 365 days long and a leap year never is, but the base fee and the monthly
	// prepayment are only ever compared against an estimate, and a day of drift in a year of
	// standing charge is smaller than the rounding on the reading it is added to.
	UtilityDaysPerYear = int64(365)
)

// The bounds a submitted number has to sit inside. They exist to keep a malformed request from
// reaching the arithmetic with something that would overflow it, not to say what a plausible tariff
// is - a meter that counts in litres and a currency with no minor unit are both allowed.
const (
	// MaximumUtilityReadingValue is the largest reading a meter may show, before scaling
	MaximumUtilityReadingValue = int64(1000000000)
	// MaximumUtilityUnitPrice is the largest price one unit may carry, before scaling
	MaximumUtilityUnitPrice = int64(100000)
	// MaximumUtilityConversionFactor is the largest calorific value or state number, before scaling
	MaximumUtilityConversionFactor = int64(1000)
	// MaximumUtilityFeeAmount is the largest annual base fee or monthly prepayment, in minor units
	MaximumUtilityFeeAmount = int64(100000000000)
	// MaximumUtilityTariffsPerMeter is how many price changes one meter may hold, and
	// MaximumUtilityReadingsPerMeter how many times it may have been read. Both are far past any
	// real meter and are only what stops one meter from being made unboundedly expensive to read.
	MaximumUtilityTariffsPerMeter  = 500
	MaximumUtilityReadingsPerMeter = 5000
)

// UtilityMeterKind is what a meter measures. It decides the icon and the words the page uses for it
// and nothing else - what a meter costs is decided by its unit and its tariff alone.
type UtilityMeterKind byte

// Utility meter kinds
const (
	UTILITY_METER_KIND_ELECTRICITY UtilityMeterKind = 1
	UTILITY_METER_KIND_GAS         UtilityMeterKind = 2
	UTILITY_METER_KIND_WATER       UtilityMeterKind = 3
	UTILITY_METER_KIND_HEATING     UtilityMeterKind = 4
	UTILITY_METER_KIND_OTHER       UtilityMeterKind = 5
)

// UtilityMeterUnit is what the dial of a meter counts in
type UtilityMeterUnit byte

// Utility meter units
const (
	// UTILITY_METER_UNIT_KWH counts energy, as an electricity meter and a heat meter do
	UTILITY_METER_UNIT_KWH UtilityMeterUnit = 1
	// UTILITY_METER_UNIT_CUBIC_METRE counts volume, as a gas meter and a water meter do
	UTILITY_METER_UNIT_CUBIC_METRE UtilityMeterUnit = 2
	// UTILITY_METER_UNIT_LITRE counts volume in litres, as some water meters do
	UTILITY_METER_UNIT_LITRE UtilityMeterUnit = 3
)

// IsValidUtilityMeterKind returns whether the given number names a kind of meter
func IsValidUtilityMeterKind(kind UtilityMeterKind) bool {
	return kind >= UTILITY_METER_KIND_ELECTRICITY && kind <= UTILITY_METER_KIND_OTHER
}

// IsValidUtilityMeterUnit returns whether the given number names a unit a meter counts in
func IsValidUtilityMeterUnit(unit UtilityMeterUnit) bool {
	return unit >= UTILITY_METER_UNIT_KWH && unit <= UTILITY_METER_UNIT_LITRE
}

// UtilityMeter is one meter the user reads: the electricity meter of a flat, its gas meter, a water
// meter.
//
// It holds no money and no consumption of its own. It is a name, a unit, and the year its supplier
// bills in; what it has used comes from the readings taken off it, and what that costs comes from
// the tariff that was in force when it was used.
//
// Nothing here ever becomes a transaction. The money that leaves the account is the monthly
// prepayment, which is already in the ledger as the standing order it is - what this table adds is
// what that money was actually worth, which is only known once the meter has been read, and which
// is never a payment of its own.
type UtilityMeter struct {
	MeterId  int64            `xorm:"PK"`
	Uid      int64            `xorm:"INDEX(IDX_utility_meter_uid_deleted_order) NOT NULL"`
	Deleted  bool             `xorm:"INDEX(IDX_utility_meter_uid_deleted_order) NOT NULL"`
	Name     string           `xorm:"VARCHAR(64) NOT NULL"`
	Kind     UtilityMeterKind `xorm:"NOT NULL"`
	Unit     UtilityMeterUnit `xorm:"NOT NULL"`
	Currency string           `xorm:"VARCHAR(3) NOT NULL"`
	// BillingYearStartMonth is the month the supplier's billing year begins, from 1 to 12. It is
	// rarely January: a contract signed in June is settled every June, and a balance counted over
	// the calendar year would be compared against a refund that covers other months than it does.
	BillingYearStartMonth byte `xorm:"NOT NULL"`
	// MeterNumber is what the supplier calls this meter - the number printed on the dial, which is
	// what a reading has to be quoted against when it is ever disputed
	MeterNumber     string `xorm:"VARCHAR(64) NOT NULL"`
	Comment         string `xorm:"VARCHAR(255) NOT NULL"`
	DisplayOrder    int32  `xorm:"INDEX(IDX_utility_meter_uid_deleted_order) NOT NULL"`
	CreatedUnixTime int64
	UpdatedUnixTime int64
	DeletedUnixTime int64
}

// UtilityTariff is what one meter costs from a given day on.
//
// A tariff is a row rather than a column of the meter because prices change, and they change in the
// middle of a billing year. The old price does not stop being true of the months it applied to, and
// an estimate that restates January at the price announced in July is an estimate of nothing. Every
// tariff of a meter is kept, and each period of use is priced by whichever ones overlap it.
type UtilityTariff struct {
	TariffId int64 `xorm:"PK"`
	Uid      int64 `xorm:"INDEX(IDX_utility_tariff_uid_deleted_meter) NOT NULL"`
	Deleted  bool  `xorm:"INDEX(IDX_utility_tariff_uid_deleted_meter) NOT NULL"`
	MeterId  int64 `xorm:"INDEX(IDX_utility_tariff_uid_deleted_meter) NOT NULL"`
	// StartDate is the first day this tariff applies to, as YYYYMMDD. It runs until the day before
	// the next tariff of the meter begins, and the earliest tariff of a meter also covers
	// everything before it - a reading older than any price on file is priced at the oldest price
	// rather than being left uncosted.
	StartDate int32 `xorm:"NOT NULL"`
	// UnitPrice is the Arbeitspreis: what one kilowatt hour costs, scaled by UtilityUnitPriceScale.
	// For a meter whose use is not converted it is what one unit as read costs instead.
	UnitPrice int64 `xorm:"NOT NULL"`
	// BaseFeeAnnual is the Grundpreis for a whole year, in minor units. A bill that quotes it per
	// month is multiplied by twelve before it gets here, so that there is one form of it to reason
	// about and a period of any length can be charged its share of it.
	BaseFeeAnnual int64 `xorm:"NOT NULL"`
	// CalorificValue is the Brennwert, the energy in one cubic metre of the gas actually delivered,
	// scaled by UtilityConversionFactorScale. It is zero for a meter whose reading is already the
	// thing being priced, and a zero here is what says that no conversion is to be done at all.
	CalorificValue int64 `xorm:"NOT NULL"`
	// StateNumber is the Zustandszahl, which corrects the volume the meter measured at the pressure
	// and temperature it stands in to the volume the price is quoted against, scaled by
	// UtilityConversionFactorScale. It is only read when CalorificValue is set.
	StateNumber int64 `xorm:"NOT NULL"`
	// MonthlyPrepayment is the Abschlag: what the supplier collects every month against a bill that
	// has not been worked out yet. It is what the estimate is compared against, and the difference
	// between the two is the refund or the demand waiting at the end of the billing year.
	MonthlyPrepayment int64  `xorm:"NOT NULL"`
	Comment           string `xorm:"VARCHAR(255) NOT NULL"`
	CreatedUnixTime   int64
	UpdatedUnixTime   int64
	DeletedUnixTime   int64
}

// UtilityReading is one number read off one meter on one day.
//
// It is the reading itself and not the use since the last one, because the reading is what is
// written on the dial and can be checked against it a year later, while a difference is a
// conclusion that cannot be. What was used between two readings is worked out from them and is
// never stored.
type UtilityReading struct {
	ReadingId int64 `xorm:"PK"`
	Uid       int64 `xorm:"INDEX(IDX_utility_reading_uid_deleted_meter) NOT NULL"`
	Deleted   bool  `xorm:"INDEX(IDX_utility_reading_uid_deleted_meter) NOT NULL"`
	MeterId   int64 `xorm:"INDEX(IDX_utility_reading_uid_deleted_meter) NOT NULL"`
	// ReadingDate is the day the meter was read, as YYYYMMDD
	ReadingDate int32 `xorm:"NOT NULL"`
	// Value is what the dial showed, scaled by UtilityReadingValueScale
	Value int64 `xorm:"NOT NULL"`
	// Estimated says the number was worked out rather than read - the supplier's own estimate, or a
	// figure interpolated for a day nobody was there. It changes no arithmetic; it only stops an
	// estimate from later being mistaken for something that was seen.
	Estimated       bool   `xorm:"NOT NULL"`
	Comment         string `xorm:"VARCHAR(255) NOT NULL"`
	CreatedUnixTime int64
	UpdatedUnixTime int64
	DeletedUnixTime int64
}

// UtilityMeterCreateRequest represents all parameters of a meter creation request
type UtilityMeterCreateRequest struct {
	Name                  string           `json:"name" binding:"required,notBlank,max=64"`
	Kind                  UtilityMeterKind `json:"kind" binding:"required,min=1,max=5"`
	Unit                  UtilityMeterUnit `json:"unit" binding:"required,min=1,max=3"`
	Currency              string           `json:"currency" binding:"required,len=3,validCurrency"`
	BillingYearStartMonth byte             `json:"billingYearStartMonth" binding:"required,min=1,max=12"`
	MeterNumber           string           `json:"meterNumber" binding:"max=64"`
	Comment               string           `json:"comment" binding:"max=255"`
	// Tariff is the first tariff of the meter, given at the same time because a meter with no price
	// cannot say what anything cost, and there is no reason to make that a second errand
	Tariff *UtilityTariffCreateRequest `json:"tariff" binding:"omitempty"`
}

// UtilityMeterModifyRequest represents all parameters of a meter modification request
type UtilityMeterModifyRequest struct {
	Id                    int64            `json:"id,string" binding:"required,min=1"`
	Name                  string           `json:"name" binding:"required,notBlank,max=64"`
	Kind                  UtilityMeterKind `json:"kind" binding:"required,min=1,max=5"`
	Unit                  UtilityMeterUnit `json:"unit" binding:"required,min=1,max=3"`
	Currency              string           `json:"currency" binding:"required,len=3,validCurrency"`
	BillingYearStartMonth byte             `json:"billingYearStartMonth" binding:"required,min=1,max=12"`
	MeterNumber           string           `json:"meterNumber" binding:"max=64"`
	Comment               string           `json:"comment" binding:"max=255"`
}

// UtilityMeterDeleteRequest represents all parameters of a meter deletion request
type UtilityMeterDeleteRequest struct {
	Id int64 `json:"id,string" binding:"required,min=1"`
}

// UtilityTariffCreateRequest represents all parameters of a tariff creation request
type UtilityTariffCreateRequest struct {
	// MeterId is which meter the tariff is for. It is left out when the tariff is given as part of
	// the meter it belongs to, which is the one case where the meter has no id to name yet.
	MeterId           int64  `json:"meterId,string" binding:"omitempty,min=1"`
	StartDate         int32  `json:"startDate" binding:"required,min=10000101,max=99991231"`
	UnitPrice         int64  `json:"unitPrice" binding:"min=0"`
	BaseFeeAnnual     int64  `json:"baseFeeAnnual" binding:"min=0"`
	CalorificValue    int64  `json:"calorificValue" binding:"min=0"`
	StateNumber       int64  `json:"stateNumber" binding:"min=0"`
	MonthlyPrepayment int64  `json:"monthlyPrepayment" binding:"min=0"`
	Comment           string `json:"comment" binding:"max=255"`
}

// UtilityTariffModifyRequest represents all parameters of a tariff modification request
type UtilityTariffModifyRequest struct {
	Id                int64  `json:"id,string" binding:"required,min=1"`
	StartDate         int32  `json:"startDate" binding:"required,min=10000101,max=99991231"`
	UnitPrice         int64  `json:"unitPrice" binding:"min=0"`
	BaseFeeAnnual     int64  `json:"baseFeeAnnual" binding:"min=0"`
	CalorificValue    int64  `json:"calorificValue" binding:"min=0"`
	StateNumber       int64  `json:"stateNumber" binding:"min=0"`
	MonthlyPrepayment int64  `json:"monthlyPrepayment" binding:"min=0"`
	Comment           string `json:"comment" binding:"max=255"`
}

// UtilityTariffDeleteRequest represents all parameters of a tariff deletion request
type UtilityTariffDeleteRequest struct {
	Id int64 `json:"id,string" binding:"required,min=1"`
}

// UtilityReadingCreateRequest represents all parameters of a meter reading creation request
type UtilityReadingCreateRequest struct {
	MeterId     int64  `json:"meterId,string" binding:"required,min=1"`
	ReadingDate int32  `json:"readingDate" binding:"required,min=10000101,max=99991231"`
	Value       int64  `json:"value" binding:"min=0"`
	Estimated   bool   `json:"estimated"`
	Comment     string `json:"comment" binding:"max=255"`
}

// UtilityReadingModifyRequest represents all parameters of a meter reading modification request
type UtilityReadingModifyRequest struct {
	Id          int64  `json:"id,string" binding:"required,min=1"`
	ReadingDate int32  `json:"readingDate" binding:"required,min=10000101,max=99991231"`
	Value       int64  `json:"value" binding:"min=0"`
	Estimated   bool   `json:"estimated"`
	Comment     string `json:"comment" binding:"max=255"`
}

// UtilityReadingDeleteRequest represents all parameters of a meter reading deletion request
type UtilityReadingDeleteRequest struct {
	Id int64 `json:"id,string" binding:"required,min=1"`
}

// UtilityTariffInfoResponse represents a view-object of one tariff
type UtilityTariffInfoResponse struct {
	Id                string `json:"id"`
	MeterId           string `json:"meterId"`
	StartDate         int32  `json:"startDate"`
	UnitPrice         int64  `json:"unitPrice"`
	BaseFeeAnnual     int64  `json:"baseFeeAnnual"`
	CalorificValue    int64  `json:"calorificValue,omitempty"`
	StateNumber       int64  `json:"stateNumber,omitempty"`
	MonthlyPrepayment int64  `json:"monthlyPrepayment"`
	Comment           string `json:"comment,omitempty"`
}

// UtilityReadingInfoResponse represents a view-object of one meter reading
type UtilityReadingInfoResponse struct {
	Id          string `json:"id"`
	MeterId     string `json:"meterId"`
	ReadingDate int32  `json:"readingDate"`
	Value       int64  `json:"value"`
	Estimated   bool   `json:"estimated,omitempty"`
	Comment     string `json:"comment,omitempty"`
}

// UtilityPeriodResponse is what happened between two readings: what was used, what it is estimated
// to have cost, and what was paid towards it over the same days.
type UtilityPeriodResponse struct {
	StartDate int32 `json:"startDate"`
	EndDate   int32 `json:"endDate"`
	Days      int32 `json:"days"`
	// StartValue and EndValue are the two readings this sits between, so that a row can be checked
	// against the dial without opening anything else
	StartValue int64 `json:"startValue"`
	EndValue   int64 `json:"endValue"`
	// UsedUnits is the difference between the two readings, in the unit the meter counts in
	UsedUnits int64 `json:"usedUnits"`
	// UsedEnergy is what that came to after conversion, scaled like a reading. It is zero for a
	// meter whose use is priced exactly as it is read.
	UsedEnergy int64 `json:"usedEnergy,omitempty"`
	// EnergyCost is the use at the unit price, and BaseFee this period's share of the standing
	// charge. They are reported apart because a month that used almost nothing still costs the base
	// fee, and a total that hides it looks like a mistake.
	EnergyCost int64 `json:"energyCost"`
	BaseFee    int64 `json:"baseFee"`
	Cost       int64 `json:"cost"`
	// Prepaid is what the monthly prepayment came to over these days
	Prepaid int64 `json:"prepaid"`
	// Difference is Prepaid less Cost: what is standing to the user's credit for this period, or
	// what is owed on it when it is negative
	Difference int64 `json:"difference"`
	// Estimated says one of the two readings this sits between was not read off the dial
	Estimated bool `json:"estimated,omitempty"`
	// Partial says this row is part of a reading interval rather than the whole of one, cut where
	// the billing year ended. The readings at the cut were interpolated over the days on either
	// side of it, so the two ends of this row are a share of a reading rather than a dial.
	Partial bool `json:"partial,omitempty"`
	// PriceChanged says more than one tariff applied over these days, so the use was split between
	// them by the number of days each one covered
	PriceChanged bool `json:"priceChanged,omitempty"`
}

// UtilityBillingYearResponse is one supplier billing year: the periods that fall in it, what they
// come to, and what the year is on course to end at.
type UtilityBillingYearResponse struct {
	// Label names the year as the supplier's year rather than the calendar's - "2026" when it
	// begins in January, "2026-2027" when it begins in any other month
	Label     string `json:"label"`
	StartDate int32  `json:"startDate"`
	EndDate   int32  `json:"endDate"`
	// Complete says the year has been read through to its end, and that the balance below is the
	// whole story rather than the story so far
	Complete bool                     `json:"complete"`
	Periods  []*UtilityPeriodResponse `json:"periods"`
	// The totals of the periods above, which cover only the days the readings account for
	UsedUnits  int64 `json:"usedUnits"`
	UsedEnergy int64 `json:"usedEnergy,omitempty"`
	Cost       int64 `json:"cost"`
	Prepaid    int64 `json:"prepaid"`
	Difference int64 `json:"difference"`
	// DaysCovered is how many days of the year the readings account for, and DaysInYear how many
	// the year holds. The projection below is the first spread over the second.
	DaysCovered int32 `json:"daysCovered"`
	DaysInYear  int32 `json:"daysInYear"`
	// ProjectedCost carries the cost so far forward at the rate it has been running at, and
	// ProjectedPrepaid counts every month of the year the supplier will collect.
	// ProjectedDifference is what the year would end at if nothing changed - the refund to expect,
	// or the demand.
	//
	// It is an extrapolation and nothing better. A gas year read only through the summer will
	// project a refund that the winter takes back.
	ProjectedCost       int64 `json:"projectedCost"`
	ProjectedPrepaid    int64 `json:"projectedPrepaid"`
	ProjectedDifference int64 `json:"projectedDifference"`
}

// UtilityMeterInfoResponse represents a view-object of one meter and everything worked out about it
type UtilityMeterInfoResponse struct {
	Id                    string           `json:"id"`
	Name                  string           `json:"name"`
	Kind                  UtilityMeterKind `json:"kind"`
	Unit                  UtilityMeterUnit `json:"unit"`
	Currency              string           `json:"currency"`
	BillingYearStartMonth byte             `json:"billingYearStartMonth"`
	MeterNumber           string           `json:"meterNumber,omitempty"`
	Comment               string           `json:"comment,omitempty"`
	DisplayOrder          int32            `json:"displayOrder"`
	// Tariffs and Readings are everything on file for this meter, newest first, so that the page
	// can show the working as well as the answer
	Tariffs  []*UtilityTariffInfoResponse  `json:"tariffs"`
	Readings []*UtilityReadingInfoResponse `json:"readings"`
	// BillingYears are the years the readings fall in, newest first
	BillingYears []*UtilityBillingYearResponse `json:"billingYears"`
}

// UtilityMeterInfoResponseSlice represents the slice data structure of UtilityMeterInfoResponse
type UtilityMeterInfoResponseSlice []*UtilityMeterInfoResponse

func (s UtilityMeterInfoResponseSlice) Len() int {
	return len(s)
}

func (s UtilityMeterInfoResponseSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s UtilityMeterInfoResponseSlice) Less(i, j int) bool {
	if s[i].DisplayOrder != s[j].DisplayOrder {
		return s[i].DisplayOrder < s[j].DisplayOrder
	}

	return s[i].Id < s[j].Id
}

// ToUtilityTariffInfoResponse returns a view-object of this tariff
func (t *UtilityTariff) ToUtilityTariffInfoResponse() *UtilityTariffInfoResponse {
	return &UtilityTariffInfoResponse{
		Id:                utils.Int64ToString(t.TariffId),
		MeterId:           utils.Int64ToString(t.MeterId),
		StartDate:         t.StartDate,
		UnitPrice:         t.UnitPrice,
		BaseFeeAnnual:     t.BaseFeeAnnual,
		CalorificValue:    t.CalorificValue,
		StateNumber:       t.StateNumber,
		MonthlyPrepayment: t.MonthlyPrepayment,
		Comment:           t.Comment,
	}
}

// ToUtilityReadingInfoResponse returns a view-object of this meter reading
func (r *UtilityReading) ToUtilityReadingInfoResponse() *UtilityReadingInfoResponse {
	return &UtilityReadingInfoResponse{
		Id:          utils.Int64ToString(r.ReadingId),
		MeterId:     utils.Int64ToString(r.MeterId),
		ReadingDate: r.ReadingDate,
		Value:       r.Value,
		Estimated:   r.Estimated,
		Comment:     r.Comment,
	}
}

// ToUtilityMeterInfoResponse returns a view-object of this meter, together with what has been
// worked out from its tariffs and its readings
func (m *UtilityMeter) ToUtilityMeterInfoResponse(tariffs []*UtilityTariffInfoResponse, readings []*UtilityReadingInfoResponse, billingYears []*UtilityBillingYearResponse) *UtilityMeterInfoResponse {
	return &UtilityMeterInfoResponse{
		Id:                    utils.Int64ToString(m.MeterId),
		Name:                  m.Name,
		Kind:                  m.Kind,
		Unit:                  m.Unit,
		Currency:              m.Currency,
		BillingYearStartMonth: m.BillingYearStartMonth,
		MeterNumber:           m.MeterNumber,
		Comment:               m.Comment,
		DisplayOrder:          m.DisplayOrder,
		Tariffs:               tariffs,
		Readings:              readings,
		BillingYears:          billingYears,
	}
}
