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

// DebtEntryGroupKind says what the things owed in a group have in common: the shopping trip they
// were bought on, or the transaction whose positions they are
export type DebtEntryGroupKind = 'receipt' | 'transaction';

// DebtEntryGroup is several things owed that belong together, shown as a single row that opens to
// reveal what it is made of.
//
// The two kinds nest, because that is how the things themselves nest: a shopping trip opens to the
// categories it was split into, and a category several of whose articles are owed opens to those
// articles. Somebody who is to pay for two of the vegetables and none of the meat is then read as
// one trip, not as a run of unrelated rows.
export interface DebtEntryGroup {
    readonly kind: DebtEntryGroupKind;
    // id is the receipt or the transaction this is the group of
    readonly id: string;
    // merchantName is the shop the trip was to, and is empty for a group of positions
    readonly merchantName: string;
    // rows are what the group opens to, one level down, which for a trip may itself be groups of
    // positions
    readonly rows: DebtEntryRow[];
    // entries is every thing owed under the group, however deep. It is what the group totals and
    // what ticking the group ticks, so that a trip is paid back whole however it is nested.
    readonly entries: DebtEntryInfoResponse[];
    readonly totalAmount: number;
    readonly currency: string;
    readonly openEntryIds: string[];
}

// DebtEntryRow is one row of the debts list: either a single thing owed, or a group of things owed
export interface DebtEntryRow {
    readonly key: string;
    // entry is the thing owed, or for a group the first thing owed in it, which is what the row is
    // dated by
    readonly entry: DebtEntryInfoResponse;
    readonly group?: DebtEntryGroup;
}

// collectEntries returns every thing owed under the given rows, in the order it is shown in
function collectEntries(rows: readonly DebtEntryRow[]): DebtEntryInfoResponse[] {
    const entries: DebtEntryInfoResponse[] = [];

    for (const row of rows) {
        if (row.group) {
            entries.push(...row.group.entries);
        } else {
            entries.push(row.entry);
        }
    }

    return entries;
}

// groupRowsBy collapses the rows answering to the same key into one row each and leaves the rest
// exactly where they were.
//
// A group stands where the first of its members stood, so the order of time the list was sorted
// into survives grouping. A key only one row answers to is left as that row: there is nothing to
// collapse, and a group of one only adds a click.
function groupRowsBy(rows: readonly DebtEntryRow[], getKey: (row: DebtEntryRow) => string | undefined, buildGroup: (key: string, groupRows: DebtEntryRow[]) => DebtEntryGroup): DebtEntryRow[] {
    const rowsByKey: Record<string, DebtEntryRow[]> = {};

    for (const row of rows) {
        const key = getKey(row);

        if (!key) {
            continue;
        }

        const keyRows: DebtEntryRow[] = rowsByKey[key] ?? [];
        keyRows.push(row);
        rowsByKey[key] = keyRows;
    }

    const groupedRows: DebtEntryRow[] = [];
    const emittedKeys: Record<string, boolean> = {};

    for (const row of rows) {
        const key = getKey(row);

        if (!key) {
            groupedRows.push(row);
            continue;
        }

        const keyRows = rowsByKey[key] as DebtEntryRow[];

        if (keyRows.length < 2) {
            groupedRows.push(row);
            continue;
        }

        if (emittedKeys[key]) {
            continue;
        }

        emittedKeys[key] = true;

        const group = buildGroup(key, keyRows);

        groupedRows.push({
            key: `${group.kind}_${key}`,
            entry: (keyRows[0] as DebtEntryRow).entry,
            group: group
        });
    }

    return groupedRows;
}

// makeGroup gathers what a group of rows comes to. Only what is still open is tickable: a settled
// row is shown for the record and there is nothing left to do to it.
function makeGroup(kind: DebtEntryGroupKind, id: string, merchantName: string, rows: DebtEntryRow[]): DebtEntryGroup {
    const entries = collectEntries(rows);
    let totalAmount = 0;
    const openEntryIds: string[] = [];

    for (const entry of entries) {
        totalAmount += entry.amount;

        if (!entry.settled) {
            openEntryIds.push(entry.id);
        }
    }

    return {
        kind: kind,
        id: id,
        merchantName: merchantName,
        rows: rows,
        entries: entries,
        totalAmount: totalAmount,
        currency: (entries[0] as DebtEntryInfoResponse).currency,
        openEntryIds: openEntryIds
    };
}

// groupDebtEntries turns what somebody owes into the rows the debts list shows, gathering the
// positions owed of one transaction under that transaction, and everything owed off one shopping
// trip under that trip.
//
// Positions are gathered first, because a transaction belongs to exactly one trip and so a group of
// its positions never straddles two of them. What is owed whole is never gathered: a transaction
// nobody picked positions out of is one thing owed and stays one row.
export function groupDebtEntries(entries: readonly DebtEntryInfoResponse[]): DebtEntryRow[] {
    const rows: DebtEntryRow[] = entries.map(entry => ({ key: entry.id, entry: entry }));

    const rowsByTransaction = groupRowsBy(
        rows,
        row => row.entry.lineItemId ? row.entry.transactionId : undefined,
        (transactionId, transactionRows) => makeGroup('transaction', transactionId, '', transactionRows)
    );

    return groupRowsBy(
        rowsByTransaction,
        row => row.entry.receiptId,
        (receiptId, receiptRows) => makeGroup('receipt', receiptId, (receiptRows[0] as DebtEntryRow).entry.merchantName ?? '', receiptRows)
    );
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
