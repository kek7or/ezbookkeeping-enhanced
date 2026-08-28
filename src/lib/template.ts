import { reversed } from '@/core/base.ts';
import { TransactionType } from '@/core/transaction.ts';
import { ScheduledTemplateFrequencyType } from '@/core/template.ts';
import { TransactionTemplate } from '@/models/transaction_template.ts';
import { type BigDecimal } from '@/core/numeral.ts';
import { BIG_DECIMAL_ZERO, parseBigDecimal } from '@/lib/numeral.ts';

export function isNoAvailableTemplate(templates: TransactionTemplate[], showHidden: boolean): boolean {
    for (const template of templates) {
        if (showHidden || !template.hidden) {
            return false;
        }
    }

    return true;
}

export function getAvailableTemplateCount(templates: TransactionTemplate[], showHidden: boolean): number {
    let count = 0;

    for (const template of templates) {
        if (showHidden || !template.hidden) {
            count++;
        }
    }

    return count;
}

export function getFirstShowingId(templates: TransactionTemplate[], showHidden: boolean): string | null {
    for (const template of templates) {
        if (showHidden || !template.hidden) {
            return template.id;
        }
    }

    return null;
}

export function getLastShowingId(templates: TransactionTemplate[], showHidden: boolean): string | null {
    for (const template of reversed(templates)) {
        if (showHidden || !template.hidden) {
            return template.id;
        }
    }

    return null;
}

// A weekly schedule is counted as 52 a year and a daily one as 365. Neither is the exact length of
// a year, and using the exact length would be worse: these figures are read as "what a year of this
// costs", and 52 weeks and 365 days are the counts a person totalling it by hand would use.
const OCCURRENCES_PER_YEAR_DAILY = 365;
const OCCURRENCES_PER_YEAR_WEEKLY = 52;
const OCCURRENCES_PER_YEAR_MONTHLY = 12;

// getScheduleOccurrencesPerYear says how many times a year a schedule fires, which is the only
// thing that makes a weekly 5 and a yearly 99 comparable at all.
//
// A frequency with no value has no day picked, and that is not zero occurrences: a monthly
// subscription bills twelve times a year whether or not the day it bills on is known. The period
// alone decides the count, and a schedule that names several days within its period fires once for
// each of them.
export function getScheduleOccurrencesPerYear(frequencyType: number | undefined, frequencyValue: string | undefined): number {
    const values = getFrequencyValues(frequencyValue);

    if (frequencyType === ScheduledTemplateFrequencyType.Daily.type) {
        return OCCURRENCES_PER_YEAR_DAILY;
    } else if (frequencyType === ScheduledTemplateFrequencyType.EveryNDays.type) {
        const days = values.length ? values[0] as number : 0;
        return days > 0 ? OCCURRENCES_PER_YEAR_DAILY / days : 0;
    } else if (frequencyType === ScheduledTemplateFrequencyType.Weekly.type) {
        return OCCURRENCES_PER_YEAR_WEEKLY * Math.max(values.length, 1);
    } else if (frequencyType === ScheduledTemplateFrequencyType.Monthly.type) {
        return OCCURRENCES_PER_YEAR_MONTHLY * Math.max(values.length, 1);
    } else if (frequencyType === ScheduledTemplateFrequencyType.Yearly.type) {
        return Math.max(values.length, 1);
    }

    return 0;
}

export interface ScheduledCostTotal {
    readonly currency: string;
    readonly yearlyExpense: BigDecimal;
    readonly yearlyIncome: BigDecimal;
}

// sumScheduledCostByCurrency totals what a list of schedules comes to over a year, kept apart by
// currency rather than converted into one. A rate would turn the total into an estimate that moves
// on its own overnight, and this figure is meant to be the arithmetic of the schedules themselves -
// the same reason a debt is never restated in another currency.
//
// Transfers are left out. Money moved between two accounts of the same ledger is neither spent nor
// earned, and counting a standing transfer into savings as expense would say you spend more than
// you do. Income and expense are kept apart for the same reason: netting a salary against a
// subscription answers no question anybody asked.
//
// Amounts are in minor units throughout, as they are stored, so nothing is rounded until it is
// shown.
export function sumScheduledCostByCurrency(templates: TransactionTemplate[], showHidden: boolean, getCurrency: (template: TransactionTemplate) => string | undefined): ScheduledCostTotal[] {
    const totalsByCurrency = new Map<string, { currency: string, yearlyExpense: BigDecimal, yearlyIncome: BigDecimal }>();

    for (const template of templates) {
        if (!showHidden && template.hidden) {
            continue;
        }

        if (template.type === TransactionType.Transfer) {
            continue;
        }

        const occurrencesPerYear = getScheduleOccurrencesPerYear(template.scheduledFrequencyType, template.scheduledFrequency);

        if (occurrencesPerYear <= 0) {
            continue;
        }

        const currency = getCurrency(template);

        if (!currency) {
            continue;
        }

        let total = totalsByCurrency.get(currency);

        if (!total) {
            total = { currency: currency, yearlyExpense: BIG_DECIMAL_ZERO, yearlyIncome: BIG_DECIMAL_ZERO };
            totalsByCurrency.set(currency, total);
        }

        const yearlyAmount = parseBigDecimal(template.sourceAmount).multiply(occurrencesPerYear);

        if (template.type === TransactionType.Income) {
            total.yearlyIncome = total.yearlyIncome.add(yearlyAmount);
        } else {
            total.yearlyExpense = total.yearlyExpense.add(yearlyAmount);
        }
    }

    return Array.from(totalsByCurrency.values()).sort((total1, total2) => total1.currency.localeCompare(total2.currency));
}

function getFrequencyValues(frequencyValue: string | undefined): number[] {
    if (!frequencyValue) {
        return [];
    }

    return frequencyValue.split(',').filter(value => !!value).map(value => parseInt(value));
}
