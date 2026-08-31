import { describe, expect, test } from 'vitest';

import { TransactionType } from '@/core/transaction.ts';
import { CategoryType } from '@/core/category.ts';
import { TemplateType, ScheduledTemplateFrequencyType } from '@/core/template.ts';
import { TransactionTemplate, type TransactionTemplateInfoResponse } from '@/models/transaction_template.ts';
import { TransactionCategory, type TransactionCategoryInfoResponse } from '@/models/transaction_category.ts';
import { BudgetPlanItem } from '@/models/budget_plan.ts';
import type { BudgetPlanAdjustment } from '@/models/budget_plan.ts';
import type { BigDecimal } from '@/core/numeral.ts';
import { parseBigDecimal } from '@/lib/numeral.ts';
import {
    getScheduleOccurrencesInMonth,
    buildScheduleLines,
    buildItemLines,
    buildWishLines,
    sumPlannedLines,
    buildCategoryBudgetTree,
    sumCategoryBudgets,
    sumRemainingToSpend,
    hasCategoryBudgetActivity
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

    // having no day to enumerate, a day-less schedule has to be asked outright whether it runs in
    // the month at all - otherwise a subscription taken out last week is charged against every month
    // there has ever been
    test('should plan nothing for a day-less schedule outside the dates it runs between', () => {
        const claude = createSchedule(ScheduledTemplateFrequencyType.Monthly.type, '', { startDate: '2026-03-15', endDate: '2026-06-10' });

        expect(getScheduleOccurrencesInMonth(claude, 2019, 1)).toBe(0);
        expect(getScheduleOccurrencesInMonth(claude, 2026, 2)).toBe(0);
        expect(getScheduleOccurrencesInMonth(claude, 2026, 3)).toBe(1);
        expect(getScheduleOccurrencesInMonth(claude, 2026, 6)).toBe(1);
        expect(getScheduleOccurrencesInMonth(claude, 2026, 7)).toBe(0);
    });

    test('should spread a day-less yearly schedule over the year only while it runs', () => {
        const domain = createSchedule(ScheduledTemplateFrequencyType.Yearly.type, '', { startDate: '2026-03-01' });

        expect(getScheduleOccurrencesInMonth(domain, 2025, 12)).toBe(0);
        expect(getScheduleOccurrencesInMonth(domain, 2026, 4)).toBeCloseTo(1 / 12);
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
            BudgetPlanItem.of({ id: '10', year: 2026, month: 8, wished: false, type: TransactionType.Expense, categoryId: '11', accountId: '1001', amount: 45000, name: 'Flight', comment: '', displayOrder: 1 })
        ], currencyOf));

        const totals = sumPlannedLines(lines, amount => amount);

        expect(inEuros(totals.income)).toBe(300000);
        expect(inEuros(totals.expense)).toBe(94930 + 45000);
        expect(inEuros(totals.net)).toBe(300000 - 94930 - 45000);
    });

    // a wish is jotted down before it is thought through, so it must survive naming no account -
    // unlike a planned item, which is dropped for the same omission
    test('should price a wish with no account in the user own currency', () => {
        const lines = buildWishLines([
            BudgetPlanItem.of({ id: '20', year: 0, month: 0, wished: true, type: TransactionType.Expense, categoryId: '', accountId: '', amount: 80000, name: 'Sofa', comment: '', displayOrder: 1 })
        ], currencyOf, 'EUR');

        expect(lines.length).toBe(1);
        expect(lines[0]?.currency).toBe('EUR');
        expect(inEuros(lines[0]?.amount as BigDecimal)).toBe(80000);
        expect(lines[0]?.categoryId).toBe('');
    });

    test('should read a wish in the currency of the account it does name', () => {
        const lines = buildWishLines([
            BudgetPlanItem.of({ id: '21', year: 0, month: 0, wished: true, type: TransactionType.Expense, categoryId: '11', accountId: '1001', amount: 120000, name: 'PC', comment: '', displayOrder: 1 })
        ], currencyOf, 'USD');

        expect(lines[0]?.currency).toBe('EUR');
    });

    test('should drop a line it cannot price rather than counting it as nothing', () => {
        const lines = buildItemLines([
            BudgetPlanItem.of({ id: '10', year: 2026, month: 8, wished: false, type: TransactionType.Expense, categoryId: '11', accountId: '9999', amount: 45000, name: 'Flight', comment: '', displayOrder: 1 })
        ], currencyOf);

        expect(lines.length).toBe(0);
    });
});

function createCategory(id: string, name: string, type: number, subCategories?: { id: string, name: string }[]): TransactionCategory {
    const response: TransactionCategoryInfoResponse = {
        id: id,
        name: name,
        parentId: '0',
        type: type,
        icon: '1',
        color: '000000',
        comment: '',
        displayOrder: 1,
        hidden: false,
        excludeFromStatistics: false,
        subCategories: (subCategories ?? []).map((subCategory, index) => ({
            id: subCategory.id,
            name: subCategory.name,
            parentId: id,
            type: type,
            icon: '1',
            color: '000000',
            comment: '',
            displayOrder: index + 1,
            hidden: false,
            excludeFromStatistics: false
        }))
    };

    return TransactionCategory.of(response);
}

function amounts(values: Record<string, number>): Record<string, BigDecimal> {
    const map: Record<string, BigDecimal> = {};

    for (const key in values) {
        map[key] = parseBigDecimal(values[key] as number);
    }

    return map;
}

function findNode(nodes: ReturnType<typeof buildCategoryBudgetTree>, categoryId: string) {
    for (const node of nodes) {
        if (node.categoryId === categoryId) {
            return node;
        }

        for (const child of node.children) {
            if (child.categoryId === categoryId) {
                return child;
            }
        }
    }

    throw new Error(`no node for ${categoryId}`);
}

describe('budgetPlan buildCategoryBudgetTree', () => {
    const food = createCategory('10', 'Food', CategoryType.Expense, [
        { id: '11', name: 'Groceries' },
        { id: '12', name: 'Restaurants' }
    ]);
    const salary = createCategory('20', 'Salary', CategoryType.Income, [
        { id: '21', name: 'Pay' }
    ]);

    test('a branch with no expectation costs what is listed under it', () => {
        const tree = buildCategoryBudgetTree([food], amounts({ '11': 20000, '12': 5000 }), {}, {});
        const branch = findNode(tree, '10');

        expect(branch.planned.toSafeIntegerNumber()).toBe(25000);
        expect(branch.budget.toSafeIntegerNumber()).toBe(25000);
        expect(branch.unallocated.toSafeIntegerNumber()).toBe(0);
        expect(branch.overAllocated).toBe(false);
    });

    test('an expectation on a primary covers what is planned under it rather than adding to it', () => {
        const tree = buildCategoryBudgetTree([food], amounts({ '11': 20000 }), {}, amounts({ '10': 60000 }));
        const branch = findNode(tree, '10');

        expect(branch.budget.toSafeIntegerNumber()).toBe(60000);
        expect(branch.planned.toSafeIntegerNumber()).toBe(20000);
        expect(branch.unallocated.toSafeIntegerNumber()).toBe(40000);
    });

    test('an expectation on a secondary raises the branch it sits in', () => {
        const tree = buildCategoryBudgetTree([food], {}, {}, amounts({ '11': 40000, '12': 20000 }));

        expect(findNode(tree, '11').budget.toSafeIntegerNumber()).toBe(40000);
        expect(findNode(tree, '10').budget.toSafeIntegerNumber()).toBe(60000);
    });

    test('both levels at once leave the difference unallocated', () => {
        const tree = buildCategoryBudgetTree([food], {}, {}, amounts({ '10': 60000, '11': 40000 }));
        const branch = findNode(tree, '10');

        expect(branch.budget.toSafeIntegerNumber()).toBe(60000);
        expect(branch.allocated.toSafeIntegerNumber()).toBe(40000);
        expect(branch.unallocated.toSafeIntegerNumber()).toBe(20000);
        expect(branch.overAllocated).toBe(false);
    });

    test('subcategories adding up past the primary win, and the primary says so', () => {
        const tree = buildCategoryBudgetTree([food], {}, {}, amounts({ '10': 50000, '11': 40000, '12': 30000 }));
        const branch = findNode(tree, '10');

        expect(branch.budget.toSafeIntegerNumber()).toBe(70000);
        expect(branch.unallocated.toSafeIntegerNumber()).toBe(0);
        expect(branch.overAllocated).toBe(true);
    });

    test('a bill larger than the expectation is not wished down to it', () => {
        const tree = buildCategoryBudgetTree([food], amounts({ '11': 80000 }), {}, amounts({ '11': 40000 }));

        expect(findNode(tree, '11').budget.toSafeIntegerNumber()).toBe(80000);
        expect(findNode(tree, '11').overAllocated).toBe(true);
        expect(findNode(tree, '10').budget.toSafeIntegerNumber()).toBe(80000);
    });

    test('what has actually happened rolls up and is left over against the budget', () => {
        const tree = buildCategoryBudgetTree([food], {}, amounts({ '11': 15000, '12': 5000 }), amounts({ '10': 60000 }));
        const branch = findNode(tree, '10');

        expect(branch.actual.toSafeIntegerNumber()).toBe(20000);
        expect(branch.remaining.toSafeIntegerNumber()).toBe(40000);
    });

    test('overspending a category shows as a negative remainder rather than a zero', () => {
        const tree = buildCategoryBudgetTree([food], {}, amounts({ '11': 50000 }), amounts({ '11': 40000 }));

        expect(findNode(tree, '11').remaining.toSafeIntegerNumber()).toBe(-10000);
    });

    test('a category the plan and the ledger name but the tree does not is kept as its own root', () => {
        const tree = buildCategoryBudgetTree([food], amounts({ '99': 1000 }), amounts({ '98': 2000 }), {});

        expect(findNode(tree, '99').budget.toSafeIntegerNumber()).toBe(1000);
        expect(findNode(tree, '98').actual.toSafeIntegerNumber()).toBe(2000);
        expect(sumCategoryBudgets(tree).expense.toSafeIntegerNumber()).toBe(1000);
    });

    test('income and expense are totalled apart', () => {
        const tree = buildCategoryBudgetTree([food, salary], amounts({ '11': 20000, '21': 300000 }), {}, {});
        const totals = sumCategoryBudgets(tree);

        expect(totals.income.toSafeIntegerNumber()).toBe(300000);
        expect(totals.expense.toSafeIntegerNumber()).toBe(20000);
        expect(totals.net.toSafeIntegerNumber()).toBe(280000);
    });

    test('with no expectation anywhere the month totals exactly what is listed', () => {
        const planned = amounts({ '11': 20000, '12': 5000, '21': 300000 });
        const totals = sumCategoryBudgets(buildCategoryBudgetTree([food, salary], planned, {}, {}));

        expect(totals.expense.toSafeIntegerNumber()).toBe(25000);
        expect(totals.income.toSafeIntegerNumber()).toBe(300000);
    });

    test('a category with nothing expected, planned or spent is not worth a row', () => {
        const tree = buildCategoryBudgetTree([food], amounts({ '11': 20000 }), {}, {});

        expect(hasCategoryBudgetActivity(findNode(tree, '11'))).toBe(true);
        expect(hasCategoryBudgetActivity(findNode(tree, '12'))).toBe(false);
        expect(hasCategoryBudgetActivity(findNode(tree, '10'))).toBe(true);
    });
});

describe('budgetPlan sumRemainingToSpend', () => {
    const food = createCategory('10', 'Food', CategoryType.Expense, [
        { id: '11', name: 'Groceries' },
        { id: '12', name: 'Restaurants' }
    ]);
    const salary = createCategory('20', 'Salary', CategoryType.Income, [
        { id: '21', name: 'Pay' }
    ]);

    test('what is left of a budget is what is still to come', () => {
        const tree = buildCategoryBudgetTree([food], {}, amounts({ '11': 15000 }), amounts({ '11': 40000 }));

        expect(sumRemainingToSpend(tree).toSafeIntegerNumber()).toBe(25000);
    });

    test('a category spent past its budget has nothing further coming, not a negative', () => {
        const tree = buildCategoryBudgetTree([food], {}, amounts({ '11': 50000 }), amounts({ '11': 40000 }));

        expect(sumRemainingToSpend(tree).toSafeIntegerNumber()).toBe(0);
    });

    test('an overspent subcategory does not eat into a sibling that is still to be spent', () => {
        const tree = buildCategoryBudgetTree([food], {}, amounts({ '11': 50000 }), amounts({ '11': 40000, '12': 20000 }));

        // groceries is 100 over and restaurants has its whole 200 left. The month still expects the
        // 200 to go out - the overspend is already spent and cannot pay for it.
        expect(sumRemainingToSpend(tree).toSafeIntegerNumber()).toBe(20000);
    });

    test('a ceiling on the branch measures the branch whole, however unevenly it was spent', () => {
        const tree = buildCategoryBudgetTree([food], {}, amounts({ '11': 50000 }), amounts({ '10': 60000, '11': 40000, '12': 20000 }));

        // Food is capped at 600 and 500 of it is gone, so 100 is still to come - not the 200 that
        // adding the subcategories up on their own would have given
        expect(sumRemainingToSpend(tree).toSafeIntegerNumber()).toBe(10000);
    });

    test('income is not something the month still has to spend', () => {
        const tree = buildCategoryBudgetTree([food, salary], {}, {}, amounts({ '21': 300000 }));

        expect(sumRemainingToSpend(tree).toSafeIntegerNumber()).toBe(0);
    });

    test('a category spent with nothing planned or expected adds nothing still to come', () => {
        const tree = buildCategoryBudgetTree([food], {}, amounts({ '12': 9500 }), {});

        expect(sumRemainingToSpend(tree).toSafeIntegerNumber()).toBe(0);
    });

    test('a scheduled bill not yet paid is still to come', () => {
        const tree = buildCategoryBudgetTree([food], amounts({ '11': 20000 }), {}, {});

        expect(sumRemainingToSpend(tree).toSafeIntegerNumber()).toBe(20000);
    });
});

describe('budgetPlan standing expectations', () => {
    const food = createCategory('10', 'Food', CategoryType.Expense, [
        { id: '11', name: 'Groceries' },
        { id: '12', name: 'Restaurants' }
    ]);

    // The merge itself is the store's, so what is checked here is the tree's half of the bargain:
    // that it says which figures are standing and that a standing figure is a ceiling like any
    // other.
    test('a standing figure is a ceiling exactly as a month of its own would be', () => {
        const tree = buildCategoryBudgetTree([food], amounts({ '11': 20000 }), {}, amounts({ '10': 35000 }), undefined, new Set(['10']));
        const branch = findNode(tree, '10');

        expect(branch.budget.toSafeIntegerNumber()).toBe(35000);
        expect(branch.unallocated.toSafeIntegerNumber()).toBe(15000);
        expect(branch.expectationIsStanding).toBe(true);
    });

    test('a category the month has overridden is not marked as standing', () => {
        const tree = buildCategoryBudgetTree([food], {}, {}, amounts({ '10': 60000 }), undefined, new Set());

        expect(findNode(tree, '10').expectation?.toSafeIntegerNumber()).toBe(60000);
        expect(findNode(tree, '10').expectationIsStanding).toBe(false);
    });

    test('standing and overridden categories sit side by side', () => {
        const tree = buildCategoryBudgetTree([food], {}, {}, amounts({ '11': 35000, '12': 5000 }), undefined, new Set(['11']));

        expect(findNode(tree, '11').expectationIsStanding).toBe(true);
        expect(findNode(tree, '12').expectationIsStanding).toBe(false);
        expect(findNode(tree, '10').budget.toSafeIntegerNumber()).toBe(40000);
    });

    test('a category with no figure at all is not standing', () => {
        const tree = buildCategoryBudgetTree([food], {}, {}, {}, undefined, new Set(['12']));

        expect(findNode(tree, '12').expectationIsStanding).toBe(false);
    });

    test('a standing figure makes a category worth a row in a month with nothing in it', () => {
        const tree = buildCategoryBudgetTree([food], {}, {}, amounts({ '11': 5000 }), undefined, new Set(['11']));

        expect(hasCategoryBudgetActivity(findNode(tree, '11'))).toBe(true);
    });

    test('a standing figure still expects its unspent part to go out', () => {
        const tree = buildCategoryBudgetTree([food], {}, amounts({ '11': 12000 }), amounts({ '11': 35000 }), undefined, new Set(['11']));

        expect(sumRemainingToSpend(tree).toSafeIntegerNumber()).toBe(23000);
    });
});
