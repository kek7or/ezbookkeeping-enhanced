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

// GetPersonsByPersonIds returns the people with the given ids that belong to this user, keyed by id
func (s *DebtService) GetPersonsByPersonIds(c core.Context, uid int64, personIds []int64) (map[int64]*models.DebtPerson, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if len(personIds) < 1 {
		return nil, errs.ErrDebtPersonIdInvalid
	}

	var persons []*models.DebtPerson
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).In("person_id", personIds).Find(&persons)

	if err != nil {
		return nil, err
	}

	personMap := make(map[int64]*models.DebtPerson, len(persons))

	for i := 0; i < len(persons); i++ {
		personMap[persons[i].PersonId] = persons[i]
	}

	return personMap, nil
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
//
// Anything this person was sharing with somebody else is divided again over whoever is left on it,
// exactly as it is when a single share is detached.
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

		// what this person owed has to be read before it is gone, because it is what says which
		// shared things have a share missing from them now
		var removedEntries []*models.DebtEntry
		err = sess.Where("uid=? AND deleted=? AND person_id=?", uid, false, personId).Find(&removedEntries)

		if err != nil {
			return err
		}

		_, err = sess.Cols("deleted", "deleted_unix_time").Where("uid=? AND deleted=? AND person_id=?", uid, false, personId).Update(entryUpdateModel)

		if err != nil {
			return err
		}

		_, err = s.resplitSharedThings(sess, uid, removedEntries, false)

		return err
	})
}

// GetAllOpenEntriesByUid returns everything that is still owed by anybody, which is what the list of
// people is totalled from.
//
// What has been written off is left out of it for the same reason what has been paid back is: it is
// no longer expected, and a total of what people owe that counted it would be asking for money
// nobody is going to hand over.
func (s *DebtService) GetAllOpenEntriesByUid(c core.Context, uid int64) ([]*models.DebtEntry, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var entries []*models.DebtEntry
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND settlement_transaction_id=? AND forgiven_unix_time=?", uid, false, 0, 0).Find(&entries)

	return entries, err
}

// GetEntriesByPersonId returns what one person owes, optionally including what is no longer owed -
// what they have paid back, and what was written off
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
		sess = sess.And("settlement_transaction_id=?", 0).And("forgiven_unix_time=?", 0)
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

		// What was written off is history in the same way. The amount is what was let go, and
		// changing it afterwards would restate a decision that has already been made.
		if entry.ForgivenUnixTime > 0 {
			return errs.ErrDebtEntryAlreadyForgiven
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

// DeleteEntries detaches things from whoever they were attached to, and divides what is left of
// anything that was shared out again over whoever is still on it
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
		// what is being detached has to be read before it is gone, because it is what says which
		// shared things have a share missing from them now
		var removedEntries []*models.DebtEntry
		err := sess.Where("uid=? AND deleted=?", uid, false).In("entry_id", entryIds).Find(&removedEntries)

		if err != nil {
			return err
		}

		deletedRows, err := sess.Cols("deleted", "deleted_unix_time").Where("uid=? AND deleted=?", uid, false).In("entry_id", entryIds).Update(updateModel)

		if err != nil {
			return err
		} else if deletedRows < 1 {
			return errs.ErrDebtEntryNotFound
		}

		_, err = s.resplitSharedThings(sess, uid, removedEntries, false)

		return err
	})
}

// ResplitSharedThingsOfUser divides again every shared thing that had a share taken off it before
// detaching started doing that by itself.
//
// A detached share is not thrown away, only struck out, and the struck-out rows are still there to
// say what each thing was once divided into. Reading them back and handing them to the same routine
// that runs on a detach replays every detach this user ever made, and leaves the shares where they
// would have been had they always been divided again.
//
// It is safe to run twice. A thing already divided over the heads left on it is one where nothing
// moves the second time, and a thing whose shares were never an even division of it is left alone
// however often it is looked at.
func (s *DebtService) ResplitSharedThingsOfUser(c core.Context, uid int64, dryRun bool) ([]*models.DebtEntryResplit, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var resplits []*models.DebtEntryResplit

	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var detachedEntries []*models.DebtEntry
		err := sess.Where("uid=? AND deleted=?", uid, true).OrderBy("entry_id asc").Find(&detachedEntries)

		if err != nil {
			return err
		}

		resplits, err = s.resplitSharedThings(sess, uid, detachedEntries, dryRun)

		return err
	})

	if err != nil {
		return nil, err
	}

	return resplits, nil
}

// sharedThing is one thing shares can be owed of: a whole transaction, or one position of one
type sharedThing struct {
	transactionId int64
	lineItemId    int64
}

// resplitSharedThings divides again what is left of the things the given entries were shares of.
//
// A thing taken off somebody's bill still costs what it cost. Their share does not become everybody
// else's problem and does not vanish either - there is simply one head fewer to divide by, so the
// whole thing is divided again over the heads that are left, the one who paid among them if they
// were counted in to begin with.
//
// Three kinds of thing are left exactly as they are. A debt entered by hand is a share of nothing,
// as there is no transaction behind it to divide. Anything already closed is history - dividing
// around a settled share would either restate a payment or charge somebody for one that has already
// been made, and dividing around a forgiven one would charge somebody more than what was let go was
// worth. And shares that are not an even division of the thing were put there by hand, where the
// numbers say what the user meant them to say and nothing here may overrule them.
func (s *DebtService) resplitSharedThings(sess *xorm.Session, uid int64, removedEntries []*models.DebtEntry, dryRun bool) ([]*models.DebtEntryResplit, error) {
	things := make([]sharedThing, 0, len(removedEntries))
	removedShares := make(map[sharedThing][]int64)
	closedThings := make(map[sharedThing]bool)

	for i := 0; i < len(removedEntries); i++ {
		entry := removedEntries[i]

		if entry.TransactionId <= 0 {
			continue
		}

		thing := sharedThing{transactionId: entry.TransactionId, lineItemId: entry.LineItemId}

		if _, exists := removedShares[thing]; !exists {
			things = append(things, thing)
		}

		removedShares[thing] = append(removedShares[thing], entry.Amount)

		if entry.SettlementTransactionId > 0 || entry.ForgivenUnixTime > 0 {
			closedThings[thing] = true
		}
	}

	resplits := make([]*models.DebtEntryResplit, 0, len(things))

	for i := 0; i < len(things); i++ {
		thing := things[i]

		if closedThings[thing] {
			continue
		}

		thingResplits, err := s.resplitSharedThing(sess, uid, thing, removedShares[thing], dryRun)

		if err != nil {
			return nil, err
		}

		resplits = append(resplits, thingResplits...)
	}

	return resplits, nil
}

// resplitSharedThing divides one thing again over the shares of it that are left, and writes them
// only where the division actually moves them. It answers which of them moved, whether or not it was
// the one to move them.
func (s *DebtService) resplitSharedThing(sess *xorm.Session, uid int64, thing sharedThing, removedShares []int64, dryRun bool) ([]*models.DebtEntryResplit, error) {
	var remainingEntries []*models.DebtEntry

	// in the order they were attached in, which is the order the shares were handed out in, so that
	// a cent that does not divide stays with whoever was handed it
	err := sess.Where("uid=? AND deleted=? AND transaction_id=? AND line_item_id=?", uid, false, thing.transactionId, thing.lineItemId).OrderBy("entry_id asc").Find(&remainingEntries)

	if err != nil {
		return nil, err
	}

	if len(remainingEntries) < 1 {
		return nil, nil
	}

	oldShares := make([]int64, 0, len(removedShares)+len(remainingEntries))
	oldShares = append(oldShares, removedShares...)

	for i := 0; i < len(remainingEntries); i++ {
		if remainingEntries[i].SettlementTransactionId > 0 || remainingEntries[i].ForgivenUnixTime > 0 {
			return nil, nil
		}

		oldShares = append(oldShares, remainingEntries[i].Amount)
	}

	totalAmount, err := s.getSharedThingAmount(sess, uid, thing)

	if err != nil {
		return nil, err
	}

	// the thing itself is no longer in the ledger, and there is nothing left to divide
	if totalAmount <= 0 {
		return nil, nil
	}

	newShares, isEvenSplit := models.ResplitEvenly(totalAmount, oldShares, len(remainingEntries))

	if !isEvenSplit {
		return nil, nil
	}

	now := time.Now().Unix()
	resplits := make([]*models.DebtEntryResplit, 0, len(remainingEntries))

	for i := 0; i < len(remainingEntries); i++ {
		entry := remainingEntries[i]

		if entry.Amount == newShares[i] {
			continue
		}

		resplits = append(resplits, &models.DebtEntryResplit{
			EntryId:       entry.EntryId,
			PersonId:      entry.PersonId,
			TransactionId: entry.TransactionId,
			LineItemId:    entry.LineItemId,
			Currency:      entry.Currency,
			OldAmount:     entry.Amount,
			NewAmount:     newShares[i],
		})

		if dryRun {
			continue
		}

		updateModel := &models.DebtEntry{
			Amount:          newShares[i],
			UpdatedUnixTime: now,
		}

		_, err := sess.ID(entry.EntryId).Cols("amount", "updated_unix_time").Where("uid=? AND deleted=?", uid, false).Update(updateModel)

		if err != nil {
			return nil, err
		}
	}

	return resplits, nil
}

// getSharedThingAmount returns what the thing the shares are owed of came to, as the positive number
// a debt is always stated in. It is zero when the thing is no longer there to be divided.
func (s *DebtService) getSharedThingAmount(sess *xorm.Session, uid int64, thing sharedThing) (int64, error) {
	if thing.lineItemId > 0 {
		lineItem := &models.TransactionReceiptLineItem{}
		has, err := sess.ID(thing.lineItemId).Where("uid=? AND deleted=?", uid, false).Get(lineItem)

		if err != nil {
			return 0, err
		} else if !has || lineItem.TransactionId != thing.transactionId {
			return 0, nil
		}

		return positiveAmount(lineItem.Amount), nil
	}

	transaction := &models.Transaction{}
	has, err := sess.ID(thing.transactionId).Where("uid=? AND deleted=?", uid, false).Get(transaction)

	if err != nil {
		return 0, err
	} else if !has {
		return 0, nil
	}

	return positiveAmount(transaction.Amount), nil
}

// positiveAmount returns an amount the way a debt states it, which is never as a negative number
func positiveAmount(amount int64) int64 {
	if amount < 0 {
		return -amount
	}

	return amount
}

// SettleEntries marks entries as paid back by one transaction.
//
// Every entry named must still be open, because settling something twice would say the money came
// back twice, and settling something that was written off would say money came back that was
// deliberately let go.
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

			if entries[i].ForgivenUnixTime > 0 {
				return errs.ErrDebtEntryAlreadyForgiven
			}
		}

		_, err = sess.Cols("settlement_transaction_id", "settled_unix_time", "updated_unix_time").Where("uid=? AND deleted=? AND settlement_transaction_id=? AND forgiven_unix_time=?", uid, false, 0, 0).In("entry_id", entryIds).Update(updateModel)

		return err
	})
}

// ForgiveEntries writes entries off: they stop being owed, and stay on the record saying so.
//
// Nothing is written to the ledger and nothing is asked of it. The money was spent when it was
// spent and was an expense then; being told it is not coming back does not move it anywhere, it
// only leaves it where it already was, as the user's own spending. That is also why forgiving is
// not the same as detaching: detaching says this was never this person's to pay, while forgiving
// says it was theirs and is being let go, and only one of those is a thing worth being able to look
// up a year later.
//
// Every entry named must still be open. What has been paid back cannot be forgiven - the money is
// already back - and forgiving twice would move the date of a decision that was made once.
func (s *DebtService) ForgiveEntries(c core.Context, uid int64, entryIds []int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if len(entryIds) < 1 {
		return errs.ErrDebtEntryIdInvalid
	}

	now := time.Now().Unix()

	updateModel := &models.DebtEntry{
		ForgivenUnixTime: now,
		UpdatedUnixTime:  now,
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

			if entries[i].ForgivenUnixTime > 0 {
				return errs.ErrDebtEntryAlreadyForgiven
			}
		}

		_, err = sess.Cols("forgiven_unix_time", "updated_unix_time").Where("uid=? AND deleted=? AND settlement_transaction_id=? AND forgiven_unix_time=?", uid, false, 0, 0).In("entry_id", entryIds).Update(updateModel)

		return err
	})
}

// ReopenEntries puts entries that are no longer owed back on the bill, for when a payment was
// recorded against the wrong things, or when something written off turns out to be coming back after
// all. The transaction that settled them is left alone - it is the user's to delete or keep.
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
		ForgivenUnixTime:        0,
		UpdatedUnixTime:         time.Now().Unix(),
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		updatedRows, err := sess.Cols("settlement_transaction_id", "settled_unix_time", "forgiven_unix_time", "updated_unix_time").Where("uid=? AND deleted=?", uid, false).In("entry_id", entryIds).Update(updateModel)

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
