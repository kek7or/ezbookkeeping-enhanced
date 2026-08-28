import { describe, expect, test } from 'vitest';

import { TransactionType } from '@/core/transaction.ts';
import { TemplateType, ScheduledTemplateFrequencyType } from '@/core/template.ts';
import { TransactionTemplate, type TransactionTemplateInfoResponse } from '@/models/transaction_template.ts';
import { BudgetPlanItem } from '@/models/budget_plan.ts';
import type { BudgetPlanAdjustment } from '@/models/budget_plan.ts';
import {
    getScheduleOccurrencesInMonth,
    buildScheduleLines,
    buildItemLines,
    sumPlannedLines
} from '@/lib/budgetPlan.ts';

function createSchedule(frequencyType: number, frequency: string, options?: { amount?: number, type?: number, name?: string, startDate?: string, endDate?: string, hidden?: boolean, id?: string }): TransactionTemplate {
    const response: TransactionTemplateInfoResponse = {
        id: options?.id ?? '1',
        timeSequenceId: '1000',
        templateType: TemplateType.Schedule.type,
        name: options?.name ?? 'Schedule',
        type: options?.type ?? TransactionType.Expense,
        categoryId: '11',
        time: 0,
        utcOffset: 120,
        sourceAccountId: '1001',
        destinationAccountId: '0',
        sourceAmount: options?.amount ?? 1000,
        destinationAmount: 0,
        hideAmount: false,
        tagIds: [],
        comment: '',
        editable: true,
        scheduledFrequencyType: frequencyType,
        scheduledFrequency: frequency,
        scheduledStartDate: options?.startDate as never,
        scheduledEndDate: options?.endDate as never,
        displayOrder: 1,
        hidden: options?.hidden ?? false
    };

    return TransactionTemplate.ofTemplate(response);
}

const currencyOfAccount: Record<string, string> = { '1001': 'EUR' };

function currencyOf(accountId: string): string | undefined {
    return currencyOfAccount[accountId];
}

function inEuros(amount: { toSafeIntegerNumber(): number }): number {
    return amount.toSafeIntegerNumber();
}

describe('getScheduleOccurrencesInMonth', () => {
    test('should count a monthly schedule once for each day it names', () => {
        expect(getScheduleOccurrencesInMonth(createSchedule(ScheduledTemplateFrequencyType.Monthly.type, '1'), 2026, 8)).toBe(1);
        expect(getScheduleOccurrencesInMonth(createSchedule(ScheduledTemplateFrequencyType.Monthly.type, '1,15'), 2026, 8)).toBe(2);
    });

    // the plan is acted on, so a month that does not contain the day must not be planned for it
    test('should not plan a day the month does not have', () => {
        expect(getScheduleOccurrencesInMonth(createSchedule(ScheduledTemplateFrequencyType.Monthly.type, '31'), 2026, 1)).toBe(1);
        expect(getScheduleOccurrencesInMonth(createSchedule(ScheduledTemplateFrequencyType.Monthly.type, '31'), 2026, 4)).toBe(0);
        expect(getScheduleOccurrencesInMonth(createSchedule(ScheduledTemplateFrequencyType.Monthly.type, '29'), 2026, 2)).toBe(0);
        expect(getScheduleOccurrencesInMonth(createSchedule(ScheduledTemplateFrequencyType.Monthly.type, '29'), 2024, 2)).toBe(1);
    });

    test('should read a negative monthly day as counting back from the end of the month', () => {
        expect(getScheduleOccurrencesInMonth(createSchedule(ScheduledTemplateFrequencyType.Monthly.type, '-1'), 2026, 2)).toBe(1);
        expect(getScheduleOccurrencesInMonth(createSchedule(ScheduledTemplateFrequencyType.Monthly.type, '-1'), 2026, 4)).toBe(1);
    });

    // a weekly 50 is 250 in a five-Sunday month and 200 in a four-Sunday one, and the whole reason
    // the month is counted rather than averaged
    test('should count the weekdays the month actually has', () => {
        expect(getScheduleOccurrencesInMonth(createSchedule(ScheduledTemplateFrequencyType.Weekly.type, '0'), 2026, 3)).toBe(5);
        expect(getScheduleOccurrencesInMonth(createSchedule(ScheduledTemplateFrequencyType.Weekly.type, '4'), 2026, 1)).toBe(5);
        expect(getScheduleOccurrencesInMonth(createSchedule(ScheduledTemplateFrequencyType.Weekly.type, '0'), 2026, 2)).toBe(4);
    });

    test('should count a daily schedule once for every day of the month', () => {
        expect(getScheduleOccurrencesInMonth(createSchedule(ScheduledTemplateFrequencyType.Daily.type, '0'), 2026, 2)).toBe(28);
        expect(getScheduleOccurrencesInMonth(createSchedule(ScheduledTemplateFrequencyType.Daily.type, '0'), 2026, 1)).toBe(31);
    });

    test('should place a yearly schedule in its own month and nowhere else', () => {
        const insurance = createSchedule(ScheduledTemplateFrequencyType.Yearly.type, '311');

        expect(getScheduleOccurrencesInMonth(insurance, 2026, 3)).toBe(1);
        expect(getScheduleOccurrencesInMonth(insurance, 2026, 4)).toBe(0);
        expect(getScheduleOccurrencesInMonth(insurance, 2026, 11)).toBe(0);
    });

    test('should step an every-N-days schedule from its start date', () => {
        const cleaner = createSchedule(ScheduledTemplateFrequencyType.EveryNDays.type, '14', { startDate: '2026-01-01' });

        expect(getScheduleOccurrencesInMonth(cleaner, 2026, 1)).toBe(3);
        expect(getScheduleOccurrencesInMonth(cleaner, 2026, 2)).toBe(2);
    });

    test('should not plan an every-N-days schedule with nothing to count from', () => {
        expect(getScheduleOccurrencesInMonth(createSchedule(ScheduledTemplateFrequencyType.EveryNDays.type, '14'), 2026, 1)).toBe(0);
    });

    test('should plan nothing outside the dates the schedule runs between', () => {
        const gym = createSchedule(ScheduledTemplateFrequencyType.Monthly.type, '1', { startDate: '2026-03-01', endDate: '2026-05-31' });

        expect(getScheduleOccurrencesInMonth(gym, 2026, 2)).toBe(0);
        expect(getScheduleOccurrencesInMonth(gym, 2026, 3)).toBe(1);
        expect(getScheduleOccurrencesInMonth(gym, 2026, 5)).toBe(1);
        expect(getScheduleOccurrencesInMonth(gym, 2026, 6)).toBe(0);
    });

    // a subscription that renews monthly on whichever day the merchant charges still costs the month
    // exactly one of itself
    test('should plan a day-less monthly subscription once in every month', () => {
        const claude = createSchedule(ScheduledTemplateFrequencyType.Monthly.type, '');

        expect(getScheduleOccurrencesInMonth(claude, 2026, 2)).toBe(1);
        expect(getScheduleOccurrencesInMonth(claude, 2026, 8)).toBe(1);
    });

    // an annual bill with no date is best met by setting a twelfth of it aside every month
    test('should spread a day-less yearly schedule evenly over the year', () => {
        const domain = createSchedule(ScheduledTemplateFrequencyType.Yearly.type, '');

        expect(getScheduleOccurrencesInMonth(domain, 2026, 4)).toBeCloseTo(1 / 12);
    });

    test('should plan nothing for a disabled schedule', () => {
        expect(getScheduleOccurrencesInMonth(createSchedule(ScheduledTemplateFrequencyType.Disabled.type, ''), 2026, 8)).toBe(0);
    });
});

describe('buildScheduleLines', () => {
    test('should cost a schedule at what it happens times what it costs', () => {
        const lines = buildScheduleLines([
            createSchedule(ScheduledTemplateFrequencyType.Weekly.type, '0', { amount: 5000, name: 'Cleaner' })
        ], {}, 2026, 3, currencyOf);

        expect(lines.length).toBe(1);
        expect(lines[0]!.occurrences).toBe(5);
        expect(inEuros(lines[0]!.amount)).toBe(25000);
    });

    test('should leave out a hidden schedule and a transfer', () => {
        const lines = buildScheduleLines([
            createSchedule(ScheduledTemplateFrequencyType.Monthly.type, '1', { hidden: true, id: '1' }),
            createSchedule(ScheduledTemplateFrequencyType.Monthly.type, '1', { type: TransactionType.Transfer, id: '2' })
        ], {}, 2026, 8, currencyOf);

        expect(lines.length).toBe(0);
    });

    test('should use this month s amount when the month says it differs', () => {
        const adjustments: Record<string, BudgetPlanAdjustment> = {
            '1': { id: '9', year: 2026, month: 10, templateId: '1', excluded: false, amount: 96000 }
        };

        const lines = buildScheduleLines([
            createSchedule(ScheduledTemplateFrequencyType.Monthly.type, '1', { amount: 94930, name: 'Apartment payment' })
        ], adjustments, 2026, 10, currencyOf);

        expect(lines[0]!.adjusted).toBe(true);
        expect(lines[0]!.unitAmount).toBe(96000);
        expect(inEuros(lines[0]!.amount)).toBe(96000);
    });

    // a subscription paused for one month is coming back, so it stays on the page and out of the total
    test('should show an excluded schedule and count it as nothing', () => {
        const adjustments: Record<string, BudgetPlanAdjustment> = {
            '1': { id: '9', year: 2026, month: 8, templateId: '1', excluded: true }
        };

        const lines = buildScheduleLines([
            createSchedule(ScheduledTemplateFrequencyType.Monthly.type, '1', { amount: 2900, name: 'FitX' })
        ], adjustments, 2026, 8, currencyOf);

        expect(lines.length).toBe(1);
        expect(lines[0]!.excluded).toBe(true);
        expect(inEuros(lines[0]!.amount)).toBe(0);
        expect(inEuros(sumPlannedLines(lines, amount => amount).expense)).toBe(0);
    });
});

describe('sumPlannedLines', () => {
    test('should keep the income and the expense of a month apart and net them', () => {
        const lines = buildScheduleLines([
            createSchedule(ScheduledTemplateFrequencyType.Monthly.type, '28', { amount: 300000, type: TransactionType.Income, name: 'Salary', id: '1' }),
            createSchedule(ScheduledTemplateFrequencyType.Monthly.type, '1', { amount: 94930, name: 'Apartment payment', id: '2' })
        ], {}, 2026, 8, currencyOf).concat(buildItemLines([
            BudgetPlanItem.of({ id: '10', year: 2026, month: 8, type: TransactionType.Expense, categoryId: '11', accountId: '1001', amount: 45000, name: 'Flight', comment: '', displayOrder: 1 })
        ], currencyOf));

        const totals = sumPlannedLines(lines, amount => amount);

        expect(inEuros(totals.income)).toBe(300000);
        expect(inEuros(totals.expense)).toBe(94930 + 45000);
        expect(inEuros(totals.net)).toBe(300000 - 94930 - 45000);
    });

    test('should drop a line it cannot price rather than counting it as nothing', () => {
        const lines = buildItemLines([
            BudgetPlanItem.of({ id: '10', year: 2026, month: 8, type: TransactionType.Expense, categoryId: '11', accountId: '9999', amount: 45000, name: 'Flight', comment: '', displayOrder: 1 })
        ], currencyOf);

        expect(lines.length).toBe(0);
    });
});
