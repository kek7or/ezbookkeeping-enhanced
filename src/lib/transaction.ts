import { CategoryType } from '@/core/category.ts';
import { TransactionType } from '@/core/transaction.ts';
import { Account } from '@/models/account.ts';
import { TransactionCategory } from '@/models/transaction_category.ts';
import { TransactionTag } from '@/models/transaction_tag.ts';
import { TransactionPicture, type TransactionPictureInfoBasicResponse } from '@/models/transaction_picture_info.ts';
import { Transaction, TransactionReceiptGroup, type TransactionListRow } from '@/models/transaction.ts';

import {
    isDefined,
    isNumber
} from './common.ts';
import {
    getTimezoneOffsetMinutes
} from './datetime.ts';
import {
    categoryTypeToTransactionType,
    isSubCategoryIdAvailable,
    getFirstVisibleCategoryId,
    getFirstAvailableSubCategoryId
} from './category.ts';

export interface SetTransactionOptions {
    time?: number;
    type?: number;
    categoryId?: string;
    accountId?: string;
    destinationAccountId?: string;
    amount?: number;
    destinationAmount?: number;
    tagIds?: string;
    comment?: string;
}

export function* allTransactionPictures(transactions: Transaction[]): Iterable<[Transaction, TransactionPictureInfoBasicResponse]> {
    for (const transaction of transactions) {
        if (transaction.pictures) {
            for (const pictureInfo of transaction.pictures) {
                yield [transaction, pictureInfo];
            }
        }
    }
}

export function setTransactionModelByTransaction(transaction: Transaction, transaction2: Transaction | null | undefined, allCategories: Record<number, TransactionCategory[]>, allCategoriesMap: Record<string, TransactionCategory>, allVisibleAccounts: Account[], allAccountsMap: Record<string, Account>, allTagsMap: Record<string, TransactionTag>, defaultAccountId: string, options: SetTransactionOptions, setContextData: boolean): void {
    if (isDefined(options.time)) {
        transaction.time = options.time;
        transaction.utcOffset = getTimezoneOffsetMinutes(transaction.time, transaction.timeZone);
    }

    if (options.type === TransactionType.Income || options.type === TransactionType.Expense || options.type === TransactionType.Transfer) {
        transaction.type = options.type;
    } else if (!options.type && options.categoryId && options.categoryId !== '0' && allCategoriesMap[options.categoryId]) {
        const category = allCategoriesMap[options.categoryId] as TransactionCategory;
        const type = categoryTypeToTransactionType(category.type);

        if (isNumber(type)) {
            transaction.type = type;
        }
    }

    if (isDefined(options.amount)) {
        transaction.sourceAmount = options.amount;
    }

    if (isDefined(options.destinationAmount)) {
        transaction.destinationAmount = options.destinationAmount;
    }

    if (allCategories[CategoryType.Expense] &&
        allCategories[CategoryType.Expense].length) {
        if (options.categoryId && options.categoryId !== '0') {
            if (isSubCategoryIdAvailable(allCategories[CategoryType.Expense], options.categoryId)) {
                transaction.expenseCategoryId = options.categoryId;
            } else {
                transaction.expenseCategoryId = getFirstAvailableSubCategoryId(allCategories[CategoryType.Expense], options.categoryId);
            }
        }

        if (!transaction.expenseCategoryId) {
            transaction.expenseCategoryId = getFirstVisibleCategoryId(allCategories[CategoryType.Expense]);
        }
    }

    if (allCategories[CategoryType.Income] &&
        allCategories[CategoryType.Income].length) {
        if (options.categoryId && options.categoryId !== '0') {
            if (isSubCategoryIdAvailable(allCategories[CategoryType.Income], options.categoryId)) {
                transaction.incomeCategoryId = options.categoryId;
            } else {
                transaction.incomeCategoryId = getFirstAvailableSubCategoryId(allCategories[CategoryType.Income], options.categoryId);
            }
        }

        if (!transaction.incomeCategoryId) {
            transaction.incomeCategoryId = getFirstVisibleCategoryId(allCategories[CategoryType.Income]);
        }
    }

    if (allCategories[CategoryType.Transfer] &&
        allCategories[CategoryType.Transfer].length) {
        if (options.categoryId && options.categoryId !== '0') {
            if (isSubCategoryIdAvailable(allCategories[CategoryType.Transfer], options.categoryId)) {
                transaction.transferCategoryId = options.categoryId;
            } else {
                transaction.transferCategoryId = getFirstAvailableSubCategoryId(allCategories[CategoryType.Transfer], options.categoryId);
            }
        }

        if (!transaction.transferCategoryId) {
            transaction.transferCategoryId = getFirstVisibleCategoryId(allCategories[CategoryType.Transfer]);
        }
    }

    if (allVisibleAccounts.length) {
        if (options.accountId && options.accountId !== '0') {
            for (const account of allVisibleAccounts) {
                if (account.id === options.accountId) {
                    transaction.sourceAccountId = options.accountId;
                    transaction.destinationAccountId = options.accountId;
                    break;
                }
            }
        }

        if (options.destinationAccountId && options.destinationAccountId !== '0') {
            for (const account of allVisibleAccounts) {
                if (account.id === options.destinationAccountId) {
                    transaction.destinationAccountId = options.destinationAccountId;
                    break;
                }
            }
        }

        if (!transaction.sourceAccountId) {
            if (defaultAccountId && allAccountsMap[defaultAccountId] && !allAccountsMap[defaultAccountId].hidden) {
                transaction.sourceAccountId = defaultAccountId;
            } else {
                transaction.sourceAccountId = allVisibleAccounts[0]!.id;
            }
        }

        if (!transaction.destinationAccountId) {
            if (defaultAccountId && allAccountsMap[defaultAccountId] && !allAccountsMap[defaultAccountId].hidden) {
                transaction.destinationAccountId = defaultAccountId;
            } else {
                transaction.destinationAccountId = allVisibleAccounts[0]!.id;
            }
        }
    }

    if (allTagsMap && options.tagIds) {
        const tagIds = options.tagIds.split(',');
        const finalTagIds = [];

        for (const tagId of tagIds) {
            const tag = allTagsMap[tagId];

            if (tag && !tag.hidden) {
                finalTagIds.push(tag.id);
            }
        }

        transaction.tagIds = finalTagIds;
    }

    if (options.comment) {
        transaction.comment = options.comment;
    }

    if (transaction2) {
        if (setContextData) {
            transaction.id = transaction2.id;
        }

        transaction.type = transaction2.type;

        if (transaction.type === TransactionType.Expense) {
            transaction.expenseCategoryId = transaction2.categoryId || '';
        } else if (transaction.type === TransactionType.Income) {
            transaction.incomeCategoryId = transaction2.categoryId || '';
        } else if (transaction.type === TransactionType.Transfer) {
            transaction.transferCategoryId = transaction2.categoryId || '';
        }

        if (setContextData) {
            transaction.time = transaction2.time;
            transaction.timeZone = transaction2.timeZone;
            transaction.utcOffset = transaction2.utcOffset;
        }

        transaction.sourceAccountId = transaction2.sourceAccountId;

        if (transaction2.destinationAccountId) {
            transaction.destinationAccountId = transaction2.destinationAccountId;
        } else {
            transaction.destinationAccountId = '';
        }

        transaction.sourceAmount = transaction2.sourceAmount;

        if (transaction2.destinationAmount) {
            transaction.destinationAmount = transaction2.destinationAmount;
        } else {
            transaction.destinationAmount = 0;
        }

        transaction.hideAmount = transaction2.hideAmount;
        transaction.tagIds = transaction2.tagIds || [];
        transaction.setPictures(TransactionPicture.ofMulti(transaction2.pictures || []));

        transaction.comment = transaction2.comment;

        if (setContextData) {
            transaction.setGeoLocation(transaction2.geoLocation);
            // the receipt lines belong to that one transaction, so they are carried only when this is
            // that transaction being opened - a draft or a template seeded from it starts without them
            transaction.lineItems = transaction2.lineItems;
            // the receipt is carried on the same terms: a duplicate is a new purchase, and claiming
            // the shopping trip it was copied from would group it with transactions it has nothing
            // to do with
            transaction.receipt = transaction2.receipt;
        }
    }
}

// groupTransactionsByReceipt turns a list of transactions into the rows the transaction list shows,
// collapsing the transactions of one shopping trip into a single row that opens to reveal them.
//
// The order the transactions came in is kept exactly: a trip takes the place of the first of its
// transactions, and everything else stays where it was. This matters because the list is sorted by
// time and the caller is free to append more pages to it - grouping must never move a row past a day
// boundary it did not already belong to.
//
// A receipt that produced only one transaction is left as an ordinary row. So is one whose other
// transactions have not been loaded yet: a shopping trip split across two pages shows the part that
// is here, and becomes a group once the rest arrives.
export function groupTransactionsByReceipt(transactions: Transaction[]): TransactionListRow[] {
    const transactionsByReceiptId: Record<string, Transaction[]> = {};

    for (const transaction of transactions) {
        if (!transaction.receipt) {
            continue;
        }

        const receiptTransactions: Transaction[] = transactionsByReceiptId[transaction.receipt.id] ?? [];
        receiptTransactions.push(transaction);
        transactionsByReceiptId[transaction.receipt.id] = receiptTransactions;
    }

    const rows: TransactionListRow[] = [];
    const emittedReceiptIds: Record<string, boolean> = {};

    for (const transaction of transactions) {
        const receipt = transaction.receipt;

        if (!receipt) {
            rows.push({
                key: transaction.id,
                transaction: transaction
            });
            continue;
        }

        const receiptTransactions = transactionsByReceiptId[receipt.id] as Transaction[];

        if (receiptTransactions.length < 2) {
            rows.push({
                key: transaction.id,
                transaction: transaction
            });
            continue;
        }

        if (emittedReceiptIds[receipt.id]) {
            continue;
        }

        emittedReceiptIds[receipt.id] = true;

        rows.push({
            key: `receipt_${receipt.id}`,
            transaction: receiptTransactions[0] as Transaction,
            receiptGroup: new TransactionReceiptGroup(receipt, receiptTransactions)
        });
    }

    return rows;
}
