package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeReceiptLineItemName(t *testing.T) {
	expectedNames := map[string]string{
		"Broccoli":             "broccoli",
		"BROCCOLI":             "broccoli",
		"  Broccoli  ":         "broccoli",
		"Kartoffeln früh":      "kartoffeln fruh",
		"Kartoffeln  früh":     "kartoffeln fruh",
		"Frischer O-Saft o.F.": "frischer o saft o f",
		"Bio Möhren":           "bio mohren",
		"Weißbrot":             "weissbrot",
		"Crème fraîche":        "creme fraiche",
		// an article number is what tells two otherwise identical lines apart, so digits stay
		"Akku Ni-MH-0532273": "akku ni mh 0532273",
		"Pfand 0,25 EM":      "pfand 0 25 em",
	}

	for name, expectedName := range expectedNames {
		assert.Equal(t, expectedName, NormalizeReceiptLineItemName(name), "line item name %s", name)
	}
}

func TestNormalizeReceiptLineItemName_NothingToKeyOn(t *testing.T) {
	for _, name := range []string{"", "   ", "---", "*** ***"} {
		assert.Equal(t, "", NormalizeReceiptLineItemName(name), "line item name %s", name)
	}
}

func TestNormalizeReceiptLineItemName_DifferentPrintingsOfOneArticleAgree(t *testing.T) {
	expectedName := NormalizeReceiptLineItemName("Frischer O-Saft o.F.")

	for _, name := range []string{"FRISCHER O-SAFT O.F.", "Frischer O Saft o F", "  frischer o-saft o.f. "} {
		assert.Equal(t, expectedName, NormalizeReceiptLineItemName(name), "line item name %s", name)
	}
}
