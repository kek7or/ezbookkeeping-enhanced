package errs

import "net/http"

// Error codes related to utility meters
var (
	ErrUtilityMeterIdInvalid            = NewNormalError(NormalSubcategoryUtilityMeter, 0, http.StatusBadRequest, "utility meter id is invalid")
	ErrUtilityMeterNotFound             = NewNormalError(NormalSubcategoryUtilityMeter, 1, http.StatusBadRequest, "utility meter not found")
	ErrUtilityMeterNameIsEmpty          = NewNormalError(NormalSubcategoryUtilityMeter, 2, http.StatusBadRequest, "utility meter name is empty")
	ErrUtilityMeterNameAlreadyExists    = NewNormalError(NormalSubcategoryUtilityMeter, 3, http.StatusBadRequest, "utility meter name already exists")
	ErrUtilityMeterKindInvalid          = NewNormalError(NormalSubcategoryUtilityMeter, 4, http.StatusBadRequest, "utility meter kind is invalid")
	ErrUtilityMeterUnitInvalid          = NewNormalError(NormalSubcategoryUtilityMeter, 5, http.StatusBadRequest, "utility meter unit is invalid")
	ErrUtilityBillingYearStartInvalid   = NewNormalError(NormalSubcategoryUtilityMeter, 6, http.StatusBadRequest, "billing year start month is invalid")
	ErrUtilityTariffIdInvalid           = NewNormalError(NormalSubcategoryUtilityMeter, 7, http.StatusBadRequest, "utility tariff id is invalid")
	ErrUtilityTariffNotFound            = NewNormalError(NormalSubcategoryUtilityMeter, 8, http.StatusBadRequest, "utility tariff not found")
	ErrUtilityTariffDateInvalid         = NewNormalError(NormalSubcategoryUtilityMeter, 9, http.StatusBadRequest, "utility tariff start date is invalid")
	ErrUtilityTariffDateAlreadyExists   = NewNormalError(NormalSubcategoryUtilityMeter, 10, http.StatusBadRequest, "a tariff of this meter already starts on this date")
	ErrUtilityTariffPriceInvalid        = NewNormalError(NormalSubcategoryUtilityMeter, 11, http.StatusBadRequest, "utility tariff price is invalid")
	ErrUtilityTariffConversionInvalid   = NewNormalError(NormalSubcategoryUtilityMeter, 12, http.StatusBadRequest, "utility tariff conversion factors are invalid")
	ErrUtilityReadingIdInvalid          = NewNormalError(NormalSubcategoryUtilityMeter, 13, http.StatusBadRequest, "meter reading id is invalid")
	ErrUtilityReadingNotFound           = NewNormalError(NormalSubcategoryUtilityMeter, 14, http.StatusBadRequest, "meter reading not found")
	ErrUtilityReadingDateInvalid        = NewNormalError(NormalSubcategoryUtilityMeter, 15, http.StatusBadRequest, "meter reading date is invalid")
	ErrUtilityReadingDateAlreadyExists  = NewNormalError(NormalSubcategoryUtilityMeter, 16, http.StatusBadRequest, "this meter has already been read on this date")
	ErrUtilityReadingValueInvalid       = NewNormalError(NormalSubcategoryUtilityMeter, 17, http.StatusBadRequest, "meter reading value is invalid")
	ErrUtilityReadingBelowPreviousValue = NewNormalError(NormalSubcategoryUtilityMeter, 18, http.StatusBadRequest, "meter reading is lower than an earlier one")
	ErrUtilityReadingAboveNextValue     = NewNormalError(NormalSubcategoryUtilityMeter, 19, http.StatusBadRequest, "meter reading is higher than a later one")
	ErrUtilityTariffsExceedLimit        = NewNormalError(NormalSubcategoryUtilityMeter, 20, http.StatusBadRequest, "this meter holds as many tariffs as it may")
	ErrUtilityReadingsExceedLimit       = NewNormalError(NormalSubcategoryUtilityMeter, 21, http.StatusBadRequest, "this meter holds as many readings as it may")
)
