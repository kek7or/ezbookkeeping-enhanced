package errs

import "net/http"

// Error codes related to debts
var (
	ErrDebtPersonIdInvalid          = NewNormalError(NormalSubcategoryDebt, 0, http.StatusBadRequest, "debt person id is invalid")
	ErrDebtPersonNotFound           = NewNormalError(NormalSubcategoryDebt, 1, http.StatusBadRequest, "debt person not found")
	ErrDebtPersonNameIsEmpty        = NewNormalError(NormalSubcategoryDebt, 2, http.StatusBadRequest, "debt person name is empty")
	ErrDebtPersonNameAlreadyExists  = NewNormalError(NormalSubcategoryDebt, 3, http.StatusBadRequest, "debt person name already exists")
	ErrDebtEntryIdInvalid           = NewNormalError(NormalSubcategoryDebt, 4, http.StatusBadRequest, "debt entry id is invalid")
	ErrDebtEntryNotFound            = NewNormalError(NormalSubcategoryDebt, 5, http.StatusBadRequest, "debt entry not found")
	ErrDebtEntryAlreadyExists       = NewNormalError(NormalSubcategoryDebt, 6, http.StatusBadRequest, "this has already been attached to this person")
	ErrDebtEntryAmountInvalid       = NewNormalError(NormalSubcategoryDebt, 7, http.StatusBadRequest, "debt entry amount is invalid")
	ErrDebtEntryLineItemNotFound    = NewNormalError(NormalSubcategoryDebt, 8, http.StatusBadRequest, "receipt position not found in this transaction")
	ErrDebtEntryAlreadySettled      = NewNormalError(NormalSubcategoryDebt, 9, http.StatusBadRequest, "debt entry has already been settled")
	ErrDebtEntryNotSettled          = NewNormalError(NormalSubcategoryDebt, 10, http.StatusBadRequest, "debt entry has not been settled")
	ErrDebtEntriesBelongToOnePerson = NewNormalError(NormalSubcategoryDebt, 11, http.StatusBadRequest, "debt entries must belong to one person")
	ErrDebtEntryIsNotManual         = NewNormalError(NormalSubcategoryDebt, 12, http.StatusBadRequest, "debt entry has a transaction and cannot be renamed")
	ErrDebtEntryDescriptionIsEmpty  = NewNormalError(NormalSubcategoryDebt, 13, http.StatusBadRequest, "debt entry description is empty")
)
