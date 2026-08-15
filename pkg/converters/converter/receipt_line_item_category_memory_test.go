package converter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func createTestLineItemCategoryMemory() *ReceiptLineItemCategoryMemory {
	return NewReceiptLineItemCategoryMemory(map[string]string{
		"Milchcreme Cookies":   "Sweets",
		"Kartoffeln früh":      "Vegetables",
		"Frischer O-Saft o.F.": "Drink",
		"Akku Ni-MH-0532273":   "Electronics",
	})
}

func TestReceiptLineItemCategoryMemory_FindsWhatWasFiledBefore(t *testing.T) {
	memory := createTestLineItemCategoryMemory()

	categoryName, exists := memory.FindCategoryName("Milchcreme Cookies")

	assert.True(t, exists)
	assert.Equal(t, "Sweets", categoryName)
}

func TestReceiptLineItemCategoryMemory_FindsAnArticlePrintedDifferently(t *testing.T) {
	memory := createTestLineItemCategoryMemory()

	// case, punctuation and diacritics carry no identity, so none of these is a fuzzy match at all
	for _, name := range []string{"MILCHCREME COOKIES", "milchcreme  cookies", "Milchcreme-Cookies."} {
		categoryName, exists := memory.FindCategoryName(name)

		assert.True(t, exists, "line item name %s", name)
		assert.Equal(t, "Sweets", categoryName, "line item name %s", name)
	}
}

func TestReceiptLineItemCategoryMemory_FindsAnArticleMisreadByALetter(t *testing.T) {
	memory := createTestLineItemCategoryMemory()

	// one letter wrong in eighteen, which is what a misread actually looks like
	categoryName, exists := memory.FindCategoryName("Milchcreme Cookles")

	assert.True(t, exists)
	assert.Equal(t, "Sweets", categoryName)
}

func TestReceiptLineItemCategoryMemory_DoesNotFindADifferentArticle(t *testing.T) {
	memory := createTestLineItemCategoryMemory()

	for _, name := range []string{"Broccoli", "Milchcreme Kuchen", "Cookies", "Akku Ni-MH-0532274 Ladegerät"} {
		_, exists := memory.FindCategoryName(name)
		assert.False(t, exists, "line item name %s", name)
	}
}

func TestReceiptLineItemCategoryMemory_ADifferingDigitIsADifferentArticle(t *testing.T) {
	memory := NewReceiptLineItemCategoryMemory(map[string]string{
		"Pfand 0,25": "Drink",
	})

	// short names have no room to differ, which is what keeps the neighbouring shelf apart
	_, exists := memory.FindCategoryName("Pfand 0,15")

	assert.False(t, exists)
}

func TestReceiptLineItemCategoryMemory_TheClosestMatchWins(t *testing.T) {
	memory := NewReceiptLineItemCategoryMemory(map[string]string{
		"Vollmilch 3,5% 1L": "Drink",
		"Vollmilch 1,5% 1L": "Diet",
	})

	categoryName, exists := memory.FindCategoryName("Vollmilch 1,5% 1L")

	assert.True(t, exists)
	assert.Equal(t, "Diet", categoryName)
}

func TestReceiptLineItemCategoryMemory_RemembersNothingUseless(t *testing.T) {
	memory := NewReceiptLineItemCategoryMemory(map[string]string{
		"Broccoli": "",
		"***":      "Food",
		"":         "Food",
	})

	assert.Equal(t, 0, memory.Len())

	_, exists := memory.FindCategoryName("Broccoli")
	assert.False(t, exists)
}

func TestReceiptLineItemCategoryMemory_AnEmptyMemoryDecidesNothing(t *testing.T) {
	var uninitialized *ReceiptLineItemCategoryMemory

	assert.Equal(t, 0, uninitialized.Len())

	_, exists := uninitialized.FindCategoryName("Broccoli")
	assert.False(t, exists)

	empty := NewReceiptLineItemCategoryMemory(nil)

	assert.Equal(t, 0, empty.Len())

	_, exists = empty.FindCategoryName("Broccoli")
	assert.False(t, exists)
}

func TestReceiptLineItemCategoryMemory_ALineWithNoNameIsNotLookedUp(t *testing.T) {
	memory := createTestLineItemCategoryMemory()

	for _, name := range []string{"", "  ", "---"} {
		_, exists := memory.FindCategoryName(name)
		assert.False(t, exists, "line item name %s", name)
	}
}

func TestReceiptLineItemCategoryMemory_ADifferingArticleNumberIsADifferentArticle(t *testing.T) {
	memory := NewReceiptLineItemCategoryMemory(map[string]string{
		"Akku Ni-MH-0532273": "Electronics",
	})

	// alike enough by letters alone, and yet a different article off a different shelf
	_, exists := memory.FindCategoryName("Akku Ni-MH-0532274")

	assert.False(t, exists)

	// the same article with the print misread is still found, because its number is untouched
	categoryName, exists := memory.FindCategoryName("Akku Ni-NH-0532273")

	assert.True(t, exists)
	assert.Equal(t, "Electronics", categoryName)
}
