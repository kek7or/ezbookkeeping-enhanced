// how far back paycheck transactions are searched for
export const PAYCHECK_LOOKBACK_MONTHS: number = 15;

// how many income transactions are read per request, the maximum the transaction list api accepts
export const PAYCHECK_PAGE_SIZE: number = 50;

// how many pages of income transactions are read at most while looking for paychecks
export const PAYCHECK_MAX_PAGE_COUNT: number = 4;

// two paychecks closer than this many days belong to the same pay period,
// so a bonus paid a few days after the salary does not start a new period
export const PAYCHECK_MIN_PERIOD_DAYS: number = 10;

// how many pay periods are offered in the date range menu
export const PAYCHECK_MAX_PERIOD_COUNT: number = 24;

export interface PaycheckTransaction {
    readonly id: string;
    readonly time: number;
    readonly amount: number;
    readonly accountId: string;
    readonly categoryId: string;
}

export interface PaycheckPeriod {
    // start of the day the paycheck arrived on, inclusive
    readonly startTime: number;
    // one second before the next paycheck, or the end of today for the current period, inclusive
    readonly endTime: number;
    // the paycheck this period starts with, the largest one when several arrived together
    readonly paycheckTime: number;
    readonly paycheckAmount: number;
    readonly paycheckAccountId: string;
    // true for the period the current time falls into
    readonly isCurrent: boolean;
    // when the next paycheck is expected, only set for the current period. the next paycheck has not
    // been received yet, so this is an estimate carried forward from how long the previous period ran
    readonly estimatedNextPaycheckTime?: number;
}
