import { describe, expect, test } from 'vitest';

import { splitAmountEvenly, sumDebtAmountsByCurrency, groupDebtEntriesByReceipt } from '@/models/debt.ts';
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

describe('groupDebtEntriesByReceipt', () => {
    test('collapses everything owed off one receipt into one row', () => {
        const rows = groupDebtEntriesByReceipt([
            createReceiptEntry('1', 'r1', 40),
            createReceiptEntry('2', 'r1', 47),
            createReceiptEntry('3', 'r1', 30)
        ]);

        expect(rows.length).toEqual(1);
        expect(rows[0]!.key).toEqual('receipt_r1');
        expect(rows[0]!.receiptGroup!.merchantName).toEqual('LIDL');
        expect(rows[0]!.receiptGroup!.totalAmount).toEqual(117);
        expect(rows[0]!.receiptGroup!.entries.length).toEqual(3);
    });

    test('leaves a receipt only one thing is owed off as an ordinary row', () => {
        const rows = groupDebtEntriesByReceipt([
            createReceiptEntry('1', 'r1', 40),
            createReceiptEntry('2', 'r2', 47),
            createReceiptEntry('3', 'r2', 30)
        ]);

        expect(rows.length).toEqual(2);
        expect(rows[0]!.key).toEqual('1');
        expect(rows[0]!.receiptGroup).toBeUndefined();
        expect(rows[1]!.key).toEqual('receipt_r2');
    });

    test('leaves what has no receipt alone', () => {
        const rows = groupDebtEntriesByReceipt([
            createReceiptEntry('1', undefined, 2500),
            createReceiptEntry('2', 'r1', 40),
            createReceiptEntry('3', 'r1', 47)
        ]);

        expect(rows.length).toEqual(2);
        expect(rows[0]!.key).toEqual('1');
        expect(rows[1]!.key).toEqual('receipt_r1');
    });

    test('keeps the order it was given, a trip standing where its first thing stood', () => {
        const rows = groupDebtEntriesByReceipt([
            createReceiptEntry('1', 'r1', 40),
            createReceiptEntry('2', undefined, 2500),
            createReceiptEntry('3', 'r1', 47)
        ]);

        expect(rows.map(row => row.key)).toEqual(['receipt_r1', '2']);
    });

    test('counts only what is still open as tickable', () => {
        const rows = groupDebtEntriesByReceipt([
            createReceiptEntry('1', 'r1', 40, true),
            createReceiptEntry('2', 'r1', 47),
            createReceiptEntry('3', 'r1', 30)
        ]);

        expect(rows[0]!.receiptGroup!.openEntryIds).toEqual(['2', '3']);
        expect(rows[0]!.receiptGroup!.totalAmount).toEqual(117);
    });

    test('groups nothing into nothing', () => {
        expect(groupDebtEntriesByReceipt([])).toEqual([]);
    });
});
