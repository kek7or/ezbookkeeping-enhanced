// A budget plan is a month, and only what cannot be derived is stored - see the model on the server
// side. The classes here are the two things that are: what was planned by hand, and how a month
// differs from a schedule.

export class BudgetPlanItem implements BudgetPlanItemInfoResponse {
    public id: string;
    public year: number;
    public month: number;
    public type: number;
    public categoryId: string;
    public accountId: string;
    public amount: number;
    public name: string;
    public comment: string;
    public displayOrder: number;

    private constructor(id: string, year: number, month: number, type: number, categoryId: string, accountId: string, amount: number, name: string, comment: string, displayOrder: number) {
        this.id = id;
        this.year = year;
        this.month = month;
        this.type = type;
        this.categoryId = categoryId;
        this.accountId = accountId;
        this.amount = amount;
        this.name = name;
        this.comment = comment;
        this.displayOrder = displayOrder;
    }

    public toCreateRequest(): BudgetPlanItemCreateRequest {
        return {
            year: this.year,
            month: this.month,
            type: this.type,
            categoryId: this.categoryId,
            accountId: this.accountId,
            amount: this.amount,
            name: this.name,
            comment: this.comment
        };
    }

    public toModifyRequest(): BudgetPlanItemModifyRequest {
        return {
            id: this.id,
            type: this.type,
            categoryId: this.categoryId,
            accountId: this.accountId,
            amount: this.amount,
            name: this.name,
            comment: this.comment
        };
    }

    public clone(): BudgetPlanItem {
        return new BudgetPlanItem(this.id, this.year, this.month, this.type, this.categoryId, this.accountId, this.amount, this.name, this.comment, this.displayOrder);
    }

    public static of(response: BudgetPlanItemInfoResponse): BudgetPlanItem {
        return new BudgetPlanItem(response.id, response.year, response.month, response.type, response.categoryId, response.accountId, response.amount, response.name, response.comment, response.displayOrder);
    }

    public static ofMulti(responses: BudgetPlanItemInfoResponse[]): BudgetPlanItem[] {
        return responses.map(response => BudgetPlanItem.of(response));
    }

    public static createNew(year: number, month: number, type: number): BudgetPlanItem {
        return new BudgetPlanItem('', year, month, type, '', '', 0, '', '', 0);
    }
}

export interface BudgetPlanAdjustment {
    readonly id: string;
    readonly year: number;
    readonly month: number;
    readonly templateId: string;
    readonly excluded: boolean;
    readonly amount?: number;
}

// A category expectation is what a whole category is expected to come to in a month, said without
// listing what it is made of - and it can be said at either level of the category tree. See the
// model on the server side for what the two levels mean together.
export interface BudgetPlanExpectation {
    readonly id: string;
    readonly year: number;
    readonly month: number;
    readonly categoryId: string;
    readonly amount: number;
}

// A standing expectation is the figure a category falls back on in every month that does not say
// otherwise. See the model on the server side for why it is a row of its own rather than something
// copied into each month.
export interface BudgetPlanStandingExpectation {
    readonly id: string;
    readonly categoryId: string;
    readonly amount: number;
}

export interface BudgetPlanGetRequest {
    readonly year: number;
    readonly month: number;
}

// Starting a month is only for going back to one that was never planned: planning anything at all
// marks the month on its own.
export interface BudgetPlanMonthStartRequest {
    readonly year: number;
    readonly month: number;
}

// Stopping a month withdraws only the statement that it was planned. What is planned in it is kept
// and comes back if the month is started again.
export interface BudgetPlanMonthStopRequest {
    readonly year: number;
    readonly month: number;
}

export interface BudgetPlanItemCreateRequest {
    readonly year: number;
    readonly month: number;
    readonly type: number;
    readonly categoryId: string;
    readonly accountId: string;
    readonly amount: number;
    readonly name: string;
    readonly comment: string;
}

export interface BudgetPlanItemModifyRequest {
    readonly id: string;
    readonly type: number;
    readonly categoryId: string;
    readonly accountId: string;
    readonly amount: number;
    readonly name: string;
    readonly comment: string;
}

export interface BudgetPlanItemDeleteRequest {
    readonly id: string;
}

export interface BudgetPlanItemCopyRequest {
    readonly fromYear: number;
    readonly fromMonth: number;
    readonly toYear: number;
    readonly toMonth: number;
}

export interface BudgetPlanAdjustmentSetRequest {
    readonly year: number;
    readonly month: number;
    readonly templateId: string;
    readonly excluded: boolean;
    readonly amount?: number;
}

export interface BudgetPlanExpectationSetRequest {
    readonly year: number;
    readonly month: number;
    readonly categoryId: string;
    // an amount of zero clears the expectation, because it says no more than having none
    readonly amount: number;
}

export interface BudgetPlanStandingExpectationSetRequest {
    readonly categoryId: string;
    // an amount of zero clears the standing figure
    readonly amount: number;
}

export interface BudgetPlanItemInfoResponse {
    readonly id: string;
    readonly year: number;
    readonly month: number;
    readonly type: number;
    readonly categoryId: string;
    readonly accountId: string;
    readonly amount: number;
    readonly name: string;
    readonly comment: string;
    readonly displayOrder: number;
}

export interface BudgetPlanAdjustmentInfoResponse {
    readonly id: string;
    readonly year: number;
    readonly month: number;
    readonly templateId: string;
    readonly excluded: boolean;
    readonly amount?: number;
}

export interface BudgetPlanExpectationInfoResponse {
    readonly id: string;
    readonly year: number;
    readonly month: number;
    readonly categoryId: string;
    readonly amount: number;
}

export interface BudgetPlanStandingExpectationInfoResponse {
    readonly id: string;
    readonly categoryId: string;
    readonly amount: number;
}

export interface BudgetPlanInfoResponse {
    readonly year: number;
    readonly month: number;
    // whether this month was planned, as opposed to being one the standing figures and the
    // schedules merely reach - see the model on the server side
    readonly planned: boolean;
    readonly items: BudgetPlanItemInfoResponse[];
    readonly adjustments: BudgetPlanAdjustmentInfoResponse[];
    readonly expectations: BudgetPlanExpectationInfoResponse[];
    readonly standing: BudgetPlanStandingExpectationInfoResponse[];
}
