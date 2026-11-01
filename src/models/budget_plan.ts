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
    // the wishlist group this is filed under, empty for none
    public wishGroupId: string;
    // fromWishlist says this was planned from the wishlist, which keeps showing it as planned
    public fromWishlist: boolean = false;
    public type: number;
    public categoryId: string;
    public accountId: string;
    public amount: number;
    public name: string;
    public comment: string;
    // where a wish can be bought or looked at, empty for none
    public url: string;
    public displayOrder: number;
    public createdTime: number;
    public updatedTime: number;

    private constructor(id: string, year: number, month: number, wished: boolean, wishGroupId: string, type: number, categoryId: string, accountId: string, amount: number, name: string, comment: string, url: string, displayOrder: number, createdTime: number, updatedTime: number) {
        this.id = id;
        this.year = year;
        this.month = month;
        this.wished = wished;
        this.wishGroupId = wishGroupId;
        this.type = type;
        this.categoryId = categoryId;
        this.accountId = accountId;
        this.amount = amount;
        this.name = name;
        this.comment = comment;
        this.url = url;
        this.displayOrder = displayOrder;
        this.createdTime = createdTime;
        this.updatedTime = updatedTime;
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

    // A wish asks for less than a planned item does: a name, a price and a description. The month
    // here is the one it is tried against, zero for none.
    public toWishCreateRequest(): BudgetPlanWishCreateRequest {
        return {
            groupId: this.wishGroupId || undefined,
            categoryId: this.categoryId || '0',
            amount: this.amount,
            name: this.name,
            comment: this.comment,
            url: this.url,
            year: this.year,
            month: this.month
        };
    }

    public toWishModifyRequest(): BudgetPlanWishModifyRequest {
        return {
            id: this.id,
            categoryId: this.categoryId || '0',
            amount: this.amount,
            name: this.name,
            comment: this.comment,
            url: this.url
        };
    }

    public clone(): BudgetPlanItem {
        const item = new BudgetPlanItem(this.id, this.year, this.month, this.wished, this.wishGroupId, this.type, this.categoryId, this.accountId, this.amount, this.name, this.comment, this.url, this.displayOrder, this.createdTime, this.updatedTime);
        item.fromWishlist = this.fromWishlist;
        return item;
    }

    public static of(response: BudgetPlanItemInfoResponse): BudgetPlanItem {
        // A wish may name no category and no account, which the server sends as the zero id - and
        // "0" is a perfectly true string, so left alone it would read as a category that is merely
        // missing rather than one that was never named. Emptied here, once, rather than guarded
        // against everywhere it is read.
        const item = new BudgetPlanItem(response.id, response.year, response.month, !!response.wished, emptyIfZero(response.wishGroupId ?? ''), response.type, emptyIfZero(response.categoryId), emptyIfZero(response.accountId), response.amount, response.name, response.comment, response.url ?? '', response.displayOrder, response.createdTime ?? 0, response.updatedTime ?? 0);
        item.fromWishlist = !!response.fromWishlist;
        return item;
    }

    public static ofMulti(responses: BudgetPlanItemInfoResponse[]): BudgetPlanItem[] {
        return responses.map(response => BudgetPlanItem.of(response));
    }

    public static createNew(year: number, month: number, type: number): BudgetPlanItem {
        return new BudgetPlanItem('', year, month, false, '', type, '', '', 0, '', '', '', 0, 0, 0);
    }

    // A new wish is always an expense, which is decided here rather than asked for. It is tried
    // against the month given, or none when both are zero.
    public static createNewWish(year: number = 0, month: number = 0, wishGroupId: string = ''): BudgetPlanItem {
        return new BudgetPlanItem('', year, month, true, wishGroupId, TransactionType.Expense, '', '', 0, '', '', '', 0, 0, 0);
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
    readonly groupId?: string;
    // '0' for none
    readonly categoryId: string;
    readonly amount: number;
    readonly name: string;
    readonly comment: string;
    readonly url: string;
    // the month the wish is tried against, both zero for none
    readonly year: number;
    readonly month: number;
}

export interface BudgetPlanWishModifyRequest {
    readonly id: string;
    // '0' for none
    readonly categoryId: string;
    readonly amount: number;
    readonly name: string;
    readonly comment: string;
    readonly url: string;
}

// Attaching tries a wish against a month without planning it. Both zero detaches it.
export interface BudgetPlanWishAttachRequest {
    readonly id: string;
    readonly year: number;
    readonly month: number;
}

export interface BudgetPlanWishNewDisplayOrderRequest {
    readonly id: string;
    readonly groupId: string;
    readonly displayOrder: number;
}

export interface BudgetPlanWishMoveRequest {
    readonly newDisplayOrders: BudgetPlanWishNewDisplayOrderRequest[];
}

export interface BudgetWishGroup {
    readonly id: string;
    name: string;
    displayOrder: number;
}

export interface BudgetWishGroupCreateRequest {
    readonly name: string;
}

export interface BudgetWishGroupModifyRequest {
    readonly id: string;
    readonly name: string;
}

export interface BudgetWishGroupDeleteRequest {
    readonly id: string;
}

export interface BudgetWishGroupMoveRequest {
    readonly newDisplayOrders: { readonly id: string, readonly displayOrder: number }[];
}

export interface BudgetWishlistInfoResponse {
    readonly groups: BudgetWishGroup[];
    readonly wishes: BudgetPlanItemInfoResponse[];
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

export interface BudgetPlanWishBuyRequest {
    readonly id: string;
    readonly transactionId: string;
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
    readonly wishGroupId?: string;
    readonly fromWishlist?: boolean;
    readonly type: number;
    readonly categoryId: string;
    readonly accountId: string;
    readonly amount: number;
    readonly name: string;
    readonly comment: string;
    readonly url?: string;
    readonly displayOrder: number;
    readonly createdTime?: number;
    readonly updatedTime?: number;
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
    // the wishes attached to this month
    readonly wishes: BudgetPlanItemInfoResponse[];
}
