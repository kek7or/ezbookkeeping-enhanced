import { TransactionType } from '@/core/transaction.ts';
import { TRANSACTION_MAX_COMMENT_LENGTH } from '@/consts/transaction.ts';

import type { TransactionCreateRequest, TransactionGeoLocationResponse, TransactionReceiptLineItem } from './transaction.ts';

export class ImportTransaction implements ImportTransactionResponse {
    public type: number;
    public categoryId: string;
    public originalCategoryName: string;
    public time: number;
    public utcOffset: number;
    public sourceAccountId: string;
    public originalSourceAccountName: string;
    public originalSourceAccountCurrency: string;
    public destinationAccountId: string;
    public originalDestinationAccountName?: string;
    public originalDestinationAccountCurrency?: string;
    public sourceAmount: number;
    public destinationAmount: number;
    public tagIds: string[];
    public originalTagNames: string[];
    public comment: string;
    public geoLocation?: TransactionGeoLocationResponse;

    public actualCategoryName: string;
    public actualSourceAccountName: string;
    public actualDestinationAccountName?: string;
    public index: number;
    public selected: boolean;
    public valid: boolean;
    // the receipt lines this transaction is the sum of, set only when it was built from a recognized
    // receipt, and kept so that the transaction can still answer what its amount is made of after the
    // import dialog is gone
    public lineItems?: TransactionReceiptLineItem[];

    private constructor(response: ImportTransactionResponse, index: number) {
        this.type = response.type;
        this.categoryId = response.categoryId;
        this.originalCategoryName = response.originalCategoryName;
        this.time = response.time;
        this.utcOffset = response.utcOffset;
        this.sourceAccountId = response.sourceAccountId;
        this.originalSourceAccountName = response.originalSourceAccountName;
        this.originalSourceAccountCurrency = response.originalSourceAccountCurrency;
        this.destinationAccountId = response.destinationAccountId || '';
        this.originalDestinationAccountName = response.originalDestinationAccountName;
        this.originalDestinationAccountCurrency = response.originalDestinationAccountCurrency;
        this.sourceAmount = response.sourceAmount;
        this.destinationAmount = response.destinationAmount || 0;
        this.tagIds = response.tagIds || [];
        this.originalTagNames = response.originalTagNames || [];
        this.comment = response.comment;
        this.geoLocation = response.geoLocation;

        this.actualCategoryName = response.originalCategoryName;
        this.actualSourceAccountName = response.originalSourceAccountName;
        this.actualDestinationAccountName = response.originalDestinationAccountName;
        this.index = index;
        this.selected = false;
        this.valid = this.isTransactionValid();
    }

    public toCreateRequest(): TransactionCreateRequest {
        return {
            type: this.type,
            categoryId: this.categoryId,
            time: this.time,
            utcOffset: this.utcOffset,
            sourceAccountId: this.sourceAccountId,
            destinationAccountId: this.type === TransactionType.Transfer ? this.destinationAccountId : '0',
            sourceAmount: this.sourceAmount,
            destinationAmount: this.type === TransactionType.Transfer ? this.destinationAmount : 0,
            hideAmount: false,
            tagIds: this.tagIds,
            pictureIds: [],
            comment: this.comment,
            geoLocation: this.geoLocation,
            clientSessionId: '',
            lineItems: this.lineItems
        };
    }

    public isTransactionValid(): boolean {
        if (this.type !== TransactionType.ModifyBalance && (!this.categoryId || this.categoryId === '0')) {
            return false;
        }

        if (!this.sourceAccountId || this.sourceAccountId === '0') {
            return false;
        }

        if (this.type === TransactionType.Transfer && (!this.destinationAccountId || this.destinationAccountId === '0')) {
            return false;
        }

        if (this.tagIds && this.tagIds.length) {
            for (const tagId of this.tagIds) {
                if (!tagId || tagId === '0') {
                    return false;
                }
            }
        }

        if (this.comment && this.comment.length > TRANSACTION_MAX_COMMENT_LENGTH) {
            return false;
        }

        return true;
    }

    public static of(response: ImportTransactionResponse, index: number): ImportTransaction {
        return new ImportTransaction(response, index);
    }
}

export interface ImportTransactionRequest {
    readonly transactions: ImportTransactionRequestItem[];
}

export interface ImportTransactionRequestItem {
    readonly time: string;
    readonly utcOffset: string;
    readonly type: string;
    readonly categoryName?: string;
    readonly sourceAccountName?: string;
    readonly destinationAccountName?: string;
    readonly sourceAmount: string;
    readonly destinationAmount?: string;
    readonly geoLocation?: string;
    readonly tagNames?: string;
    readonly comment?: string;
}

export interface ImportTransactionResponse {
    readonly type: number;
    readonly categoryId: string;
    readonly originalCategoryName: string;
    readonly time: number;
    readonly utcOffset: number;
    readonly sourceAccountId: string;
    readonly originalSourceAccountName: string;
    readonly originalSourceAccountCurrency: string;
    readonly destinationAccountId?: string;
    readonly originalDestinationAccountName?: string;
    readonly originalDestinationAccountCurrency?: string;
    readonly sourceAmount: number;
    readonly destinationAmount?: number;
    readonly tagIds: string[];
    readonly originalTagNames: string[];
    readonly comment: string;
    readonly geoLocation?: TransactionGeoLocationResponse;
}

export type ImportTransactionWarningType = 'receiptTotalMismatch' | 'receiptLinesNotItemized';

export interface ImportTransactionWarningResponse {
    readonly type: ImportTransactionWarningType;
    // how many lines the warning is about: the recognized ones for "receiptTotalMismatch",
    // the lost ones for "receiptLinesNotItemized"
    readonly lineItemCount?: number;
    readonly calculatedTotal?: string;
    readonly statedTotal?: string;
    readonly difference?: string;
    // the printed lines that were read off the receipt but never turned into a transaction, absent
    // when they could not be identified individually
    readonly missingLines?: string[];
}

// a single line read off a receipt, with the deposits and discounts printed under it already charged
// against it. The amount is in minor units, exactly like ImportTransactionResponse.sourceAmount, so
// that regrouping the lines is integer arithmetic and cannot drift from what the server parsed.
export interface ImportReceiptLineItemResponse {
    readonly name: string;
    readonly amount: number;
    readonly categoryName: string;
    // whether this line hands money back rather than charging for a purchase, which keeps it in a
    // transaction of its own instead of cancelling out the purchases of the same category
    readonly refund?: boolean;
    // whether the category is the user's own answer from an earlier receipt rather than the model's
    // guess, so that the lines they did not have to categorize can be marked as such
    readonly remembered?: boolean;
}

// ReceiptLineItemCategoryRememberRequest is what the import learned from a receipt the user imported:
// which category each of its lines ended up in, which is where a line of that article will start next
// time it is bought
export interface ReceiptLineItemCategoryRememberRequest {
    readonly items: ReceiptLineItemCategoryRememberItem[];
}

export interface ReceiptLineItemCategoryRememberItem {
    readonly name: string;
    readonly categoryId: string;
}

export interface ImportReceiptResponse {
    readonly lineItems: ImportReceiptLineItemResponse[];
}

export interface ImportTransactionResponsePageWrapper {
    readonly items: ImportTransactionResponse[];
    readonly totalCount: number;
    readonly warnings?: ImportTransactionWarningResponse[];
    // the individual receipt lines the items were aggregated from, absent for every import that is
    // not a receipt image
    readonly receipt?: ImportReceiptResponse;
}
