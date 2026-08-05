import { TransactionType } from '@/core/transaction.ts';

import {
    ImportTransaction,
    type ImportTransactionResponse,
    type ImportTransactionResponsePageWrapper,
    type ImportReceiptLineItemResponse
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
    public name: string;
    public amount: number;

    private constructor(response: ImportReceiptLineItemResponse) {
        this.id = `receiptLineItem_${nextLineItemId++}`;
        this.originalCategoryName = response.categoryName;
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
    public categoryId: string;
    public tagIds: string[];
    public lineItems: ImportReceiptLineItem[];
    public selected: boolean;

    public constructor(categoryId: string, originalCategoryName: string, lineItems: ImportReceiptLineItem[]) {
        this.id = `receiptCategoryGroup_${nextCategoryGroupId++}`;
        this.originalCategoryName = originalCategoryName;
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

    public get isImportable(): boolean {
        return this.lineItems.length > 0 && this.totalAmount > 0;
    }
}

// ImportReceipt is one recognized receipt image: the lines read off it, grouped into the transactions
// they will be imported as. Everything a receipt states once - when it was paid and from which account
// - is held here rather than on each group, because every transaction of one receipt shares it.
export class ImportReceipt {
    public readonly index: number;
    public readonly fileName: string;
    public readonly type: TransactionType;
    public readonly time: number;
    public readonly utcOffset: number;
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
        const categoryGroupsByCategoryName: Record<string, ImportReceiptCategoryGroup> = {};

        for (const lineItem of lineItems) {
            let categoryGroup = categoryGroupsByCategoryName[lineItem.categoryName];

            if (!categoryGroup) {
                categoryGroup = new ImportReceiptCategoryGroup(categoryIdsByCategoryName[lineItem.categoryName] ?? '0', lineItem.categoryName, []);
                categoryGroupsByCategoryName[lineItem.categoryName] = categoryGroup;
                categoryGroups.push(categoryGroup);
            }

            categoryGroup.lineItems.push(ImportReceiptLineItem.of(lineItem));
        }

        return new ImportReceipt(index, fileName, firstTransaction, categoryGroups);
    }
}
