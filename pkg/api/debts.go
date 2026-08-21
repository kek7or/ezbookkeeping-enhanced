package api

import (
	"fmt"
	"sort"
	"strings"
	"time"

	debtexporter "github.com/mayswind/ezbookkeeping/pkg/converters/debt"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// The words the receipt is written with. The debts feature speaks English wherever it has anything
// of its own to say, and the sheet handed to somebody else is no different.
const (
	debtReceiptDefaultFileName         = "Receipt"
	debtReceiptUnnamedReceiptTitle     = "Receipt"
	debtReceiptUnnamedTransactionTitle = "Transaction"
)

// DebtsApi represents the api of what other people owe the user
type DebtsApi struct {
	debts        *services.DebtService
	transactions *services.TransactionService
	accounts     *services.AccountService
	categories   *services.TransactionCategoryService
	users        *services.UserService
}

// Initialize a debt api singleton instance
var (
	Debts = &DebtsApi{
		debts:        services.Debts,
		transactions: services.Transactions,
		accounts:     services.Accounts,
		categories:   services.TransactionCategories,
		users:        services.Users,
	}
)

// PersonListHandler returns the people who owe the current user money, with what each of them owes
func (a *DebtsApi) PersonListHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	persons, err := a.debts.GetAllPersonsByUid(c, uid)

	if err != nil {
		log.Errorf(c, "[debts.PersonListHandler] failed to get people for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	openEntries, err := a.debts.GetAllOpenEntriesByUid(c, uid)

	if err != nil {
		log.Errorf(c, "[debts.PersonListHandler] failed to get open debt entries for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	openAmounts, openCounts := a.getOpenAmountsByPerson(openEntries)
	personResps := make(models.DebtPersonInfoResponseSlice, len(persons))

	for i := 0; i < len(persons); i++ {
		person := persons[i]
		personResps[i] = person.ToDebtPersonInfoResponse(openAmounts[person.PersonId], openCounts[person.PersonId])
	}

	sort.Sort(personResps)

	return personResps, nil
}

// PersonCreateHandler saves a new person by request parameters for current user
func (a *DebtsApi) PersonCreateHandler(c *core.WebContext) (any, *errs.Error) {
	var personCreateReq models.DebtPersonCreateRequest
	err := c.ShouldBindJSON(&personCreateReq)

	if err != nil {
		log.Warnf(c, "[debts.PersonCreateHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	maxOrderId, err := a.debts.GetMaxDisplayOrder(c, uid)

	if err != nil {
		log.Errorf(c, "[debts.PersonCreateHandler] failed to get max display order for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	person := &models.DebtPerson{
		Uid:          uid,
		Name:         personCreateReq.Name,
		DisplayOrder: maxOrderId + 1,
	}

	err = a.debts.CreatePerson(c, person)

	if err != nil {
		log.Errorf(c, "[debts.PersonCreateHandler] failed to create person for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[debts.PersonCreateHandler] user \"uid:%d\" has created a new person \"id:%d\" successfully", uid, person.PersonId)

	return person.ToDebtPersonInfoResponse(nil, 0), nil
}

// PersonModifyHandler saves an existed person modification by request parameters for current user
func (a *DebtsApi) PersonModifyHandler(c *core.WebContext) (any, *errs.Error) {
	var personModifyReq models.DebtPersonModifyRequest
	err := c.ShouldBindJSON(&personModifyReq)

	if err != nil {
		log.Warnf(c, "[debts.PersonModifyHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	person, err := a.debts.GetPersonByPersonId(c, uid, personModifyReq.Id)

	if err != nil {
		log.Errorf(c, "[debts.PersonModifyHandler] failed to get person \"id:%d\" for user \"uid:%d\", because %s", personModifyReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	if person.Name == personModifyReq.Name {
		return nil, errs.ErrNothingWillBeUpdated
	}

	newPerson := &models.DebtPerson{
		PersonId: person.PersonId,
		Uid:      uid,
		Name:     personModifyReq.Name,
	}

	err = a.debts.ModifyPerson(c, newPerson)

	if err != nil {
		log.Errorf(c, "[debts.PersonModifyHandler] failed to modify person \"id:%d\" for user \"uid:%d\", because %s", personModifyReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[debts.PersonModifyHandler] user \"uid:%d\" has modified person \"id:%d\" successfully", uid, personModifyReq.Id)

	person.Name = newPerson.Name

	return person.ToDebtPersonInfoResponse(nil, 0), nil
}

// PersonDeleteHandler deletes an existed person by request parameters for current user
func (a *DebtsApi) PersonDeleteHandler(c *core.WebContext) (any, *errs.Error) {
	var personDeleteReq models.DebtPersonDeleteRequest
	err := c.ShouldBindJSON(&personDeleteReq)

	if err != nil {
		log.Warnf(c, "[debts.PersonDeleteHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.debts.DeletePerson(c, uid, personDeleteReq.Id)

	if err != nil {
		log.Errorf(c, "[debts.PersonDeleteHandler] failed to delete person \"id:%d\" for user \"uid:%d\", because %s", personDeleteReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[debts.PersonDeleteHandler] user \"uid:%d\" has deleted person \"id:%d\"", uid, personDeleteReq.Id)

	return true, nil
}

// EntryListHandler returns what one person owes the current user
func (a *DebtsApi) EntryListHandler(c *core.WebContext) (any, *errs.Error) {
	var entryListReq models.DebtEntryListRequest
	err := c.ShouldBindQuery(&entryListReq)

	if err != nil {
		log.Warnf(c, "[debts.EntryListHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	_, err = a.debts.GetPersonByPersonId(c, uid, entryListReq.PersonId)

	if err != nil {
		log.Errorf(c, "[debts.EntryListHandler] failed to get person \"id:%d\" for user \"uid:%d\", because %s", entryListReq.PersonId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	entries, err := a.debts.GetEntriesByPersonId(c, uid, entryListReq.PersonId, entryListReq.IncludeSettled)

	if err != nil {
		log.Errorf(c, "[debts.EntryListHandler] failed to get debt entries of person \"id:%d\" for user \"uid:%d\", because %s", entryListReq.PersonId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	entryResps, err := a.getDescribedEntryResponses(c, uid, entries)

	if err != nil {
		log.Errorf(c, "[debts.EntryListHandler] failed to describe debt entries of person \"id:%d\" for user \"uid:%d\", because %s", entryListReq.PersonId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	sort.Sort(entryResps)

	return entryResps, nil
}

// EntryListByTransactionHandler returns what is owed of one transaction, so that the transaction can
// show who is to pay for it and for which of its positions
func (a *DebtsApi) EntryListByTransactionHandler(c *core.WebContext) (any, *errs.Error) {
	var entryListReq models.DebtEntryListByTransactionRequest
	err := c.ShouldBindQuery(&entryListReq)

	if err != nil {
		log.Warnf(c, "[debts.EntryListByTransactionHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	entries, err := a.debts.GetEntriesByTransactionId(c, uid, entryListReq.TransactionId)

	if err != nil {
		log.Errorf(c, "[debts.EntryListByTransactionHandler] failed to get debt entries of transaction \"id:%d\" for user \"uid:%d\", because %s", entryListReq.TransactionId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	entryResps := make(models.DebtEntryInfoResponseSlice, len(entries))

	for i := 0; i < len(entries); i++ {
		entryResps[i] = entries[i].ToDebtEntryInfoResponse()
	}

	sort.Sort(entryResps)

	return entryResps, nil
}

// EntryExportHandler returns what one person still owes as a spreadsheet, so that the person can be
// handed a receipt for it.
//
// It is not gated by the data export setting, which governs handing out the ledger itself. This is
// one person's own bill - the same rows the debts page already shows, addressed to the person who
// has to pay them - and a user who may read it on screen may carry it out of the room.
func (a *DebtsApi) EntryExportHandler(c *core.WebContext) ([]byte, string, *errs.Error) {
	var exportReq models.DebtEntryExportRequest
	err := c.ShouldBindQuery(&exportReq)

	if err != nil {
		log.Warnf(c, "[debts.EntryExportHandler] parse request failed, because %s", err.Error())
		return nil, "", errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	clientTimezone, err := c.GetClientTimezone()

	if err != nil {
		log.Warnf(c, "[debts.EntryExportHandler] cannot get client timezone, because %s", err.Error())
		clientTimezone = time.Local
	}

	uid := c.GetCurrentUid()
	user, err := a.users.GetUserById(c, uid)

	if err != nil {
		if !errs.IsCustomError(err) {
			log.Warnf(c, "[debts.EntryExportHandler] failed to get user for user \"uid:%d\", because %s", uid, err.Error())
		}

		return nil, "", errs.ErrUserNotFound
	}

	person, err := a.debts.GetPersonByPersonId(c, uid, exportReq.PersonId)

	if err != nil {
		log.Errorf(c, "[debts.EntryExportHandler] failed to get person \"id:%d\" for user \"uid:%d\", because %s", exportReq.PersonId, uid, err.Error())
		return nil, "", errs.Or(err, errs.ErrOperationFailed)
	}

	entries, err := a.debts.GetEntriesByPersonId(c, uid, exportReq.PersonId, false)

	if err != nil {
		log.Errorf(c, "[debts.EntryExportHandler] failed to get debt entries of person \"id:%d\" for user \"uid:%d\", because %s", exportReq.PersonId, uid, err.Error())
		return nil, "", errs.Or(err, errs.ErrOperationFailed)
	}

	if len(entries) < 1 {
		return nil, "", errs.ErrDebtPersonOwesNothing
	}

	entryResps, err := a.getDescribedEntryResponses(c, uid, entries)

	if err != nil {
		log.Errorf(c, "[debts.EntryExportHandler] failed to describe debt entries of person \"id:%d\" for user \"uid:%d\", because %s", exportReq.PersonId, uid, err.Error())
		return nil, "", errs.Or(err, errs.ErrOperationFailed)
	}

	categoryNames, err := a.getCategoryNames(c, uid)

	if err != nil {
		log.Errorf(c, "[debts.EntryExportHandler] failed to get transaction categories for user \"uid:%d\", because %s", uid, err.Error())
		return nil, "", errs.ErrOperationFailed
	}

	content, err := debtexporter.WriteDebtReceiptXlsx(entryResps, &debtexporter.DebtReceiptContext{
		PersonName:              person.Name,
		UserName:                user.Nickname,
		GeneratedTime:           time.Now(),
		Timezone:                clientTimezone,
		CategoryNames:           categoryNames,
		UnnamedReceiptTitle:     debtReceiptUnnamedReceiptTitle,
		UnnamedTransactionTitle: debtReceiptUnnamedTransactionTitle,
	})

	if err != nil {
		log.Errorf(c, "[debts.EntryExportHandler] failed to write the receipt of person \"id:%d\" for user \"uid:%d\", because %s", exportReq.PersonId, uid, err.Error())
		return nil, "", errs.ErrOperationFailed
	}

	return content, a.getReceiptFileName(person.Name, clientTimezone), nil
}

// getCategoryNames names every category of the user, so that a transaction owed whole can be called
// what it is called everywhere else
func (a *DebtsApi) getCategoryNames(c *core.WebContext, uid int64) (map[int64]string, error) {
	categories, err := a.categories.GetAllCategoriesByUid(c, uid, 0, -1)

	if err != nil {
		return nil, err
	}

	categoryNames := make(map[int64]string, len(categories))

	for i := 0; i < len(categories); i++ {
		categoryNames[categories[i].CategoryId] = categories[i].Name
	}

	return categoryNames, nil
}

// getReceiptFileName names the downloaded file after the person and the day it was drawn up.
//
// The person's name is written by the user and the file name is written into a response header, so
// everything that is not plainly a letter or a digit is dropped rather than escaped. A name of only
// such characters leaves the file called what it is, which is a receipt.
func (a *DebtsApi) getReceiptFileName(personName string, clientTimezone *time.Location) string {
	safeName := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}

		return -1
	}, personName)

	if safeName == "" {
		safeName = debtReceiptDefaultFileName
	}

	currentTime := utils.FormatUnixTimeToLongDate(time.Now().Unix(), clientTimezone)
	currentTime = strings.Replace(currentTime, "-", "_", -1)

	return fmt.Sprintf("%s_%s_%s.xlsx", debtReceiptDefaultFileName, safeName, currentTime)
}

// EntryCreateBatchHandler attaches transactions and receipt positions to a person for current user
func (a *DebtsApi) EntryCreateBatchHandler(c *core.WebContext) (any, *errs.Error) {
	var entryCreateReq models.DebtEntryCreateBatchRequest
	err := c.ShouldBindJSON(&entryCreateReq)

	if err != nil {
		log.Warnf(c, "[debts.EntryCreateBatchHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	if len(entryCreateReq.Entries) > models.MaximumDebtEntriesCountOfOneRequest {
		return nil, errs.ErrDebtEntryIdInvalid
	}

	uid := c.GetCurrentUid()

	// every entry is owed by somebody: the person the request names, or the one the entry names when
	// a thing is being shared out
	personIds := make([]int64, 0, len(entryCreateReq.Entries))
	entryPersonIds := make([]int64, len(entryCreateReq.Entries))

	for i := 0; i < len(entryCreateReq.Entries); i++ {
		personId := entryCreateReq.Entries[i].PersonId

		if personId <= 0 {
			personId = entryCreateReq.PersonId
		}

		if personId <= 0 {
			log.Warnf(c, "[debts.EntryCreateBatchHandler] entry %d names no person for user \"uid:%d\"", i, uid)
			return nil, errs.ErrDebtPersonIdInvalid
		}

		entryPersonIds[i] = personId
		personIds = append(personIds, personId)
	}

	personMap, err := a.debts.GetPersonsByPersonIds(c, uid, personIds)

	if err != nil {
		log.Errorf(c, "[debts.EntryCreateBatchHandler] failed to get people for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	for i := 0; i < len(entryPersonIds); i++ {
		if _, exists := personMap[entryPersonIds[i]]; !exists {
			log.Warnf(c, "[debts.EntryCreateBatchHandler] the person \"id:%d\" does not exist for user \"uid:%d\"", entryPersonIds[i], uid)
			return nil, errs.ErrDebtPersonNotFound
		}
	}

	transactionIds := make([]int64, 0, len(entryCreateReq.Entries))
	lineItemIds := make([]int64, 0, len(entryCreateReq.Entries))

	for i := 0; i < len(entryCreateReq.Entries); i++ {
		transactionIds = append(transactionIds, entryCreateReq.Entries[i].TransactionId)

		if entryCreateReq.Entries[i].LineItemId > 0 {
			lineItemIds = append(lineItemIds, entryCreateReq.Entries[i].LineItemId)
		}
	}

	transactions, err := a.transactions.GetTransactionsByTransactionIds(c, uid, transactionIds)

	if err != nil {
		log.Errorf(c, "[debts.EntryCreateBatchHandler] failed to get transactions for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	transactionMap := a.transactions.GetTransactionMapByList(transactions)

	lineItemMap, err := a.transactions.GetReceiptLineItemsByLineItemIds(c, uid, lineItemIds)

	if err != nil {
		log.Errorf(c, "[debts.EntryCreateBatchHandler] failed to get receipt positions for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	accountIds := make([]int64, 0, len(transactions))

	for i := 0; i < len(transactions); i++ {
		accountIds = append(accountIds, transactions[i].AccountId)
	}

	accountMap, err := a.accounts.GetAccountsByAccountIds(c, uid, accountIds)

	if err != nil {
		log.Errorf(c, "[debts.EntryCreateBatchHandler] failed to get accounts for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	entries := make([]*models.DebtEntry, len(entryCreateReq.Entries))

	for i := 0; i < len(entryCreateReq.Entries); i++ {
		entryReq := entryCreateReq.Entries[i]
		transaction, exists := transactionMap[entryReq.TransactionId]

		if !exists {
			log.Warnf(c, "[debts.EntryCreateBatchHandler] transaction \"id:%d\" does not exist for user \"uid:%d\"", entryReq.TransactionId, uid)
			return nil, errs.ErrTransactionNotFound
		}

		// The amount of what is attached is what is owed unless the user says otherwise, and it is
		// taken as a positive number - an expense is stored as one and a debt is not a negative debt.
		amount := transaction.Amount

		if entryReq.LineItemId > 0 {
			lineItem, exists := lineItemMap[entryReq.LineItemId]

			if !exists || lineItem.TransactionId != transaction.TransactionId {
				log.Warnf(c, "[debts.EntryCreateBatchHandler] receipt position \"id:%d\" does not belong to transaction \"id:%d\" for user \"uid:%d\"", entryReq.LineItemId, entryReq.TransactionId, uid)
				return nil, errs.ErrDebtEntryLineItemNotFound
			}

			amount = lineItem.Amount
		}

		if entryReq.Amount > 0 {
			amount = entryReq.Amount
		} else if amount < 0 {
			amount = -amount
		}

		if amount <= 0 {
			log.Warnf(c, "[debts.EntryCreateBatchHandler] cannot attach an amount of %d for user \"uid:%d\"", amount, uid)
			return nil, errs.ErrDebtEntryAmountInvalid
		}

		currency := ""

		if account, exists := accountMap[transaction.AccountId]; exists {
			currency = account.Currency
		}

		entries[i] = &models.DebtEntry{
			PersonId:        entryPersonIds[i],
			TransactionId:   transaction.TransactionId,
			LineItemId:      entryReq.LineItemId,
			Amount:          amount,
			Currency:        currency,
			TransactionTime: transaction.TransactionTime,
		}
	}

	err = a.debts.CreateEntries(c, uid, entries)

	if err != nil {
		log.Errorf(c, "[debts.EntryCreateBatchHandler] failed to create debt entries for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[debts.EntryCreateBatchHandler] user \"uid:%d\" has attached %d entries to %d people", uid, len(entries), len(personMap))

	entryResps, err := a.getDescribedEntryResponses(c, uid, entries)

	if err != nil {
		log.Errorf(c, "[debts.EntryCreateBatchHandler] failed to describe new debt entries for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	sort.Sort(entryResps)

	return entryResps, nil
}

// EntryCreateManualHandler records a debt that has no transaction behind it for current user
func (a *DebtsApi) EntryCreateManualHandler(c *core.WebContext) (any, *errs.Error) {
	var entryCreateReq models.DebtEntryCreateManualRequest
	err := c.ShouldBindJSON(&entryCreateReq)

	if err != nil {
		log.Warnf(c, "[debts.EntryCreateManualHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	_, err = a.debts.GetPersonByPersonId(c, uid, entryCreateReq.PersonId)

	if err != nil {
		log.Errorf(c, "[debts.EntryCreateManualHandler] failed to get person \"id:%d\" for user \"uid:%d\", because %s", entryCreateReq.PersonId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	entry := &models.DebtEntry{
		Uid:         uid,
		PersonId:    entryCreateReq.PersonId,
		Description: entryCreateReq.Description,
		Amount:      entryCreateReq.Amount,
		Currency:    entryCreateReq.Currency,
		// the time is kept the way every other time in this ledger is, so that a debt entered by
		// hand sorts among the ones that came from transactions instead of beside them
		TransactionTime: utils.GetMinTransactionTimeFromUnixTime(entryCreateReq.Time),
	}

	err = a.debts.CreateManualEntry(c, entry)

	if err != nil {
		log.Errorf(c, "[debts.EntryCreateManualHandler] failed to create manual debt entry for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[debts.EntryCreateManualHandler] user \"uid:%d\" has recorded debt entry \"id:%d\" by hand for person \"id:%d\"", uid, entry.EntryId, entryCreateReq.PersonId)

	return entry.ToDebtEntryInfoResponse(), nil
}

// EntryModifyHandler changes what is owed of one entry for current user
func (a *DebtsApi) EntryModifyHandler(c *core.WebContext) (any, *errs.Error) {
	var entryModifyReq models.DebtEntryModifyRequest
	err := c.ShouldBindJSON(&entryModifyReq)

	if err != nil {
		log.Warnf(c, "[debts.EntryModifyHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.debts.ModifyEntry(c, uid, entryModifyReq.Id, entryModifyReq.Amount, entryModifyReq.Description)

	if err != nil {
		log.Errorf(c, "[debts.EntryModifyHandler] failed to modify debt entry \"id:%d\" for user \"uid:%d\", because %s", entryModifyReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[debts.EntryModifyHandler] user \"uid:%d\" has modified debt entry \"id:%d\"", uid, entryModifyReq.Id)

	return true, nil
}

// EntryDeleteHandler detaches entries from the person they were attached to for current user
func (a *DebtsApi) EntryDeleteHandler(c *core.WebContext) (any, *errs.Error) {
	var entryDeleteReq models.DebtEntryDeleteRequest
	err := c.ShouldBindJSON(&entryDeleteReq)

	if err != nil {
		log.Warnf(c, "[debts.EntryDeleteHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	entryIds, errParse := a.parseEntryIds(c, entryDeleteReq.Ids)

	if errParse != nil {
		return nil, errParse
	}

	err = a.debts.DeleteEntries(c, uid, entryIds)

	if err != nil {
		log.Errorf(c, "[debts.EntryDeleteHandler] failed to delete debt entries for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[debts.EntryDeleteHandler] user \"uid:%d\" has deleted %d debt entries", uid, len(entryIds))

	return true, nil
}

// EntrySettleHandler marks entries as paid back by a transaction for current user
func (a *DebtsApi) EntrySettleHandler(c *core.WebContext) (any, *errs.Error) {
	var entrySettleReq models.DebtEntrySettleRequest
	err := c.ShouldBindJSON(&entrySettleReq)

	if err != nil {
		log.Warnf(c, "[debts.EntrySettleHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	entryIds, errParse := a.parseEntryIds(c, entrySettleReq.Ids)

	if errParse != nil {
		return nil, errParse
	}

	// The repayment must be a transaction this user actually has, because a settled entry points at
	// it forever afterwards as the answer to when the money came back
	_, err = a.transactions.GetTransactionByTransactionId(c, uid, entrySettleReq.SettlementTransactionId)

	if err != nil {
		log.Errorf(c, "[debts.EntrySettleHandler] failed to get settlement transaction \"id:%d\" for user \"uid:%d\", because %s", entrySettleReq.SettlementTransactionId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	err = a.debts.SettleEntries(c, uid, entryIds, entrySettleReq.SettlementTransactionId)

	if err != nil {
		log.Errorf(c, "[debts.EntrySettleHandler] failed to settle debt entries for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[debts.EntrySettleHandler] user \"uid:%d\" has settled %d debt entries with transaction \"id:%d\"", uid, len(entryIds), entrySettleReq.SettlementTransactionId)

	return true, nil
}

// EntryReopenHandler puts settled entries back on the bill for current user
func (a *DebtsApi) EntryReopenHandler(c *core.WebContext) (any, *errs.Error) {
	var entryReopenReq models.DebtEntryReopenRequest
	err := c.ShouldBindJSON(&entryReopenReq)

	if err != nil {
		log.Warnf(c, "[debts.EntryReopenHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	entryIds, errParse := a.parseEntryIds(c, entryReopenReq.Ids)

	if errParse != nil {
		return nil, errParse
	}

	err = a.debts.ReopenEntries(c, uid, entryIds)

	if err != nil {
		log.Errorf(c, "[debts.EntryReopenHandler] failed to reopen debt entries for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[debts.EntryReopenHandler] user \"uid:%d\" has reopened %d debt entries", uid, len(entryIds))

	return true, nil
}

// parseEntryIds turns the textual ids of a request into entry ids
func (a *DebtsApi) parseEntryIds(c *core.WebContext, textualIds []string) ([]int64, *errs.Error) {
	if len(textualIds) > models.MaximumDebtEntriesCountOfOneRequest {
		return nil, errs.ErrDebtEntryIdInvalid
	}

	entryIds, err := utils.StringArrayToInt64Array(textualIds)

	if err != nil {
		log.Warnf(c, "[debts.parseEntryIds] parse debt entry ids failed, because %s", err.Error())
		return nil, errs.ErrDebtEntryIdInvalid
	}

	return entryIds, nil
}

// getOpenAmountsByPerson returns what each person still owes, per currency, and how many things they
// owe it for
func (a *DebtsApi) getOpenAmountsByPerson(entries []*models.DebtEntry) (map[int64][]*models.DebtAmount, map[int64]int32) {
	amountsByPerson := make(map[int64][]*models.DebtAmount)
	countsByPerson := make(map[int64]int32)

	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		countsByPerson[entry.PersonId]++

		amounts := amountsByPerson[entry.PersonId]
		found := false

		for j := 0; j < len(amounts); j++ {
			if amounts[j].Currency == entry.Currency {
				amounts[j].Amount += entry.Amount
				found = true
				break
			}
		}

		if !found {
			amounts = append(amounts, &models.DebtAmount{
				Currency: entry.Currency,
				Amount:   entry.Amount,
			})
		}

		amountsByPerson[entry.PersonId] = amounts
	}

	return amountsByPerson, countsByPerson
}

// getDescribedEntryResponses returns the view-objects of the given entries, each carrying what the
// transaction and the receipt position it points at say about it.
//
// An entry whose transaction has since left the ledger is answered as missing rather than dropped:
// it is still owed, and hiding it would quietly change the total.
func (a *DebtsApi) getDescribedEntryResponses(c *core.WebContext, uid int64, entries []*models.DebtEntry) (models.DebtEntryInfoResponseSlice, error) {
	entryResps := make(models.DebtEntryInfoResponseSlice, len(entries))

	for i := 0; i < len(entries); i++ {
		entryResps[i] = entries[i].ToDebtEntryInfoResponse()
	}

	if len(entries) < 1 {
		return entryResps, nil
	}

	transactionIds := make([]int64, 0, len(entries))
	lineItemIds := make([]int64, 0, len(entries))

	for i := 0; i < len(entries); i++ {
		if entries[i].TransactionId > 0 {
			transactionIds = append(transactionIds, entries[i].TransactionId)
		}

		if entries[i].LineItemId > 0 {
			lineItemIds = append(lineItemIds, entries[i].LineItemId)
		}
	}

	transactionMap := make(map[int64]*models.Transaction)
	var transactions []*models.Transaction

	// entries entered by hand contribute no transaction ids, so a person who has only those is
	// answered without asking the ledger anything
	if len(transactionIds) > 0 {
		var err error
		transactions, err = a.transactions.GetTransactionsByTransactionIds(c, uid, transactionIds)

		if err != nil {
			return nil, err
		}

		transactionMap = a.transactions.GetTransactionMapByList(transactions)
	}

	lineItemMap, err := a.transactions.GetReceiptLineItemsByLineItemIds(c, uid, lineItemIds)

	if err != nil {
		return nil, err
	}

	receiptIds := make([]int64, 0, len(transactions))

	for i := 0; i < len(transactions); i++ {
		if transactions[i].ReceiptId > 0 {
			receiptIds = append(receiptIds, transactions[i].ReceiptId)
		}
	}

	receiptMap, err := a.transactions.GetReceiptsByReceiptIds(c, uid, receiptIds)

	if err != nil {
		return nil, err
	}

	for i := 0; i < len(entries); i++ {
		entryResp := entryResps[i]

		// a debt entered by hand describes itself and has no transaction to be missing
		if entries[i].TransactionId <= 0 {
			continue
		}

		transaction, exists := transactionMap[entries[i].TransactionId]

		if !exists {
			entryResp.Missing = true
			continue
		}

		entryResp.CategoryId = transaction.CategoryId
		entryResp.Comment = transaction.Comment

		entryResp.ReceiptId = transaction.ReceiptId

		if receipt, exists := receiptMap[transaction.ReceiptId]; exists {
			entryResp.MerchantName = receipt.MerchantName
		}

		if entries[i].LineItemId > 0 {
			if lineItem, exists := lineItemMap[entries[i].LineItemId]; exists {
				entryResp.Name = lineItem.Name
			} else {
				entryResp.Missing = true
			}
		}
	}

	return entryResps, nil
}
