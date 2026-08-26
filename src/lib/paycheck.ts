import { TransactionType } from '@/core/transaction.ts';
import {
    type PaycheckTransaction,
    type PaycheckPeriod,
    PAYCHECK_MIN_PERIOD_DAYS
} from '@/core/paycheck.ts';

import type { TransactionInfoResponse } from '@/models/transaction.ts';

import {
    getDayFirstUnixTimeBySpecifiedUnixTime,
    getGregorianCalendarYearAndMonthFromUnixTime,
    getUnixTimeAfterUnixTime
} from './datetime.ts';

const SECONDS_PER_DAY: number = 24 * 60 * 60;

export function isPaycheckCategorySelected(paycheckCategoryIds: Record<string, boolean> | undefined): boolean {
    if (!paycheckCategoryIds) {
        return false;
    }

    for (const categoryId in paycheckCategoryIds) {
        if (!Object.prototype.hasOwnProperty.call(paycheckCategoryIds, categoryId)) {
            continue;
        }

        if (paycheckCategoryIds[categoryId]) {
            return true;
        }
    }

    return false;
}

// picks the transactions that start a pay period out of all income transactions in the lookback window.
// when the user has told us which categories their pay arrives in, every income transaction in those
// categories counts; otherwise the largest income of each calendar month is assumed to be the paycheck
export function selectPaycheckTransactions(transactions: TransactionInfoResponse[], paycheckCategoryIds: Record<string, boolean> | undefined): PaycheckTransaction[] {
    const incomeTransactions: PaycheckTransaction[] = [];

    for (const transaction of transactions) {
        if (transaction.type !== TransactionType.Income || transaction.sourceAmount <= 0) {
            continue;
        }

        incomeTransactions.push({
            id: transaction.id,
            time: transaction.time,
            amount: transaction.sourceAmount,
            accountId: transaction.sourceAccountId,
            categoryId: transaction.categoryId
        });
    }

    if (isPaycheckCategorySelected(paycheckCategoryIds)) {
        return incomeTransactions.filter(transaction => !!paycheckCategoryIds?.[transaction.categoryId]);
    }

    const largestIncomeOfMonth: Record<string, PaycheckTransaction> = {};

    for (const transaction of incomeTransactions) {
        const yearMonth = getGregorianCalendarYearAndMonthFromUnixTime(transaction.time);

        if (!yearMonth) {
            continue;
        }

        const currentLargest = largestIncomeOfMonth[yearMonth];

        if (!currentLargest || transaction.amount > currentLargest.amount) {
            largestIncomeOfMonth[yearMonth] = transaction;
        }
    }

    return Object.values(largestIncomeOfMonth);
}

// turns paycheck transactions into the pay periods they open. paychecks arriving within
// PAYCHECK_MIN_PERIOD_DAYS of each other are one period, opened by the earliest of them and
// represented by the largest. the returned periods are ordered from the newest to the oldest
export function buildPaycheckPeriods(paychecks: PaycheckTransaction[], currentUnixTime: number, currentPeriodEndTime: number): PaycheckPeriod[] {
    if (!paychecks.length) {
        return [];
    }

    const ascendingPaychecks = paychecks.slice().sort((paycheck1, paycheck2) => paycheck1.time - paycheck2.time);
    const clusters: PaycheckTransaction[][] = [];

    for (const paycheck of ascendingPaychecks) {
        const currentCluster = clusters[clusters.length - 1];
        const clusterStartTime = currentCluster ? getDayFirstUnixTimeBySpecifiedUnixTime((currentCluster[0] as PaycheckTransaction).time) : 0;

        if (currentCluster && getDayFirstUnixTimeBySpecifiedUnixTime(paycheck.time) - clusterStartTime < PAYCHECK_MIN_PERIOD_DAYS * SECONDS_PER_DAY) {
            currentCluster.push(paycheck);
        } else {
            clusters.push([ paycheck ]);
        }
    }

    const periods: PaycheckPeriod[] = [];

    for (let i = 0; i < clusters.length; i++) {
        const cluster = clusters[i] as PaycheckTransaction[];
        const startTime = getDayFirstUnixTimeBySpecifiedUnixTime((cluster[0] as PaycheckTransaction).time);
        const nextCluster = clusters[i + 1];
        const endTime = nextCluster
            ? getDayFirstUnixTimeBySpecifiedUnixTime((nextCluster[0] as PaycheckTransaction).time) - 1
            : currentPeriodEndTime;

        if (endTime < startTime) {
            continue;
        }

        let representativePaycheck = cluster[0] as PaycheckTransaction;

        for (const paycheck of cluster) {
            if (paycheck.amount > representativePaycheck.amount) {
                representativePaycheck = paycheck;
            }
        }

        periods.push({
            startTime: startTime,
            endTime: endTime,
            paycheckTime: representativePaycheck.time,
            paycheckAmount: representativePaycheck.amount,
            paycheckAccountId: representativePaycheck.accountId,
            isCurrent: startTime <= currentUnixTime && currentUnixTime <= endTime,
            estimatedNextPaycheckTime: nextCluster ? undefined : getUnixTimeAfterUnixTime(startTime, 1, 'months')
        });
    }

    return periods.reverse();
}

export function findPaycheckPeriodIndex(periods: PaycheckPeriod[], startTime: number, endTime: number): number {
    for (let i = 0; i < periods.length; i++) {
        const period = periods[i] as PaycheckPeriod;

        if (period.startTime === startTime && period.endTime === endTime) {
            return i;
        }
    }

    return -1;
}

// how many days are left until the next paycheck is expected, zero once it is due
export function getPaycheckPeriodRemainingDays(period: PaycheckPeriod, currentUnixTime: number): number {
    if (!period.isCurrent || !period.estimatedNextPaycheckTime) {
        return 0;
    }

    const today = getDayFirstUnixTimeBySpecifiedUnixTime(currentUnixTime);
    const nextPaycheckDay = getDayFirstUnixTimeBySpecifiedUnixTime(period.estimatedNextPaycheckTime);

    if (nextPaycheckDay <= today) {
        return 0;
    }

    return Math.round((nextPaycheckDay - today) / SECONDS_PER_DAY);
}
