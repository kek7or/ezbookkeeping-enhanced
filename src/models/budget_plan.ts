import { TransactionType } from '@/core/transaction.ts';

// A budget plan is a month, and only what cannot be derived is stored - see the model on the server
// side. The classes here are the two things that are: what was planned by hand, and how a month
// differs from a schedule.

function emptyIfZero(id: string): string {
    return !id || id === '0' ? '' : id;
}

// One class covers both a planned item and a wish, because they are the same row on the server: a
// wish is one with no month chosen yet. See the model there for why assigning a wish to a month is
// a change to that row rather than a new one.
export class BudgetPlanItem implements BudgetPlanItemInfoResponse {
    public id: string;
    public year: number;
    public month: number;
    // wished says this is on the wishlist and in no month's budget
    public wished: boolean;
    public type: number;
    public categoryId: string;
    public accountId: string;
    public amount: number;
    public name: string;
    public comment: string;
    public displayOrder: number;

    private constructor(id: string, year: number, month: number, wished: boolean, type: number, categoryId: string, accountId: string, amount: number, name: string, comment: string, displayOrder: number) {
        this.id = id;
        this.year = year;
        this.month = month;
        this.wished = wished;
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

    // A wish asks for less than a planned item does, so the two requests are not the same shape:
    // there is no month, no type - a wishlist is things somebody wants to buy - and the category and
    // the account may be left unnamed.
    public toWishCreateRequest(): BudgetPlanWishCreateRequest {
        return {
            categoryId: this.categoryId || undefined,
            accountId: this.accountId || undefined,
            amount: this.amount,
            name: this.name,
            comment: this.comment
        };
    }

    public toWishModifyRequest(): BudgetPlanWishModifyRequest {
        return {
            id: this.id,
            categoryId: this.categoryId || undefined,
            accountId: this.accountId || undefined,
            amount: this.amount,
            name: this.name,
            comment: this.comment
        };
    }

    public clone(): BudgetPlanItem {
        return new BudgetPlanItem(this.id, this.year, this.month, this.wished, this.type, this.categoryId, this.accountId, this.amount, this.name, this.comment, this.displayOrder);
    }

    public static of(response: BudgetPlanItemInfoResponse): BudgetPlanItem {
        // A wish may name no category and no account, which the server sends as the zero id - and
        // "0" is a perfectly true string, so left alone it would read as a category that is merely
        // missing rather than one that was never named. Emptied here, once, rather than guarded
        // against everywhere it is read.
        return new BudgetPlanItem(response.id, response.year, response.month, !!response.wished, response.type, emptyIfZero(response.categoryId), emptyIfZero(response.accountId), response.amount, response.name, response.comment, response.displayOrder);
    }

    public static ofMulti(responses: BudgetPlanItemInfoResponse[]): BudgetPlanItem[] {
        return responses.map(response => BudgetPlanItem.of(response));
    }

    public static createNew(year: number, month: number, type: number): BudgetPlanItem {
        return new BudgetPlanItem('', year, month, false, type, '', '', 0, '', '', 0);
    }

    // A new wish belongs to no month and is always an expense, both of which are decided here
    // rather than asked for
    public static createNewWish(): BudgetPlanItem {
        return new BudgetPlanItem('', 0, 0, true, TransactionType.Expense, '', '', 0, '', '', 0);
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

export interface BudgetPlanWishCreateRequest {
    // both are optional: a wish is written down before it is thought through
    readonly categoryId?: string;
    readonly accountId?: string;
    readonly amount: number;
    readonly name: string;
    readonly comment: string;
}

export interface BudgetPlanWishModifyRequest {
    readonly id: string;
    readonly categoryId?: string;
    readonly accountId?: string;
    readonly amount: number;
    readonly name: string;
    readonly comment: string;
}

// Assigning is the act of deciding to buy the thing: it stops being a wish and is planned for the
// month it is assigned to.
export interface BudgetPlanWishAssignRequest {
    readonly id: string;
    readonly year: number;
    readonly month: number;
}

export interface BudgetPlanWishUnassignRequest {
    readonly id: string;
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
    readonly wished: boolean;
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
    // the wishlist belongs to no month and is sent with every one, because a wish is tried against
    // whichever month is being looked at
    readonly wishes: BudgetPlanItemInfoResponse[];
}
