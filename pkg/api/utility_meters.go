package api

import (
	"sort"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// UtilityMetersApi represents the api of the meters a user reads and what they cost
type UtilityMetersApi struct {
	meters *services.UtilityMeterService
}

// Initialize a utility meter api singleton instance
var (
	UtilityMeters = &UtilityMetersApi{
		meters: services.UtilityMeters,
	}
)

// MeterListHandler returns the meters of the current user, with everything worked out about each
// of them.
//
// The whole page comes back in one response. A meter is a handful of tariffs and a reading a month,
// which is small enough that fetching it in pieces would cost more round trips than it saves rows,
// and the totals only mean anything once all of it is there anyway.
func (a *UtilityMetersApi) MeterListHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	meters, err := a.meters.GetAllMetersByUid(c, uid)

	if err != nil {
		log.Errorf(c, "[utility_meters.MeterListHandler] failed to get meters for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	tariffs, err := a.meters.GetAllTariffsByUid(c, uid)

	if err != nil {
		log.Errorf(c, "[utility_meters.MeterListHandler] failed to get tariffs for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	readings, err := a.meters.GetAllReadingsByUid(c, uid)

	if err != nil {
		log.Errorf(c, "[utility_meters.MeterListHandler] failed to get meter readings for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	tariffsByMeterId := make(map[int64][]*models.UtilityTariff, len(meters))
	readingsByMeterId := make(map[int64][]*models.UtilityReading, len(meters))

	for i := 0; i < len(tariffs); i++ {
		tariffsByMeterId[tariffs[i].MeterId] = append(tariffsByMeterId[tariffs[i].MeterId], tariffs[i])
	}

	for i := 0; i < len(readings); i++ {
		readingsByMeterId[readings[i].MeterId] = append(readingsByMeterId[readings[i].MeterId], readings[i])
	}

	meterResps := make(models.UtilityMeterInfoResponseSlice, len(meters))

	for i := 0; i < len(meters); i++ {
		meter := meters[i]
		meterTariffs := tariffsByMeterId[meter.MeterId]
		meterReadings := readingsByMeterId[meter.MeterId]

		meterResps[i] = meter.ToUtilityMeterInfoResponse(
			toUtilityTariffInfoResponses(meterTariffs),
			toUtilityReadingInfoResponses(meterReadings),
			services.CalculateUtilityBillingYears(meter, meterTariffs, meterReadings),
		)
	}

	sort.Sort(meterResps)

	return meterResps, nil
}

// MeterCreateHandler saves a new meter by request parameters for current user
func (a *UtilityMetersApi) MeterCreateHandler(c *core.WebContext) (any, *errs.Error) {
	var meterCreateReq models.UtilityMeterCreateRequest
	err := c.ShouldBindJSON(&meterCreateReq)

	if err != nil {
		log.Warnf(c, "[utility_meters.MeterCreateHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	if !models.IsValidUtilityMeterKind(meterCreateReq.Kind) {
		return nil, errs.ErrUtilityMeterKindInvalid
	}

	if !models.IsValidUtilityMeterUnit(meterCreateReq.Unit) {
		return nil, errs.ErrUtilityMeterUnitInvalid
	}

	uid := c.GetCurrentUid()
	maxOrderId, err := a.meters.GetMaxDisplayOrder(c, uid)

	if err != nil {
		log.Errorf(c, "[utility_meters.MeterCreateHandler] failed to get max display order for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	meter := &models.UtilityMeter{
		Uid:                   uid,
		Name:                  meterCreateReq.Name,
		Kind:                  meterCreateReq.Kind,
		Unit:                  meterCreateReq.Unit,
		Currency:              meterCreateReq.Currency,
		BillingYearStartMonth: meterCreateReq.BillingYearStartMonth,
		MeterNumber:           meterCreateReq.MeterNumber,
		Comment:               meterCreateReq.Comment,
		DisplayOrder:          maxOrderId + 1,
	}

	var tariff *models.UtilityTariff

	if meterCreateReq.Tariff != nil {
		tariff, err = a.buildUtilityTariff(uid, 0, meterCreateReq.Tariff)

		if err != nil {
			return nil, errs.Or(err, errs.ErrOperationFailed)
		}
	}

	err = a.meters.CreateMeter(c, meter, tariff)

	if err != nil {
		log.Errorf(c, "[utility_meters.MeterCreateHandler] failed to create meter for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[utility_meters.MeterCreateHandler] user \"uid:%d\" has created a new meter \"id:%d\" successfully", uid, meter.MeterId)

	tariffResps := make([]*models.UtilityTariffInfoResponse, 0, 1)

	if tariff != nil {
		tariffResps = append(tariffResps, tariff.ToUtilityTariffInfoResponse())
	}

	return meter.ToUtilityMeterInfoResponse(tariffResps, make([]*models.UtilityReadingInfoResponse, 0), make([]*models.UtilityBillingYearResponse, 0)), nil
}

// MeterModifyHandler saves an existed meter by request parameters for current user
func (a *UtilityMetersApi) MeterModifyHandler(c *core.WebContext) (any, *errs.Error) {
	var meterModifyReq models.UtilityMeterModifyRequest
	err := c.ShouldBindJSON(&meterModifyReq)

	if err != nil {
		log.Warnf(c, "[utility_meters.MeterModifyHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	if !models.IsValidUtilityMeterKind(meterModifyReq.Kind) {
		return nil, errs.ErrUtilityMeterKindInvalid
	}

	if !models.IsValidUtilityMeterUnit(meterModifyReq.Unit) {
		return nil, errs.ErrUtilityMeterUnitInvalid
	}

	uid := c.GetCurrentUid()
	meter := &models.UtilityMeter{
		Uid:                   uid,
		MeterId:               meterModifyReq.Id,
		Name:                  meterModifyReq.Name,
		Kind:                  meterModifyReq.Kind,
		Unit:                  meterModifyReq.Unit,
		Currency:              meterModifyReq.Currency,
		BillingYearStartMonth: meterModifyReq.BillingYearStartMonth,
		MeterNumber:           meterModifyReq.MeterNumber,
		Comment:               meterModifyReq.Comment,
	}

	err = a.meters.ModifyMeter(c, meter)

	if err != nil {
		log.Errorf(c, "[utility_meters.MeterModifyHandler] failed to modify meter \"id:%d\" for user \"uid:%d\", because %s", meterModifyReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[utility_meters.MeterModifyHandler] user \"uid:%d\" has modified meter \"id:%d\" successfully", uid, meterModifyReq.Id)

	return true, nil
}

// MeterDeleteHandler deletes an existed meter by request parameters for current user
func (a *UtilityMetersApi) MeterDeleteHandler(c *core.WebContext) (any, *errs.Error) {
	var meterDeleteReq models.UtilityMeterDeleteRequest
	err := c.ShouldBindJSON(&meterDeleteReq)

	if err != nil {
		log.Warnf(c, "[utility_meters.MeterDeleteHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.meters.DeleteMeter(c, uid, meterDeleteReq.Id)

	if err != nil {
		log.Errorf(c, "[utility_meters.MeterDeleteHandler] failed to delete meter \"id:%d\" for user \"uid:%d\", because %s", meterDeleteReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[utility_meters.MeterDeleteHandler] user \"uid:%d\" has deleted meter \"id:%d\" successfully", uid, meterDeleteReq.Id)

	return true, nil
}

// TariffCreateHandler saves a new tariff of a meter by request parameters for current user
func (a *UtilityMetersApi) TariffCreateHandler(c *core.WebContext) (any, *errs.Error) {
	var tariffCreateReq models.UtilityTariffCreateRequest
	err := c.ShouldBindJSON(&tariffCreateReq)

	if err != nil {
		log.Warnf(c, "[utility_meters.TariffCreateHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	if tariffCreateReq.MeterId <= 0 {
		return nil, errs.ErrUtilityMeterIdInvalid
	}

	uid := c.GetCurrentUid()

	// the meter is read back rather than trusted, so that a tariff can only ever be hung on a meter
	// this user actually has
	_, err = a.meters.GetMeterByMeterId(c, uid, tariffCreateReq.MeterId)

	if err != nil {
		log.Errorf(c, "[utility_meters.TariffCreateHandler] failed to get meter \"id:%d\" for user \"uid:%d\", because %s", tariffCreateReq.MeterId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	tariff, err := a.buildUtilityTariff(uid, tariffCreateReq.MeterId, &tariffCreateReq)

	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	err = a.meters.CreateTariff(c, tariff)

	if err != nil {
		log.Errorf(c, "[utility_meters.TariffCreateHandler] failed to create tariff of meter \"id:%d\" for user \"uid:%d\", because %s", tariffCreateReq.MeterId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[utility_meters.TariffCreateHandler] user \"uid:%d\" has created a new tariff \"id:%d\" of meter \"id:%d\" successfully", uid, tariff.TariffId, tariff.MeterId)

	return tariff.ToUtilityTariffInfoResponse(), nil
}

// TariffModifyHandler saves an existed tariff by request parameters for current user
func (a *UtilityMetersApi) TariffModifyHandler(c *core.WebContext) (any, *errs.Error) {
	var tariffModifyReq models.UtilityTariffModifyRequest
	err := c.ShouldBindJSON(&tariffModifyReq)

	if err != nil {
		log.Warnf(c, "[utility_meters.TariffModifyHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	oldTariff, err := a.meters.GetTariffByTariffId(c, uid, tariffModifyReq.Id)

	if err != nil {
		log.Errorf(c, "[utility_meters.TariffModifyHandler] failed to get tariff \"id:%d\" for user \"uid:%d\", because %s", tariffModifyReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	tariff, err := a.buildUtilityTariff(uid, oldTariff.MeterId, &models.UtilityTariffCreateRequest{
		StartDate:         tariffModifyReq.StartDate,
		UnitPrice:         tariffModifyReq.UnitPrice,
		BaseFeeAnnual:     tariffModifyReq.BaseFeeAnnual,
		CalorificValue:    tariffModifyReq.CalorificValue,
		StateNumber:       tariffModifyReq.StateNumber,
		MonthlyPrepayment: tariffModifyReq.MonthlyPrepayment,
		Comment:           tariffModifyReq.Comment,
	})

	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	tariff.TariffId = tariffModifyReq.Id
	err = a.meters.ModifyTariff(c, tariff)

	if err != nil {
		log.Errorf(c, "[utility_meters.TariffModifyHandler] failed to modify tariff \"id:%d\" for user \"uid:%d\", because %s", tariffModifyReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[utility_meters.TariffModifyHandler] user \"uid:%d\" has modified tariff \"id:%d\" successfully", uid, tariffModifyReq.Id)

	return tariff.ToUtilityTariffInfoResponse(), nil
}

// TariffDeleteHandler deletes an existed tariff by request parameters for current user
func (a *UtilityMetersApi) TariffDeleteHandler(c *core.WebContext) (any, *errs.Error) {
	var tariffDeleteReq models.UtilityTariffDeleteRequest
	err := c.ShouldBindJSON(&tariffDeleteReq)

	if err != nil {
		log.Warnf(c, "[utility_meters.TariffDeleteHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.meters.DeleteTariff(c, uid, tariffDeleteReq.Id)

	if err != nil {
		log.Errorf(c, "[utility_meters.TariffDeleteHandler] failed to delete tariff \"id:%d\" for user \"uid:%d\", because %s", tariffDeleteReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[utility_meters.TariffDeleteHandler] user \"uid:%d\" has deleted tariff \"id:%d\" successfully", uid, tariffDeleteReq.Id)

	return true, nil
}

// ReadingCreateHandler saves a new meter reading by request parameters for current user
func (a *UtilityMetersApi) ReadingCreateHandler(c *core.WebContext) (any, *errs.Error) {
	var readingCreateReq models.UtilityReadingCreateRequest
	err := c.ShouldBindJSON(&readingCreateReq)

	if err != nil {
		log.Warnf(c, "[utility_meters.ReadingCreateHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	_, err = a.meters.GetMeterByMeterId(c, uid, readingCreateReq.MeterId)

	if err != nil {
		log.Errorf(c, "[utility_meters.ReadingCreateHandler] failed to get meter \"id:%d\" for user \"uid:%d\", because %s", readingCreateReq.MeterId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	if err := validateUtilityReadingValue(readingCreateReq.Value); err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	if _, err := utils.ParseFromNumericYearMonthDay(readingCreateReq.ReadingDate); err != nil {
		return nil, errs.ErrUtilityReadingDateInvalid
	}

	if err := a.checkUtilityReadingAgainstNeighbours(c, uid, readingCreateReq.MeterId, readingCreateReq.ReadingDate, readingCreateReq.Value, 0); err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	reading := &models.UtilityReading{
		Uid:         uid,
		MeterId:     readingCreateReq.MeterId,
		ReadingDate: readingCreateReq.ReadingDate,
		Value:       readingCreateReq.Value,
		Estimated:   readingCreateReq.Estimated,
		Comment:     readingCreateReq.Comment,
	}

	err = a.meters.CreateReading(c, reading)

	if err != nil {
		log.Errorf(c, "[utility_meters.ReadingCreateHandler] failed to create reading of meter \"id:%d\" for user \"uid:%d\", because %s", readingCreateReq.MeterId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[utility_meters.ReadingCreateHandler] user \"uid:%d\" has created a new reading \"id:%d\" of meter \"id:%d\" successfully", uid, reading.ReadingId, reading.MeterId)

	return reading.ToUtilityReadingInfoResponse(), nil
}

// ReadingModifyHandler saves an existed meter reading by request parameters for current user
func (a *UtilityMetersApi) ReadingModifyHandler(c *core.WebContext) (any, *errs.Error) {
	var readingModifyReq models.UtilityReadingModifyRequest
	err := c.ShouldBindJSON(&readingModifyReq)

	if err != nil {
		log.Warnf(c, "[utility_meters.ReadingModifyHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	oldReading, err := a.meters.GetReadingByReadingId(c, uid, readingModifyReq.Id)

	if err != nil {
		log.Errorf(c, "[utility_meters.ReadingModifyHandler] failed to get reading \"id:%d\" for user \"uid:%d\", because %s", readingModifyReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	if err := validateUtilityReadingValue(readingModifyReq.Value); err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	if _, err := utils.ParseFromNumericYearMonthDay(readingModifyReq.ReadingDate); err != nil {
		return nil, errs.ErrUtilityReadingDateInvalid
	}

	if err := a.checkUtilityReadingAgainstNeighbours(c, uid, oldReading.MeterId, readingModifyReq.ReadingDate, readingModifyReq.Value, readingModifyReq.Id); err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	reading := &models.UtilityReading{
		Uid:         uid,
		ReadingId:   readingModifyReq.Id,
		MeterId:     oldReading.MeterId,
		ReadingDate: readingModifyReq.ReadingDate,
		Value:       readingModifyReq.Value,
		Estimated:   readingModifyReq.Estimated,
		Comment:     readingModifyReq.Comment,
	}

	err = a.meters.ModifyReading(c, reading)

	if err != nil {
		log.Errorf(c, "[utility_meters.ReadingModifyHandler] failed to modify reading \"id:%d\" for user \"uid:%d\", because %s", readingModifyReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[utility_meters.ReadingModifyHandler] user \"uid:%d\" has modified reading \"id:%d\" successfully", uid, readingModifyReq.Id)

	return reading.ToUtilityReadingInfoResponse(), nil
}

// ReadingDeleteHandler deletes an existed meter reading by request parameters for current user
func (a *UtilityMetersApi) ReadingDeleteHandler(c *core.WebContext) (any, *errs.Error) {
	var readingDeleteReq models.UtilityReadingDeleteRequest
	err := c.ShouldBindJSON(&readingDeleteReq)

	if err != nil {
		log.Warnf(c, "[utility_meters.ReadingDeleteHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.meters.DeleteReading(c, uid, readingDeleteReq.Id)

	if err != nil {
		log.Errorf(c, "[utility_meters.ReadingDeleteHandler] failed to delete reading \"id:%d\" for user \"uid:%d\", because %s", readingDeleteReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[utility_meters.ReadingDeleteHandler] user \"uid:%d\" has deleted reading \"id:%d\" successfully", uid, readingDeleteReq.Id)

	return true, nil
}

// buildUtilityTariff checks a submitted tariff and returns it as the row it would be stored as
func (a *UtilityMetersApi) buildUtilityTariff(uid int64, meterId int64, req *models.UtilityTariffCreateRequest) (*models.UtilityTariff, error) {
	if _, err := utils.ParseFromNumericYearMonthDay(req.StartDate); err != nil {
		return nil, errs.ErrUtilityTariffDateInvalid
	}

	if req.UnitPrice < 0 || req.UnitPrice > models.MaximumUtilityUnitPrice*models.UtilityUnitPriceScale {
		return nil, errs.ErrUtilityTariffPriceInvalid
	}

	if req.BaseFeeAnnual < 0 || req.BaseFeeAnnual > models.MaximumUtilityFeeAmount {
		return nil, errs.ErrUtilityTariffPriceInvalid
	}

	if req.MonthlyPrepayment < 0 || req.MonthlyPrepayment > models.MaximumUtilityFeeAmount {
		return nil, errs.ErrUtilityTariffPriceInvalid
	}

	maximumFactor := models.MaximumUtilityConversionFactor * models.UtilityConversionFactorScale

	if req.CalorificValue < 0 || req.CalorificValue > maximumFactor {
		return nil, errs.ErrUtilityTariffConversionInvalid
	}

	if req.StateNumber < 0 || req.StateNumber > maximumFactor {
		return nil, errs.ErrUtilityTariffConversionInvalid
	}

	// a state number with no calorific value beside it would silently do nothing, because a volume
	// corrected to standard conditions and then not converted into energy is still a volume. It is
	// refused rather than ignored, so that a gas tariff missing its Brennwert is noticed now and not
	// in the total.
	if req.CalorificValue <= 0 && req.StateNumber > 0 {
		return nil, errs.ErrUtilityTariffConversionInvalid
	}

	return &models.UtilityTariff{
		Uid:               uid,
		MeterId:           meterId,
		StartDate:         req.StartDate,
		UnitPrice:         req.UnitPrice,
		BaseFeeAnnual:     req.BaseFeeAnnual,
		CalorificValue:    req.CalorificValue,
		StateNumber:       req.StateNumber,
		MonthlyPrepayment: req.MonthlyPrepayment,
		Comment:           req.Comment,
	}, nil
}

// checkUtilityReadingAgainstNeighbours refuses a reading that could not have come off the dial.
//
// A meter counts upwards and never back, so a reading below an earlier one or above a later one is
// a typo - a digit dropped, or a date entered as the wrong month. Caught here it is one field to
// correct; saved, it becomes a month of impossible use and a refund estimate to match.
func (a *UtilityMetersApi) checkUtilityReadingAgainstNeighbours(c *core.WebContext, uid int64, meterId int64, readingDate int32, value int64, excludeReadingId int64) error {
	previous, next, err := a.meters.GetNeighbouringReadings(c, uid, meterId, readingDate, excludeReadingId)

	if err != nil {
		return err
	}

	if previous != nil && value < previous.Value {
		return errs.ErrUtilityReadingBelowPreviousValue
	}

	if next != nil && value > next.Value {
		return errs.ErrUtilityReadingAboveNextValue
	}

	return nil
}

// validateUtilityReadingValue returns whether a reading is a number a dial could show
func validateUtilityReadingValue(value int64) error {
	if value < 0 || value > models.MaximumUtilityReadingValue*models.UtilityReadingValueScale {
		return errs.ErrUtilityReadingValueInvalid
	}

	return nil
}

// toUtilityTariffInfoResponses returns the tariffs of one meter as view-objects, newest first
func toUtilityTariffInfoResponses(tariffs []*models.UtilityTariff) []*models.UtilityTariffInfoResponse {
	resps := make([]*models.UtilityTariffInfoResponse, len(tariffs))

	for i := 0; i < len(tariffs); i++ {
		resps[i] = tariffs[i].ToUtilityTariffInfoResponse()
	}

	sort.Slice(resps, func(i, j int) bool {
		if resps[i].StartDate != resps[j].StartDate {
			return resps[i].StartDate > resps[j].StartDate
		}

		return resps[i].Id > resps[j].Id
	})

	return resps
}

// toUtilityReadingInfoResponses returns the readings of one meter as view-objects, newest first
func toUtilityReadingInfoResponses(readings []*models.UtilityReading) []*models.UtilityReadingInfoResponse {
	resps := make([]*models.UtilityReadingInfoResponse, len(readings))

	for i := 0; i < len(readings); i++ {
		resps[i] = readings[i].ToUtilityReadingInfoResponse()
	}

	sort.Slice(resps, func(i, j int) bool {
		if resps[i].ReadingDate != resps[j].ReadingDate {
			return resps[i].ReadingDate > resps[j].ReadingDate
		}

		return resps[i].Id > resps[j].Id
	})

	return resps
}
