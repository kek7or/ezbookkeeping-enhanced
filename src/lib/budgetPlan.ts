import { TransactionType } from '@/core/transaction.ts';
import { CategoryType } from '@/core/category.ts';
import { ScheduledTemplateFrequencyType } from '@/core/template.ts';
import { type BigDecimal } from '@/core/numeral.ts';
import { TransactionTemplate } from '@/models/transaction_template.ts';
import { TransactionCategory } from '@/models/transaction_category.ts';
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
// - the annual bill is best met by setting aside a twelfth of it every month. It still counts for
// nothing outside the months the schedule runs in, which has to be asked outright: having no day to
// enumerate, it would otherwise charge itself against every month there has ever been.
export function getScheduleOccurrencesInMonth(template: TransactionTemplate, year: number, month: number): number {
    const frequencyType = template.scheduledFrequencyType;
    const values = parseFrequencyValues(template.scheduledFrequency);

    if (frequencyType === undefined || frequencyType === ScheduledTemplateFrequencyType.Disabled.type) {
        return 0;
    }

    const daysInMonth = getDaysInMonth(year, month);
    const startDayNumber = toDayNumber(template.scheduledStartDate);
    const endDayNumber = toDayNumber(template.scheduledEndDate);
    const firstDayOfMonth = year * 10000 + month * 100 + 1;
    const lastDayOfMonth = year * 10000 + month * 100 + daysInMonth;

    if (startDayNumber !== null && startDayNumber > lastDayOfMonth) {
        return 0;
    }

    if (endDayNumber !== null && endDayNumber < firstDayOfMonth) {
        return 0;
    }

    if (!values.length) {
        if (frequencyType === ScheduledTemplateFrequencyType.Monthly.type) {
            return 1;
        }

        return getScheduleOccurrencesPerYear(frequencyType, template.scheduledFrequency) / MONTHS_PER_YEAR;
    }

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

// getPlannedTypesByCategory says, for each category the plan names, whether what is planned in it
// is money coming in or going out. It is only consulted for a category that is no longer in the
// tree, where there is nothing else left to read the type off.
export function getPlannedTypesByCategory(lines: PlannedLine[]): Record<string, number> {
    const types: Record<string, number> = {};

    for (const line of lines) {
        types[line.categoryId] = line.type;
    }

    return types;
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

// CategoryBudgetNode is one category of the plan, at either level of the two the categories have.
//
// Both levels can carry an expectation and they mean different things. A primary category's is the
// ceiling for the whole branch - six hundred for Food. A secondary's is that branch divided up -
// four hundred of the six for Groceries. Set both and the difference between them is money that has
// a home in the branch but not yet in a subcategory, which is exactly the state a month is usually
// planned in.
export interface CategoryBudgetNode {
    readonly categoryId: string;
    readonly parentId: string;
    readonly type: number;
    // expectation is what was typed against this category, and null when nothing was. Zero is never
    // stored, because an expectation of zero says no more than the absence of one.
    readonly expectation: BigDecimal | null;
    // expectationIsStanding says the figure is the one this category falls back on every month
    // rather than something said about this month in particular. The arithmetic does not care -
    // both are the same ceiling - but the page has to, because the two are cleared differently and
    // a standing figure that looked like a one-off would be edited by somebody expecting it to stay
    // put for a month.
    readonly expectationIsStanding: boolean;
    // plannedDirect is what the lines filed on this category itself come to. planned adds the
    // children to it, so a primary's planned is what its whole branch is itemised at.
    readonly plannedDirect: BigDecimal;
    readonly planned: BigDecimal;
    // allocated is what this node's contents come to once each child is taken at its own budget
    // rather than at what it is itemised for - the figure the expectation is a ceiling over.
    readonly allocated: BigDecimal;
    // budget is what this category costs the month. See the rule in resolveNode.
    readonly budget: BigDecimal;
    readonly actualDirect: BigDecimal;
    readonly actual: BigDecimal;
    // remaining is budget less what has actually happened, and goes negative when overspent -
    // which is the number worth showing, so it is not clamped
    readonly remaining: BigDecimal;
    // unallocated is the part of this category's budget that nothing under it accounts for yet:
    // the two hundred of Food that is neither Groceries nor Restaurants.
    readonly unallocated: BigDecimal;
    // overAllocated says the contents come to more than the ceiling. The contents win - a bill that
    // exists cannot be wished down - so this is a flag, not a correction.
    readonly overAllocated: boolean;
    readonly children: CategoryBudgetNode[];
}

// buildCategoryBudgetTree puts the plan, the ledger and the expectations onto the category tree.
//
// An expectation covers what is already planned under its category rather than adding to it, which
// is the only rule that lets the two levels coexist. Adding would double-count the moment someone
// says six hundred for Food and four hundred for Groceries; covering means the branch costs six
// hundred either way, and the four hundred is a statement about where inside it the money goes.
//
// Categories named by the plan or the ledger that are not in the tree - deleted, or arriving from
// an import - are kept as roots of their own rather than dropped, because a figure that vanishes
// from a total is worse than one filed under a name nobody recognises.
export function buildCategoryBudgetTree(categories: TransactionCategory[], plannedByCategory: Record<string, BigDecimal>, actualByCategory: Record<string, BigDecimal>, expectationByCategory: Record<string, BigDecimal>, orphanTypeByCategory?: Record<string, number>, standingCategoryIds?: ReadonlySet<string>): CategoryBudgetNode[] {
    const roots: CategoryBudgetNode[] = [];
    const placed = new Set<string>();

    for (const category of categories) {
        const children: CategoryBudgetNode[] = [];

        for (const subCategory of (category.subCategories || [])) {
            placed.add(subCategory.id);
            children.push(resolveNode(subCategory.id, category.id, toTransactionType(subCategory.type), [], plannedByCategory, actualByCategory, expectationByCategory, standingCategoryIds));
        }

        placed.add(category.id);
        roots.push(resolveNode(category.id, '0', toTransactionType(category.type), children, plannedByCategory, actualByCategory, expectationByCategory, standingCategoryIds));
    }

    const orphanIds = new Set<string>();

    for (const categoryId in plannedByCategory) {
        if (!placed.has(categoryId)) {
            orphanIds.add(categoryId);
        }
    }

    for (const categoryId in actualByCategory) {
        if (!placed.has(categoryId)) {
            orphanIds.add(categoryId);
        }
    }

    for (const categoryId of orphanIds) {
        // A category the plan names but the tree does not still has lines, and a line knows whether
        // it is money coming in or going out even when the category it points at has been deleted.
        // Only where there is no line either - a figure that is in the ledger alone - is there
        // nothing to read, and then it is taken as money going out, the safer of the two guesses.
        const type = orphanTypeByCategory?.[categoryId] ?? TransactionType.Expense;

        roots.push(resolveNode(categoryId, '0', type, [], plannedByCategory, actualByCategory, expectationByCategory, standingCategoryIds));
    }

    return roots;
}

// sumCategoryBudgets is what the month comes to once the expectations are taken into account. With
// no expectation anywhere it is exactly the sum of the planned lines, which is why adding the
// feature does not move anybody's existing figures.
export function sumCategoryBudgets(nodes: CategoryBudgetNode[]): PlanTotals {
    let income = BIG_DECIMAL_ZERO;
    let expense = BIG_DECIMAL_ZERO;

    for (const node of nodes) {
        if (node.type === TransactionType.Income) {
            income = income.add(node.budget);
        } else {
            expense = expense.add(node.budget);
        }
    }

    return {
        income: income,
        expense: expense,
        net: income.subtract(expense)
    };
}

// sumRemainingToSpend is what the plan still expects to go out: the part of every budget that has
// not been spent yet.
//
// Nothing here is allowed to come out negative. A category already spent past its budget has
// nothing further coming from the plan, but the overspend is real and is already counted in what
// has been spent - so letting it go negative would spend one category's overrun out of another
// category's remaining budget, and the month would look better than it is.
//
// Which level does the clamping is not a detail. Where a category carries an expectation of its
// own, that expectation is a ceiling over everything beneath it, and the branch is measured whole:
// six hundred for Food with five hundred gone leaves a hundred, however unevenly the five hundred
// fell across the subcategories. Where it does not, there is no ceiling to measure against and the
// subcategories are each measured alone - otherwise a grocery overspend would quietly cancel the
// restaurant budget that is still there to be used.
export function sumRemainingToSpend(nodes: CategoryBudgetNode[]): BigDecimal {
    let total = BIG_DECIMAL_ZERO;

    for (const node of nodes) {
        if (node.type === TransactionType.Income) {
            continue;
        }

        total = total.add(remainingOf(node));
    }

    return total;
}

function remainingOf(node: CategoryBudgetNode): BigDecimal {
    if (node.expectation || !node.children.length) {
        return positivePart(node.budget.subtract(node.actual));
    }

    // an uncapped branch is the sum of its parts, plus whatever is filed on the branch itself
    let total = positivePart(node.plannedDirect.subtract(node.actualDirect));

    for (const child of node.children) {
        total = total.add(remainingOf(child));
    }

    return total;
}

function positivePart(amount: BigDecimal): BigDecimal {
    return amount.isPositive() ? amount : BIG_DECIMAL_ZERO;
}

// hasCategoryBudgetActivity says whether a category is worth a row of its own: something was
// expected of it, something is planned in it, or something has already happened in it. A category
// that is none of those is one of the many a person keeps and does not use this month.
export function hasCategoryBudgetActivity(node: CategoryBudgetNode): boolean {
    return node.expectation !== null || !node.planned.isZero() || !node.actual.isZero();
}

function resolveNode(categoryId: string, parentId: string, type: number, children: CategoryBudgetNode[], plannedByCategory: Record<string, BigDecimal>, actualByCategory: Record<string, BigDecimal>, expectationByCategory: Record<string, BigDecimal>, standingCategoryIds?: ReadonlySet<string>): CategoryBudgetNode {
    const expectation = expectationByCategory[categoryId] ?? null;
    const plannedDirect = plannedByCategory[categoryId] ?? BIG_DECIMAL_ZERO;
    const actualDirect = actualByCategory[categoryId] ?? BIG_DECIMAL_ZERO;

    let planned = plannedDirect;
    let allocated = plannedDirect;
    let actual = actualDirect;

    for (const child of children) {
        planned = planned.add(child.planned);
        // each child enters its parent at its own budget, so a subcategory given a figure of its
        // own raises the branch even when nothing is itemised inside it
        allocated = allocated.add(child.budget);
        actual = actual.add(child.actual);
    }

    const budget = expectation && expectation.greaterThan(allocated) ? expectation : allocated;

    return {
        categoryId: categoryId,
        parentId: parentId,
        type: type,
        expectation: expectation,
        expectationIsStanding: !!expectation && !!standingCategoryIds?.has(categoryId),
        plannedDirect: plannedDirect,
        planned: planned,
        allocated: allocated,
        budget: budget,
        actualDirect: actualDirect,
        actual: actual,
        remaining: budget.subtract(actual),
        unallocated: budget.subtract(allocated),
        overAllocated: !!expectation && allocated.greaterThan(expectation),
        children: children
    };
}

function toTransactionType(categoryType: number): number {
    return categoryType === CategoryType.Income ? TransactionType.Income : TransactionType.Expense;
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
