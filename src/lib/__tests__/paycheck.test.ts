import { describe, expect, it, beforeAll } from 'vitest';
import moment from 'moment-timezone';

import { TransactionType } from '@/core/transaction.ts';
import type { PaycheckTransaction } from '@/core/paycheck.ts';
import type { TransactionInfoResponse } from '@/models/transaction.ts';

import {
    selectPaycheckTransactions,
    buildPaycheckPeriods,
    findPaycheckPeriodIndex,
    getPaycheckPeriodRemainingDays
} from '@/lib/paycheck.ts';

// Set test environment timezone to UTC, since the test data constants are in UTC
beforeAll(() => {
    moment.tz.setDefault('UTC');
});

function unixTimeOf(dateTime: string): number {
    return moment.utc(dateTime).unix();
}

function paycheckOf(dateTime: string, amount: number, categoryId: string = '11'): PaycheckTransaction {
    return {
        id: `${dateTime}-${amount}`,
        time: unixTimeOf(dateTime),
        amount: amount,
        accountId: '1',
        categoryId: categoryId
    };
}

function incomeTransactionOf(dateTime: string, amount: number, categoryId: string): TransactionInfoResponse {
    return {
        id: `${dateTime}-${amount}`,
        type: TransactionType.Income,
        time: unixTimeOf(dateTime),
        sourceAmount: amount,
        sourceAccountId: '1',
        categoryId: categoryId
    } as unknown as TransactionInfoResponse;
}

describe('selectPaycheckTransactions', () => {
    it('should keep every income transaction of the selected categories', () => {
        const transactions = [
            incomeTransactionOf('2026-07-25T09:00:00Z', 300000, 'salary'),
            incomeTransactionOf('2026-07-28T09:00:00Z', 5000, 'interest'),
            incomeTransactionOf('2026-08-25T09:00:00Z', 300000, 'salary')
        ];

        const paychecks = selectPaycheckTransactions(transactions, { 'salary': true });

        expect(paychecks.map(paycheck => paycheck.amount)).toEqual([ 300000, 300000 ]);
        expect(paychecks.every(paycheck => paycheck.categoryId === 'salary')).toEqual(true);
    });

    it('should ignore categories that are present but not selected', () => {
        const transactions = [
            incomeTransactionOf('2026-08-25T09:00:00Z', 300000, 'salary'),
            incomeTransactionOf('2026-08-28T09:00:00Z', 5000, 'interest')
        ];

        const paychecks = selectPaycheckTransactions(transactions, { 'salary': true, 'interest': false });

        expect(paychecks.length).toEqual(1);
        expect(paychecks[0]?.categoryId).toEqual('salary');
    });

    it('should fall back to the largest income of each month when no category is selected', () => {
        const transactions = [
            incomeTransactionOf('2026-07-25T09:00:00Z', 300000, 'salary'),
            incomeTransactionOf('2026-07-03T09:00:00Z', 400000, 'bonus'),
            incomeTransactionOf('2026-08-24T09:00:00Z', 310000, 'salary'),
            incomeTransactionOf('2026-08-28T09:00:00Z', 5000, 'interest')
        ];

        const paychecks = selectPaycheckTransactions(transactions, {});

        expect(paychecks.map(paycheck => paycheck.amount).sort()).toEqual([ 310000, 400000 ]);
    });

    it('should ignore transactions that are not income', () => {
        const expense = {
            id: 'expense',
            type: TransactionType.Expense,
            time: unixTimeOf('2026-08-25T09:00:00Z'),
            sourceAmount: 900000,
            sourceAccountId: '1',
            categoryId: 'groceries'
        } as unknown as TransactionInfoResponse;

        expect(selectPaycheckTransactions([ expense ], {})).toEqual([]);
    });
});

describe('buildPaycheckPeriods', () => {
    it('should return no period when there is no paycheck', () => {
        expect(buildPaycheckPeriods([], unixTimeOf('2026-08-26T12:00:00Z'), unixTimeOf('2026-08-26T23:59:59Z'))).toEqual([]);
    });

    it('should open a period on the day of each paycheck and order them newest first', () => {
        const currentUnixTime = unixTimeOf('2026-08-26T12:00:00Z');
        const currentPeriodEndTime = unixTimeOf('2026-08-26T23:59:59Z');

        const periods = buildPaycheckPeriods([
            paycheckOf('2026-06-25T09:00:00Z', 300000),
            paycheckOf('2026-07-24T09:00:00Z', 300000),
            paycheckOf('2026-08-25T09:00:00Z', 320000)
        ], currentUnixTime, currentPeriodEndTime);

        expect(periods.length).toEqual(3);

        expect(periods[0]?.startTime).toEqual(unixTimeOf('2026-08-25T00:00:00Z'));
        expect(periods[0]?.endTime).toEqual(currentPeriodEndTime);
        expect(periods[0]?.isCurrent).toEqual(true);
        expect(periods[0]?.paycheckAmount).toEqual(320000);

        expect(periods[1]?.startTime).toEqual(unixTimeOf('2026-07-24T00:00:00Z'));
        expect(periods[1]?.endTime).toEqual(unixTimeOf('2026-08-24T23:59:59Z'));
        expect(periods[1]?.isCurrent).toEqual(false);

        expect(periods[2]?.startTime).toEqual(unixTimeOf('2026-06-25T00:00:00Z'));
        expect(periods[2]?.endTime).toEqual(unixTimeOf('2026-07-23T23:59:59Z'));
    });

    it('should keep a period that started before the pay day it was expected on', () => {
        // the 25th falls on a Sunday, so the pay arrives on the 24th and the period starts a day early
        const periods = buildPaycheckPeriods([
            paycheckOf('2026-09-25T09:00:00Z', 300000),
            paycheckOf('2026-10-24T09:00:00Z', 300000)
        ], unixTimeOf('2026-10-26T12:00:00Z'), unixTimeOf('2026-10-26T23:59:59Z'));

        expect(periods[1]?.startTime).toEqual(unixTimeOf('2026-09-25T00:00:00Z'));
        expect(periods[1]?.endTime).toEqual(unixTimeOf('2026-10-23T23:59:59Z'));
        expect(periods[0]?.startTime).toEqual(unixTimeOf('2026-10-24T00:00:00Z'));
    });

    it('should treat paychecks arriving close together as one period', () => {
        const periods = buildPaycheckPeriods([
            paycheckOf('2026-08-25T09:00:00Z', 300000),
            paycheckOf('2026-08-28T09:00:00Z', 500000)
        ], unixTimeOf('2026-09-02T12:00:00Z'), unixTimeOf('2026-09-02T23:59:59Z'));

        expect(periods.length).toEqual(1);
        // the period opens with the earlier paycheck but is represented by the larger one
        expect(periods[0]?.startTime).toEqual(unixTimeOf('2026-08-25T00:00:00Z'));
        expect(periods[0]?.paycheckAmount).toEqual(500000);
        expect(periods[0]?.paycheckTime).toEqual(unixTimeOf('2026-08-28T09:00:00Z'));
    });

    it('should start a new period once the paychecks are far enough apart', () => {
        const periods = buildPaycheckPeriods([
            paycheckOf('2026-08-01T09:00:00Z', 300000),
            paycheckOf('2026-08-15T09:00:00Z', 300000)
        ], unixTimeOf('2026-08-20T12:00:00Z'), unixTimeOf('2026-08-20T23:59:59Z'));

        expect(periods.length).toEqual(2);
    });

    it('should expect the next paycheck a month after the current period started', () => {
        const periods = buildPaycheckPeriods([
            paycheckOf('2026-07-25T09:00:00Z', 300000),
            paycheckOf('2026-08-25T09:00:00Z', 300000)
        ], unixTimeOf('2026-08-26T12:00:00Z'), unixTimeOf('2026-08-26T23:59:59Z'));

        expect(periods[0]?.estimatedNextPaycheckTime).toEqual(unixTimeOf('2026-09-25T00:00:00Z'));
        expect(periods[1]?.estimatedNextPaycheckTime).toBeUndefined();
    });
});

describe('findPaycheckPeriodIndex', () => {
    it('should find the period matching the time range and report a missing one', () => {
        const periods = buildPaycheckPeriods([
            paycheckOf('2026-07-25T09:00:00Z', 300000),
            paycheckOf('2026-08-25T09:00:00Z', 300000)
        ], unixTimeOf('2026-08-26T12:00:00Z'), unixTimeOf('2026-08-26T23:59:59Z'));

        const period = periods[1];

        expect(findPaycheckPeriodIndex(periods, period?.startTime as number, period?.endTime as number)).toEqual(1);
        expect(findPaycheckPeriodIndex(periods, 0, 0)).toEqual(-1);
    });
});

describe('getPaycheckPeriodRemainingDays', () => {
    it('should count the days left until the next paycheck is expected', () => {
        const currentUnixTime = unixTimeOf('2026-08-26T12:00:00Z');
        const periods = buildPaycheckPeriods([
            paycheckOf('2026-08-25T09:00:00Z', 300000)
        ], currentUnixTime, unixTimeOf('2026-08-26T23:59:59Z'));

        expect(getPaycheckPeriodRemainingDays(periods[0]!, currentUnixTime)).toEqual(30);
    });

    it('should report no days left once the next paycheck is due', () => {
        const currentUnixTime = unixTimeOf('2026-09-26T12:00:00Z');
        const periods = buildPaycheckPeriods([
            paycheckOf('2026-08-25T09:00:00Z', 300000)
        ], currentUnixTime, unixTimeOf('2026-09-26T23:59:59Z'));

        expect(getPaycheckPeriodRemainingDays(periods[0]!, currentUnixTime)).toEqual(0);
    });

    it('should report no days left for a period that has already ended', () => {
        const currentUnixTime = unixTimeOf('2026-08-26T12:00:00Z');
        const periods = buildPaycheckPeriods([
            paycheckOf('2026-07-25T09:00:00Z', 300000),
            paycheckOf('2026-08-25T09:00:00Z', 300000)
        ], currentUnixTime, unixTimeOf('2026-08-26T23:59:59Z'));

        expect(getPaycheckPeriodRemainingDays(periods[1]!, currentUnixTime)).toEqual(0);
    });
});
