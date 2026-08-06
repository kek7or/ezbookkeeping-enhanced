package utils

import (
	"unicode/utf8"
)

// windows1252ReverseMap maps the printable characters Windows-1252 assigns to
// bytes 0x80-0x9F back to those bytes.
//
// This matters more than it looks: "ß" is UTF-8 C3 9F, and 0x9F has no Latin-1
// character at all, so the corruption is almost always displayed and re-encoded
// through Windows-1252 — which renders it "Ÿ". Without this table, every German
// word containing "ß" or "€" would be left corrupted.
var windows1252ReverseMap = map[rune]byte{
	'€': 0x80, // €
	'‚': 0x82, // ‚
	'ƒ': 0x83, // ƒ
	'„': 0x84, // „
	'…': 0x85, // …
	'†': 0x86, // †
	'‡': 0x87, // ‡
	'ˆ': 0x88, // ˆ
	'‰': 0x89, // ‰
	'Š': 0x8A, // Š
	'‹': 0x8B, // ‹
	'Œ': 0x8C, // Œ
	'Ž': 0x8E, // Ž
	'‘': 0x91, // '
	'’': 0x92, // '
	'“': 0x93, // "
	'”': 0x94, // "
	'•': 0x95, // •
	'–': 0x96, // –
	'—': 0x97, // —
	'˜': 0x98, // ˜
	'™': 0x99, // ™
	'š': 0x9A, // š
	'›': 0x9B, // ›
	'œ': 0x9C, // œ
	'ž': 0x9E, // ž
	'Ÿ': 0x9F, // Ÿ
}

// RepairMojibake reverses the classic "UTF-8 read as Latin-1" corruption, where
// "Hähnchen" arrives as "HÃ¤hnchen" and "Nestlé" as "NestlÃ©".
//
// This exists because local vision models transcribing non-English receipts
// emit the corruption themselves: the model writes the two codepoints U+00C3
// U+00A4 where the receipt prints "ä". The bytes are transported faithfully by
// everything downstream, so there is nothing else to fix — the text is simply
// wrong by the time it is read, and has to be repaired after the fact.
//
// The transform is only applied when it is unambiguous, which makes it safe to
// run over text that was never corrupted:
//
//   - Any rune above U+00FF means the text carries real multi-byte content the
//     model got right, so it is left alone.
//   - The reinterpreted bytes must form valid UTF-8. Genuine Latin-1 text such
//     as "Café" fails this check, because a lone 0xE9 is not a valid sequence,
//     and is therefore returned untouched.
//   - The result must not introduce control characters, which is what
//     reinterpreting binary-ish text would produce.
//
// A string with no high runes at all is returned immediately, so ordinary
// ASCII pays almost nothing.
func RepairMojibake(text string) string {
	if text == "" {
		return text
	}

	hasHighRune := false

	for _, r := range text {
		if r >= 0x80 {
			hasHighRune = true
		}

		if r > 0xFF {
			if _, isWindows1252 := windows1252ReverseMap[r]; !isWindows1252 {
				// Real multi-byte content: this string is already correct.
				return text
			}
		}
	}

	if !hasHighRune {
		return text
	}

	raw := make([]byte, 0, len(text))

	for _, r := range text {
		if b, isWindows1252 := windows1252ReverseMap[r]; isWindows1252 {
			raw = append(raw, b)
			continue
		}

		raw = append(raw, byte(r))
	}

	if !utf8.Valid(raw) {
		return text
	}

	repaired := string(raw)

	if repaired == text {
		return text
	}

	// Reinterpreting text that was not mojibake can land on control characters.
	// Both ranges matter: C0 (U+0000-U+001F) and C1 (U+0080-U+009F), the latter
	// being what a stray "Â" decodes into. Anything that produces one is not a
	// receipt line, so keep the original.
	for _, r := range repaired {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return text
		}

		if r >= 0x80 && r <= 0x9F {
			return text
		}
	}

	return repaired
}

// RepairMojibakeSlice repairs every element of a slice in place
func RepairMojibakeSlice(texts []string) {
	for i := 0; i < len(texts); i++ {
		texts[i] = RepairMojibake(texts[i])
	}
}

// ContainsMojibake reports whether repairing the text would change it. Used for
// logging that the corruption happened, so a model that needs replacing does
// not silently hide behind the repair.
func ContainsMojibake(text string) bool {
	return RepairMojibake(text) != text
}

// RepairMojibakeInAllFields repairs every string in a set of fields, returning
// whether anything changed.
func RepairMojibakeInAllFields(fields ...*string) bool {
	changed := false

	for _, field := range fields {
		if field == nil {
			continue
		}

		repaired := RepairMojibake(*field)

		if repaired != *field {
			*field = repaired
			changed = true
		}
	}

	return changed
}
