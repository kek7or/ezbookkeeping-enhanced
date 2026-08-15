package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseReceiptTransactionComment(t *testing.T) {
	// what a receipt import writes: the names of the lines it summed, joined by ", "
	assert.Equal(t, []string{"Broccoli", "Kartoffeln früh", "Milchcreme Cookies"}, parseReceiptTransactionComment("Broccoli, Kartoffeln früh, Milchcreme Cookies"))

	// a category summed from a single line is still a list
	assert.Equal(t, []string{"Papiertragetasche"}, parseReceiptTransactionComment("Papiertragetasche"))
}

func TestParseReceiptTransactionComment_ACommaInsideANameIsNotASeparator(t *testing.T) {
	// German prints its decimals with a comma and no space after it, which is exactly why the names
	// are joined with ", " rather than ","
	assert.Equal(t, []string{"Pfand 0,25 EM", "Jasmin Reis 0,5kg"}, parseReceiptTransactionComment("Pfand 0,25 EM, Jasmin Reis 0,5kg"))
}

func TestParseReceiptTransactionComment_KeepsTheNamesBeforeATruncation(t *testing.T) {
	// the comment was cut on an article boundary, so every name before the ellipsis is whole and only
	// the articles beyond it were lost
	assert.Equal(t, []string{"Broccoli", "Kartoffeln früh"}, parseReceiptTransactionComment("Broccoli, Kartoffeln früh…"))
}

func TestParseReceiptTransactionComment_DropsAHalfWrittenName(t *testing.T) {
	// a single name cut short has no boundary to fall back on, and half an article name would be
	// remembered as an article of its own
	assert.Nil(t, parseReceiptTransactionComment(strings.Repeat("Kartoffeln", 30)+"…"))
}

func TestParseReceiptTransactionComment_LeavesOutWhatCanNeverBeMatchedAgain(t *testing.T) {
	// written before the mojibake repair, these names can never equal a correctly read one
	assert.Equal(t, []string{"Broccoli"}, parseReceiptTransactionComment("Broccoli, Kartoffeln frÃ¼h"))
	assert.Equal(t, []string{"Broccoli"}, parseReceiptTransactionComment("Broccoli, NestlÃ© Schokolade"))

	// a name that was always correct is kept, umlaut and all
	assert.Equal(t, []string{"Bio Möhren", "Nestlé Schokolade"}, parseReceiptTransactionComment("Bio Möhren, Nestlé Schokolade"))

	// nothing that carries an identity at all
	assert.Equal(t, []string{}, parseReceiptTransactionComment("---, ***"))
	assert.Equal(t, []string{}, parseReceiptTransactionComment(""))
}
