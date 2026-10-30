package errs

import "net/http"

// Error codes related to budget plans
var (
	ErrBudgetPlanItemIdInvalid            = NewNormalError(NormalSubcategoryBudgetPlan, 0, http.StatusBadRequest, "budget plan item id is invalid")
	ErrBudgetPlanItemNotFound             = NewNormalError(NormalSubcategoryBudgetPlan, 1, http.StatusBadRequest, "budget plan item not found")
	ErrBudgetPlanItemTypeInvalid          = NewNormalError(NormalSubcategoryBudgetPlan, 2, http.StatusBadRequest, "a planned item must be an income or an expense")
	ErrBudgetPlanMonthInvalid             = NewNormalError(NormalSubcategoryBudgetPlan, 3, http.StatusBadRequest, "budget plan month is invalid")
	ErrBudgetPlanNothingToCopy            = NewNormalError(NormalSubcategoryBudgetPlan, 4, http.StatusBadRequest, "there is nothing planned in that month to copy")
	ErrBudgetPlanCopyToSameMonth          = NewNormalError(NormalSubcategoryBudgetPlan, 5, http.StatusBadRequest, "cannot copy a month's plan into itself")
	ErrBudgetPlanAdjustmentNotSet         = NewNormalError(NormalSubcategoryBudgetPlan, 6, http.StatusBadRequest, "this schedule is not adjusted in this month")
	ErrBudgetPlanHasTooManyItems          = NewNormalError(NormalSubcategoryBudgetPlan, 7, http.StatusBadRequest, "this month already has too many planned items")
	ErrBudgetPlanExpectationAmountInvalid = NewNormalError(NormalSubcategoryBudgetPlan, 8, http.StatusBadRequest, "a category expectation cannot be negative")
	ErrBudgetPlanHasTooManyExpectations   = NewNormalError(NormalSubcategoryBudgetPlan, 9, http.StatusBadRequest, "this month already has too many category expectations")
	ErrBudgetPlanWishNotFound             = NewNormalError(NormalSubcategoryBudgetPlan, 10, http.StatusBadRequest, "wish not found")
	ErrBudgetPlanHasTooManyWishes         = NewNormalError(NormalSubcategoryBudgetPlan, 11, http.StatusBadRequest, "the wishlist already has too many things on it")
	ErrBudgetWishGroupNotFound            = NewNormalError(NormalSubcategoryBudgetPlan, 12, http.StatusBadRequest, "wishlist group not found")
	ErrBudgetWishGroupHasTooManyGroups    = NewNormalError(NormalSubcategoryBudgetPlan, 13, http.StatusBadRequest, "the wishlist already has too many groups")
	ErrBudgetPlanWishMonthInvalid         = NewNormalError(NormalSubcategoryBudgetPlan, 14, http.StatusBadRequest, "a wish needs both a year and a month, or neither")
	ErrBudgetPlanWishCategoryRequired     = NewNormalError(NormalSubcategoryBudgetPlan, 15, http.StatusBadRequest, "choose a category before planning this for a month")
	ErrBudgetPlanItemFromWishlist         = NewNormalError(NormalSubcategoryBudgetPlan, 16, http.StatusBadRequest, "this was planned from the wishlist; send it back to the wishlist and delete it there")
)
