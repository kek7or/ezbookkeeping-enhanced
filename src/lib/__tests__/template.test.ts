import { describe, expect, test } from 'vitest';

import { TransactionType } from '@/core/transaction.ts';
import { TemplateType, ScheduledTemplateFrequencyType } from '@/core/template.ts';
import { TransactionTemplate, type TransactionTemplateInfoResponse } from '@/models/transaction_template.ts';
import { getScheduleOccurrencesPerYear, sumScheduledCostByCurrency } from '@/lib/template.ts';

// the standing payments and subscriptions of a household, as the template list receives them
function createSchedule(id: string, name: string, type: number, sourceAmount: number, frequencyType: number, frequency: string, options?: { accountId?: string, hidden?: boolean }): TransactionTemplate {
    const response: TransactionTemplateInfoResponse = {
        id: id,
        timeSequenceId: id + '000',
        templateType: TemplateType.Schedule.type,
        name: name,
        type: type,
        categoryId: '11',
        time: 0,
        utcOffset: 120,
        sourceAccountId: options?.accountId ?? '1001',
        destinationAccountId: '0',
        sourceAmount: sourceAmount,
        destinationAmount: 0,
        hideAmount: false,
        tagIds: [],
        comment: '',
        editable: true,
        scheduledFrequencyType: frequencyType,
        scheduledFrequency: frequency,
        displayOrder: 1,
        hidden: options?.hidden ?? false
    };

    return TransactionTemplate.ofTemplate(response);
}

const currencyOfAccount: Record<string, string> = {
    '1001': 'EUR',
    '1002': 'USD'
};

function currencyOf(template: TransactionTemplate): string | undefined {
    return currencyOfAccount[template.sourceAccountId];
}

describe('getScheduleOccurrencesPerYear', () => {
    test('should count one occurrence per named day of the period', () => {
        expect(getScheduleOccurrencesPerYear(ScheduledTemplateFrequencyType.Monthly.type, '3')).toBe(12);
        expect(getScheduleOccurrencesPerYear(ScheduledTemplateFrequencyType.Monthly.type, '1,15')).toBe(24);
        expect(getScheduleOccurrencesPerYear(ScheduledTemplateFrequencyType.Weekly.type, '1')).toBe(52);
        expect(getScheduleOccurrencesPerYear(ScheduledTemplateFrequencyType.Yearly.type, '101')).toBe(1);
        expect(getScheduleOccurrencesPerYear(ScheduledTemplateFrequencyType.Daily.type, '0')).toBe(365);
        expect(getScheduleOccurrencesPerYear(ScheduledTemplateFrequencyType.EveryNDays.type, '73')).toBe(5);
    });

    // the whole point of the day-less recurrence: the period still counts
    test('should count the period when no day is named', () => {
        expect(getScheduleOccurrencesPerYear(ScheduledTemplateFrequencyType.Monthly.type, '')).toBe(12);
        expect(getScheduleOccurrencesPerYear(ScheduledTemplateFrequencyType.Weekly.type, '')).toBe(52);
        expect(getScheduleOccurrencesPerYear(ScheduledTemplateFrequencyType.Yearly.type, '')).toBe(1);
        expect(getScheduleOccurrencesPerYear(ScheduledTemplateFrequencyType.Monthly.type, undefined)).toBe(12);
    });

    test('should count nothing for a disabled schedule or an interval it cannot read', () => {
        expect(getScheduleOccurrencesPerYear(ScheduledTemplateFrequencyType.Disabled.type, '')).toBe(0);
        expect(getScheduleOccurrencesPerYear(ScheduledTemplateFrequencyType.EveryNDays.type, '')).toBe(0);
        expect(getScheduleOccurrencesPerYear(ScheduledTemplateFrequencyType.EveryNDays.type, '0')).toBe(0);
    });
});

describe('sumScheduledCostByCurrency', () => {
    test('should bring a month, a year and a fortnight to the same yearly figure', () => {
        const totals = sumScheduledCostByCurrency([
            createSchedule('1', 'Apartment payment', TransactionType.Expense, 95000, ScheduledTemplateFrequencyType.Monthly.type, '3'),
            createSchedule('2', 'Insurance', TransactionType.Expense, 12000, ScheduledTemplateFrequencyType.Yearly.type, '101'),
            createSchedule('3', 'Cleaner', TransactionType.Expense, 4000, ScheduledTemplateFrequencyType.EveryNDays.type, '73')
        ], false, currencyOf);

        expect(totals.length).toBe(1);
        expect(totals[0]!.currency).toBe('EUR');
        expect(totals[0]!.yearlyExpense.toSafeIntegerNumber()).toBe(95000 * 12 + 12000 + 4000 * 5);
        expect(totals[0]!.yearlyIncome.isZero()).toBe(true);
    });

    test('should count a subscription whose day is unknown as twelve charges a year', () => {
        const totals = sumScheduledCostByCurrency([
            createSchedule('1', 'Claude', TransactionType.Expense, 2300, ScheduledTemplateFrequencyType.Monthly.type, '')
        ], false, currencyOf);

        expect(totals[0]!.yearlyExpense.toSafeIntegerNumber()).toBe(2300 * 12);
    });

    test('should keep currencies apart rather than converting them', () => {
        const totals = sumScheduledCostByCurrency([
            createSchedule('1', 'Strom', TransactionType.Expense, 8000, ScheduledTemplateFrequencyType.Monthly.type, '1'),
            createSchedule('2', 'Claude', TransactionType.Expense, 2000, ScheduledTemplateFrequencyType.Monthly.type, '', { accountId: '1002' })
        ], false, currencyOf);

        expect(totals.map(total => total.currency)).toEqual(['EUR', 'USD']);
        expect(totals[0]!.yearlyExpense.toSafeIntegerNumber()).toBe(8000 * 12);
        expect(totals[1]!.yearlyExpense.toSafeIntegerNumber()).toBe(2000 * 12);
    });

    test('should keep income out of the expense figure', () => {
        const totals = sumScheduledCostByCurrency([
            createSchedule('1', 'Salary', TransactionType.Income, 300000, ScheduledTemplateFrequencyType.Monthly.type, '28'),
            createSchedule('2', 'FitX', TransactionType.Expense, 2999, ScheduledTemplateFrequencyType.Monthly.type, '')
        ], false, currencyOf);

        expect(totals[0]!.yearlyIncome.toSafeIntegerNumber()).toBe(300000 * 12);
        expect(totals[0]!.yearlyExpense.toSafeIntegerNumber()).toBe(2999 * 12);
    });

    // a standing transfer into savings is neither spent nor earned
    test('should leave transfers out entirely', () => {
        const totals = sumScheduledCostByCurrency([
            createSchedule('1', 'To savings', TransactionType.Transfer, 50000, ScheduledTemplateFrequencyType.Monthly.type, '1')
        ], false, currencyOf);

        expect(totals.length).toBe(0);
    });

    test('should follow the list in what it counts, so a hidden row is only counted when it is shown', () => {
        const templates = [
            createSchedule('1', 'Strom', TransactionType.Expense, 8000, ScheduledTemplateFrequencyType.Monthly.type, '1'),
            createSchedule('2', 'Cancelled service', TransactionType.Expense, 1000, ScheduledTemplateFrequencyType.Monthly.type, '1', { hidden: true })
        ];

        expect(sumScheduledCostByCurrency(templates, false, currencyOf)[0]!.yearlyExpense.toSafeIntegerNumber()).toBe(8000 * 12);
        expect(sumScheduledCostByCurrency(templates, true, currencyOf)[0]!.yearlyExpense.toSafeIntegerNumber()).toBe(9000 * 12);
    });

    test('should skip a schedule whose account it cannot price', () => {
        const totals = sumScheduledCostByCurrency([
            createSchedule('1', 'Gone account', TransactionType.Expense, 8000, ScheduledTemplateFrequencyType.Monthly.type, '1', { accountId: '9999' })
        ], false, currencyOf);

        expect(totals.length).toBe(0);
    });
});
