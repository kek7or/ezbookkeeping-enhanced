package errs

import "net/http"

// Error codes related to budget plans
var (
	ErrBudgetPlanItemIdInvalid    = NewNormalError(NormalSubcategoryBudgetPlan, 0, http.StatusBadRequest, "budget plan item id is invalid")
	ErrBudgetPlanItemNotFound     = NewNormalError(NormalSubcategoryBudgetPlan, 1, http.StatusBadRequest, "budget plan item not found")
	ErrBudgetPlanItemTypeInvalid  = NewNormalError(NormalSubcategoryBudgetPlan, 2, http.StatusBadRequest, "a planned item must be an income or an expense")
	ErrBudgetPlanMonthInvalid     = NewNormalError(NormalSubcategoryBudgetPlan, 3, http.StatusBadRequest, "budget plan month is invalid")
	ErrBudgetPlanNothingToCopy    = NewNormalError(NormalSubcategoryBudgetPlan, 4, http.StatusBadRequest, "there is nothing planned in that month to copy")
	ErrBudgetPlanCopyToSameMonth  = NewNormalError(NormalSubcategoryBudgetPlan, 5, http.StatusBadRequest, "cannot copy a month's plan into itself")
	ErrBudgetPlanAdjustmentNotSet = NewNormalError(NormalSubcategoryBudgetPlan, 6, http.StatusBadRequest, "this schedule is not adjusted in this month")
	ErrBudgetPlanHasTooManyItems  = NewNormalError(NormalSubcategoryBudgetPlan, 7, http.StatusBadRequest, "this month already has too many planned items")
)
