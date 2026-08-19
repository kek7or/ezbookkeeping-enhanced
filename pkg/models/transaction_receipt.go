package models

// MaximumReceiptsCountOfImport is the largest number of receipts one import may carry. A batch is a
// stack of receipts photographed in one sitting, not an archive, and the limit is what stops a
// malformed request from writing an unbounded number of rows.
const MaximumReceiptsCountOfImport = 200

// TransactionReceipt is one shopping trip: the receipt an import read, and the identity every
// transaction it produced shares.
//
// A receipt image is imported as one transaction per category, which is what the ledger needs but
// not what the user did - they went to one shop, once, and paid one total. This row is what says so.
// It carries the two facts that belong to the whole receipt rather than to any one of its categories:
// where the shopping was done, and the total the till printed.
//
// It is written when a receipt is imported and never afterwards. A transaction created by hand, one
// imported from a spreadsheet, or one imported from a receipt before this existed simply belongs to
// no receipt, and is shown on its own exactly as before.
type TransactionReceipt struct {
	ReceiptId int64 `xorm:"PK"`
	Uid       int64 `xorm:"INDEX(IDX_transaction_receipt_uid_deleted) NOT NULL"`
	Deleted   bool  `xorm:"INDEX(IDX_transaction_receipt_uid_deleted) NOT NULL"`
	// MerchantName is the shop as printed on the receipt, empty when the model could not read one
	MerchantName string `xorm:"VARCHAR(255) NOT NULL"`
	// PrintedTotal is the total the till printed, in minor units. It is kept rather than recomputed
	// because it is the receipt's own claim about itself: once the transactions have been edited by
	// hand, the sum of them no longer answers what the paper said.
	PrintedTotal int64 `xorm:"NOT NULL"`
	// HasPrintedTotal tells a receipt whose total was never read from one that totalled zero, which
	// a fully discounted basket genuinely can
	HasPrintedTotal bool  `xorm:"NOT NULL"`
	TransactionTime int64 `xorm:"NOT NULL"`
	CreatedUnixTime int64
	UpdatedUnixTime int64
	DeletedUnixTime int64
}

// TransactionReceiptRequest represents one receipt submitted with an import
type TransactionReceiptRequest struct {
	MerchantName    string `json:"merchantName" binding:"max=255"`
	PrintedTotal    int64  `json:"printedTotal" binding:"min=-999999999999999,max=999999999999999"`
	HasPrintedTotal bool   `json:"hasPrintedTotal"`
}

// TransactionReceiptInfoResponse represents a view-object of the receipt a transaction was imported from
type TransactionReceiptInfoResponse struct {
	Id           int64  `json:"id,string"`
	MerchantName string `json:"merchantName,omitempty"`
	// PrintedTotal is in minor units, like every other amount in a transaction response, and is only
	// meaningful when HasPrintedTotal says the receipt actually stated one
	PrintedTotal    int64 `json:"printedTotal,omitempty"`
	HasPrintedTotal bool  `json:"hasPrintedTotal,omitempty"`
}

// ToTransactionReceiptInfoResponse returns a view-object according to this receipt
func (r *TransactionReceipt) ToTransactionReceiptInfoResponse() *TransactionReceiptInfoResponse {
	return &TransactionReceiptInfoResponse{
		Id:              r.ReceiptId,
		MerchantName:    r.MerchantName,
		PrintedTotal:    r.PrintedTotal,
		HasPrintedTotal: r.HasPrintedTotal,
	}
}

// TransactionReceiptBatch is the receipts a batch of imported transactions was read from, and which
// of them each transaction belongs to.
//
// The two are kept together because neither is usable alone: the receipts have no ids until they are
// written, and the transactions cannot be stamped with an id that does not exist yet. ReceiptIndexes
// maps an index into the transaction slice to an index into Receipts, and a transaction missing from
// it belongs to no receipt.
type TransactionReceiptBatch struct {
	Receipts       []*TransactionReceipt
	ReceiptIndexes map[int]int
}
