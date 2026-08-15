import { describe, expect, test } from 'vitest';

import { TransactionType } from '@/core/transaction.ts';
import { ImportReceipt } from '@/models/imported_receipt.ts';
import type { ImportTransactionResponse, ImportTransactionResponsePageWrapper } from '@/models/imported_transaction.ts';

function createTransactionResponse(categoryId: string, categoryName: string, sourceAmount: number, comment: string): ImportTransactionResponse {
    return {
        type: TransactionType.Expense,
        categoryId: categoryId,
        originalCategoryName: categoryName,
        time: 1785269520,
        utcOffset: 120,
        sourceAccountId: '1001',
        originalSourceAccountName: 'Girokonto',
        originalSourceAccountCurrency: 'EUR',
        sourceAmount: sourceAmount,
        tagIds: [],
        originalTagNames: [],
        comment: comment
    };
}

// the Lidl receipt of the aggregation tests, as the server returns it: the transactions it summed,
// and the lines it summed them from
function createReceiptResponse(): ImportTransactionResponsePageWrapper {
    return {
        items: [
            createTransactionResponse('11', 'Food', 955, 'Broccoli, Kartoffeln früh, Milchcreme Cookies'),
            createTransactionResponse('12', 'Drink', 458, 'Frischer O-Saft o.F., Monster Mango Loco'),
            createTransactionResponse('13', 'Electronics', 1398, 'Akku Ni-MH-0532273, Akku Ni-MH-0532273')
        ],
        totalCount: 3,
        receipt: {
            lineItems: [
                { name: 'Broccoli', amount: 149, categoryName: 'Food' },
                { name: 'Kartoffeln früh', amount: 299, categoryName: 'Food' },
                { name: 'Frischer O-Saft o.F.', amount: 284, categoryName: 'Drink' },
                { name: 'Monster Mango Loco', amount: 174, categoryName: 'Drink' },
                { name: 'Milchcreme Cookies', amount: 507, categoryName: 'Food' },
                { name: 'Akku Ni-MH-0532273', amount: 699, categoryName: 'Electronics' },
                { name: 'Akku Ni-MH-0532273', amount: 699, categoryName: 'Electronics' }
            ]
        }
    };
}

describe('ImportReceipt', () => {
    test('groups the receipt lines the same way the server summed them', () => {
        const receipt = ImportReceipt.of(createReceiptResponse(), 0, 'receipt.jpg');

        expect(receipt).toBeTruthy();
        expect(receipt?.categoryGroups.length).toBe(3);
        expect(receipt?.lineItemCount).toBe(7);
        expect(receipt?.totalAmount).toBe(2811);

        const transactions = receipt!.toImportTransactions(0);

        expect(transactions.length).toBe(3);

        for (let i = 0; i < transactions.length; i++) {
            const expected = createReceiptResponse().items[i]!;
            expect(transactions[i]!.categoryId).toBe(expected.categoryId);
            expect(transactions[i]!.sourceAmount).toBe(expected.sourceAmount);
            expect(transactions[i]!.comment).toBe(expected.comment);
            expect(transactions[i]!.valid).toBe(true);
            expect(transactions[i]!.selected).toBe(true);
        }
    });

    test('moves the amount and the name with the line item', () => {
        const receipt = ImportReceipt.of(createReceiptResponse(), 0, 'receipt.jpg')!;
        const foodGroup = receipt.categoryGroups[0]!;
        const drinkGroup = receipt.categoryGroups[1]!;

        // the model read "Milchcreme Cookies" as food, it is moved to where it belongs
        const movedLineItem = foodGroup.lineItems[2]!;
        foodGroup.lineItems.splice(2, 1);
        drinkGroup.lineItems.push(movedLineItem);

        expect(foodGroup.totalAmount).toBe(448);
        expect(foodGroup.description).toBe('Broccoli, Kartoffeln früh');
        expect(drinkGroup.totalAmount).toBe(965);
        expect(drinkGroup.description).toBe('Frischer O-Saft o.F., Monster Mango Loco, Milchcreme Cookies');

        // what the receipt cost in total cannot change by moving a line inside it
        expect(receipt.totalAmount).toBe(2811);

        const transactions = receipt.toImportTransactions(0);
        expect(transactions[0]!.sourceAmount).toBe(448);
        expect(transactions[1]!.sourceAmount).toBe(965);
    });

    test('correcting a misread price only changes its own category', () => {
        const receipt = ImportReceipt.of(createReceiptResponse(), 0, 'receipt.jpg')!;
        const foodGroup = receipt.categoryGroups[0]!;

        foodGroup.lineItems[2]!.amount = 149;

        expect(foodGroup.totalAmount).toBe(597);
        expect(receipt.categoryGroups[1]!.totalAmount).toBe(458);
        expect(receipt.totalAmount).toBe(2453);
    });

    test('an emptied category is not imported and a new one can take its lines', () => {
        const receipt = ImportReceipt.of(createReceiptResponse(), 0, 'receipt.jpg')!;
        const electronicsGroup = receipt.categoryGroups[2]!;
        const newGroup = receipt.addCategoryGroup('14', 'Houseware');

        newGroup.lineItems.push(...electronicsGroup.lineItems.splice(0, 2));

        expect(electronicsGroup.isImportable).toBe(false);

        const transactions = receipt.toImportTransactions(0);

        expect(transactions.length).toBe(3);
        expect(transactions[2]!.categoryId).toBe('14');
        expect(transactions[2]!.sourceAmount).toBe(1398);
        expect(transactions.map(transaction => transaction.index)).toEqual([0, 1, 2]);
    });

    test('the receipt time is when every transaction of it happened', () => {
        const receipt = ImportReceipt.of(createReceiptResponse(), 0, 'receipt.jpg')!;
        const correctedTime = 1785269520 - 3 * 60 * 60;

        receipt.time = correctedTime;

        const transactions = receipt.toImportTransactions(0);

        expect(transactions.length).toBe(3);

        for (const transaction of transactions) {
            expect(transaction.time).toBe(correctedTime);
            // the receipt was paid where it was paid, so correcting the time does not move the zone
            expect(transaction.utcOffset).toBe(120);
        }
    });

    test('keeps what was handed back out of the purchases of the same category', () => {
        // the empties are worth more than the drinks bought here, so summing the two together would
        // leave a group of -6.83 and take the drinks out of the import with it
        const response = createReceiptResponse();
        response.receipt!.lineItems.push({ name: 'Pfandrückgabe', amount: -1175, categoryName: 'Drink', refund: true });

        const receipt = ImportReceipt.of(response, 0, 'receipt.jpg')!;

        expect(receipt.categoryGroups.length).toBe(4);
        expect(receipt.lineItemCount).toBe(8);
        expect(receipt.totalAmount).toBe(1636);

        const refundGroup = receipt.categoryGroups[3]!;

        expect(refundGroup.refund).toBe(true);
        expect(refundGroup.originalCategoryName).toBe('Drink');
        expect(refundGroup.totalAmount).toBe(-1175);

        // the drinks bought on this receipt are still their own transaction
        expect(receipt.categoryGroups[1]!.totalAmount).toBe(458);

        const transactions = receipt.toImportTransactions(0);

        expect(transactions.length).toBe(4);
        expect(transactions[3]!.sourceAmount).toBe(-1175);
        expect(transactions[3]!.comment).toBe('Pfandrückgabe');

        // money handed back is a transaction like any other, it is only the sign that differs
        expect(transactions[3]!.valid).toBe(true);
        expect(transactions[3]!.selected).toBe(true);
    });

    test('a category that comes to nothing is not imported, one that comes to less than nothing is', () => {
        const response = createReceiptResponse();
        response.receipt!.lineItems.push({ name: 'Pfandrückgabe', amount: -1175, categoryName: 'Drink', refund: true });

        const receipt = ImportReceipt.of(response, 0, 'receipt.jpg')!;
        const refundGroup = receipt.categoryGroups[3]!;

        expect(refundGroup.isImportable).toBe(true);

        refundGroup.lineItems[0]!.amount = 0;
        expect(refundGroup.isImportable).toBe(false);
        expect(receipt.toImportTransactions(0).length).toBe(3);
    });

    test('leaves an import that is not a receipt to the ordinary import table', () => {
        const withoutLineItems = createReceiptResponse();
        expect(ImportReceipt.of({ items: withoutLineItems.items, totalCount: 3 }, 0, 'receipt.jpg')).toBeUndefined();

        // a transfer or an income cannot be summed out of receipt lines booked to one account
        const withIncome = createReceiptResponse();
        (withIncome.items[1] as { type: TransactionType }).type = TransactionType.Income;
        expect(ImportReceipt.of(withIncome, 0, 'receipt.jpg')).toBeUndefined();
    });
});
