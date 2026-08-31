import { describe, test, expect } from 'vitest';

import { TransactionType } from '@/core/transaction.ts';
import type { BigDecimal } from '@/core/numeral.ts';
import { BIG_DECIMAL_ZERO, parseBigDecimal } from '@/lib/numeral.ts';

import { TransactionCategory } from '@/models/transaction_category.ts';
import type { TransactionCategoryInfoResponse } from '@/models/transaction_category.ts';
import { BudgetPlanItem } from '@/models/budget_plan.ts';

import {
    type PlannedLine,
    buildWishLines,
    buildItemLines,
    sumPlannedLinesByCategory,
    getPlannedTypesByCategory,
    buildCategoryBudgetTree,
    sumCategoryBudgets,
    sumRemainingToSpend
} from '@/lib/budgetPlan.ts';

// The one figure the wishlist exists to produce: what the month keeps if a thing is bought. It is
// worked out by running the whole category tree again with the wish in it, and these tests are why
// that is worth the trouble - the shortcut of subtracting the price is wrong wherever a category
// carries an expectation, which is wherever somebody has taken the trouble to set one.

function createCategory(id: string, name: string, type: number): TransactionCategory {
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
        subCategories: []
    };

    return TransactionCategory.of(response);
}

const currencyOfAccount: Record<string, string> = { '1001': 'EUR' };

function currencyOf(accountId: string): string | undefined {
    return currencyOfAccount[accountId];
}

function identity(amount: BigDecimal): BigDecimal {
    return amount;
}

function wish(id: string, categoryId: string, amount: number): BudgetPlanItem {
    return BudgetPlanItem.of({
        id: id,
        year: 0,
        month: 0,
        wished: true,
        type: TransactionType.Expense,
        categoryId: categoryId,
        accountId: '',
        amount: amount,
        name: 'Sofa',
        comment: '',
        displayOrder: 1
    });
}

function plannedItem(id: string, categoryId: string, amount: number): BudgetPlanItem {
    return BudgetPlanItem.of({
        id: id,
        year: 2026,
        month: 10,
        wished: false,
        type: TransactionType.Expense,
        categoryId: categoryId,
        accountId: '1001',
        amount: amount,
        name: 'Planned',
        comment: '',
        displayOrder: 1
    });
}

// netForLines is the store's own arithmetic, repeated here so the rule can be tested without a
// store: the income the month actually has, less what it has already spent and what it has left to
// spend.
function netForLines(lines: PlannedLine[], expectations: Record<string, BigDecimal>, income: BigDecimal): BigDecimal {
    const categories = [
        createCategory('11', 'Furniture', 2),
        createCategory('12', 'Food', 2)
    ];

    const tree = buildCategoryBudgetTree(
        categories,
        sumPlannedLinesByCategory(lines, identity),
        {},
        expectations,
        getPlannedTypesByCategory(lines),
        new Set<string>()
    );

    const totals = sumCategoryBudgets(tree);
    const basis = income.greaterThan(totals.income) ? income : totals.income;

    return basis.subtract(BIG_DECIMAL_ZERO.add(sumRemainingToSpend(tree)));
}

function euros(amount: BigDecimal): number {
    return amount.toSafeIntegerNumber();
}

describe('what a wish would do to the month', () => {
    const income = parseBigDecimal(300000);

    test('should take the whole price off a category nothing is expected of', () => {
        const before = netForLines([], {}, income);
        const after = netForLines(buildWishLines([wish('1', '11', 80000)], currencyOf, 'EUR'), {}, income);

        expect(euros(before)).toBe(300000);
        expect(euros(after)).toBe(300000 - 80000);
    });

    // the case the shortcut gets wrong. Furniture is already expected to come to 1,000 this month,
    // so an 800 sofa filed under it costs the month nothing it was not already costing - the money
    // was set aside for exactly this.
    test('should cost the month nothing where the category already expects it', () => {
        const expectations = { '11': parseBigDecimal(100000) };

        const before = netForLines([], expectations, income);
        const after = netForLines(buildWishLines([wish('1', '11', 80000)], currencyOf, 'EUR'), expectations, income);

        expect(euros(before)).toBe(300000 - 100000);
        expect(euros(after)).toBe(300000 - 100000);
        expect(euros(after)).toBe(euros(before));
    });

    // and where it costs more than was set aside, only the part that overruns is felt
    test('should cost only what overruns the expectation', () => {
        const expectations = { '11': parseBigDecimal(100000) };
        const lines = buildWishLines([wish('1', '11', 130000)], currencyOf, 'EUR');

        expect(euros(netForLines(lines, expectations, income))).toBe(300000 - 130000);
    });

    // a wish filed under nothing has no category to be covered by, so it is felt in full even where
    // other categories have money set aside
    test('should take the whole price off an uncategorised wish', () => {
        const expectations = { '11': parseBigDecimal(100000) };
        const lines = buildWishLines([wish('1', '', 80000)], currencyOf, 'EUR');

        expect(euros(netForLines(lines, expectations, income))).toBe(300000 - 100000 - 80000);
    });

    // Nobody buys one thing, so the figure that matters is what a handful of them come to together.
    // The wishlist shows two: what they are priced at, and what they cost the month - which are not
    // the same wherever a category has already made room for one of them.
    test('should charge the month for a basket of wishes together', () => {
        const lines = buildWishLines([
            wish('1', '11', 80000),
            wish('2', '12', 30000)
        ], currencyOf, 'EUR');

        const before = netForLines([], {}, income);
        const after = netForLines(lines, {}, income);

        expect(euros(before.subtract(after))).toBe(80000 + 30000);
    });

    test('should count only the part of a basket nothing had made room for', () => {
        const expectations = { '11': parseBigDecimal(100000) };
        const lines = buildWishLines([
            wish('1', '11', 80000),
            wish('2', '12', 30000)
        ], currencyOf, 'EUR');

        const before = netForLines([], expectations, income);
        const after = netForLines(lines, expectations, income);

        // the two are priced at 1,100 between them, and the sofa's 800 was already budgeted for
        const priced = 80000 + 30000;
        const cost = euros(before.subtract(after));

        expect(cost).toBe(30000);
        expect(priced - cost).toBe(80000);
    });

    // what is already planned under the category counts against the same expectation the wish does,
    // so the two share the money set aside rather than each taking it
    test('should share the expectation with what is already planned under it', () => {
        const expectations = { '11': parseBigDecimal(100000) };
        const planned = buildItemLines([plannedItem('9', '11', 60000)], currencyOf);
        const withWish = planned.concat(buildWishLines([wish('1', '11', 30000)], currencyOf, 'EUR'));

        expect(euros(netForLines(planned, expectations, income))).toBe(300000 - 100000);
        expect(euros(netForLines(withWish, expectations, income))).toBe(300000 - 100000);
    });
});
