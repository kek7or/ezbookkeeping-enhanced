import { describe, expect, test } from 'vitest';

import { TransactionType } from '@/core/transaction.ts';
import { Transaction, type TransactionInfoResponse } from '@/models/transaction.ts';
import { groupTransactionsByReceipt } from '@/lib/transaction.ts';

// the Lidl receipt as the transaction list receives it: one transaction per category, all booked at
// the same second, all pointing back at the shopping trip they were read from
function createTransaction(id: string, categoryId: string, sourceAmount: number, receiptId?: string): Transaction {
    const response: TransactionInfoResponse = {
        id: id,
        timeSequenceId: `${id}000`,
        type: TransactionType.Expense,
        categoryId: categoryId,
        time: 1785269520,
        utcOffset: 120,
        sourceAccountId: '1001',
        destinationAccountId: '0',
        sourceAmount: sourceAmount,
        destinationAmount: 0,
        hideAmount: false,
        tagIds: [],
        comment: '',
        editable: true,
        receipt: receiptId ? { id: receiptId, merchantName: 'Lidl', printedTotal: 3279, hasPrintedTotal: true } : undefined
    };

    return Transaction.of(response);
}

describe('groupTransactionsByReceipt', () => {
    test('should collapse the transactions of one receipt into a single row', () => {
        const rows = groupTransactionsByReceipt([
            createTransaction('1', '11', 955, '900'),
            createTransaction('2', '12', 458, '900'),
            createTransaction('3', '13', 1398, '900')
        ]);

        expect(rows.length).toBe(1);
        expect(rows[0]!.receiptGroup).toBeDefined();
        expect(rows[0]!.receiptGroup!.transactions.length).toBe(3);
        expect(rows[0]!.receiptGroup!.totalAmount).toBe(2811);
        expect(rows[0]!.transaction.id).toBe('1');
    });

    test('should leave transactions without a receipt on their own', () => {
        const rows = groupTransactionsByReceipt([
            createTransaction('1', '11', 955),
            createTransaction('2', '12', 458)
        ]);

        expect(rows.length).toBe(2);
        expect(rows[0]!.receiptGroup).toBeUndefined();
        expect(rows[1]!.receiptGroup).toBeUndefined();
    });

    test('should leave a receipt that produced only one transaction as an ordinary row', () => {
        const rows = groupTransactionsByReceipt([
            createTransaction('1', '11', 955, '900')
        ]);

        expect(rows.length).toBe(1);
        expect(rows[0]!.receiptGroup).toBeUndefined();
        expect(rows[0]!.transaction.id).toBe('1');
    });

    test('should keep the order of the surrounding transactions and take the place of the first member', () => {
        const rows = groupTransactionsByReceipt([
            createTransaction('1', '11', 300),
            createTransaction('2', '12', 458, '900'),
            createTransaction('3', '13', 1398, '900'),
            createTransaction('4', '14', 700)
        ]);

        expect(rows.length).toBe(3);
        expect(rows[0]!.transaction.id).toBe('1');
        expect(rows[1]!.receiptGroup!.transactions.map(transaction => transaction.id)).toEqual(['2', '3']);
        expect(rows[2]!.transaction.id).toBe('4');
    });

    test('should keep two receipts of the same day apart', () => {
        const rows = groupTransactionsByReceipt([
            createTransaction('1', '11', 955, '900'),
            createTransaction('2', '12', 458, '900'),
            createTransaction('3', '11', 200, '901'),
            createTransaction('4', '12', 300, '901')
        ]);

        expect(rows.length).toBe(2);
        expect(rows[0]!.receiptGroup!.receipt.id).toBe('900');
        expect(rows[1]!.receiptGroup!.receipt.id).toBe('901');
    });

    test('should report a total that no longer matches what the till printed', () => {
        const matching = groupTransactionsByReceipt([
            createTransaction('1', '11', 1279, '900'),
            createTransaction('2', '12', 2000, '900')
        ]);

        expect(matching[0]!.receiptGroup!.matchesPrintedTotal).toBe(true);

        // one of the two amounts corrected by hand after the import, so the trip no longer adds up
        const corrected = groupTransactionsByReceipt([
            createTransaction('1', '11', 1000, '900'),
            createTransaction('2', '12', 2000, '900')
        ]);

        expect(corrected[0]!.receiptGroup!.matchesPrintedTotal).toBe(false);
    });

    test('should show a trip as hidden as soon as one of its transactions is', () => {
        const transactions = [
            createTransaction('1', '11', 955, '900'),
            createTransaction('2', '12', 458, '900')
        ];
        transactions[1]!.hideAmount = true;

        const rows = groupTransactionsByReceipt(transactions);

        expect(rows[0]!.receiptGroup!.hasHiddenAmount).toBe(true);
    });
});
