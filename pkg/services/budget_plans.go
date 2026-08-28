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

// maximumItemsCountOfBudgetPlanMonth is what stops one month's plan from growing without bound.
// A month with more than this many separately planned things is not being planned, and copying
// such a month forward would multiply the problem every time.
const maximumItemsCountOfBudgetPlanMonth = 200

// BudgetPlanService represents the service of what a month is planned to cost
type BudgetPlanService struct {
	ServiceUsingDB
	ServiceUsingUuid
}

// Initialize a budget plan service singleton instance
var (
	BudgetPlans = &BudgetPlanService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
		ServiceUsingUuid: ServiceUsingUuid{
			container: uuid.Container,
		},
	}
)

// GetItemsByMonth returns everything planned by hand for one month
func (s *BudgetPlanService) GetItemsByMonth(c core.Context, uid int64, year int32, month int32) ([]*models.BudgetPlanItem, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var items []*models.BudgetPlanItem
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND year=? AND month=?", uid, false, year, month).OrderBy("display_order asc").Find(&items)

	return items, err
}

// GetAdjustmentsByMonth returns every way one month differs from the schedules
func (s *BudgetPlanService) GetAdjustmentsByMonth(c core.Context, uid int64, year int32, month int32) ([]*models.BudgetPlanScheduleAdjustment, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var adjustments []*models.BudgetPlanScheduleAdjustment
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND year=? AND month=?", uid, false, year, month).Find(&adjustments)

	return adjustments, err
}

// GetItemByItemId returns one planned item
func (s *BudgetPlanService) GetItemByItemId(c core.Context, uid int64, itemId int64) (*models.BudgetPlanItem, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if itemId <= 0 {
		return nil, errs.ErrBudgetPlanItemIdInvalid
	}

	item := &models.BudgetPlanItem{}
	has, err := s.UserDataDB(uid).NewSession(c).ID(itemId).Where("uid=? AND deleted=?", uid, false).Get(item)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, errs.ErrBudgetPlanItemNotFound
	}

	return item, nil
}

// CreateItem plans one more thing for a month
func (s *BudgetPlanService) CreateItem(c core.Context, item *models.BudgetPlanItem) error {
	if item.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	count, err := s.getItemCount(c, item.Uid, item.Year, item.Month)

	if err != nil {
		return err
	} else if count >= maximumItemsCountOfBudgetPlanMonth {
		return errs.ErrBudgetPlanHasTooManyItems
	}

	maxOrder, err := s.getMaxDisplayOrder(c, item.Uid, item.Year, item.Month)

	if err != nil {
		return err
	}

	item.ItemId = s.GenerateUuid(uuid.UUID_TYPE_DEFAULT)

	if item.ItemId < 1 {
		return errs.ErrSystemIsBusy
	}

	item.Deleted = false
	item.DisplayOrder = maxOrder + 1
	item.CreatedUnixTime = time.Now().Unix()
	item.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(item.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Insert(item)
		return err
	})
}

// ModifyItem changes what was planned. The month it belongs to is not among the things that can
// change: moving a plan item to another month is adding it there and removing it here, and doing it
// as a move would let it slip past the per-month limit.
func (s *BudgetPlanService) ModifyItem(c core.Context, item *models.BudgetPlanItem) error {
	if item.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	item.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(item.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		updatedRows, err := sess.ID(item.ItemId).Cols("type", "category_id", "account_id", "amount", "name", "comment", "updated_unix_time").Where("uid=? AND deleted=?", item.Uid, false).Update(item)

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrBudgetPlanItemNotFound
		}

		return nil
	})
}

// DeleteItem takes one planned thing back out of the month
func (s *BudgetPlanService) DeleteItem(c core.Context, uid int64, itemId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	now := time.Now().Unix()
	updateModel := &models.BudgetPlanItem{
		Deleted:         true,
		DeletedUnixTime: now,
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		deletedRows, err := sess.ID(itemId).Cols("deleted", "deleted_unix_time").Where("uid=? AND deleted=?", uid, false).Update(updateModel)

		if err != nil {
			return err
		} else if deletedRows < 1 {
			return errs.ErrBudgetPlanItemNotFound
		}

		return nil
	})
}

// CopyItems copies everything planned by hand in one month into another, appending to whatever is
// already planned there rather than replacing it - a plan half filled in is not something to throw
// away because the last month is being copied over it.
func (s *BudgetPlanService) CopyItems(c core.Context, uid int64, fromYear int32, fromMonth int32, toYear int32, toMonth int32) (int, error) {
	if uid <= 0 {
		return 0, errs.ErrUserIdInvalid
	}

	if fromYear == toYear && fromMonth == toMonth {
		return 0, errs.ErrBudgetPlanCopyToSameMonth
	}

	sourceItems, err := s.GetItemsByMonth(c, uid, fromYear, fromMonth)

	if err != nil {
		return 0, err
	}

	if len(sourceItems) < 1 {
		return 0, errs.ErrBudgetPlanNothingToCopy
	}

	existingCount, err := s.getItemCount(c, uid, toYear, toMonth)

	if err != nil {
		return 0, err
	}

	if int(existingCount)+len(sourceItems) > maximumItemsCountOfBudgetPlanMonth {
		return 0, errs.ErrBudgetPlanHasTooManyItems
	}

	maxOrder, err := s.getMaxDisplayOrder(c, uid, toYear, toMonth)

	if err != nil {
		return 0, err
	}

	now := time.Now().Unix()
	newItems := make([]*models.BudgetPlanItem, 0, len(sourceItems))

	for i := 0; i < len(sourceItems); i++ {
		sourceItem := sourceItems[i]
		itemId := s.GenerateUuid(uuid.UUID_TYPE_DEFAULT)

		if itemId < 1 {
			return 0, errs.ErrSystemIsBusy
		}

		newItems = append(newItems, &models.BudgetPlanItem{
			ItemId:          itemId,
			Uid:             uid,
			Deleted:         false,
			Year:            toYear,
			Month:           toMonth,
			Type:            sourceItem.Type,
			CategoryId:      sourceItem.CategoryId,
			AccountId:       sourceItem.AccountId,
			Amount:          sourceItem.Amount,
			Name:            sourceItem.Name,
			Comment:         sourceItem.Comment,
			DisplayOrder:    maxOrder + int32(i) + 1,
			CreatedUnixTime: now,
			UpdatedUnixTime: now,
		})
	}

	err = s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		for i := 0; i < len(newItems); i++ {
			if _, err := sess.Insert(newItems[i]); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	return len(newItems), nil
}

// SetAdjustment records how one month differs from one schedule, replacing whatever was said about
// that schedule in that month before. An adjustment that says nothing - not excluded, no amount of
// its own - is a deletion, because carrying an empty row would make the schedule look adjusted when
// it is not.
func (s *BudgetPlanService) SetAdjustment(c core.Context, adjustment *models.BudgetPlanScheduleAdjustment) error {
	if adjustment.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	now := time.Now().Unix()

	return s.UserDataDB(adjustment.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		clearModel := &models.BudgetPlanScheduleAdjustment{
			Deleted:         true,
			DeletedUnixTime: now,
		}

		_, err := sess.Cols("deleted", "deleted_unix_time").
			Where("uid=? AND deleted=? AND year=? AND month=? AND template_id=?", adjustment.Uid, false, adjustment.Year, adjustment.Month, adjustment.TemplateId).
			Update(clearModel)

		if err != nil {
			return err
		}

		if !adjustment.Excluded && adjustment.Amount == nil {
			return nil
		}

		adjustment.AdjustmentId = s.GenerateUuid(uuid.UUID_TYPE_DEFAULT)

		if adjustment.AdjustmentId < 1 {
			return errs.ErrSystemIsBusy
		}

		adjustment.Deleted = false
		adjustment.CreatedUnixTime = now
		adjustment.UpdatedUnixTime = now

		_, err = sess.Insert(adjustment)

		return err
	})
}

func (s *BudgetPlanService) getItemCount(c core.Context, uid int64, year int32, month int32) (int64, error) {
	return s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND year=? AND month=?", uid, false, year, month).Count(&models.BudgetPlanItem{})
}

func (s *BudgetPlanService) getMaxDisplayOrder(c core.Context, uid int64, year int32, month int32) (int32, error) {
	item := &models.BudgetPlanItem{}
	has, err := s.UserDataDB(uid).NewSession(c).Cols("display_order").Where("uid=? AND deleted=? AND year=? AND month=?", uid, false, year, month).OrderBy("display_order desc").Limit(1).Get(item)

	if err != nil {
		return 0, err
	}

	if has {
		return item.DisplayOrder, nil
	}

	return 0, nil
}
