import { TransactionType } from '@/core/transaction.ts';

import {
    ImportTransaction,
    type ImportTransactionResponse,
    type ImportTransactionResponsePageWrapper,
    type ImportReceiptLineItemResponse,
    type ReceiptLineItemCategoryRememberItem
} from './imported_transaction.ts';

let nextLineItemId: number = 0;
let nextCategoryGroupId: number = 0;

// ImportReceiptLineItem is one purchased line of a receipt, as the model read it.
//
// The identity is a counter rather than the name or the position, because a receipt may print the
// same article twice and either copy may be dragged somewhere else, so the two have to stay apart.
export class ImportReceiptLineItem {
    public readonly id: string;
    public readonly originalCategoryName: string;
    public readonly refund: boolean;
    // whether this line was already filed where the user put it the last time they bought this
    // article, rather than where the model guessed it belonged
    public readonly remembered: boolean;
    public name: string;
    public amount: number;

    private constructor(response: ImportReceiptLineItemResponse) {
        this.id = `receiptLineItem_${nextLineItemId++}`;
        this.originalCategoryName = response.categoryName;
        this.refund = !!response.refund;
        this.remembered = !!response.remembered;
        this.name = response.name;
        this.amount = response.amount;
    }

    public static of(response: ImportReceiptLineItemResponse): ImportReceiptLineItem {
        return new ImportReceiptLineItem(response);
    }
}

// ImportReceiptCategoryGroup is the set of receipt lines booked to one category, which is imported as
// a single transaction whose amount is the sum of its lines
export class ImportReceiptCategoryGroup {
    public readonly id: string;
    public readonly originalCategoryName: string;
    // whether this group holds what was handed back at the till rather than what was bought. Such a
    // group is kept apart from the purchases of the same category, so that a returned bag of empties
    // is a transaction of its own instead of cancelling out the drinks bought on the same receipt.
    public readonly refund: boolean;
    public categoryId: string;
    public tagIds: string[];
    public lineItems: ImportReceiptLineItem[];
    public selected: boolean;

    public constructor(categoryId: string, originalCategoryName: string, lineItems: ImportReceiptLineItem[], refund: boolean = false) {
        this.id = `receiptCategoryGroup_${nextCategoryGroupId++}`;
        this.originalCategoryName = originalCategoryName;
        this.refund = refund;
        this.categoryId = categoryId;
        this.tagIds = [];
        this.lineItems = lineItems;
        this.selected = true;
    }

    // the sum is over minor units, so moving a line between categories can never introduce a rounding
    // difference the way summing formatted amounts would
    public get totalAmount(): number {
        let totalAmount = 0;

        for (const lineItem of this.lineItems) {
            totalAmount += lineItem.amount;
        }

        return totalAmount;
    }

    // the description always follows the lines it is made of, so that a line moved to another category
    // is named in the transaction it now belongs to and nowhere else
    public get description(): string {
        return this.lineItems.map(lineItem => lineItem.name).filter(name => !!name).join(', ');
    }

    // a group that came to nothing has nothing to import, but one that came to less than nothing does:
    // that is the money handed back at the till, booked as a negative expense against the category the
    // deposit was charged to, so that the two cancel each other out over time
    public get isImportable(): boolean {
        return this.lineItems.length > 0 && this.totalAmount !== 0;
    }
}

// ImportReceipt is one recognized receipt image: the lines read off it, grouped into the transactions
// they will be imported as. Everything a receipt states once - when it was paid and from which account
// - is held here rather than on each group, because every transaction of one receipt shares it.
export class ImportReceipt {
    public readonly index: number;
    public readonly fileName: string;
    public readonly type: TransactionType;
    public readonly utcOffset: number;
    // when the shopping was paid for. Every transaction of one receipt is booked at this one time, so
    // correcting a date the model misread on the image corrects all of them at once.
    public time: number;
    public readonly originalSourceAccountName: string;
    public readonly originalSourceAccountCurrency: string;
    public sourceAccountId: string;
    public categoryGroups: ImportReceiptCategoryGroup[];

    private constructor(index: number, fileName: string, firstTransaction: ImportTransactionResponse, categoryGroups: ImportReceiptCategoryGroup[]) {
        this.index = index;
        this.fileName = fileName;
        this.type = TransactionType.Expense;
        this.time = firstTransaction.time;
        this.utcOffset = firstTransaction.utcOffset;
        this.originalSourceAccountName = firstTransaction.originalSourceAccountName;
        this.originalSourceAccountCurrency = firstTransaction.originalSourceAccountCurrency;
        this.sourceAccountId = firstTransaction.sourceAccountId;
        this.categoryGroups = categoryGroups;
    }

    public get totalAmount(): number {
        let totalAmount = 0;

        for (const categoryGroup of this.categoryGroups) {
            totalAmount += categoryGroup.totalAmount;
        }

        return totalAmount;
    }

    public get lineItemCount(): number {
        let count = 0;

        for (const categoryGroup of this.categoryGroups) {
            count += categoryGroup.lineItems.length;
        }

        return count;
    }

    public addCategoryGroup(categoryId: string, categoryName: string): ImportReceiptCategoryGroup {
        const categoryGroup = new ImportReceiptCategoryGroup(categoryId, categoryName, []);
        this.categoryGroups.push(categoryGroup);
        return categoryGroup;
    }

    // the transactions are rebuilt from the groups rather than patched, so that what is imported is
    // always exactly what the groups currently show
    public toImportTransactions(startIndex: number): ImportTransaction[] {
        const transactions: ImportTransaction[] = [];

        for (const categoryGroup of this.categoryGroups) {
            if (!categoryGroup.isImportable) {
                continue;
            }

            const transaction = ImportTransaction.of({
                type: this.type,
                categoryId: categoryGroup.categoryId,
                originalCategoryName: categoryGroup.originalCategoryName,
                time: this.time,
                utcOffset: this.utcOffset,
                sourceAccountId: this.sourceAccountId,
                originalSourceAccountName: this.originalSourceAccountName,
                originalSourceAccountCurrency: this.originalSourceAccountCurrency,
                sourceAmount: categoryGroup.totalAmount,
                tagIds: categoryGroup.tagIds,
                originalTagNames: [],
                comment: categoryGroup.description
            }, startIndex + transactions.length);

            transaction.selected = categoryGroup.selected;
            transactions.push(transaction);
        }

        return transactions;
    }

    // toRememberedLineItemCategories returns what this receipt taught the import: every line that will
    // be imported, and the category it ended up in.
    //
    // Every line is reported rather than only the ones the user moved. A line the model happened to
    // file correctly this time is no more certain to be filed correctly the next time - the whole
    // point is that a category once settled stops depending on the model at all.
    //
    // A group the user emptied or unselected teaches nothing, because they did not accept it.
    public toRememberedLineItemCategories(): ReceiptLineItemCategoryRememberItem[] {
        const items: ReceiptLineItemCategoryRememberItem[] = [];

        for (const categoryGroup of this.categoryGroups) {
            if (!categoryGroup.isImportable || !categoryGroup.selected || !categoryGroup.categoryId || categoryGroup.categoryId === '0') {
                continue;
            }

            for (const lineItem of categoryGroup.lineItems) {
                if (!lineItem.name) {
                    continue;
                }

                items.push({
                    name: lineItem.name,
                    categoryId: categoryGroup.categoryId
                });
            }
        }

        return items;
    }

    // of returns the receipt one recognized image was read as, or undefined when the response holds
    // nothing that can be edited line by line - a text import, an image the model answered with whole
    // transactions instead of receipt lines, or a receipt whose lines the server could not return.
    public static of(response: ImportTransactionResponsePageWrapper, index: number, fileName: string): ImportReceipt | undefined {
        const lineItems = response.receipt?.lineItems;

        if (!lineItems || lineItems.length < 1 || !response.items || response.items.length < 1) {
            return undefined;
        }

        const firstTransaction = response.items[0] as ImportTransactionResponse;

        // every line of a receipt is an expense booked against the account that paid for it, and the
        // grouping below relies on that, so anything else is left to the ordinary import table
        for (const transaction of response.items) {
            if (transaction.type !== TransactionType.Expense
                || transaction.time !== firstTransaction.time
                || transaction.sourceAccountId !== firstTransaction.sourceAccountId) {
                return undefined;
            }
        }

        // the category a line was assigned to is only a name, and the transaction the server built
        // from that group is what already resolved it to a category of this user
        const categoryIdsByCategoryName: Record<string, string> = {};

        for (const transaction of response.items) {
            if (!(transaction.originalCategoryName in categoryIdsByCategoryName)) {
                categoryIdsByCategoryName[transaction.originalCategoryName] = transaction.categoryId;
            }
        }

        const categoryGroups: ImportReceiptCategoryGroup[] = [];
        const categoryGroupsByKey: Record<string, ImportReceiptCategoryGroup> = {};

        // the lines are grouped exactly as the server grouped them into transactions - by category, and
        // by whether they were charged or handed back - so that what the user sees adds up to what would
        // be imported if they changed nothing
        for (const lineItem of lineItems) {
            const refund = !!lineItem.refund;
            const categoryGroupKey = `${refund ? 'refund' : 'purchase'}:${lineItem.categoryName}`;
            let categoryGroup = categoryGroupsByKey[categoryGroupKey];

            if (!categoryGroup) {
                categoryGroup = new ImportReceiptCategoryGroup(categoryIdsByCategoryName[lineItem.categoryName] ?? '0', lineItem.categoryName, [], refund);
                categoryGroupsByKey[categoryGroupKey] = categoryGroup;
                categoryGroups.push(categoryGroup);
            }

            categoryGroup.lineItems.push(ImportReceiptLineItem.of(lineItem));
        }

        return new ImportReceipt(index, fileName, firstTransaction, categoryGroups);
    }
}
