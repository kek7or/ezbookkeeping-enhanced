package models

import (
	"strings"
	"unicode"
)

// maxReceiptLineItemNameLength is the longest article name the memory keys on. A receipt prints
// nothing near this, and a longer name is truncated rather than rejected, because losing the
// memory of a line is worse than remembering a shortened version of it.
const maxReceiptLineItemNameLength = 255

// receiptLineItemNameFoldings are the letters folded onto a plainer one before a name is compared,
// so that a model that dropped the diaeresis off "Kartoffeln früh" is still understood to mean the
// same article. German receipts are where this matters most, but folding costs nothing elsewhere.
var receiptLineItemNameFoldings = map[rune]string{
	'ä': "a",
	'ö': "o",
	'ü': "u",
	'ß': "ss",
	'á': "a",
	'à': "a",
	'â': "a",
	'é': "e",
	'è': "e",
	'ê': "e",
	'í': "i",
	'ì': "i",
	'î': "i",
	'ó': "o",
	'ò': "o",
	'ô': "o",
	'ú': "u",
	'ù': "u",
	'û': "u",
	'ñ': "n",
	'ç': "c",
}

// ReceiptLineItemCategory is one remembered answer to the question the model keeps being asked anew:
// which category does this article belong to.
//
// The answer is the user's, taken from where the line ended up when they imported the receipt, and it
// is applied to later receipts before the model's own guess is used. That is what turns categorizing a
// weekly shop from a chore into something that only has to be done for articles never bought before.
type ReceiptLineItemCategory struct {
	Id  int64 `xorm:"PK"`
	Uid int64 `xorm:"UNIQUE(UQE_receipt_line_item_category_uid_normalized_name) INDEX(IDX_receipt_line_item_category_uid) NOT NULL"`
	// NormalizedName is what the lookup keys on: the printed name with its case, punctuation and
	// diacritics taken off, so that the dozen ways a till prints one article all find the same row.
	NormalizedName string `xorm:"UNIQUE(UQE_receipt_line_item_category_uid_normalized_name) VARCHAR(255) NOT NULL"`
	// Name is the article as it was last printed, kept only so that the entry can be shown to the user
	// as something they recognize rather than as its lookup key.
	Name       string `xorm:"VARCHAR(255) NOT NULL"`
	CategoryId int64  `xorm:"NOT NULL"`
	// TimesUsed counts the imports this answer has been given in, which is what tells an article bought
	// every week apart from one bought once by accident.
	TimesUsed       int32 `xorm:"NOT NULL"`
	CreatedUnixTime int64
	UpdatedUnixTime int64
}

// ReceiptLineItemCategoryRememberRequest represents all parameters of a request to remember which
// category the lines of an imported receipt belong to
type ReceiptLineItemCategoryRememberRequest struct {
	Items []*ReceiptLineItemCategoryRememberItem `json:"items" binding:"required,min=1,max=1000"`
}

// ReceiptLineItemCategoryRememberItem is one article and the category the user filed it under
type ReceiptLineItemCategoryRememberItem struct {
	Name       string `json:"name" binding:"required,notBlank,max=255"`
	CategoryId int64  `json:"categoryId,string" binding:"required,min=1"`
}

// NormalizeReceiptLineItemName reduces a printed article name to the key its category is remembered
// under.
//
// A till prints one article differently from one receipt to the next, and a model reading that print
// adds differences of its own, so the name is stripped down to the letters and digits that carry its
// identity: case, diacritics and punctuation are dropped and runs of whitespace collapsed. What is
// left is compared exactly first and only then by similarity, which keeps the common case - the same
// weekly article - off the fuzzy path entirely.
func NormalizeReceiptLineItemName(name string) string {
	var builder strings.Builder
	builder.Grow(len(name))

	pendingSeparator := false

	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if folded, exists := receiptLineItemNameFoldings[r]; exists {
			if pendingSeparator && builder.Len() > 0 {
				builder.WriteByte(' ')
			}

			pendingSeparator = false
			builder.WriteString(folded)
			continue
		}

		// a digit is part of what an article is called - the two batteries of a receipt are told apart
		// by their article number alone - so only what separates words is thrown away
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if pendingSeparator && builder.Len() > 0 {
				builder.WriteByte(' ')
			}

			pendingSeparator = false
			builder.WriteRune(r)
			continue
		}

		pendingSeparator = true
	}

	normalizedName := builder.String()

	if len(normalizedName) > maxReceiptLineItemNameLength {
		normalizedName = strings.TrimSpace(normalizedName[:maxReceiptLineItemNameLength])
	}

	return normalizedName
}
