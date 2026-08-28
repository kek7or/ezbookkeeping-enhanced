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

export interface BudgetPlanGetRequest {
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

export interface BudgetPlanInfoResponse {
    readonly year: number;
    readonly month: number;
    readonly items: BudgetPlanItemInfoResponse[];
    readonly adjustments: BudgetPlanAdjustmentInfoResponse[];
}
