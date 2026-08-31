package models

// A budget plan is a month, and almost nothing about it is stored.
//
// Everything a month already commits is known: the schedules say what recurs and the ledger says
// what has happened. Copying those into a plan would be writing down an answer that goes stale the
// moment a subscription changes price, and then there would be two places to correct it. So the
// plan holds only what cannot be derived - the one-off the user knows is coming, the month where a
// schedule is not going to behave as it usually does, and the bare fact that the month was planned
// at all - and everything else is worked out from the schedules and the transactions each time it
// is asked for. That is also what makes the whole page recompute the instant anything changes:
// there is nothing cached to invalidate.

// BudgetPlanMonth says that a month was planned. It carries nothing else - no total, no name, no
// settings - because everything a month is planned to cost is worked out from the rows that follow
// and from the schedules and the ledger.
//
// It exists because whether somebody meant to plan a month is the one thing about that month which
// cannot be derived. Without it every month there has ever been or will be shows a plan: the
// standing expectations apply to every month by construction, and the schedules to every month they
// run in. A confident plan drawn across a month from before the user had ever opened the page, with
// real spending from the ledger set against it, is a comparison against a figure nobody ever chose.
//
// The row is written the first time anything is planned for the month, so nobody has to ask for it
// in the ordinary course of planning, and can be written on its own to start a month before there
// is anything in it. No month is exempt: a month two years out can be filled in from the standing
// figures and the schedules as readily as a month two years back, and doing so reads as a decision
// about a month nobody has decided anything about. A month is planned when somebody says so.
//
// There is meant to be one row per month, which nothing enforces. Two concurrent first writes can
// both insert one, and it does not matter: the row has no contents, so a duplicate says exactly
// what the first one says.
type BudgetPlanMonth struct {
	MonthId int64 `xorm:"PK"`
	Uid     int64 `xorm:"INDEX(IDX_budget_plan_month_uid_deleted_year_month) NOT NULL"`
	// Deleted is a soft delete, as everywhere else. Nothing removes a planned month today; the
	// column is here so that unplanning one later does not throw away what was planned in it.
	Deleted         bool  `xorm:"INDEX(IDX_budget_plan_month_uid_deleted_year_month) NOT NULL"`
	Year            int32 `xorm:"INDEX(IDX_budget_plan_month_uid_deleted_year_month) NOT NULL"`
	Month           int32 `xorm:"INDEX(IDX_budget_plan_month_uid_deleted_year_month) NOT NULL"`
	CreatedUnixTime int64
	DeletedUnixTime int64
}

// BudgetPlanItem is one thing planned for a month that no schedule would ever produce - a flight, a
// birthday, the dentist. It is not a transaction: no money has moved, no account balance reflects
// it, and nothing in the ledger knows it exists. It says only that a certain amount is expected to
// leave (or arrive) in a certain month, under a certain category.
//
// It carries its own Name because a category is not a description: three separate things planned
// under "Other Expense" have to be tellable apart in the list.
//
// The same row is also how a wish is kept - something wanted rather than something planned - because
// the two are the same thing at different moments. A sofa on the wishlist and a sofa planned for
// October differ in whether a month has been chosen for it, not in what it is, and assigning one to
// a month must not lose the name it was given or the comment explaining it. So a wish is this row
// with Wished set and no month, and assigning it clears the flag and fills the month in. Nothing is
// copied and nothing is left behind.
type BudgetPlanItem struct {
	ItemId int64 `xorm:"PK"`
	Uid    int64 `xorm:"INDEX(IDX_budget_plan_item_uid_deleted_year_month) INDEX(IDX_budget_plan_item_uid_deleted_wished) NOT NULL"`
	// Deleted is a soft delete, as everywhere else, so that a plan item removed by mistake is
	// recoverable from the data export
	Deleted bool `xorm:"INDEX(IDX_budget_plan_item_uid_deleted_year_month) INDEX(IDX_budget_plan_item_uid_deleted_wished) NOT NULL"`
	// Wished says this is on the wishlist rather than in any month's budget: wanted, not planned. A
	// wish counts towards nothing and appears in no month's total - it is there to be tried against
	// a month and see what it would do - until it is assigned to one, which is the act of deciding
	// to buy it and is what stops it being a wish.
	//
	// It carries a default because it is added to a table that already exists, and a NOT NULL column
	// without one cannot be added to a populated table.
	Wished bool `xorm:"INDEX(IDX_budget_plan_item_uid_deleted_wished) NOT NULL DEFAULT 0"`
	// Year and Month are the month this is planned for, kept as two numbers rather than a timestamp
	// because a plan belongs to a calendar month and to no particular instant inside it. Both are
	// zero while the row is a wish, which has not been given a month yet - that being the whole
	// difference between wanting a thing and planning for it.
	Year  int32 `xorm:"INDEX(IDX_budget_plan_item_uid_deleted_year_month) NOT NULL"`
	Month int32 `xorm:"INDEX(IDX_budget_plan_item_uid_deleted_year_month) NOT NULL"`
	// Type is income or expense. A transfer is neither spent nor earned and has no place in a plan
	// of what a month costs, so it is rejected rather than stored and ignored. A wish is always an
	// expense: a wishlist is things somebody wants to buy.
	Type TransactionType `xorm:"NOT NULL"`
	// CategoryId is zero on a wish that has not been filed under anything. A wish is jotted down in
	// a hurry - the point is to get the figure written before it is forgotten - so it is allowed to
	// name nothing, and such a wish moves the month's totals without appearing in the category tree.
	CategoryId int64 `xorm:"NOT NULL"`
	// AccountId is which account this is expected to move through, and is what gives the amount its
	// currency - the same way a scheduled template takes its currency from its account. It is zero
	// on a wish that names no account, whose amount is then read in the user's own currency.
	AccountId       int64  `xorm:"NOT NULL"`
	Amount          int64  `xorm:"NOT NULL"`
	Name            string `xorm:"VARCHAR(64) NOT NULL"`
	Comment         string `xorm:"VARCHAR(255) NOT NULL"`
	DisplayOrder    int32  `xorm:"NOT NULL"`
	CreatedUnixTime int64
	UpdatedUnixTime int64
	DeletedUnixTime int64
}

// BudgetPlanScheduleAdjustment is how one month differs from a schedule that otherwise recurs
// unchanged. The gym is paused over the summer; the rent goes up in October; the insurance is
// taken twice this year because of a renewal date.
//
// Editing the template itself would be wrong for all of these: the template says what recurs, and
// changing it rewrites what every other month is planned to cost. An adjustment says only that one
// month is different, and leaves the recurrence alone.
//
// There is at most one adjustment per template per month, which the service enforces rather than
// the database, because the row is soft-deleted and a unique index would count the tombstones.
type BudgetPlanScheduleAdjustment struct {
	AdjustmentId int64 `xorm:"PK"`
	Uid          int64 `xorm:"INDEX(IDX_budget_plan_adjustment_uid_deleted_year_month) NOT NULL"`
	Deleted      bool  `xorm:"INDEX(IDX_budget_plan_adjustment_uid_deleted_year_month) NOT NULL"`
	Year         int32 `xorm:"INDEX(IDX_budget_plan_adjustment_uid_deleted_year_month) NOT NULL"`
	Month        int32 `xorm:"INDEX(IDX_budget_plan_adjustment_uid_deleted_year_month) NOT NULL"`
	TemplateId   int64 `xorm:"NOT NULL"`
	// Excluded says this schedule is not happening in this month at all
	Excluded bool `xorm:"NOT NULL"`
	// Amount is what this schedule costs in this month instead of what the template says, and is
	// null when only the exclusion is being set. It is a pointer because zero is a meaningful
	// override: a month where something is billed but comes to nothing is not the same as a month
	// with no override at all.
	Amount          *int64
	CreatedUnixTime int64
	UpdatedUnixTime int64
	DeletedUnixTime int64
}

// BudgetPlanCategoryExpectation is what a whole category is expected to cost in a month, said
// without listing what it is made of. Weekly food shopping is not worth planning line by line, but
// the four hundred it comes to every month is worth planning.
//
// It is set at either level of the category tree, and the two mean different things. On a primary
// category it is the ceiling for the whole branch; on a secondary it is that branch divided up.
// Both can be set at once, which is the point: six hundred for Food, of which four hundred is
// Groceries, leaves two hundred that has a home in the branch but not yet in a subcategory.
//
// An expectation covers what is already planned under the category rather than adding to it. Six
// hundred expected for Food with a two hundred grocery delivery already scheduled means Food costs
// six hundred this month, two hundred of it accounted for - not eight hundred. Where what is
// already planned comes to more than the expectation, the plan wins, because a bill that exists
// cannot be wished down to the figure someone hoped for.
//
// There is at most one expectation per category per month, enforced by the service rather than the
// database for the same reason the adjustments are: a unique index would count the tombstones.
type BudgetPlanCategoryExpectation struct {
	ExpectationId int64 `xorm:"PK"`
	Uid           int64 `xorm:"INDEX(IDX_budget_plan_expectation_uid_deleted_year_month) NOT NULL"`
	Deleted       bool  `xorm:"INDEX(IDX_budget_plan_expectation_uid_deleted_year_month) NOT NULL"`
	Year          int32 `xorm:"INDEX(IDX_budget_plan_expectation_uid_deleted_year_month) NOT NULL"`
	Month         int32 `xorm:"INDEX(IDX_budget_plan_expectation_uid_deleted_year_month) NOT NULL"`
	CategoryId    int64 `xorm:"NOT NULL"`
	// Amount is never negative: an expectation is what a category is expected to come to, and a
	// category that earns is an income category. Zero is not stored at all - see SetExpectation.
	Amount          int64 `xorm:"NOT NULL"`
	CreatedUnixTime int64
	UpdatedUnixTime int64
	DeletedUnixTime int64
}

// BudgetPlanStandingExpectation is what a category is expected to come to in every month, rather
// than in one of them. Food is around three hundred and fifty most months; work meals are fifty;
// houseware is fifty. None of that is worth retyping twelve times a year.
//
// It is the figure a month falls back on. Where a month has an expectation of its own for the same
// category that one wins, so December can say six hundred for Food without disturbing the standing
// three hundred and fifty that every other month uses. This is why the two are separate rows rather
// than one row that gets edited: overriding a month must not destroy the standing figure, and
// clearing the override has to leave something to fall back to.
//
// There is at most one standing expectation per category, enforced by the service rather than the
// database, because the row is soft-deleted and a unique index would count the tombstones.
type BudgetPlanStandingExpectation struct {
	StandingId int64 `xorm:"PK"`
	Uid        int64 `xorm:"INDEX(IDX_budget_plan_standing_uid_deleted) NOT NULL"`
	Deleted    bool  `xorm:"INDEX(IDX_budget_plan_standing_uid_deleted) NOT NULL"`
	CategoryId int64 `xorm:"NOT NULL"`
	// Amount is never negative and is never zero: zero says no more than having no standing figure
	// at all, so it is stored as the absence of a row - see SetStandingExpectation.
	Amount          int64 `xorm:"NOT NULL"`
	CreatedUnixTime int64
	UpdatedUnixTime int64
	DeletedUnixTime int64
}

// BudgetPlanGetRequest represents all parameters of a request for the plan of one month
type BudgetPlanGetRequest struct {
	Year  int32 `form:"year" binding:"required,min=1,max=9999"`
	Month int32 `form:"month" binding:"required,min=1,max=12"`
}

// BudgetPlanMonthStartRequest represents all parameters of a request to start planning a month
// that nothing has been planned for yet. Ordinary planning starts a month on its own, so this is
// only for going back to a month that was never planned and filling it in now.
type BudgetPlanMonthStartRequest struct {
	Year  int32 `json:"year" binding:"required,min=1,max=9999"`
	Month int32 `json:"month" binding:"required,min=1,max=12"`
}

// BudgetPlanMonthStopRequest represents all parameters of a request to take a month back out of the
// plan. What is planned in the month is kept - see StopMonth.
type BudgetPlanMonthStopRequest struct {
	Year  int32 `json:"year" binding:"required,min=1,max=9999"`
	Month int32 `json:"month" binding:"required,min=1,max=12"`
}

// BudgetPlanItemCreateRequest represents all parameters of a plan item creation request
type BudgetPlanItemCreateRequest struct {
	Year       int32           `json:"year" binding:"required,min=1,max=9999"`
	Month      int32           `json:"month" binding:"required,min=1,max=12"`
	Type       TransactionType `json:"type" binding:"required"`
	CategoryId int64           `json:"categoryId,string" binding:"required,min=1"`
	AccountId  int64           `json:"accountId,string" binding:"required,min=1"`
	Amount     int64           `json:"amount" binding:"min=-999999999999999,max=999999999999999"`
	Name       string          `json:"name" binding:"required,notBlank,max=64"`
	Comment    string          `json:"comment" binding:"max=255"`
}

// BudgetPlanItemModifyRequest represents all parameters of a plan item modification request
type BudgetPlanItemModifyRequest struct {
	Id         int64           `json:"id,string" binding:"required,min=1"`
	Type       TransactionType `json:"type" binding:"required"`
	CategoryId int64           `json:"categoryId,string" binding:"required,min=1"`
	AccountId  int64           `json:"accountId,string" binding:"required,min=1"`
	Amount     int64           `json:"amount" binding:"min=-999999999999999,max=999999999999999"`
	Name       string          `json:"name" binding:"required,notBlank,max=64"`
	Comment    string          `json:"comment" binding:"max=255"`
}

// BudgetPlanWishCreateRequest represents all parameters of a request to add something to the
// wishlist. It asks for less than planning something does: a wish is jotted down before it is
// thought through, and demanding a category and an account for a thing somebody has only just
// thought of is how a wishlist stops being used.
type BudgetPlanWishCreateRequest struct {
	// CategoryId and AccountId are optional, and zero means none was named
	CategoryId int64  `json:"categoryId,string" binding:"omitempty,min=1"`
	AccountId  int64  `json:"accountId,string" binding:"omitempty,min=1"`
	Amount     int64  `json:"amount" binding:"min=0,max=999999999999999"`
	Name       string `json:"name" binding:"required,notBlank,max=64"`
	Comment    string `json:"comment" binding:"max=255"`
}

// BudgetPlanWishModifyRequest represents all parameters of a request to change something on the
// wishlist
type BudgetPlanWishModifyRequest struct {
	Id         int64  `json:"id,string" binding:"required,min=1"`
	CategoryId int64  `json:"categoryId,string" binding:"omitempty,min=1"`
	AccountId  int64  `json:"accountId,string" binding:"omitempty,min=1"`
	Amount     int64  `json:"amount" binding:"min=0,max=999999999999999"`
	Name       string `json:"name" binding:"required,notBlank,max=64"`
	Comment    string `json:"comment" binding:"max=255"`
}

// BudgetPlanWishAssignRequest represents all parameters of a request to put one wish into one
// month's budget, which is the act of deciding to buy it
type BudgetPlanWishAssignRequest struct {
	Id    int64 `json:"id,string" binding:"required,min=1"`
	Year  int32 `json:"year" binding:"required,min=1,max=9999"`
	Month int32 `json:"month" binding:"required,min=1,max=12"`
}

// BudgetPlanWishUnassignRequest represents all parameters of a request to take something back off a
// month and return it to the wishlist, for a purchase decided on and then thought better of
type BudgetPlanWishUnassignRequest struct {
	Id int64 `json:"id,string" binding:"required,min=1"`
}

// BudgetPlanItemDeleteRequest represents all parameters of a plan item deletion request
type BudgetPlanItemDeleteRequest struct {
	Id int64 `json:"id,string" binding:"required,min=1"`
}

// BudgetPlanItemCopyRequest represents all parameters of a request to copy the items of one month
// into another. Planning a month rarely starts from nothing - most of what was planned last month
// is planned again - and retyping it is the fastest way to stop using a plan at all.
type BudgetPlanItemCopyRequest struct {
	FromYear  int32 `json:"fromYear" binding:"required,min=1,max=9999"`
	FromMonth int32 `json:"fromMonth" binding:"required,min=1,max=12"`
	ToYear    int32 `json:"toYear" binding:"required,min=1,max=9999"`
	ToMonth   int32 `json:"toMonth" binding:"required,min=1,max=12"`
}

// BudgetPlanAdjustmentSetRequest represents all parameters of a request to say how one month
// differs from one schedule. Clearing both the exclusion and the amount removes the adjustment.
type BudgetPlanAdjustmentSetRequest struct {
	Year       int32 `json:"year" binding:"required,min=1,max=9999"`
	Month      int32 `json:"month" binding:"required,min=1,max=12"`
	TemplateId int64 `json:"templateId,string" binding:"required,min=1"`
	Excluded   bool  `json:"excluded"`
	// Amount is null to leave the template's own amount in force for this month
	Amount *int64 `json:"amount" binding:"omitempty,min=-999999999999999,max=999999999999999"`
}

// BudgetPlanExpectationSetRequest represents all parameters of a request to say what one category
// is expected to come to in one month. An expectation of zero is the same as no expectation at all
// - what is already planned under the category stands either way - so zero clears it.
type BudgetPlanExpectationSetRequest struct {
	Year       int32 `json:"year" binding:"required,min=1,max=9999"`
	Month      int32 `json:"month" binding:"required,min=1,max=12"`
	CategoryId int64 `json:"categoryId,string" binding:"required,min=1"`
	Amount     int64 `json:"amount" binding:"min=0,max=999999999999999"`
}

// BudgetPlanStandingExpectationSetRequest represents all parameters of a request to say what one
// category is expected to come to in every month. An amount of zero clears the standing figure.
type BudgetPlanStandingExpectationSetRequest struct {
	CategoryId int64 `json:"categoryId,string" binding:"required,min=1"`
	Amount     int64 `json:"amount" binding:"min=0,max=999999999999999"`
}

// BudgetPlanItemInfoResponse represents a view-object of one planned item
type BudgetPlanItemInfoResponse struct {
	Id           int64           `json:"id,string"`
	Year         int32           `json:"year"`
	Month        int32           `json:"month"`
	Wished       bool            `json:"wished"`
	Type         TransactionType `json:"type"`
	CategoryId   int64           `json:"categoryId,string"`
	AccountId    int64           `json:"accountId,string"`
	Amount       int64           `json:"amount"`
	Name         string          `json:"name"`
	Comment      string          `json:"comment"`
	DisplayOrder int32           `json:"displayOrder"`
}

// BudgetPlanAdjustmentInfoResponse represents a view-object of one schedule adjustment
type BudgetPlanAdjustmentInfoResponse struct {
	Id         int64  `json:"id,string"`
	Year       int32  `json:"year"`
	Month      int32  `json:"month"`
	TemplateId int64  `json:"templateId,string"`
	Excluded   bool   `json:"excluded"`
	Amount     *int64 `json:"amount,omitempty"`
}

// BudgetPlanExpectationInfoResponse represents a view-object of one category expectation
type BudgetPlanExpectationInfoResponse struct {
	Id         int64 `json:"id,string"`
	Year       int32 `json:"year"`
	Month      int32 `json:"month"`
	CategoryId int64 `json:"categoryId,string"`
	Amount     int64 `json:"amount"`
}

// BudgetPlanStandingExpectationInfoResponse represents a view-object of one standing expectation
type BudgetPlanStandingExpectationInfoResponse struct {
	Id         int64 `json:"id,string"`
	CategoryId int64 `json:"categoryId,string"`
	Amount     int64 `json:"amount"`
}

// BudgetPlanInfoResponse is everything stored about one month's plan. What the month costs is not
// in here: it is worked out from these, the schedules and the ledger, by whoever is displaying it.
type BudgetPlanInfoResponse struct {
	Year  int32 `json:"year"`
	Month int32 `json:"month"`
	// Planned says this month was planned rather than merely being a month the standing figures and
	// the schedules happen to reach. It is what tells a month nobody ever planned apart from one
	// that was planned and came to nothing.
	Planned      bool                                 `json:"planned"`
	Items        []*BudgetPlanItemInfoResponse        `json:"items"`
	Adjustments  []*BudgetPlanAdjustmentInfoResponse  `json:"adjustments"`
	Expectations []*BudgetPlanExpectationInfoResponse `json:"expectations"`
	// Wishes belong to no month and are sent with every one, because a wish is tried against
	// whichever month is being looked at. They are kept in a list of their own rather than mixed
	// into Items so that nothing which totals a month can reach them by accident.
	Wishes []*BudgetPlanItemInfoResponse `json:"wishes"`
	// Standing is not month-specific, and is sent with every month because every month may need to
	// fall back on it
	Standing []*BudgetPlanStandingExpectationInfoResponse `json:"standing"`
}

// ToBudgetPlanItemInfoResponse returns a view-object according to database model
func (i *BudgetPlanItem) ToBudgetPlanItemInfoResponse() *BudgetPlanItemInfoResponse {
	return &BudgetPlanItemInfoResponse{
		Id:           i.ItemId,
		Year:         i.Year,
		Month:        i.Month,
		Wished:       i.Wished,
		Type:         i.Type,
		CategoryId:   i.CategoryId,
		AccountId:    i.AccountId,
		Amount:       i.Amount,
		Name:         i.Name,
		Comment:      i.Comment,
		DisplayOrder: i.DisplayOrder,
	}
}

// ToBudgetPlanAdjustmentInfoResponse returns a view-object according to database model
func (a *BudgetPlanScheduleAdjustment) ToBudgetPlanAdjustmentInfoResponse() *BudgetPlanAdjustmentInfoResponse {
	return &BudgetPlanAdjustmentInfoResponse{
		Id:         a.AdjustmentId,
		Year:       a.Year,
		Month:      a.Month,
		TemplateId: a.TemplateId,
		Excluded:   a.Excluded,
		Amount:     a.Amount,
	}
}

// ToBudgetPlanExpectationInfoResponse returns a view-object according to database model
func (e *BudgetPlanCategoryExpectation) ToBudgetPlanExpectationInfoResponse() *BudgetPlanExpectationInfoResponse {
	return &BudgetPlanExpectationInfoResponse{
		Id:         e.ExpectationId,
		Year:       e.Year,
		Month:      e.Month,
		CategoryId: e.CategoryId,
		Amount:     e.Amount,
	}
}

// ToBudgetPlanStandingExpectationInfoResponse returns a view-object according to database model
func (e *BudgetPlanStandingExpectation) ToBudgetPlanStandingExpectationInfoResponse() *BudgetPlanStandingExpectationInfoResponse {
	return &BudgetPlanStandingExpectationInfoResponse{
		Id:         e.StandingId,
		CategoryId: e.CategoryId,
		Amount:     e.Amount,
	}
}

// BudgetPlanItemInfoResponseSlice represents the slice data structure of BudgetPlanItemInfoResponse
type BudgetPlanItemInfoResponseSlice []*BudgetPlanItemInfoResponse

// Len returns the count of items
func (s BudgetPlanItemInfoResponseSlice) Len() int {
	return len(s)
}

// Swap swaps two items
func (s BudgetPlanItemInfoResponseSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less reports whether the first item is less than the second one
func (s BudgetPlanItemInfoResponseSlice) Less(i, j int) bool {
	return s[i].DisplayOrder < s[j].DisplayOrder
}
