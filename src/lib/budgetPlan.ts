import { TransactionType } from '@/core/transaction.ts';
import { ScheduledTemplateFrequencyType } from '@/core/template.ts';
import { type BigDecimal } from '@/core/numeral.ts';
import { TransactionTemplate } from '@/models/transaction_template.ts';
import { type BudgetPlanAdjustment, BudgetPlanItem } from '@/models/budget_plan.ts';

import { BIG_DECIMAL_ZERO, parseBigDecimal } from '@/lib/numeral.ts';
import { getScheduleOccurrencesPerYear } from '@/lib/template.ts';

const MONTHS_PER_YEAR = 12;

// PlannedLineSource says where a line of the plan came from, which decides what can be done to it.
// A schedule cannot be edited from the plan - the template says what recurs, and only this month's
// difference from it belongs here - while an item planned by hand is the plan's own and is edited
// in place.
export enum PlannedLineSource {
    Schedule = 'schedule',
    Item = 'item'
}

export interface PlannedLine {
    readonly source: PlannedLineSource;
    // id is the template id for a schedule and the item id for a planned item. It is unique only
    // within a source, which is why the two are never mixed in one list without it.
    readonly id: string;
    readonly name: string;
    readonly type: number;
    readonly categoryId: string;
    readonly accountId: string;
    readonly currency: string;
    // occurrences is how many times this happens in the month. It is a whole number for anything
    // with a day, and a fraction for a recurrence that has none - see getScheduleOccurrencesInMonth.
    readonly occurrences: number;
    // unitAmount is what one occurrence costs, after any adjustment for this month
    readonly unitAmount: number;
    // amount is what the month costs because of this line, in minor units of currency
    readonly amount: BigDecimal;
    // excluded lines are shown and not counted: a subscription paused for one month should stay
    // visible, because it is coming back
    readonly excluded: boolean;
    // adjusted says this month's figure is not the schedule's own
    readonly adjusted: boolean;
}

export interface PlanTotals {
    readonly income: BigDecimal;
    readonly expense: BigDecimal;
    readonly net: BigDecimal;
}

// getScheduleOccurrencesInMonth counts how many times a schedule fires inside one calendar month.
//
// The monthly figure is counted rather than averaged, because a plan is acted on. A weekly 50 is
// 250 in a month with five of that weekday and 200 in a month with four, and a plan that says 216.67
// every month is wrong twelve times a year. The days are enumerated and each one asked whether the
// recurrence lands on it, which is the same question the cron asks, and which gets the start and end
// dates of the schedule right without any separate arithmetic.
//
// A recurrence with no day named is the exception, because there is no day to ask about. Such a
// schedule is spread evenly over the year: a monthly one comes out at exactly one occurrence a
// month, and a yearly one at a twelfth of itself, which is the figure worth planning against anyway
// - the annual bill is best met by setting aside a twelfth of it every month.
export function getScheduleOccurrencesInMonth(template: TransactionTemplate, year: number, month: number): number {
    const frequencyType = template.scheduledFrequencyType;
    const values = parseFrequencyValues(template.scheduledFrequency);

    if (frequencyType === undefined || frequencyType === ScheduledTemplateFrequencyType.Disabled.type) {
        return 0;
    }

    if (!values.length) {
        if (frequencyType === ScheduledTemplateFrequencyType.Monthly.type) {
            return 1;
        }

        return getScheduleOccurrencesPerYear(frequencyType, template.scheduledFrequency) / MONTHS_PER_YEAR;
    }

    const daysInMonth = getDaysInMonth(year, month);
    const startDayNumber = toDayNumber(template.scheduledStartDate);
    const endDayNumber = toDayNumber(template.scheduledEndDate);

    let occurrences = 0;

    for (let day = 1; day <= daysInMonth; day++) {
        const dayNumber = year * 10000 + month * 100 + day;

        if (startDayNumber !== null && dayNumber < startDayNumber) {
            continue;
        }

        if (endDayNumber !== null && dayNumber > endDayNumber) {
            continue;
        }

        if (firesOnDay(frequencyType, values, year, month, day, daysInMonth, template.scheduledStartDate)) {
            occurrences++;
        }
    }

    return occurrences;
}

// buildScheduleLines turns the scheduled transactions into what they cost this particular month,
// with whatever this month says differs from them already applied.
export function buildScheduleLines(templates: TransactionTemplate[], adjustmentsByTemplateId: Record<string, BudgetPlanAdjustment>, year: number, month: number, getCurrency: (accountId: string) => string | undefined): PlannedLine[] {
    const lines: PlannedLine[] = [];

    for (const template of templates) {
        if (template.hidden) {
            continue;
        }

        // a transfer moves money between two accounts of one ledger and is neither spent nor
        // earned, so it has no place in what a month costs
        if (template.type === TransactionType.Transfer) {
            continue;
        }

        const occurrences = getScheduleOccurrencesInMonth(template, year, month);

        if (occurrences <= 0) {
            continue;
        }

        const currency = getCurrency(template.sourceAccountId);

        if (!currency) {
            continue;
        }

        const adjustment = adjustmentsByTemplateId[template.id];
        const adjustedAmount = adjustment && adjustment.amount !== undefined && adjustment.amount !== null ? adjustment.amount : null;
        const unitAmount = adjustedAmount !== null ? adjustedAmount : template.sourceAmount;
        const excluded = !!adjustment && adjustment.excluded;

        lines.push({
            source: PlannedLineSource.Schedule,
            id: template.id,
            name: template.name,
            type: template.type,
            categoryId: template.categoryId,
            accountId: template.sourceAccountId,
            currency: currency,
            occurrences: occurrences,
            unitAmount: unitAmount,
            amount: excluded ? BIG_DECIMAL_ZERO : parseBigDecimal(unitAmount).multiply(occurrences),
            excluded: excluded,
            adjusted: adjustedAmount !== null
        });
    }

    return lines;
}

// buildItemLines turns what was planned by hand into the same shape the schedules produce, so that
// the two can be totalled and compared without either being a special case.
export function buildItemLines(items: BudgetPlanItem[], getCurrency: (accountId: string) => string | undefined): PlannedLine[] {
    const lines: PlannedLine[] = [];

    for (const item of items) {
        const currency = getCurrency(item.accountId);

        if (!currency) {
            continue;
        }

        lines.push({
            source: PlannedLineSource.Item,
            id: item.id,
            name: item.name,
            type: item.type,
            categoryId: item.categoryId,
            accountId: item.accountId,
            currency: currency,
            occurrences: 1,
            unitAmount: item.amount,
            amount: parseBigDecimal(item.amount),
            excluded: false,
            adjusted: false
        });
    }

    return lines;
}

// sumPlannedLines totals what the month is planned to cost, in one currency.
//
// Every line is converted into that currency first, unlike the schedules page which refuses to
// convert at all. The difference is that this figure is put next to what has actually been spent,
// and the ledger's own totals arrive already converted - two numbers that cannot be subtracted are
// worth less here than two that carry a rate's worth of imprecision.
export function sumPlannedLines(lines: PlannedLine[], convert: (amount: BigDecimal, currency: string) => BigDecimal | null): PlanTotals {
    let income = BIG_DECIMAL_ZERO;
    let expense = BIG_DECIMAL_ZERO;

    for (const line of lines) {
        if (line.excluded) {
            continue;
        }

        const converted = convert(line.amount, line.currency);

        if (!converted) {
            continue;
        }

        if (line.type === TransactionType.Income) {
            income = income.add(converted);
        } else {
            expense = expense.add(converted);
        }
    }

    return {
        income: income,
        expense: expense,
        net: income.subtract(expense)
    };
}

// sumPlannedLinesByCategory is what the planned-against-actual table is built on: the plan reduced
// to one figure per category, which is the level the ledger can answer at.
export function sumPlannedLinesByCategory(lines: PlannedLine[], convert: (amount: BigDecimal, currency: string) => BigDecimal | null): Record<string, BigDecimal> {
    const totals: Record<string, BigDecimal> = {};

    for (const line of lines) {
        if (line.excluded) {
            continue;
        }

        const converted = convert(line.amount, line.currency);

        if (!converted) {
            continue;
        }

        totals[line.categoryId] = (totals[line.categoryId] ?? BIG_DECIMAL_ZERO).add(converted);
    }

    return totals;
}

function firesOnDay(frequencyType: number, values: number[], year: number, month: number, day: number, daysInMonth: number, startDate: string | undefined): boolean {
    if (frequencyType === ScheduledTemplateFrequencyType.Daily.type) {
        return true;
    }

    if (frequencyType === ScheduledTemplateFrequencyType.Weekly.type) {
        return values.indexOf(getWeekday(year, month, day)) >= 0;
    }

    if (frequencyType === ScheduledTemplateFrequencyType.Monthly.type) {
        // a negative day counts back from the end of the month, so -1 is the last day of whatever
        // length this month happens to be - the same normalization the cron does
        return values.some(value => (value < 0 ? daysInMonth + value + 1 : value) === day);
    }

    if (frequencyType === ScheduledTemplateFrequencyType.Yearly.type) {
        return values.indexOf(month * 100 + day) >= 0;
    }

    if (frequencyType === ScheduledTemplateFrequencyType.EveryNDays.type) {
        const interval = values[0] as number;
        const startDayNumber = toDayNumber(startDate);

        // an every-N-days schedule counts from its start date, and without one there is nothing to
        // count from - the server refuses to store it and the cron passes over it
        if (interval <= 0 || startDayNumber === null) {
            return false;
        }

        const daysSinceStart = daysBetween(startDayNumber, year * 10000 + month * 100 + day);

        return daysSinceStart >= 0 && daysSinceStart % interval === 0;
    }

    return false;
}

function parseFrequencyValues(frequencyValue: string | undefined): number[] {
    if (!frequencyValue) {
        return [];
    }

    return frequencyValue.split(',').filter(value => !!value).map(value => parseInt(value));
}

// The calendar arithmetic is done in UTC throughout. A schedule's days are named in its own
// timezone's calendar - the 1st of the month is the 1st wherever the person paying lives - and
// reading them as UTC dates is what keeps a browser in another timezone from shifting them by a day.
function getDaysInMonth(year: number, month: number): number {
    return new Date(Date.UTC(year, month, 0)).getUTCDate();
}

function getWeekday(year: number, month: number, day: number): number {
    return new Date(Date.UTC(year, month - 1, day)).getUTCDay();
}

function toDayNumber(date: string | undefined): number | null {
    if (!date) {
        return null;
    }

    const parts = date.split('-');

    if (parts.length !== 3) {
        return null;
    }

    const year = parseInt(parts[0] as string);
    const month = parseInt(parts[1] as string);
    const day = parseInt(parts[2] as string);

    if (isNaN(year) || isNaN(month) || isNaN(day)) {
        return null;
    }

    return year * 10000 + month * 100 + day;
}

function daysBetween(fromDayNumber: number, toDayNumber2: number): number {
    const from = Date.UTC(Math.trunc(fromDayNumber / 10000), Math.trunc(fromDayNumber / 100) % 100 - 1, fromDayNumber % 100);
    const to = Date.UTC(Math.trunc(toDayNumber2 / 10000), Math.trunc(toDayNumber2 / 100) % 100 - 1, toDayNumber2 % 100);

    return Math.round((to - from) / 86400000);
}
