package models

import (
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// MaximumDebtEntriesCountOfOneRequest is the largest number of debt entries one request may attach,
// settle or reopen at once. Attaching is done a receipt at a time and settling a visit at a time,
// so the limit is only what stops a malformed request from touching an unbounded number of rows.
const MaximumDebtEntriesCountOfOneRequest = 500

// DebtPerson is somebody who owes the user money.
//
// It is a name in this user's ledger and nothing else - not an account of this program, not somebody
// who can log in, not somebody the money is tracked for. The user is the only one who ever sees it,
// and the only reason it exists is so that the things bought for that person can be counted together.
type DebtPerson struct {
	PersonId        int64  `xorm:"PK"`
	Uid             int64  `xorm:"INDEX(IDX_debt_person_uid_deleted_order) NOT NULL"`
	Deleted         bool   `xorm:"INDEX(IDX_debt_person_uid_deleted_order) NOT NULL"`
	Name            string `xorm:"VARCHAR(64) NOT NULL"`
	DisplayOrder    int32  `xorm:"INDEX(IDX_debt_person_uid_deleted_order) NOT NULL"`
	CreatedUnixTime int64
	UpdatedUnixTime int64
	DeletedUnixTime int64
}

// DebtEntry is one thing a person owes: a whole transaction, one position of one, or a loan that
// never passed through the ledger at all.
//
// The money has already been spent and is already in the ledger - the expense was real and it stays
// an expense. What this row adds is that somebody else is meant to pay for it, which is a fact about
// a person and not about an account, and so is kept beside the transaction rather than inside it.
//
// Not every debt has a transaction to point at. Cash handed over, a bill settled from an account this
// program does not keep, something bought before the ledger existed - these are owed just as much,
// and an entry with no transaction is how they are recorded. Such an entry carries its own
// Description, because there is nothing else to name it by.
//
// Amount is stored rather than read back from the transaction it points at. Half of a shared taxi is
// owed and the whole fare is not, and a transaction that is later corrected must not silently change
// what somebody was told they owe.
type DebtEntry struct {
	EntryId  int64 `xorm:"PK"`
	Uid      int64 `xorm:"INDEX(IDX_debt_entry_uid_deleted_person) INDEX(IDX_debt_entry_uid_deleted_transaction) NOT NULL"`
	Deleted  bool  `xorm:"INDEX(IDX_debt_entry_uid_deleted_person) INDEX(IDX_debt_entry_uid_deleted_transaction) NOT NULL"`
	PersonId int64 `xorm:"INDEX(IDX_debt_entry_uid_deleted_person) NOT NULL"`
	// TransactionId is the transaction the money was spent by - a position is owed through the
	// transaction it was summed into - and is zero for a debt entered by hand
	TransactionId int64 `xorm:"INDEX(IDX_debt_entry_uid_deleted_transaction) NOT NULL"`
	// LineItemId is the receipt position that is owed, or zero when the whole transaction is
	LineItemId int64 `xorm:"NOT NULL"`
	// Description is what a debt entered by hand is called, and is empty for one that has a
	// transaction to be named by. It carries a default because it is added to a table that already
	// exists, and a NOT NULL column without one cannot be added to a populated table.
	Description string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	// Amount is what is owed, in minor units of Currency, always positive
	Amount int64 `xorm:"NOT NULL"`
	// Currency is the currency the money was spent in, taken from the account at the time of
	// attaching, so that what is owed is never quietly restated in another currency
	Currency        string `xorm:"VARCHAR(3) NOT NULL"`
	TransactionTime int64  `xorm:"NOT NULL"`
	// SettlementTransactionId is the transaction that paid this back, and zero while it is unpaid.
	// It is what makes a settled entry answerable: it says not only that the money came back but
	// which payment it came back in.
	SettlementTransactionId int64 `xorm:"NOT NULL"`
	SettledUnixTime         int64 `xorm:"NOT NULL"`
	CreatedUnixTime         int64
	UpdatedUnixTime         int64
	DeletedUnixTime         int64
}

// DebtPersonGetRequest represents all parameters of a request to get one person
type DebtPersonGetRequest struct {
	Id int64 `form:"id,string" binding:"required,min=1"`
}

// DebtPersonCreateRequest represents all parameters of a person creation request
type DebtPersonCreateRequest struct {
	Name string `json:"name" binding:"required,notBlank,max=64"`
}

// DebtPersonModifyRequest represents all parameters of a person modification request
type DebtPersonModifyRequest struct {
	Id   int64  `json:"id,string" binding:"required,min=1"`
	Name string `json:"name" binding:"required,notBlank,max=64"`
}

// DebtPersonDeleteRequest represents all parameters of a person deletion request
type DebtPersonDeleteRequest struct {
	Id int64 `json:"id,string" binding:"required,min=1"`
}

// DebtEntryListRequest represents all parameters of a request to list what one person owes
type DebtEntryListRequest struct {
	PersonId int64 `form:"personId,string" binding:"required,min=1"`
	// IncludeSettled asks for what has already been paid back as well as what is still open
	IncludeSettled bool `form:"includeSettled"`
}

// DebtEntryListByTransactionRequest represents all parameters of a request to list what is owed of
// one transaction, which is what a transaction being looked at needs to know to show its positions
type DebtEntryListByTransactionRequest struct {
	TransactionId int64 `form:"transactionId,string" binding:"required,min=1"`
}

// DebtEntryExportRequest represents all parameters of a request for the receipt of what one person
// still owes.
//
// It names only the person, because a receipt is always for everything still open. What has been
// paid back is deliberately not offered: the sheet is handed to somebody as a bill, and a paid row
// on a bill is an invitation to pay it twice.
type DebtEntryExportRequest struct {
	PersonId int64 `form:"personId,string" binding:"required,min=1"`
}

// DebtEntryCreateRequest represents one thing to be attached to a person
type DebtEntryCreateRequest struct {
	// PersonId is who owes this one. It is absent when the whole request is owed by the single
	// person named at the top of it, and set when one thing is shared out among several people -
	// a dish everybody ate is one entry per eater, each for that eater's share.
	PersonId      int64 `json:"personId,string"`
	TransactionId int64 `json:"transactionId,string" binding:"required,min=1"`
	// LineItemId is the position that is owed, or absent when the whole transaction is
	LineItemId int64 `json:"lineItemId,string"`
	// Amount is what is owed, in minor units, or absent to owe the full amount of what is attached
	Amount int64 `json:"amount" binding:"min=0,max=999999999999999"`
}

// DebtEntryCreateBatchRequest represents all parameters of a request to attach things to a person.
//
// A batch, because a receipt is read as a whole - the positions somebody is to pay for are ticked
// off in one sitting, and attaching them one request at a time would let half of them land. A split
// is the same thing seen from the other side: the shares of one position are only meaningful
// together, so they are written together or not at all.
type DebtEntryCreateBatchRequest struct {
	// PersonId is who owes everything in this batch, and may be left out when every entry names its
	// own person, which is what a split does
	PersonId int64                     `json:"personId,string"`
	Entries  []*DebtEntryCreateRequest `json:"entries" binding:"required,min=1,max=500,dive"`
}

// DebtEntryCreateManualRequest represents all parameters of a request to record a debt by hand.
//
// It names what is owed, what it comes to and when it happened, because there is no transaction here
// to be asked any of that.
type DebtEntryCreateManualRequest struct {
	PersonId    int64  `json:"personId,string" binding:"required,min=1"`
	Description string `json:"description" binding:"required,notBlank,max=255"`
	Amount      int64  `json:"amount" binding:"required,min=1,max=999999999999999"`
	Currency    string `json:"currency" binding:"required,len=3,validCurrency"`
	Time        int64  `json:"time" binding:"required,min=1"`
}

// DebtEntryModifyRequest represents all parameters of a request to change what is owed of one entry
type DebtEntryModifyRequest struct {
	Id     int64 `json:"id,string" binding:"required,min=1"`
	Amount int64 `json:"amount" binding:"required,min=1,max=999999999999999"`
	// Description renames a debt entered by hand. It is absent when only the amount is being
	// changed, and is refused for an entry that has a transaction to be named by.
	Description string `json:"description" binding:"omitempty,notBlank,max=255"`
}

// DebtEntryDeleteRequest represents all parameters of a request to detach entries from a person
type DebtEntryDeleteRequest struct {
	Ids []string `json:"ids" binding:"required,min=1,max=500"`
}

// DebtEntrySettleRequest represents all parameters of a request to mark entries paid back.
//
// The transaction is created first, through the ordinary transaction api, and named here afterwards.
// Money coming back is an income like any other and has nothing about it that this page should be
// allowed to record differently.
type DebtEntrySettleRequest struct {
	Ids                     []string `json:"ids" binding:"required,min=1,max=500"`
	SettlementTransactionId int64    `json:"settlementTransactionId,string" binding:"required,min=1"`
}

// DebtEntryReopenRequest represents all parameters of a request to put settled entries back on the bill
type DebtEntryReopenRequest struct {
	Ids []string `json:"ids" binding:"required,min=1,max=500"`
}

// DebtAmount is what is owed in one currency. Debts are not added across currencies, because a sum
// of two currencies is a number that is true of neither of them.
type DebtAmount struct {
	Currency string `json:"currency"`
	Amount   int64  `json:"amount"`
}

// DebtPersonInfoResponse represents a view-object of a person and what they currently owe
type DebtPersonInfoResponse struct {
	Id           int64  `json:"id,string"`
	Name         string `json:"name"`
	DisplayOrder int32  `json:"displayOrder"`
	// OpenAmounts is what is still owed, one entry per currency, and is empty when nothing is
	OpenAmounts []*DebtAmount `json:"openAmounts,omitempty"`
	OpenCount   int32         `json:"openCount"`
}

// DebtEntryInfoResponse represents a view-object of one thing a person owes.
//
// It carries what the transaction it points at says about it, because a list of debts that shows
// only amounts and dates is a list nobody can check against their memory of what was bought.
type DebtEntryInfoResponse struct {
	Id                      int64  `json:"id,string"`
	PersonId                int64  `json:"personId,string"`
	TransactionId           int64  `json:"transactionId,string"`
	LineItemId              int64  `json:"lineItemId,string,omitempty"`
	Amount                  int64  `json:"amount"`
	Currency                string `json:"currency"`
	TransactionTime         int64  `json:"time"`
	Settled                 bool   `json:"settled,omitempty"`
	SettlementTransactionId int64  `json:"settlementTransactionId,string,omitempty"`
	SettledTime             int64  `json:"settledTime,omitempty"`
	// Manual says this debt was entered by hand and has no transaction behind it
	Manual bool `json:"manual,omitempty"`
	// Name is what the position is called on the receipt, or what a debt entered by hand was called,
	// and is empty when the whole transaction is owed rather than one of its positions
	Name string `json:"name,omitempty"`
	// CategoryId and Comment are the transaction's own, so that a whole transaction can be named
	// on the debts page the same way it is named everywhere else
	CategoryId int64  `json:"categoryId,string,omitempty"`
	Comment    string `json:"comment,omitempty"`
	// ReceiptId is the shopping trip the transaction belongs to, when it came from one. It is what
	// lets the things owed off one receipt be shown together rather than as a run of unrelated rows.
	ReceiptId int64 `json:"receiptId,string,omitempty"`
	// MerchantName is the shop of the receipt the transaction was imported from, when it had one
	MerchantName string `json:"merchantName,omitempty"`
	// Missing says the transaction this points at is no longer in the ledger. The entry is still
	// shown and still counted, because the money was still spent - it just can no longer say on
	// what.
	Missing bool `json:"missing,omitempty"`
}

// ToDebtPersonInfoResponse returns a view-object according to database model
func (p *DebtPerson) ToDebtPersonInfoResponse(openAmounts []*DebtAmount, openCount int32) *DebtPersonInfoResponse {
	return &DebtPersonInfoResponse{
		Id:           p.PersonId,
		Name:         p.Name,
		DisplayOrder: p.DisplayOrder,
		OpenAmounts:  openAmounts,
		OpenCount:    openCount,
	}
}

// ToDebtEntryInfoResponse returns a view-object according to database model
func (e *DebtEntry) ToDebtEntryInfoResponse() *DebtEntryInfoResponse {
	return &DebtEntryInfoResponse{
		Id:                      e.EntryId,
		PersonId:                e.PersonId,
		TransactionId:           e.TransactionId,
		LineItemId:              e.LineItemId,
		Amount:                  e.Amount,
		Currency:                e.Currency,
		TransactionTime:         utils.GetUnixTimeFromTransactionTime(e.TransactionTime),
		Settled:                 e.SettlementTransactionId > 0,
		SettlementTransactionId: e.SettlementTransactionId,
		SettledTime:             e.SettledUnixTime,
		Manual:                  e.TransactionId <= 0,
		Name:                    e.Description,
	}
}

// DebtPersonInfoResponseSlice represents the slice data structure of DebtPersonInfoResponse
type DebtPersonInfoResponseSlice []*DebtPersonInfoResponse

// Len returns the count of items
func (s DebtPersonInfoResponseSlice) Len() int {
	return len(s)
}

// Swap swaps two items
func (s DebtPersonInfoResponseSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less reports whether the first item is less than the second one
func (s DebtPersonInfoResponseSlice) Less(i, j int) bool {
	if s[i].DisplayOrder != s[j].DisplayOrder {
		return s[i].DisplayOrder < s[j].DisplayOrder
	}

	return s[i].Id < s[j].Id
}

// DebtEntryInfoResponseSlice represents the slice data structure of DebtEntryInfoResponse
type DebtEntryInfoResponseSlice []*DebtEntryInfoResponse

// Len returns the count of items
func (s DebtEntryInfoResponseSlice) Len() int {
	return len(s)
}

// Swap swaps two items
func (s DebtEntryInfoResponseSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less reports whether the first item is shown before the second one, which is newest first, the
// order the transaction list itself is read in
func (s DebtEntryInfoResponseSlice) Less(i, j int) bool {
	if s[i].TransactionTime != s[j].TransactionTime {
		return s[i].TransactionTime > s[j].TransactionTime
	}

	if s[i].TransactionId != s[j].TransactionId {
		return s[i].TransactionId > s[j].TransactionId
	}

	return s[i].Id < s[j].Id
}
