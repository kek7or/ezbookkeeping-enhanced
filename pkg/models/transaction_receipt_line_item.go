package models

// TransactionReceiptLineItem is one line of a receipt, kept next to the transaction its price was
// summed into.
//
// The transaction only records the total of a category, which is the number the ledger needs and the
// one number a receipt does not print. These rows are what makes that total answerable: they say which
// articles it is made of and what each of them cost, in the order the till printed them.
//
// They are written when a receipt is imported and by hand afterwards, so any transaction can be
// itemized: a shop that prints no receipt worth photographing, or one bought before the ledger
// existed, is answerable in exactly the same way as one that was read off a till.
type TransactionReceiptLineItem struct {
	LineItemId      int64  `xorm:"PK"`
	Uid             int64  `xorm:"INDEX(IDX_transaction_receipt_line_item_uid_deleted_transaction_id) NOT NULL"`
	Deleted         bool   `xorm:"INDEX(IDX_transaction_receipt_line_item_uid_deleted_transaction_id) NOT NULL"`
	TransactionId   int64  `xorm:"INDEX(IDX_transaction_receipt_line_item_uid_deleted_transaction_id) NOT NULL"`
	DisplayOrder    int32  `xorm:"NOT NULL"`
	Name            string `xorm:"VARCHAR(255) NOT NULL"`
	Amount          int64  `xorm:"NOT NULL"`
	CreatedUnixTime int64
	UpdatedUnixTime int64
	DeletedUnixTime int64
}

// TransactionReceiptLineItemRequest represents one receipt line submitted with an imported transaction
type TransactionReceiptLineItemRequest struct {
	Name   string `json:"name" binding:"required,notBlank,max=255"`
	Amount int64  `json:"amount" binding:"min=-999999999999999,max=999999999999999"`
}

// TransactionReceiptLineItemModifyItem is one position as the user has just written it.
//
// It carries an id when it is a position that already exists and no id when it is being added. That
// is what keeps a position somebody owes pointing at the same article while its neighbours are
// rewritten around it - an itemization rewritten wholesale would take every debt down with it.
type TransactionReceiptLineItemModifyItem struct {
	Id     int64  `json:"id,string"`
	Name   string `json:"name" binding:"required,notBlank,max=255"`
	Amount int64  `json:"amount" binding:"min=-999999999999999,max=999999999999999"`
}

// TransactionReceiptLineItemModifyRequest represents all parameters of a request to itemize a
// transaction into positions.
//
// It carries the whole itemization rather than one change to it, because positions are read as a
// list and their order is part of what they say. What the request leaves out is no longer a
// position of this transaction.
type TransactionReceiptLineItemModifyRequest struct {
	TransactionId int64                                   `json:"transactionId,string" binding:"required,min=1"`
	LineItems     []*TransactionReceiptLineItemModifyItem `json:"lineItems" binding:"omitempty,max=1000,dive"`
}

// TransactionReceiptLineItemResponse represents a view-object of one receipt line of a transaction
type TransactionReceiptLineItemResponse struct {
	// Id is what lets a single position be pointed at from elsewhere - it is how one article of a
	// shopping trip can be said to be owed by somebody
	Id     int64  `json:"id,string"`
	Name   string `json:"name"`
	Amount int64  `json:"amount"`
}

// ToTransactionReceiptLineItemResponse returns a view-object according to this receipt line
func (l *TransactionReceiptLineItem) ToTransactionReceiptLineItemResponse() *TransactionReceiptLineItemResponse {
	return &TransactionReceiptLineItemResponse{
		Id:     l.LineItemId,
		Name:   l.Name,
		Amount: l.Amount,
	}
}

// TransactionReceiptLineItemSlice represents the slice data structure of TransactionReceiptLineItem
type TransactionReceiptLineItemSlice []*TransactionReceiptLineItem

// Len returns the count of items
func (s TransactionReceiptLineItemSlice) Len() int {
	return len(s)
}

// Swap swaps two items
func (s TransactionReceiptLineItemSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less reports whether the first item is printed before the second one on the receipt
func (s TransactionReceiptLineItemSlice) Less(i, j int) bool {
	return s[i].DisplayOrder < s[j].DisplayOrder
}
