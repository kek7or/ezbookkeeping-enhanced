import { describe, expect, test } from 'vitest';

import { splitAmountEvenly, sumDebtAmountsByCurrency, groupDebtEntries } from '@/models/debt.ts';
import type { DebtEntryInfoResponse } from '@/models/debt.ts';

function createEntry(amount: number, currency: string): DebtEntryInfoResponse {
    return {
        id: '1',
        personId: '1',
        transactionId: '1',
        amount: amount,
        currency: currency,
        time: 0
    };
}

function createReceiptEntry(id: string, receiptId: string | undefined, amount: number, settled?: boolean): DebtEntryInfoResponse {
    return {
        id: id,
        personId: '1',
        transactionId: `t${id}`,
        amount: amount,
        currency: 'EUR',
        time: 0,
        settled: settled,
        receiptId: receiptId,
        merchantName: receiptId ? 'LIDL' : undefined
    };
}

function createWholeTransactionEntry(id: string, receiptId: string | undefined, transactionId: string, amount: number): DebtEntryInfoResponse {
    return {
        id: id,
        personId: '1',
        transactionId: transactionId,
        amount: amount,
        currency: 'EUR',
        time: 0,
        receiptId: receiptId,
        merchantName: receiptId ? 'LIDL' : undefined
    };
}

// a position is owed through the transaction it was summed into, and says which article of it it is
function createPositionEntry(id: string, receiptId: string | undefined, transactionId: string, amount: number, settled?: boolean): DebtEntryInfoResponse {
    return {
        ...createWholeTransactionEntry(id, receiptId, transactionId, amount),
        lineItemId: `l${id}`,
        name: `Article ${id}`,
        settled: settled
    };
}

describe('splitAmountEvenly', () => {
    test('divides an amount that divides evenly', () => {
        expect(splitAmountEvenly(3000, 3)).toEqual([1000, 1000, 1000]);
    });

    test('hands the cents that do not divide to the front', () => {
        expect(splitAmountEvenly(1000, 3)).toEqual([334, 333, 333]);
        expect(splitAmountEvenly(1001, 3)).toEqual([334, 334, 333]);
    });

    test('keeps the shares adding up to what was divided', () => {
        for (let amount = 0; amount < 200; amount++) {
            for (let shareCount = 1; shareCount <= 7; shareCount++) {
                const shares = splitAmountEvenly(amount, shareCount);
                const total = shares.reduce((sum, share) => sum + share, 0);

                expect(shares.length).toEqual(shareCount);
                expect(total).toEqual(amount);
            }
        }
    });

    test('gives one share the whole amount when nobody else is on it', () => {
        expect(splitAmountEvenly(699, 1)).toEqual([699]);
    });

    test('divides nothing into nothing', () => {
        expect(splitAmountEvenly(0, 3)).toEqual([0, 0, 0]);
    });

    test('answers an empty split with no shares', () => {
        expect(splitAmountEvenly(1000, 0)).toEqual([]);
    });
});

describe('sumDebtAmountsByCurrency', () => {
    test('totals each currency on its own', () => {
        const totals = sumDebtAmountsByCurrency([
            createEntry(1000, 'EUR'),
            createEntry(250, 'USD'),
            createEntry(699, 'EUR')
        ]);

        expect(totals).toEqual([
            { currency: 'EUR', amount: 1699 },
            { currency: 'USD', amount: 250 }
        ]);
    });

    test('totals nothing to nothing', () => {
        expect(sumDebtAmountsByCurrency([])).toEqual([]);
    });
});

describe('groupDebtEntries', () => {
    test('collapses everything owed off one receipt into one row', () => {
        const rows = groupDebtEntries([
            createReceiptEntry('1', 'r1', 40),
            createReceiptEntry('2', 'r1', 47),
            createReceiptEntry('3', 'r1', 30)
        ]);

        expect(rows.length).toEqual(1);
        expect(rows[0]!.key).toEqual('receipt_r1');
        expect(rows[0]!.group!.kind).toEqual('receipt');
        expect(rows[0]!.group!.merchantName).toEqual('LIDL');
        expect(rows[0]!.group!.totalAmount).toEqual(117);
        expect(rows[0]!.group!.entries.length).toEqual(3);
        expect(rows[0]!.group!.rows.length).toEqual(3);
    });

    test('leaves a receipt only one thing is owed off as an ordinary row', () => {
        const rows = groupDebtEntries([
            createReceiptEntry('1', 'r1', 40),
            createReceiptEntry('2', 'r2', 47),
            createReceiptEntry('3', 'r2', 30)
        ]);

        expect(rows.length).toEqual(2);
        expect(rows[0]!.key).toEqual('1');
        expect(rows[0]!.group).toBeUndefined();
        expect(rows[1]!.key).toEqual('receipt_r2');
    });

    test('leaves what has no receipt alone', () => {
        const rows = groupDebtEntries([
            createReceiptEntry('1', undefined, 2500),
            createReceiptEntry('2', 'r1', 40),
            createReceiptEntry('3', 'r1', 47)
        ]);

        expect(rows.length).toEqual(2);
        expect(rows[0]!.key).toEqual('1');
        expect(rows[1]!.key).toEqual('receipt_r1');
    });

    test('keeps the order it was given, a trip standing where its first thing stood', () => {
        const rows = groupDebtEntries([
            createReceiptEntry('1', 'r1', 40),
            createReceiptEntry('2', undefined, 2500),
            createReceiptEntry('3', 'r1', 47)
        ]);

        expect(rows.map(row => row.key)).toEqual(['receipt_r1', '2']);
    });

    test('counts only what is still open as tickable', () => {
        const rows = groupDebtEntries([
            createReceiptEntry('1', 'r1', 40, true),
            createReceiptEntry('2', 'r1', 47),
            createReceiptEntry('3', 'r1', 30)
        ]);

        expect(rows[0]!.group!.openEntryIds).toEqual(['2', '3']);
        expect(rows[0]!.group!.totalAmount).toEqual(117);
    });

    test('gathers the positions owed of one transaction under that transaction', () => {
        const rows = groupDebtEntries([
            createPositionEntry('1', 'r1', 't1', 40),
            createPositionEntry('2', 'r1', 't1', 47),
            createReceiptEntry('3', 'r1', 30)
        ]);

        expect(rows.length).toEqual(1);

        const receiptGroup = rows[0]!.group!;
        expect(receiptGroup.kind).toEqual('receipt');
        expect(receiptGroup.totalAmount).toEqual(117);
        expect(receiptGroup.entries.map(entry => entry.id)).toEqual(['1', '2', '3']);
        expect(receiptGroup.rows.map(row => row.key)).toEqual(['transaction_t1', '3']);

        const transactionGroup = receiptGroup.rows[0]!.group!;
        expect(transactionGroup.kind).toEqual('transaction');
        expect(transactionGroup.id).toEqual('t1');
        expect(transactionGroup.totalAmount).toEqual(87);
        expect(transactionGroup.openEntryIds).toEqual(['1', '2']);
    });

    test('leaves a transaction only one position of which is owed as an ordinary row', () => {
        const rows = groupDebtEntries([
            createPositionEntry('1', 'r1', 't1', 40),
            createPositionEntry('2', 'r1', 't2', 47)
        ]);

        expect(rows[0]!.group!.rows.map(row => row.key)).toEqual(['1', '2']);
    });

    test('never gathers what is owed whole, however many of them share a transaction', () => {
        const rows = groupDebtEntries([
            createWholeTransactionEntry('1', 'r1', 't1', 40),
            createWholeTransactionEntry('2', 'r1', 't1', 47)
        ]);

        expect(rows[0]!.group!.rows.map(row => row.key)).toEqual(['1', '2']);
    });

    test('stands a transaction beside what is owed whole of the same transaction', () => {
        const rows = groupDebtEntries([
            createWholeTransactionEntry('1', 'r1', 't1', 100),
            createPositionEntry('2', 'r1', 't1', 40),
            createPositionEntry('3', 'r1', 't1', 47)
        ]);

        expect(rows[0]!.group!.rows.map(row => row.key)).toEqual(['1', 'transaction_t1']);
    });

    test('gathers positions that belong to no receipt at all', () => {
        const rows = groupDebtEntries([
            createPositionEntry('1', undefined, 't1', 40),
            createPositionEntry('2', undefined, 't1', 47)
        ]);

        expect(rows.length).toEqual(1);
        expect(rows[0]!.key).toEqual('transaction_t1');
        expect(rows[0]!.group!.totalAmount).toEqual(87);
    });

    test('groups nothing into nothing', () => {
        expect(groupDebtEntries([])).toEqual([]);
    });
});
