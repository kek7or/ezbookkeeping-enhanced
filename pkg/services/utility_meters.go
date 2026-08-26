package services

import (
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

// UtilityMeterService represents the service of the meters a user reads and what they cost
type UtilityMeterService struct {
	ServiceUsingDB
	ServiceUsingUuid
}

// Initialize a utility meter service singleton instance
var (
	UtilityMeters = &UtilityMeterService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
		ServiceUsingUuid: ServiceUsingUuid{
			container: uuid.Container,
		},
	}
)

// GetAllMetersByUid returns all meters of the user
func (s *UtilityMeterService) GetAllMetersByUid(c core.Context, uid int64) ([]*models.UtilityMeter, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var meters []*models.UtilityMeter
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).OrderBy("display_order asc").Find(&meters)

	return meters, err
}

// GetMeterByMeterId returns one meter of the user
func (s *UtilityMeterService) GetMeterByMeterId(c core.Context, uid int64, meterId int64) (*models.UtilityMeter, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if meterId <= 0 {
		return nil, errs.ErrUtilityMeterIdInvalid
	}

	meter := &models.UtilityMeter{}
	has, err := s.UserDataDB(uid).NewSession(c).ID(meterId).Where("uid=? AND deleted=?", uid, false).Get(meter)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, errs.ErrUtilityMeterNotFound
	}

	return meter, nil
}

// GetAllTariffsByUid returns every tariff of every meter of the user, because the page shows every
// meter at once and a query for each of them would say the same thing more slowly
func (s *UtilityMeterService) GetAllTariffsByUid(c core.Context, uid int64) ([]*models.UtilityTariff, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var tariffs []*models.UtilityTariff
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).OrderBy("meter_id asc, start_date asc").Find(&tariffs)

	return tariffs, err
}

// GetAllReadingsByUid returns every reading of every meter of the user
func (s *UtilityMeterService) GetAllReadingsByUid(c core.Context, uid int64) ([]*models.UtilityReading, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var readings []*models.UtilityReading
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).OrderBy("meter_id asc, reading_date asc").Find(&readings)

	return readings, err
}

// GetTariffByTariffId returns one tariff of the user
func (s *UtilityMeterService) GetTariffByTariffId(c core.Context, uid int64, tariffId int64) (*models.UtilityTariff, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if tariffId <= 0 {
		return nil, errs.ErrUtilityTariffIdInvalid
	}

	tariff := &models.UtilityTariff{}
	has, err := s.UserDataDB(uid).NewSession(c).ID(tariffId).Where("uid=? AND deleted=?", uid, false).Get(tariff)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, errs.ErrUtilityTariffNotFound
	}

	return tariff, nil
}

// GetReadingByReadingId returns one meter reading of the user
func (s *UtilityMeterService) GetReadingByReadingId(c core.Context, uid int64, readingId int64) (*models.UtilityReading, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if readingId <= 0 {
		return nil, errs.ErrUtilityReadingIdInvalid
	}

	reading := &models.UtilityReading{}
	has, err := s.UserDataDB(uid).NewSession(c).ID(readingId).Where("uid=? AND deleted=?", uid, false).Get(reading)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, errs.ErrUtilityReadingNotFound
	}

	return reading, nil
}

// GetMaxDisplayOrder returns the largest display order of the meters of the user
func (s *UtilityMeterService) GetMaxDisplayOrder(c core.Context, uid int64) (int32, error) {
	if uid <= 0 {
		return 0, errs.ErrUserIdInvalid
	}

	meter := &models.UtilityMeter{}
	has, err := s.UserDataDB(uid).NewSession(c).Cols("uid", "display_order").Where("uid=? AND deleted=?", uid, false).OrderBy("display_order desc").Limit(1).Get(meter)

	if err != nil {
		return 0, err
	}

	if has {
		return meter.DisplayOrder, nil
	}

	return 0, nil
}

// CreateMeter saves a new meter, with the first tariff of it when one was given.
//
// The two are written in one transaction because a meter with no price on file can report what it
// has used and nothing about what that was worth, which is the whole question the page exists to
// answer. Half of that saved is worse than neither.
func (s *UtilityMeterService) CreateMeter(c core.Context, meter *models.UtilityMeter, tariff *models.UtilityTariff) error {
	if meter.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	exists, err := s.existsMeterName(c, meter.Uid, meter.Name, 0)

	if err != nil {
		return err
	} else if exists {
		return errs.ErrUtilityMeterNameAlreadyExists
	}

	meter.MeterId = s.GenerateUuid(uuid.UUID_TYPE_DEFAULT)

	if meter.MeterId < 1 {
		return errs.ErrSystemIsBusy
	}

	now := time.Now().Unix()
	meter.Deleted = false
	meter.CreatedUnixTime = now
	meter.UpdatedUnixTime = now

	if tariff != nil {
		tariff.TariffId = s.GenerateUuid(uuid.UUID_TYPE_DEFAULT)

		if tariff.TariffId < 1 {
			return errs.ErrSystemIsBusy
		}

		tariff.Uid = meter.Uid
		tariff.MeterId = meter.MeterId
		tariff.Deleted = false
		tariff.CreatedUnixTime = now
		tariff.UpdatedUnixTime = now
	}

	return s.UserDataDB(meter.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Insert(meter)

		if err != nil {
			return err
		}

		if tariff != nil {
			_, err = sess.Insert(tariff)
		}

		return err
	})
}

// ModifyMeter updates an existed meter
func (s *UtilityMeterService) ModifyMeter(c core.Context, meter *models.UtilityMeter) error {
	if meter.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	exists, err := s.existsMeterName(c, meter.Uid, meter.Name, meter.MeterId)

	if err != nil {
		return err
	} else if exists {
		return errs.ErrUtilityMeterNameAlreadyExists
	}

	meter.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(meter.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		updatedRows, err := sess.ID(meter.MeterId).Cols("name", "kind", "unit", "currency", "billing_year_start_month", "meter_number", "comment", "updated_unix_time").Where("uid=? AND deleted=?", meter.Uid, false).Update(meter)

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrUtilityMeterNotFound
		}

		return nil
	})
}

// DeleteMeter removes a meter with its tariffs and its readings.
//
// They go with it because neither is anything on its own: a price with no meter prices nothing, and
// a number off a dial nobody kept is not a reading of anything.
func (s *UtilityMeterService) DeleteMeter(c core.Context, uid int64, meterId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if meterId <= 0 {
		return errs.ErrUtilityMeterIdInvalid
	}

	now := time.Now().Unix()

	meterUpdateModel := &models.UtilityMeter{
		Deleted:         true,
		DeletedUnixTime: now,
	}

	tariffUpdateModel := &models.UtilityTariff{
		Deleted:         true,
		DeletedUnixTime: now,
	}

	readingUpdateModel := &models.UtilityReading{
		Deleted:         true,
		DeletedUnixTime: now,
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		deletedRows, err := sess.ID(meterId).Cols("deleted", "deleted_unix_time").Where("uid=? AND deleted=?", uid, false).Update(meterUpdateModel)

		if err != nil {
			return err
		} else if deletedRows < 1 {
			return errs.ErrUtilityMeterNotFound
		}

		_, err = sess.Cols("deleted", "deleted_unix_time").Where("uid=? AND deleted=? AND meter_id=?", uid, false, meterId).Update(tariffUpdateModel)

		if err != nil {
			return err
		}

		_, err = sess.Cols("deleted", "deleted_unix_time").Where("uid=? AND deleted=? AND meter_id=?", uid, false, meterId).Update(readingUpdateModel)

		return err
	})
}

// CreateTariff saves a new tariff of a meter
func (s *UtilityMeterService) CreateTariff(c core.Context, tariff *models.UtilityTariff) error {
	if tariff.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	exists, err := s.existsTariffStartDate(c, tariff.Uid, tariff.MeterId, tariff.StartDate, 0)

	if err != nil {
		return err
	} else if exists {
		return errs.ErrUtilityTariffDateAlreadyExists
	}

	count, err := s.countTariffs(c, tariff.Uid, tariff.MeterId)

	if err != nil {
		return err
	} else if count >= models.MaximumUtilityTariffsPerMeter {
		return errs.ErrUtilityTariffsExceedLimit
	}

	tariff.TariffId = s.GenerateUuid(uuid.UUID_TYPE_DEFAULT)

	if tariff.TariffId < 1 {
		return errs.ErrSystemIsBusy
	}

	now := time.Now().Unix()
	tariff.Deleted = false
	tariff.CreatedUnixTime = now
	tariff.UpdatedUnixTime = now

	return s.UserDataDB(tariff.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Insert(tariff)
		return err
	})
}

// ModifyTariff updates an existed tariff
func (s *UtilityMeterService) ModifyTariff(c core.Context, tariff *models.UtilityTariff) error {
	if tariff.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	exists, err := s.existsTariffStartDate(c, tariff.Uid, tariff.MeterId, tariff.StartDate, tariff.TariffId)

	if err != nil {
		return err
	} else if exists {
		return errs.ErrUtilityTariffDateAlreadyExists
	}

	tariff.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(tariff.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		updatedRows, err := sess.ID(tariff.TariffId).Cols("start_date", "unit_price", "base_fee_annual", "calorific_value", "state_number", "monthly_prepayment", "comment", "updated_unix_time").Where("uid=? AND deleted=?", tariff.Uid, false).Update(tariff)

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrUtilityTariffNotFound
		}

		return nil
	})
}

// DeleteTariff removes a tariff of a meter
func (s *UtilityMeterService) DeleteTariff(c core.Context, uid int64, tariffId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if tariffId <= 0 {
		return errs.ErrUtilityTariffIdInvalid
	}

	updateModel := &models.UtilityTariff{
		Deleted:         true,
		DeletedUnixTime: time.Now().Unix(),
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		deletedRows, err := sess.ID(tariffId).Cols("deleted", "deleted_unix_time").Where("uid=? AND deleted=?", uid, false).Update(updateModel)

		if err != nil {
			return err
		} else if deletedRows < 1 {
			return errs.ErrUtilityTariffNotFound
		}

		return nil
	})
}

// CreateReading saves a new meter reading
func (s *UtilityMeterService) CreateReading(c core.Context, reading *models.UtilityReading) error {
	if reading.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	exists, err := s.existsReadingDate(c, reading.Uid, reading.MeterId, reading.ReadingDate, 0)

	if err != nil {
		return err
	} else if exists {
		return errs.ErrUtilityReadingDateAlreadyExists
	}

	count, err := s.countReadings(c, reading.Uid, reading.MeterId)

	if err != nil {
		return err
	} else if count >= models.MaximumUtilityReadingsPerMeter {
		return errs.ErrUtilityReadingsExceedLimit
	}

	reading.ReadingId = s.GenerateUuid(uuid.UUID_TYPE_DEFAULT)

	if reading.ReadingId < 1 {
		return errs.ErrSystemIsBusy
	}

	now := time.Now().Unix()
	reading.Deleted = false
	reading.CreatedUnixTime = now
	reading.UpdatedUnixTime = now

	return s.UserDataDB(reading.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Insert(reading)
		return err
	})
}

// ModifyReading updates an existed meter reading
func (s *UtilityMeterService) ModifyReading(c core.Context, reading *models.UtilityReading) error {
	if reading.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	exists, err := s.existsReadingDate(c, reading.Uid, reading.MeterId, reading.ReadingDate, reading.ReadingId)

	if err != nil {
		return err
	} else if exists {
		return errs.ErrUtilityReadingDateAlreadyExists
	}

	reading.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(reading.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		updatedRows, err := sess.ID(reading.ReadingId).Cols("reading_date", "value", "estimated", "comment", "updated_unix_time").Where("uid=? AND deleted=?", reading.Uid, false).Update(reading)

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrUtilityReadingNotFound
		}

		return nil
	})
}

// DeleteReading removes a meter reading
func (s *UtilityMeterService) DeleteReading(c core.Context, uid int64, readingId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if readingId <= 0 {
		return errs.ErrUtilityReadingIdInvalid
	}

	updateModel := &models.UtilityReading{
		Deleted:         true,
		DeletedUnixTime: time.Now().Unix(),
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		deletedRows, err := sess.ID(readingId).Cols("deleted", "deleted_unix_time").Where("uid=? AND deleted=?", uid, false).Update(updateModel)

		if err != nil {
			return err
		} else if deletedRows < 1 {
			return errs.ErrUtilityReadingNotFound
		}

		return nil
	})
}

// GetNeighbouringReadings returns the readings either side of a given day, which is what says
// whether a number about to be written down is possible: a meter only counts upwards, so a reading
// has to sit between the one before it and the one after it.
func (s *UtilityMeterService) GetNeighbouringReadings(c core.Context, uid int64, meterId int64, readingDate int32, excludeReadingId int64) (*models.UtilityReading, *models.UtilityReading, error) {
	if uid <= 0 {
		return nil, nil, errs.ErrUserIdInvalid
	}

	previous := &models.UtilityReading{}
	sess := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND meter_id=? AND reading_date<?", uid, false, meterId, readingDate)

	if excludeReadingId > 0 {
		sess = sess.And("reading_id<>?", excludeReadingId)
	}

	hasPrevious, err := sess.OrderBy("reading_date desc").Limit(1).Get(previous)

	if err != nil {
		return nil, nil, err
	}

	next := &models.UtilityReading{}
	sess = s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND meter_id=? AND reading_date>?", uid, false, meterId, readingDate)

	if excludeReadingId > 0 {
		sess = sess.And("reading_id<>?", excludeReadingId)
	}

	hasNext, err := sess.OrderBy("reading_date asc").Limit(1).Get(next)

	if err != nil {
		return nil, nil, err
	}

	if !hasPrevious {
		previous = nil
	}

	if !hasNext {
		next = nil
	}

	return previous, next, nil
}

// existsMeterName returns whether the user already has a meter of this name, ignoring the meter
// being renamed
func (s *UtilityMeterService) existsMeterName(c core.Context, uid int64, name string, excludeMeterId int64) (bool, error) {
	sess := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND name=?", uid, false, name)

	if excludeMeterId > 0 {
		sess = sess.And("meter_id<>?", excludeMeterId)
	}

	return sess.Limit(1).Exist(&models.UtilityMeter{})
}

// existsTariffStartDate returns whether a tariff of this meter already begins on this day. Two
// prices that begin on the same day cannot both be in force, and there would be no saying which of
// them a period was to be costed at.
func (s *UtilityMeterService) existsTariffStartDate(c core.Context, uid int64, meterId int64, startDate int32, excludeTariffId int64) (bool, error) {
	sess := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND meter_id=? AND start_date=?", uid, false, meterId, startDate)

	if excludeTariffId > 0 {
		sess = sess.And("tariff_id<>?", excludeTariffId)
	}

	return sess.Limit(1).Exist(&models.UtilityTariff{})
}

// existsReadingDate returns whether this meter has already been read on this day
func (s *UtilityMeterService) existsReadingDate(c core.Context, uid int64, meterId int64, readingDate int32, excludeReadingId int64) (bool, error) {
	sess := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND meter_id=? AND reading_date=?", uid, false, meterId, readingDate)

	if excludeReadingId > 0 {
		sess = sess.And("reading_id<>?", excludeReadingId)
	}

	return sess.Limit(1).Exist(&models.UtilityReading{})
}

// countTariffs returns how many prices are on file for one meter
func (s *UtilityMeterService) countTariffs(c core.Context, uid int64, meterId int64) (int64, error) {
	return s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND meter_id=?", uid, false, meterId).Count(&models.UtilityTariff{})
}

// countReadings returns how many times one meter has been read
func (s *UtilityMeterService) countReadings(c core.Context, uid int64, meterId int64) (int64, error) {
	return s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND meter_id=?", uid, false, meterId).Count(&models.UtilityReading{})
}
