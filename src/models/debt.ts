// DebtAmount is what is owed in one currency. Debts are never added across currencies, because a
// sum of two currencies is a number that is true of neither of them.
export interface DebtAmount {
    readonly currency: string;
    readonly amount: number;
}

// DebtPersonInfoResponse is one person who owes the user money, and what they currently owe
export interface DebtPersonInfoResponse {
    readonly id: string;
    readonly name: string;
    readonly displayOrder: number;
    readonly openAmounts?: DebtAmount[];
    readonly openCount: number;
}

// DebtEntryInfoResponse is one thing a person owes: a whole transaction, one position of one, or a
// loan that never passed through the ledger.
//
// It carries what the transaction it points at says about it, so that a list of debts reads as the
// things that were bought rather than as a column of amounts.
export interface DebtEntryInfoResponse {
    readonly id: string;
    readonly personId: string;
    readonly transactionId: string;
    readonly lineItemId?: string;
    readonly amount: number;
    readonly currency: string;
    readonly time: number;
    readonly settled?: boolean;
    readonly settlementTransactionId?: string;
    readonly settledTime?: number;
    // manual says this debt was entered by hand and has no transaction behind it
    readonly manual?: boolean;
    // name is what the position is called on the receipt, or what a debt entered by hand was called,
    // and is absent when the whole transaction is owed rather than one of its positions
    readonly name?: string;
    readonly categoryId?: string;
    readonly comment?: string;
    // receiptId is the shopping trip the transaction belongs to, when it came from one
    readonly receiptId?: string;
    readonly merchantName?: string;
    // missing says the transaction this points at has left the ledger. It is still owed and still
    // counted - it just can no longer say what it was for.
    readonly missing?: boolean;
}

export interface DebtPersonCreateRequest {
    readonly name: string;
}

export interface DebtPersonModifyRequest {
    readonly id: string;
    readonly name: string;
}

export interface DebtPersonDeleteRequest {
    readonly id: string;
}

export interface DebtEntryCreateRequest {
    // personId is who owes this one, set when a thing is shared out among several people. It is
    // absent when the whole request is owed by the single person it names.
    readonly personId?: string;
    readonly transactionId: string;
    readonly lineItemId?: string;
    // amount is what is owed, in minor units, or absent to owe the full amount of what is attached
    readonly amount?: number;
}

export interface DebtEntryCreateBatchRequest {
    // personId is who owes everything in the request, and is left out when every entry names its
    // own person, which is what a split does
    readonly personId?: string;
    readonly entries: DebtEntryCreateRequest[];
}

// DebtEntryCreateManualRequest records a debt that has no transaction behind it, which has to name
// itself because there is no transaction here to be asked
export interface DebtEntryCreateManualRequest {
    readonly personId: string;
    readonly description: string;
    readonly amount: number;
    readonly currency: string;
    readonly time: number;
}

export interface DebtEntryModifyRequest {
    readonly id: string;
    readonly amount: number;
    // description renames a debt entered by hand, and is refused for one that has a transaction
    readonly description?: string;
}

export interface DebtEntryDeleteRequest {
    readonly ids: string[];
}

export interface DebtEntrySettleRequest {
    readonly ids: string[];
    readonly settlementTransactionId: string;
}

export interface DebtEntryReopenRequest {
    readonly ids: string[];
}

// DebtEntryReceiptGroup is everything owed off one shopping trip, shown as a single row that opens
// to reveal what it is made of
export interface DebtEntryReceiptGroup {
    readonly receiptId: string;
    readonly merchantName: string;
    readonly entries: DebtEntryInfoResponse[];
    readonly totalAmount: number;
    readonly currency: string;
    readonly openEntryIds: string[];
}

// DebtEntryRow is one row of the debts list: either a single thing owed, or a shopping trip several
// things were owed off
export interface DebtEntryRow {
    readonly key: string;
    readonly entry: DebtEntryInfoResponse;
    readonly receiptGroup?: DebtEntryReceiptGroup;
}

// groupDebtEntriesByReceipt turns what somebody owes into the rows the debts list shows, collapsing
// everything owed off one shopping trip into a single row.
//
// The order the entries came in is kept exactly: a trip takes the place of the first thing owed off
// it, and everything else stays where it was, so the list stays in the order of time it was sorted
// into. A receipt only one thing is owed off is left as an ordinary row - there is nothing to
// collapse and a group of one only adds a click.
export function groupDebtEntriesByReceipt(entries: DebtEntryInfoResponse[]): DebtEntryRow[] {
    const entriesByReceiptId: Record<string, DebtEntryInfoResponse[]> = {};

    for (const entry of entries) {
        if (!entry.receiptId) {
            continue;
        }

        const receiptEntries: DebtEntryInfoResponse[] = entriesByReceiptId[entry.receiptId] ?? [];
        receiptEntries.push(entry);
        entriesByReceiptId[entry.receiptId] = receiptEntries;
    }

    const rows: DebtEntryRow[] = [];
    const emittedReceiptIds: Record<string, boolean> = {};

    for (const entry of entries) {
        const receiptId = entry.receiptId;

        if (!receiptId) {
            rows.push({ key: entry.id, entry: entry });
            continue;
        }

        const receiptEntries = entriesByReceiptId[receiptId] as DebtEntryInfoResponse[];

        if (receiptEntries.length < 2) {
            rows.push({ key: entry.id, entry: entry });
            continue;
        }

        if (emittedReceiptIds[receiptId]) {
            continue;
        }

        emittedReceiptIds[receiptId] = true;

        let totalAmount = 0;
        const openEntryIds: string[] = [];

        for (const receiptEntry of receiptEntries) {
            totalAmount += receiptEntry.amount;

            if (!receiptEntry.settled) {
                openEntryIds.push(receiptEntry.id);
            }
        }

        rows.push({
            key: `receipt_${receiptId}`,
            entry: receiptEntries[0] as DebtEntryInfoResponse,
            receiptGroup: {
                receiptId: receiptId,
                merchantName: entry.merchantName ?? '',
                entries: receiptEntries,
                totalAmount: totalAmount,
                currency: entry.currency,
                openEntryIds: openEntryIds
            }
        });
    }

    return rows;
}

// splitAmountEvenly divides an amount into the given number of shares, in minor units.
//
// The shares add up to exactly what was divided. Three people cannot each pay a third of 10,00, so
// the cents that do not divide are handed out one apiece from the front, and the caller decides who
// stands at the front - the one who paid, so that a rounding cent is absorbed rather than charged
// to a friend.
export function splitAmountEvenly(amount: number, shareCount: number): number[] {
    if (shareCount < 1) {
        return [];
    }

    const baseShare = Math.trunc(amount / shareCount);
    let remainder = amount - baseShare * shareCount;
    const shares: number[] = [];

    for (let i = 0; i < shareCount; i++) {
        if (remainder > 0) {
            shares.push(baseShare + 1);
            remainder--;
        } else {
            shares.push(baseShare);
        }
    }

    return shares;
}

// sumDebtAmountsByCurrency totals what a set of amounts comes to, one total per currency.
//
// It takes anything that states a currency and an amount, because the same question is asked of the
// entries of one person and of the standing totals of all of them.
export function sumDebtAmountsByCurrency(entries: readonly DebtAmount[]): DebtAmount[] {
    const totals: Record<string, number> = {};
    const currencies: string[] = [];

    for (const entry of entries) {
        if (!Object.prototype.hasOwnProperty.call(totals, entry.currency)) {
            totals[entry.currency] = 0;
            currencies.push(entry.currency);
        }

        totals[entry.currency] = (totals[entry.currency] as number) + entry.amount;
    }

    return currencies.map(currency => ({
        currency: currency,
        amount: totals[currency] as number
    }));
}
