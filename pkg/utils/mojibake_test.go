package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepairMojibake_GermanReceiptLines(t *testing.T) {
	// Exactly what qwen3-vl returned for a real receipt: the model wrote the
	// codepoints U+00C3 U+00A4 where the receipt prints "ä".
	assert.Equal(t, "Hähnchen-Brustfilet", RepairMojibake("HÃ¤hnchen-Brustfilet"))
	assert.Equal(t, "Nestlé Minis Nuts", RepairMojibake("NestlÃ© Minis Nuts"))
	assert.Equal(t, "Müller Käse", RepairMojibake("MÃ¼ller KÃ¤se"))
}

func TestRepairMojibake_HandlesWindows1252HighBytes(t *testing.T) {
	// "ß" is UTF-8 C3 9F, and 0x9F is unassigned in Latin-1, so the corruption
	// surfaces through Windows-1252 as "Ÿ". Without the reverse table these
	// would be left corrupted.
	assert.Equal(t, "Große Brötchen", RepairMojibake("GroÃŸe BrÃ¶tchen"))
	assert.Equal(t, "Straße", RepairMojibake("StraÃŸe"))
	assert.Equal(t, "6,79 €", RepairMojibake("6,79 â‚¬"))
}

func TestRepairMojibake_LeavesCorrectTextAlone(t *testing.T) {
	// Already correct: these carry runes above U+00FF once decoded, and must
	// never be "repaired" a second time.
	assert.Equal(t, "Hähnchen-Brustfilet", RepairMojibake("Hähnchen-Brustfilet"))
	assert.Equal(t, "Nestlé Minis Nuts", RepairMojibake("Nestlé Minis Nuts"))
	assert.Equal(t, "Große Brötchen", RepairMojibake("Große Brötchen"))
}

func TestRepairMojibake_LeavesAsciiAlone(t *testing.T) {
	assert.Equal(t, "", RepairMojibake(""))
	assert.Equal(t, "Mozzarella", RepairMojibake("Mozzarella"))
	assert.Equal(t, "Kin.Sch-Bons+25g", RepairMojibake("Kin.Sch-Bons+25g"))
	assert.Equal(t, "Total 15.42", RepairMojibake("Total 15.42"))
}

func TestRepairMojibake_LeavesGenuineLatin1Alone(t *testing.T) {
	// A lone high rune is not a valid UTF-8 lead sequence once reinterpreted,
	// so text that merely contains an accent is returned untouched.
	assert.Equal(t, "Café", RepairMojibake("Café"))
	assert.Equal(t, "naïve", RepairMojibake("naïve"))
	assert.Equal(t, "Ölwechsel", RepairMojibake("Ölwechsel"))
}

func TestRepairMojibake_LeavesNonLatinScriptsAlone(t *testing.T) {
	assert.Equal(t, "Молоко", RepairMojibake("Молоко"))
	assert.Equal(t, "牛乳", RepairMojibake("牛乳"))
	assert.Equal(t, "€ 6,79", RepairMojibake("€ 6,79"))
}

func TestRepairMojibake_LeavesIncompleteSequencesAlone(t *testing.T) {
	// A lone lead byte is not valid UTF-8 once reinterpreted, so it is kept as
	// it is rather than turned into something else.
	loneLead := string(rune(0x00C3))
	assert.Equal(t, loneLead, RepairMojibake(loneLead))

	// A lead byte followed by a non-continuation character is equally invalid.
	leadThenAscii := string(rune(0x00C3)) + "Z"
	assert.Equal(t, leadThenAscii, RepairMojibake(leadThenAscii))
}

func TestRepairMojibake_RefusesToProduceControlCharacters(t *testing.T) {
	// C2 80 is valid UTF-8 but decodes to U+0080, a control character. Text
	// like that is not a receipt line, so the original is kept.
	controlProducing := string(rune(0x00C2)) + string(rune(0x0080))
	assert.Equal(t, controlProducing, RepairMojibake(controlProducing))
}

func TestContainsMojibake(t *testing.T) {
	assert.True(t, ContainsMojibake("HÃ¤hnchen-Brustfilet"))
	assert.False(t, ContainsMojibake("Hähnchen-Brustfilet"))
	assert.False(t, ContainsMojibake("Mozzarella"))
}

func TestRepairMojibakeSlice(t *testing.T) {
	lines := []string{"NestlÃ© Minis Nuts", "Mozzarella", "HÃ¤hnchen-Brustfilet"}
	RepairMojibakeSlice(lines)

	assert.Equal(t, []string{"Nestlé Minis Nuts", "Mozzarella", "Hähnchen-Brustfilet"}, lines)
}

func TestRepairMojibakeInAllFields(t *testing.T) {
	name := "HÃ¤hnchen-Brustfilet"
	category := "Food"
	unchanged := "Mozzarella"

	assert.True(t, RepairMojibakeInAllFields(&name, &category))
	assert.Equal(t, "Hähnchen-Brustfilet", name)
	assert.Equal(t, "Food", category)

	assert.False(t, RepairMojibakeInAllFields(&unchanged))
	assert.Equal(t, "Mozzarella", unchanged)

	assert.False(t, RepairMojibakeInAllFields(nil))
}
