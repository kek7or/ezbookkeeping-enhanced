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

// DebtService represents the service of what other people owe the user
type DebtService struct {
	ServiceUsingDB
	ServiceUsingUuid
}

// Initialize a debt service singleton instance
var (
	Debts = &DebtService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
		ServiceUsingUuid: ServiceUsingUuid{
			container: uuid.Container,
		},
	}
)

// GetAllPersonsByUid returns all people who owe the user money
func (s *DebtService) GetAllPersonsByUid(c core.Context, uid int64) ([]*models.DebtPerson, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var persons []*models.DebtPerson
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).OrderBy("display_order asc").Find(&persons)

	return persons, err
}

// GetPersonByPersonId returns one person according to person id
func (s *DebtService) GetPersonByPersonId(c core.Context, uid int64, personId int64) (*models.DebtPerson, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if personId <= 0 {
		return nil, errs.ErrDebtPersonIdInvalid
	}

	person := &models.DebtPerson{}
	has, err := s.UserDataDB(uid).NewSession(c).ID(personId).Where("uid=? AND deleted=?", uid, false).Get(person)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, errs.ErrDebtPersonNotFound
	}

	return person, nil
}

// GetMaxDisplayOrder returns the display order of the person listed last
func (s *DebtService) GetMaxDisplayOrder(c core.Context, uid int64) (int32, error) {
	if uid <= 0 {
		return 0, errs.ErrUserIdInvalid
	}

	person := &models.DebtPerson{}
	has, err := s.UserDataDB(uid).NewSession(c).Cols("uid", "deleted", "display_order").Where("uid=? AND deleted=?", uid, false).OrderBy("display_order desc").Limit(1).Get(person)

	if err != nil {
		return 0, err
	}

	if has {
		return person.DisplayOrder, nil
	}

	return 0, nil
}

// CreatePerson saves a new person to database
func (s *DebtService) CreatePerson(c core.Context, person *models.DebtPerson) error {
	if person.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	exists, err := s.existsPersonName(c, person.Uid, person.Name, 0)

	if err != nil {
		return err
	} else if exists {
		return errs.ErrDebtPersonNameAlreadyExists
	}

	person.PersonId = s.GenerateUuid(uuid.UUID_TYPE_DEFAULT)

	if person.PersonId < 1 {
		return errs.ErrSystemIsBusy
	}

	person.Deleted = false
	person.CreatedUnixTime = time.Now().Unix()
	person.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(person.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Insert(person)
		return err
	})
}

// ModifyPerson renames an existed person
func (s *DebtService) ModifyPerson(c core.Context, person *models.DebtPerson) error {
	if person.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	exists, err := s.existsPersonName(c, person.Uid, person.Name, person.PersonId)

	if err != nil {
		return err
	} else if exists {
		return errs.ErrDebtPersonNameAlreadyExists
	}

	person.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(person.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		updatedRows, err := sess.ID(person.PersonId).Cols("name", "updated_unix_time").Where("uid=? AND deleted=?", person.Uid, false).Update(person)

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrDebtPersonNotFound
		}

		return nil
	})
}

// DeletePerson removes a person and everything attached to them.
//
// The entries go with the person because they are nothing without one - an entry says who owes for
// a transaction, and with the person gone there is nobody left for it to say that about. The
// transactions themselves are untouched: the money was spent either way.
func (s *DebtService) DeletePerson(c core.Context, uid int64, personId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	now := time.Now().Unix()

	personUpdateModel := &models.DebtPerson{
		Deleted:         true,
		DeletedUnixTime: now,
	}

	entryUpdateModel := &models.DebtEntry{
		Deleted:         true,
		DeletedUnixTime: now,
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		deletedRows, err := sess.ID(personId).Cols("deleted", "deleted_unix_time").Where("uid=? AND deleted=?", uid, false).Update(personUpdateModel)

		if err != nil {
			return err
		} else if deletedRows < 1 {
			return errs.ErrDebtPersonNotFound
		}

		_, err = sess.Cols("deleted", "deleted_unix_time").Where("uid=? AND deleted=? AND person_id=?", uid, false, personId).Update(entryUpdateModel)

		return err
	})
}

// GetAllOpenEntriesByUid returns everything that is still owed by anybody, which is what the list of
// people is totalled from
func (s *DebtService) GetAllOpenEntriesByUid(c core.Context, uid int64) ([]*models.DebtEntry, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var entries []*models.DebtEntry
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND settlement_transaction_id=?", uid, false, 0).Find(&entries)

	return entries, err
}

// GetEntriesByPersonId returns what one person owes, optionally including what they have already paid back
func (s *DebtService) GetEntriesByPersonId(c core.Context, uid int64, personId int64, includeSettled bool) ([]*models.DebtEntry, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if personId <= 0 {
		return nil, errs.ErrDebtPersonIdInvalid
	}

	var entries []*models.DebtEntry
	sess := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND person_id=?", uid, false, personId)

	if !includeSettled {
		sess = sess.And("settlement_transaction_id=?", 0)
	}

	err := sess.Find(&entries)

	return entries, err
}

// GetEntriesByTransactionId returns what is owed of one transaction, both as a whole and position by
// position, so that a transaction being looked at can show who is to pay for what
func (s *DebtService) GetEntriesByTransactionId(c core.Context, uid int64, transactionId int64) ([]*models.DebtEntry, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if transactionId <= 0 {
		return nil, errs.ErrTransactionIdInvalid
	}

	var entries []*models.DebtEntry
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND transaction_id=?", uid, false, transactionId).Find(&entries)

	return entries, err
}

// GetEntriesByEntryIds returns the entries with the given ids that belong to this user
func (s *DebtService) GetEntriesByEntryIds(c core.Context, uid int64, entryIds []int64) ([]*models.DebtEntry, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if len(entryIds) < 1 {
		return nil, errs.ErrDebtEntryIdInvalid
	}

	var entries []*models.DebtEntry
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).In("entry_id", entryIds).Find(&entries)

	return entries, err
}

// CreateEntries attaches things to a person.
//
// The whole batch is written or none of it is, and an entry that is already attached to that person
// stops the batch rather than being written twice - being told twice that the same thing is owed is
// how a total quietly doubles.
func (s *DebtService) CreateEntries(c core.Context, uid int64, entries []*models.DebtEntry) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if len(entries) < 1 {
		return errs.ErrDebtEntryNotFound
	}

	entryUuids := s.GenerateUuids(uuid.UUID_TYPE_DEFAULT, uint16(len(entries)))

	if len(entryUuids) < len(entries) {
		return errs.ErrSystemIsBusy
	}

	now := time.Now().Unix()

	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		entry.EntryId = entryUuids[i]
		entry.Uid = uid
		entry.Deleted = false
		entry.CreatedUnixTime = now
		entry.UpdatedUnixTime = now
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		for i := 0; i < len(entries); i++ {
			entry := entries[i]
			exists, err := sess.Where("uid=? AND deleted=? AND person_id=? AND transaction_id=? AND line_item_id=?", uid, false, entry.PersonId, entry.TransactionId, entry.LineItemId).Limit(1).Exist(&models.DebtEntry{})

			if err != nil {
				return err
			} else if exists {
				return errs.ErrDebtEntryAlreadyExists
			}
		}

		for i := 0; i < len(entries); i++ {
			_, err := sess.Insert(entries[i])

			if err != nil {
				return err
			}
		}

		return nil
	})
}

// CreateManualEntry records a debt that has no transaction behind it.
//
// It skips the check that stops the same thing being attached to one person twice, because there is
// no thing here to compare - two loans of the same amount on the same day are two loans, and only the
// user can say whether the second one is a mistake.
func (s *DebtService) CreateManualEntry(c core.Context, entry *models.DebtEntry) error {
	if entry.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if entry.Description == "" {
		return errs.ErrDebtEntryDescriptionIsEmpty
	}

	if entry.Amount <= 0 {
		return errs.ErrDebtEntryAmountInvalid
	}

	entry.EntryId = s.GenerateUuid(uuid.UUID_TYPE_DEFAULT)

	if entry.EntryId < 1 {
		return errs.ErrSystemIsBusy
	}

	entry.TransactionId = 0
	entry.LineItemId = 0
	entry.Deleted = false
	entry.CreatedUnixTime = time.Now().Unix()
	entry.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(entry.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Insert(entry)
		return err
	})
}

// ModifyEntry changes what is owed of one entry, and what a debt entered by hand is called when a
// new description is given
func (s *DebtService) ModifyEntry(c core.Context, uid int64, entryId int64, amount int64, description string) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if entryId <= 0 {
		return errs.ErrDebtEntryIdInvalid
	}

	if amount <= 0 {
		return errs.ErrDebtEntryAmountInvalid
	}

	updateModel := &models.DebtEntry{
		Amount:          amount,
		Description:     description,
		UpdatedUnixTime: time.Now().Unix(),
	}

	updateCols := []string{"amount", "updated_unix_time"}

	if description != "" {
		updateCols = append(updateCols, "description")
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		entry := &models.DebtEntry{}
		has, err := sess.ID(entryId).Where("uid=? AND deleted=?", uid, false).Get(entry)

		if err != nil {
			return err
		} else if !has {
			return errs.ErrDebtEntryNotFound
		}

		// only a debt entered by hand is named by this row; one with a transaction is named by that
		// transaction, and renaming it here would put a second name on the same spending
		if description != "" && entry.TransactionId > 0 {
			return errs.ErrDebtEntryIsNotManual
		}

		// What has been paid back is history. Changing the amount of a settled entry would restate a
		// payment that has already happened, and the entry would no longer add up to the transaction
		// that settled it.
		if entry.SettlementTransactionId > 0 {
			return errs.ErrDebtEntryAlreadySettled
		}

		updatedRows, err := sess.ID(entryId).Cols(updateCols...).Where("uid=? AND deleted=?", uid, false).Update(updateModel)

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrDebtEntryNotFound
		}

		return nil
	})
}

// DeleteEntries detaches things from whoever they were attached to
func (s *DebtService) DeleteEntries(c core.Context, uid int64, entryIds []int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if len(entryIds) < 1 {
		return errs.ErrDebtEntryIdInvalid
	}

	updateModel := &models.DebtEntry{
		Deleted:         true,
		DeletedUnixTime: time.Now().Unix(),
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		deletedRows, err := sess.Cols("deleted", "deleted_unix_time").Where("uid=? AND deleted=?", uid, false).In("entry_id", entryIds).Update(updateModel)

		if err != nil {
			return err
		} else if deletedRows < 1 {
			return errs.ErrDebtEntryNotFound
		}

		return nil
	})
}

// SettleEntries marks entries as paid back by one transaction.
//
// Every entry named must still be open, because settling something twice would say the money came
// back twice.
func (s *DebtService) SettleEntries(c core.Context, uid int64, entryIds []int64, settlementTransactionId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if len(entryIds) < 1 {
		return errs.ErrDebtEntryIdInvalid
	}

	if settlementTransactionId <= 0 {
		return errs.ErrTransactionIdInvalid
	}

	now := time.Now().Unix()

	updateModel := &models.DebtEntry{
		SettlementTransactionId: settlementTransactionId,
		SettledUnixTime:         now,
		UpdatedUnixTime:         now,
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var entries []*models.DebtEntry
		err := sess.Where("uid=? AND deleted=?", uid, false).In("entry_id", entryIds).Find(&entries)

		if err != nil {
			return err
		}

		if len(entries) != len(entryIds) {
			return errs.ErrDebtEntryNotFound
		}

		for i := 0; i < len(entries); i++ {
			if entries[i].SettlementTransactionId > 0 {
				return errs.ErrDebtEntryAlreadySettled
			}
		}

		_, err = sess.Cols("settlement_transaction_id", "settled_unix_time", "updated_unix_time").Where("uid=? AND deleted=? AND settlement_transaction_id=?", uid, false, 0).In("entry_id", entryIds).Update(updateModel)

		return err
	})
}

// ReopenEntries puts settled entries back on the bill, for when a payment was recorded against the
// wrong things. The transaction that settled them is left alone - it is the user's to delete or keep.
func (s *DebtService) ReopenEntries(c core.Context, uid int64, entryIds []int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if len(entryIds) < 1 {
		return errs.ErrDebtEntryIdInvalid
	}

	updateModel := &models.DebtEntry{
		SettlementTransactionId: 0,
		SettledUnixTime:         0,
		UpdatedUnixTime:         time.Now().Unix(),
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		updatedRows, err := sess.Cols("settlement_transaction_id", "settled_unix_time", "updated_unix_time").Where("uid=? AND deleted=?", uid, false).In("entry_id", entryIds).Update(updateModel)

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrDebtEntryNotFound
		}

		return nil
	})
}

// existsPersonName returns whether the user already has somebody by this name, ignoring the person
// being renamed
func (s *DebtService) existsPersonName(c core.Context, uid int64, name string, exceptPersonId int64) (bool, error) {
	if name == "" {
		return false, errs.ErrDebtPersonNameIsEmpty
	}

	sess := s.UserDataDB(uid).NewSession(c).Cols("uid", "deleted", "name").Where("uid=? AND deleted=? AND name=?", uid, false, name)

	if exceptPersonId > 0 {
		sess = sess.And("person_id<>?", exceptPersonId)
	}

	return sess.Limit(1).Exist(&models.DebtPerson{})
}
