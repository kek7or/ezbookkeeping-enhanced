package converter

import (
	"sort"
	"strings"

	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// ReceiptLineItemNameSimilarityThreshold is how alike a receipt line has to be to a remembered one
// before it is taken to be the same article.
//
// At nine tenths a name survives about one misread character for every ten it has, which covers what
// a till and a model actually differ over - a dropped diaeresis, a plural, a stray digit of a weight -
// while keeping the neighbouring articles of a shelf apart: "Vollmilch" and "Vollmilch Bio" share far
// less than this, and so should be categorized on their own.
const ReceiptLineItemNameSimilarityThreshold = 0.9

// ReceiptLineItemCategoryMemory is what the user has already decided about the lines of their earlier
// receipts: which category each article belongs to.
//
// It is built once per import and handed to the importer, so that the categorizing work is done by
// what the user has already answered wherever possible, and by the model only for articles never
// bought before. Keeping it out of the importer as a prepared lookup is what lets the converters stay
// free of the database.
type ReceiptLineItemCategoryMemory struct {
	categoryNamesByNormalizedName map[string]string
	// normalizedNames is held sorted so that a fuzzy match with two equally close candidates always
	// resolves to the same one, whatever order the rows came out of the database in
	normalizedNames []string
	// digitsByNormalizedName is the digits of each remembered name in the order they are printed,
	// kept so that the fuzzy pass can insist on them without picking every name apart again
	digitsByNormalizedName map[string]string
}

// NewReceiptLineItemCategoryMemory returns the memory of the given article names and the categories
// they belong to, keyed by article name as it was printed
func NewReceiptLineItemCategoryMemory(categoryNamesByLineItemName map[string]string) *ReceiptLineItemCategoryMemory {
	categoryNamesByNormalizedName := make(map[string]string, len(categoryNamesByLineItemName))

	for lineItemName, categoryName := range categoryNamesByLineItemName {
		normalizedName := models.NormalizeReceiptLineItemName(lineItemName)

		if normalizedName == "" || categoryName == "" {
			continue
		}

		categoryNamesByNormalizedName[normalizedName] = categoryName
	}

	normalizedNames := make([]string, 0, len(categoryNamesByNormalizedName))
	digitsByNormalizedName := make(map[string]string, len(categoryNamesByNormalizedName))

	for normalizedName := range categoryNamesByNormalizedName {
		normalizedNames = append(normalizedNames, normalizedName)
		digitsByNormalizedName[normalizedName] = digitsOf(normalizedName)
	}

	sort.Strings(normalizedNames)

	return &ReceiptLineItemCategoryMemory{
		categoryNamesByNormalizedName: categoryNamesByNormalizedName,
		normalizedNames:               normalizedNames,
		digitsByNormalizedName:        digitsByNormalizedName,
	}
}

// Len returns how many articles are remembered
func (m *ReceiptLineItemCategoryMemory) Len() int {
	if m == nil {
		return 0
	}

	return len(m.categoryNamesByNormalizedName)
}

// FindCategoryName returns the category the given receipt line was filed under before, and whether it
// was recognized at all.
//
// The exact name is tried first, and only a line never seen in that form is compared against every
// remembered one, taking the closest match that is alike enough. The comparison is over normalized
// names, so the fuzzy pass is left to deal with genuine misreadings rather than with punctuation.
func (m *ReceiptLineItemCategoryMemory) FindCategoryName(lineItemName string) (string, bool) {
	if m == nil || len(m.categoryNamesByNormalizedName) < 1 {
		return "", false
	}

	normalizedName := models.NormalizeReceiptLineItemName(lineItemName)

	if normalizedName == "" {
		return "", false
	}

	if categoryName, exists := m.categoryNamesByNormalizedName[normalizedName]; exists {
		return categoryName, true
	}

	bestMatchName := ""
	bestSimilarity := ReceiptLineItemNameSimilarityThreshold
	digits := digitsOf(normalizedName)

	for _, rememberedName := range m.normalizedNames {
		// two names cannot be alike enough to matter when one is far longer than the other, and the
		// length of a string is known without comparing it, so most of the list is skipped this way
		if !canReachSimilarityThreshold(normalizedName, rememberedName) {
			continue
		}

		// a digit is never a near miss. "Pfand 0,25" and "Pfand 0,15" differ by a tenth of one
		// character and by everything that matters, and the longer the article number the more alike
		// two entirely different articles look, so a guess is only allowed where the numbers agree.
		if m.digitsByNormalizedName[rememberedName] != digits {
			continue
		}

		similarity := utils.StringSimilarity(normalizedName, rememberedName)

		if similarity >= bestSimilarity {
			bestSimilarity = similarity
			bestMatchName = rememberedName
		}
	}

	if bestMatchName == "" {
		return "", false
	}

	return m.categoryNamesByNormalizedName[bestMatchName], true
}

// digitsOf returns the digits of a normalized name in the order they are printed, which is what has
// to agree before two names may be taken for the same article on similarity alone
func digitsOf(normalizedName string) string {
	var builder strings.Builder

	for _, r := range normalizedName {
		if r >= '0' && r <= '9' {
			builder.WriteRune(r)
		}
	}

	return builder.String()
}

// canReachSimilarityThreshold reports whether two strings could possibly be alike enough, judging by
// their lengths alone. Turning one into the other costs at least the difference between them.
func canReachSimilarityThreshold(a string, b string) bool {
	aLength := len([]rune(a))
	bLength := len([]rune(b))

	longerLength := aLength
	lengthDifference := aLength - bLength

	if bLength > aLength {
		longerLength = bLength
		lengthDifference = bLength - aLength
	}

	return 1-float64(lengthDifference)/float64(longerLength) >= ReceiptLineItemNameSimilarityThreshold
}
