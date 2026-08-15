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

// ReceiptLineItemCategoryService owns what the user has taught the import about their receipts:
// which category each article they have bought belongs to.
//
// The answers are recorded when a receipt is imported and read back when the next one is recognized,
// so that categorizing a shop is work that only has to be done once per article rather than once per
// receipt. Nothing here decides anything - it only remembers what the user already decided.
type ReceiptLineItemCategoryService struct {
	ServiceUsingDB
	ServiceUsingUuid
}

// Initialize a receipt line item category service singleton instance
var (
	ReceiptLineItemCategories = &ReceiptLineItemCategoryService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
		ServiceUsingUuid: ServiceUsingUuid{
			container: uuid.Container,
		},
	}
)

// GetAllByUid returns every article category the specified user has taught the import
func (s *ReceiptLineItemCategoryService) GetAllByUid(c core.Context, uid int64) ([]*models.ReceiptLineItemCategory, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var lineItemCategories []*models.ReceiptLineItemCategory
	err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).Find(&lineItemCategories)

	if err != nil {
		return nil, err
	}

	return lineItemCategories, nil
}

// Remember records which category each of the given articles belongs to, for the specified user.
//
// An article already known is updated rather than added a second time, because the user's latest
// answer is the one that counts: dragging a line somewhere else and importing is how a category
// recorded by mistake is corrected, and that only works if the newer answer replaces the older one.
func (s *ReceiptLineItemCategoryService) Remember(c core.Context, uid int64, items []*models.ReceiptLineItemCategoryRememberItem) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if len(items) < 1 {
		return nil
	}

	// one receipt may print the same article twice, and the two copies may by then have been dragged
	// to different categories, so the last of them decides - as it does when the user corrects an
	// entry across two imports
	itemsByNormalizedName := make(map[string]*models.ReceiptLineItemCategoryRememberItem, len(items))
	normalizedNames := make([]string, 0, len(items))

	for _, item := range items {
		if item == nil || item.CategoryId <= 0 {
			continue
		}

		normalizedName := models.NormalizeReceiptLineItemName(item.Name)

		if normalizedName == "" {
			continue
		}

		if _, exists := itemsByNormalizedName[normalizedName]; !exists {
			normalizedNames = append(normalizedNames, normalizedName)
		}

		itemsByNormalizedName[normalizedName] = item
	}

	if len(normalizedNames) < 1 {
		return nil
	}

	now := time.Now().Unix()

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		for _, normalizedName := range normalizedNames {
			item := itemsByNormalizedName[normalizedName]

			updateModel := &models.ReceiptLineItemCategory{
				Name:            item.Name,
				CategoryId:      item.CategoryId,
				UpdatedUnixTime: now,
			}

			updatedRows, err := sess.Cols("name", "category_id", "updated_unix_time").
				SetExpr("times_used", "times_used + 1").
				Where("uid=? AND normalized_name=?", uid, normalizedName).
				Update(updateModel)

			if err != nil {
				return err
			}

			if updatedRows > 0 {
				continue
			}

			_, err = sess.Insert(&models.ReceiptLineItemCategory{
				Id:              s.GenerateUuid(uuid.UUID_TYPE_DEFAULT),
				Uid:             uid,
				NormalizedName:  normalizedName,
				Name:            item.Name,
				CategoryId:      item.CategoryId,
				TimesUsed:       1,
				CreatedUnixTime: now,
				UpdatedUnixTime: now,
			})

			if err != nil {
				return err
			}
		}

		return nil
	})
}
