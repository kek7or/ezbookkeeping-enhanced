package api

import (
	"sort"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
)

// BudgetPlansApi represents the api of what a month is planned to cost
type BudgetPlansApi struct {
	budgetPlans *services.BudgetPlanService
	accounts    *services.AccountService
	categories  *services.TransactionCategoryService
}

// Initialize a budget plan api singleton instance
var (
	BudgetPlans = &BudgetPlansApi{
		budgetPlans: services.BudgetPlans,
		accounts:    services.Accounts,
		categories:  services.TransactionCategories,
	}
)

// BudgetPlanGetHandler returns everything stored about one month's plan for the current user.
//
// What the month is planned to cost is not computed here. It is the schedules, the ledger and these
// two lists put together, and it is put together wherever it is displayed - so that changing any
// one of the three shows up immediately without a round trip asking the server to add up again.
func (a *BudgetPlansApi) BudgetPlanGetHandler(c *core.WebContext) (any, *errs.Error) {
	var planGetReq models.BudgetPlanGetRequest
	err := c.ShouldBindQuery(&planGetReq)

	if err != nil {
		log.Warnf(c, "[budget_plans.BudgetPlanGetHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	items, err := a.budgetPlans.GetItemsByMonth(c, uid, planGetReq.Year, planGetReq.Month)

	if err != nil {
		log.Errorf(c, "[budget_plans.BudgetPlanGetHandler] failed to get plan items for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	adjustments, err := a.budgetPlans.GetAdjustmentsByMonth(c, uid, planGetReq.Year, planGetReq.Month)

	if err != nil {
		log.Errorf(c, "[budget_plans.BudgetPlanGetHandler] failed to get plan adjustments for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	itemResps := make(models.BudgetPlanItemInfoResponseSlice, len(items))

	for i := 0; i < len(items); i++ {
		itemResps[i] = items[i].ToBudgetPlanItemInfoResponse()
	}

	sort.Sort(itemResps)

	adjustmentResps := make([]*models.BudgetPlanAdjustmentInfoResponse, len(adjustments))

	for i := 0; i < len(adjustments); i++ {
		adjustmentResps[i] = adjustments[i].ToBudgetPlanAdjustmentInfoResponse()
	}

	expectations, err := a.budgetPlans.GetExpectationsByMonth(c, uid, planGetReq.Year, planGetReq.Month)

	if err != nil {
		log.Errorf(c, "[budget_plans.BudgetPlanGetHandler] failed to get category expectations for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	expectationResps := make([]*models.BudgetPlanExpectationInfoResponse, len(expectations))

	for i := 0; i < len(expectations); i++ {
		expectationResps[i] = expectations[i].ToBudgetPlanExpectationInfoResponse()
	}

	standing, err := a.budgetPlans.GetStandingExpectations(c, uid)

	if err != nil {
		log.Errorf(c, "[budget_plans.BudgetPlanGetHandler] failed to get standing expectations for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	standingResps := make([]*models.BudgetPlanStandingExpectationInfoResponse, len(standing))

	for i := 0; i < len(standing); i++ {
		standingResps[i] = standing[i].ToBudgetPlanStandingExpectationInfoResponse()
	}

	planned, err := a.budgetPlans.IsMonthPlanned(c, uid, planGetReq.Year, planGetReq.Month)

	if err != nil {
		log.Errorf(c, "[budget_plans.BudgetPlanGetHandler] failed to check whether the month was planned for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return &models.BudgetPlanInfoResponse{
		Year:         planGetReq.Year,
		Month:        planGetReq.Month,
		Planned:      planned,
		Items:        itemResps,
		Adjustments:  adjustmentResps,
		Expectations: expectationResps,
		Standing:     standingResps,
	}, nil
}

// BudgetPlanMonthStartHandler marks a month as planned for the current user without anything being
// planned in it yet. Planning anything marks the month on its own, so this is only for going back
// to a month that was never planned and filling it in now.
func (a *BudgetPlansApi) BudgetPlanMonthStartHandler(c *core.WebContext) (any, *errs.Error) {
	var monthStartReq models.BudgetPlanMonthStartRequest
	err := c.ShouldBindJSON(&monthStartReq)

	if err != nil {
		log.Warnf(c, "[budget_plans.BudgetPlanMonthStartHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.budgetPlans.StartMonth(c, uid, monthStartReq.Year, monthStartReq.Month)

	if err != nil {
		log.Errorf(c, "[budget_plans.BudgetPlanMonthStartHandler] failed to start plan month for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[budget_plans.BudgetPlanMonthStartHandler] user \"uid:%d\" has started planning %d-%d", uid, monthStartReq.Year, monthStartReq.Month)

	return true, nil
}

// BudgetPlanMonthStopHandler takes a month back out of the plan for the current user. Whatever is
// planned in the month is kept, and comes back if the month is started again.
func (a *BudgetPlansApi) BudgetPlanMonthStopHandler(c *core.WebContext) (any, *errs.Error) {
	var monthStopReq models.BudgetPlanMonthStopRequest
	err := c.ShouldBindJSON(&monthStopReq)

	if err != nil {
		log.Warnf(c, "[budget_plans.BudgetPlanMonthStopHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.budgetPlans.StopMonth(c, uid, monthStopReq.Year, monthStopReq.Month)

	if err != nil {
		log.Errorf(c, "[budget_plans.BudgetPlanMonthStopHandler] failed to stop plan month for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[budget_plans.BudgetPlanMonthStopHandler] user \"uid:%d\" has stopped planning %d-%d", uid, monthStopReq.Year, monthStopReq.Month)

	return true, nil
}

// BudgetPlanItemCreateHandler plans one more thing for a month for the current user
func (a *BudgetPlansApi) BudgetPlanItemCreateHandler(c *core.WebContext) (any, *errs.Error) {
	var itemCreateReq models.BudgetPlanItemCreateRequest
	err := c.ShouldBindJSON(&itemCreateReq)

	if err != nil {
		log.Warnf(c, "[budget_plans.BudgetPlanItemCreateHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	if itemCreateReq.Type != models.TRANSACTION_TYPE_INCOME && itemCreateReq.Type != models.TRANSACTION_TYPE_EXPENSE {
		return nil, errs.ErrBudgetPlanItemTypeInvalid
	}

	uid := c.GetCurrentUid()

	if err := a.verifyCategoryAndAccount(c, uid, itemCreateReq.CategoryId, itemCreateReq.AccountId); err != nil {
		return nil, err
	}

	item := &models.BudgetPlanItem{
		Uid:        uid,
		Year:       itemCreateReq.Year,
		Month:      itemCreateReq.Month,
		Type:       itemCreateReq.Type,
		CategoryId: itemCreateReq.CategoryId,
		AccountId:  itemCreateReq.AccountId,
		Amount:     itemCreateReq.Amount,
		Name:       itemCreateReq.Name,
		Comment:    itemCreateReq.Comment,
	}

	if err := a.budgetPlans.CreateItem(c, item); err != nil {
		log.Errorf(c, "[budget_plans.BudgetPlanItemCreateHandler] failed to create plan item for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[budget_plans.BudgetPlanItemCreateHandler] user \"uid:%d\" has planned a new item \"id:%d\" for %d-%d", uid, item.ItemId, item.Year, item.Month)

	return item.ToBudgetPlanItemInfoResponse(), nil
}

// BudgetPlanItemModifyHandler saves a change to something already planned for the current user
func (a *BudgetPlansApi) BudgetPlanItemModifyHandler(c *core.WebContext) (any, *errs.Error) {
	var itemModifyReq models.BudgetPlanItemModifyRequest
	err := c.ShouldBindJSON(&itemModifyReq)

	if err != nil {
		log.Warnf(c, "[budget_plans.BudgetPlanItemModifyHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	if itemModifyReq.Type != models.TRANSACTION_TYPE_INCOME && itemModifyReq.Type != models.TRANSACTION_TYPE_EXPENSE {
		return nil, errs.ErrBudgetPlanItemTypeInvalid
	}

	uid := c.GetCurrentUid()

	if err := a.verifyCategoryAndAccount(c, uid, itemModifyReq.CategoryId, itemModifyReq.AccountId); err != nil {
		return nil, err
	}

	item, err := a.budgetPlans.GetItemByItemId(c, uid, itemModifyReq.Id)

	if err != nil {
		log.Errorf(c, "[budget_plans.BudgetPlanItemModifyHandler] failed to get plan item \"id:%d\" for user \"uid:%d\", because %s", itemModifyReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	if item.Type == itemModifyReq.Type &&
		item.CategoryId == itemModifyReq.CategoryId &&
		item.AccountId == itemModifyReq.AccountId &&
		item.Amount == itemModifyReq.Amount &&
		item.Name == itemModifyReq.Name &&
		item.Comment == itemModifyReq.Comment {
		return nil, errs.ErrNothingWillBeUpdated
	}

	newItem := &models.BudgetPlanItem{
		ItemId:     item.ItemId,
		Uid:        uid,
		Year:       item.Year,
		Month:      item.Month,
		Type:       itemModifyReq.Type,
		CategoryId: itemModifyReq.CategoryId,
		AccountId:  itemModifyReq.AccountId,
		Amount:     itemModifyReq.Amount,
		Name:       itemModifyReq.Name,
		Comment:    itemModifyReq.Comment,
	}

	if err := a.budgetPlans.ModifyItem(c, newItem); err != nil {
		log.Errorf(c, "[budget_plans.BudgetPlanItemModifyHandler] failed to modify plan item \"id:%d\" for user \"uid:%d\", because %s", item.ItemId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	newItem.DisplayOrder = item.DisplayOrder

	log.Infof(c, "[budget_plans.BudgetPlanItemModifyHandler] user \"uid:%d\" has updated plan item \"id:%d\"", uid, item.ItemId)

	return newItem.ToBudgetPlanItemInfoResponse(), nil
}

// BudgetPlanItemDeleteHandler takes one planned thing back out of a month for the current user
func (a *BudgetPlansApi) BudgetPlanItemDeleteHandler(c *core.WebContext) (any, *errs.Error) {
	var itemDeleteReq models.BudgetPlanItemDeleteRequest
	err := c.ShouldBindJSON(&itemDeleteReq)

	if err != nil {
		log.Warnf(c, "[budget_plans.BudgetPlanItemDeleteHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	if err := a.budgetPlans.DeleteItem(c, uid, itemDeleteReq.Id); err != nil {
		log.Errorf(c, "[budget_plans.BudgetPlanItemDeleteHandler] failed to delete plan item \"id:%d\" for user \"uid:%d\", because %s", itemDeleteReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[budget_plans.BudgetPlanItemDeleteHandler] user \"uid:%d\" has deleted plan item \"id:%d\"", uid, itemDeleteReq.Id)

	return true, nil
}

// BudgetPlanItemCopyHandler copies everything planned by hand in one month into another for the
// current user
func (a *BudgetPlansApi) BudgetPlanItemCopyHandler(c *core.WebContext) (any, *errs.Error) {
	var itemCopyReq models.BudgetPlanItemCopyRequest
	err := c.ShouldBindJSON(&itemCopyReq)

	if err != nil {
		log.Warnf(c, "[budget_plans.BudgetPlanItemCopyHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	copiedCount, err := a.budgetPlans.CopyMonth(c, uid, itemCopyReq.FromYear, itemCopyReq.FromMonth, itemCopyReq.ToYear, itemCopyReq.ToMonth)

	if err != nil {
		log.Errorf(c, "[budget_plans.BudgetPlanItemCopyHandler] failed to copy plan items for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[budget_plans.BudgetPlanItemCopyHandler] user \"uid:%d\" has copied %d plan items from %d-%d into %d-%d", uid, copiedCount, itemCopyReq.FromYear, itemCopyReq.FromMonth, itemCopyReq.ToYear, itemCopyReq.ToMonth)

	return copiedCount, nil
}

// BudgetPlanAdjustmentSetHandler records how one month differs from one schedule for the current user
func (a *BudgetPlansApi) BudgetPlanAdjustmentSetHandler(c *core.WebContext) (any, *errs.Error) {
	var adjustmentSetReq models.BudgetPlanAdjustmentSetRequest
	err := c.ShouldBindJSON(&adjustmentSetReq)

	if err != nil {
		log.Warnf(c, "[budget_plans.BudgetPlanAdjustmentSetHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	adjustment := &models.BudgetPlanScheduleAdjustment{
		Uid:        uid,
		Year:       adjustmentSetReq.Year,
		Month:      adjustmentSetReq.Month,
		TemplateId: adjustmentSetReq.TemplateId,
		Excluded:   adjustmentSetReq.Excluded,
		Amount:     adjustmentSetReq.Amount,
	}

	if err := a.budgetPlans.SetAdjustment(c, adjustment); err != nil {
		log.Errorf(c, "[budget_plans.BudgetPlanAdjustmentSetHandler] failed to set adjustment for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[budget_plans.BudgetPlanAdjustmentSetHandler] user \"uid:%d\" has adjusted template \"id:%d\" for %d-%d", uid, adjustment.TemplateId, adjustment.Year, adjustment.Month)

	if adjustment.AdjustmentId < 1 {
		return nil, nil
	}

	return adjustment.ToBudgetPlanAdjustmentInfoResponse(), nil
}

// BudgetPlanExpectationSetHandler records what one category is expected to come to in one month for
// the current user
func (a *BudgetPlansApi) BudgetPlanExpectationSetHandler(c *core.WebContext) (any, *errs.Error) {
	var expectationSetReq models.BudgetPlanExpectationSetRequest
	err := c.ShouldBindJSON(&expectationSetReq)

	if err != nil {
		log.Warnf(c, "[budget_plans.BudgetPlanExpectationSetHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	if err := a.verifyCategory(c, uid, expectationSetReq.CategoryId); err != nil {
		return nil, err
	}

	expectation := &models.BudgetPlanCategoryExpectation{
		Uid:        uid,
		Year:       expectationSetReq.Year,
		Month:      expectationSetReq.Month,
		CategoryId: expectationSetReq.CategoryId,
		Amount:     expectationSetReq.Amount,
	}

	if err := a.budgetPlans.SetExpectation(c, expectation); err != nil {
		log.Errorf(c, "[budget_plans.BudgetPlanExpectationSetHandler] failed to set expectation for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[budget_plans.BudgetPlanExpectationSetHandler] user \"uid:%d\" has set an expectation on category \"id:%d\" for %d-%d", uid, expectation.CategoryId, expectation.Year, expectation.Month)

	// an expectation cleared to nothing has no row to return, and the client takes the absence as
	// the deletion it is
	if expectation.ExpectationId < 1 {
		return nil, nil
	}

	return expectation.ToBudgetPlanExpectationInfoResponse(), nil
}

// BudgetPlanStandingExpectationSetHandler records what one category is expected to come to in every
// month for the current user
func (a *BudgetPlansApi) BudgetPlanStandingExpectationSetHandler(c *core.WebContext) (any, *errs.Error) {
	var standingSetReq models.BudgetPlanStandingExpectationSetRequest
	err := c.ShouldBindJSON(&standingSetReq)

	if err != nil {
		log.Warnf(c, "[budget_plans.BudgetPlanStandingExpectationSetHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	if err := a.verifyCategory(c, uid, standingSetReq.CategoryId); err != nil {
		return nil, err
	}

	expectation := &models.BudgetPlanStandingExpectation{
		Uid:        uid,
		CategoryId: standingSetReq.CategoryId,
		Amount:     standingSetReq.Amount,
	}

	if err := a.budgetPlans.SetStandingExpectation(c, expectation); err != nil {
		log.Errorf(c, "[budget_plans.BudgetPlanStandingExpectationSetHandler] failed to set standing expectation for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[budget_plans.BudgetPlanStandingExpectationSetHandler] user \"uid:%d\" has set a standing expectation on category \"id:%d\"", uid, expectation.CategoryId)

	if expectation.StandingId < 1 {
		return nil, nil
	}

	return expectation.ToBudgetPlanStandingExpectationInfoResponse(), nil
}

// verifyCategoryAndAccount refuses a plan item pointing at a category or an account that is not the
// user's own. A plan is only worth anything if what it names is the same thing the ledger names.
func (a *BudgetPlansApi) verifyCategoryAndAccount(c *core.WebContext, uid int64, categoryId int64, accountId int64) *errs.Error {
	if err := a.verifyCategory(c, uid, categoryId); err != nil {
		return err
	}

	account, err := a.accounts.GetAccountByAccountId(c, uid, accountId)

	if err != nil {
		log.Warnf(c, "[budget_plans.verifyCategoryAndAccount] failed to get account \"id:%d\" for user \"uid:%d\", because %s", accountId, uid, err.Error())
		return errs.Or(err, errs.ErrAccountNotFound)
	}

	if account == nil {
		return errs.ErrAccountNotFound
	}

	return nil
}

// verifyCategory refuses anything pointing at a category that is not the user's own. It is worth
// checking on an expectation as much as on a plan item: an expectation set against a category that
// does not exist would never be shown anywhere, and would sit in the data unexplained.
func (a *BudgetPlansApi) verifyCategory(c *core.WebContext, uid int64, categoryId int64) *errs.Error {
	category, err := a.categories.GetCategoryByCategoryId(c, uid, categoryId)

	if err != nil {
		log.Warnf(c, "[budget_plans.verifyCategory] failed to get category \"id:%d\" for user \"uid:%d\", because %s", categoryId, uid, err.Error())
		return errs.Or(err, errs.ErrTransactionCategoryNotFound)
	}

	if category == nil {
		return errs.ErrTransactionCategoryNotFound
	}

	return nil
}
