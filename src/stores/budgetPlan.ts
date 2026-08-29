import { ref, computed } from 'vue';
import { defineStore } from 'pinia';

import { useUserStore } from './user.ts';
import { useAccountsStore } from './account.ts';
import { useTransactionCategoriesStore } from './transactionCategory.ts';
import { useTransactionTemplatesStore } from './transactionTemplate.ts';
import { useExchangeRatesStore } from './exchangeRates.ts';

import { CategoryType } from '@/core/category.ts';
import { TemplateType } from '@/core/template.ts';
import { TransactionType } from '@/core/transaction.ts';
import type { BigDecimal } from '@/core/numeral.ts';
import { KeywordMatchMode } from '@/core/text.ts';

import { TransactionTemplate } from '@/models/transaction_template.ts';
import { TransactionCategory } from '@/models/transaction_category.ts';
import { type BudgetPlanAdjustment, type BudgetPlanExpectation, BudgetPlanItem } from '@/models/budget_plan.ts';
import type { TransactionStatisticResponseItem } from '@/models/transaction.ts';

import {
    type PlannedLine,
    type PlanTotals,
    type CategoryBudgetNode,
    PlannedLineSource,
    buildScheduleLines,
    buildItemLines,
    sumPlannedLines,
    sumPlannedLinesByCategory,
    getPlannedTypesByCategory,
    buildCategoryBudgetTree,
    sumCategoryBudgets,
    sumRemainingToSpend
} from '@/lib/budgetPlan.ts';
import { BIG_DECIMAL_ZERO, parseBigDecimal } from '@/lib/numeral.ts';
import { getYearMonthFirstUnixTime, getYearMonthLastUnixTime, getCurrentDateTime } from '@/lib/datetime.ts';
import services from '@/lib/services.ts';
import logger from '@/lib/logger.ts';

export const useBudgetPlanStore = defineStore('budgetPlan', () => {
    const userStore = useUserStore();
    const accountsStore = useAccountsStore();
    const transactionCategoriesStore = useTransactionCategoriesStore();
    const transactionTemplatesStore = useTransactionTemplatesStore();
    const exchangeRatesStore = useExchangeRatesStore();

    const now = getCurrentDateTime();

    const year = ref<number>(now.getGregorianCalendarYear());
    const month = ref<number>(now.getGregorianCalendarMonth());
    const planItems = ref<BudgetPlanItem[]>([]);
    const planAdjustments = ref<BudgetPlanAdjustment[]>([]);
    const planExpectations = ref<BudgetPlanExpectation[]>([]);
    const actualItems = ref<TransactionStatisticResponseItem[]>([]);
    const planStateInvalid = ref<boolean>(true);

    const defaultCurrency = computed<string>(() => userStore.currentUserDefaultCurrency);

    // Everything below is derived. Nothing is cached and nothing is invalidated by hand: a plan item
    // saved, a schedule renamed, a month stepped through, all of them land in one of the refs above
    // and every figure on the page follows from it. That is the whole reason the plan stores only
    // what cannot be worked out.
    const adjustmentsByTemplateId = computed<Record<string, BudgetPlanAdjustment>>(() => {
        const map: Record<string, BudgetPlanAdjustment> = {};

        for (const adjustment of planAdjustments.value) {
            map[adjustment.templateId] = adjustment;
        }

        return map;
    });

    const scheduledTemplates = computed<TransactionTemplate[]>(() => transactionTemplatesStore.allTransactionTemplates[TemplateType.Schedule.type] || []);

    const scheduleLines = computed<PlannedLine[]>(() => buildScheduleLines(scheduledTemplates.value, adjustmentsByTemplateId.value, year.value, month.value, getAccountCurrency));

    const itemLines = computed<PlannedLine[]>(() => buildItemLines(planItems.value, getAccountCurrency));

    const allLines = computed<PlannedLine[]>(() => scheduleLines.value.concat(itemLines.value));

    const incomeLines = computed<PlannedLine[]>(() => allLines.value.filter(line => line.type === TransactionType.Income));
    const expenseLines = computed<PlannedLine[]>(() => allLines.value.filter(line => line.type !== TransactionType.Income));

    // plannedLineTotals is what the month is itemised at - every schedule and every planned item
    // added up, and nothing else. It is not what the month costs: a category expected to come to
    // more than what is listed under it costs the more. The two are kept apart because the
    // difference between them is a real thing to show, namely money set aside but not yet spoken for.
    const plannedLineTotals = computed<PlanTotals>(() => sumPlannedLines(allLines.value, convertToDefaultCurrency));

    // The committed part is what recurs whether or not anything is decided this month. What is left
    // of the income after it is the figure a month is actually planned within.
    const committedExpense = computed<BigDecimal>(() => sumPlannedLines(scheduleLines.value, convertToDefaultCurrency).expense);

    const plannedByCategory = computed<Record<string, BigDecimal>>(() => sumPlannedLinesByCategory(allLines.value, convertToDefaultCurrency));

    const expectationByCategory = computed<Record<string, BigDecimal>>(() => {
        const totals: Record<string, BigDecimal> = {};

        for (const expectation of planExpectations.value) {
            totals[expectation.categoryId] = parseBigDecimal(expectation.amount);
        }

        return totals;
    });

    // Only the two spending types are planned against. A transfer moves money between two accounts
    // of one ledger and is neither earned nor spent, so there is nothing about it to expect.
    const primaryCategories = computed<TransactionCategory[]>(() => {
        const expense = transactionCategoriesStore.allTransactionCategories[CategoryType.Expense] || [];
        const income = transactionCategoriesStore.allTransactionCategories[CategoryType.Income] || [];

        return expense.concat(income);
    });

    const actualByCategory = computed<Record<string, BigDecimal>>(() => {
        const totals: Record<string, BigDecimal> = {};

        for (const item of actualItems.value) {
            // a transfer names a related account and is neither spent nor earned, so it is left out
            // for the same reason it is left out of the plan
            if (item.relatedAccountId && item.relatedAccountId !== '0') {
                continue;
            }

            const account = accountsStore.allAccountsMap[item.accountId];

            if (!account) {
                continue;
            }

            const converted = convertToDefaultCurrency(parseBigDecimal(item.amount), account.currency);

            if (!converted) {
                continue;
            }

            totals[item.categoryId] = (totals[item.categoryId] ?? BIG_DECIMAL_ZERO).add(converted);
        }

        return totals;
    });

    // The tree is where the plan, the ledger and the expectations meet, and every figure the page
    // shows below the hero comes out of it.
    const categoryBudgetTree = computed<CategoryBudgetNode[]>(() => buildCategoryBudgetTree(primaryCategories.value, plannedByCategory.value, actualByCategory.value, expectationByCategory.value, getPlannedTypesByCategory(allLines.value)));

    // What the month is planned to cost is the tree's own total, not the sum of the lines: a
    // category expected to come to more than what is listed under it costs the more. With no
    // expectation set anywhere the two are the same figure.
    const plannedTotals = computed<PlanTotals>(() => sumCategoryBudgets(categoryBudgetTree.value));

    const actualTotals = computed<PlanTotals>(() => {
        let income = BIG_DECIMAL_ZERO;
        let expense = BIG_DECIMAL_ZERO;

        for (const categoryId in actualByCategory.value) {
            const amount = actualByCategory.value[categoryId] as BigDecimal;
            const category = transactionCategoriesStore.allTransactionCategoriesMap[categoryId];

            if (category && category.type === CategoryType.Income) {
                income = income.add(amount);
            } else if (category) {
                expense = expense.add(amount);
            }
        }

        return {
            income: income,
            expense: expense,
            net: income.subtract(expense)
        };
    });

    // incomeBasis is the income the month actually has, foreseen or not.
    //
    // Measuring a plan against planned income alone is wrong for the commonest case there is: a
    // salary is not a scheduled transaction, it simply arrives, so a person who has never set up an
    // income schedule has a planned income of nothing and every month they plan reads as an
    // overspend of the entire plan. Money already in the account is income the month has whether or
    // not anything foresaw it, so the larger of the two is taken - which also means a bonus lifts
    // the figure rather than being ignored for having been unplanned.
    const incomeBasis = computed<BigDecimal>(() => {
        const planned = plannedTotals.value.income;
        const actual = actualTotals.value.income;

        return actual.greaterThan(planned) ? actual : planned;
    });

    // What the plan still expects to go out. The rule for which level a budget is measured at is
    // in sumRemainingToSpend, and it is not obvious - it is worth reading there.
    const remainingToSpend = computed<BigDecimal>(() => sumRemainingToSpend(categoryBudgetTree.value));

    // projectedExpense is what the month will have cost by the end of it: what has actually gone
    // out, plus what is still to come.
    //
    // This is not the same as what the month was planned to cost, and the difference matters. A
    // month planned at 2,124 that has already spent 3,069 will not cost 2,124 - the money is gone.
    // Measuring against the plan alone would have said there was still over 1,400 to spend.
    const projectedExpense = computed<BigDecimal>(() => actualTotals.value.expense.add(remainingToSpend.value));

    // What is left of the income once the month has cost what it is going to cost. This is the
    // figure the page is really for, and it goes negative when the month runs past its income.
    const monthNet = computed<BigDecimal>(() => incomeBasis.value.subtract(projectedExpense.value));

    function getAccountCurrency(accountId: string): string | undefined {
        return accountsStore.allAccountsMap[accountId]?.currency;
    }

    function convertToDefaultCurrency(amount: BigDecimal, currency: string): BigDecimal | null {
        if (currency === defaultCurrency.value) {
            return amount;
        }

        return exchangeRatesStore.getExchangedAmount(amount, currency, defaultCurrency.value);
    }

    function setMonth(newYear: number, newMonth: number): void {
        year.value = newYear;
        month.value = newMonth;
        planStateInvalid.value = true;
    }

    function loadBudgetPlan({ force }: { force: boolean }): Promise<void> {
        const requestedYear = year.value;
        const requestedMonth = month.value;

        return Promise.all([
            transactionTemplatesStore.loadAllTemplates({ templateType: TemplateType.Schedule.type, force: force }),
            transactionCategoriesStore.loadAllCategories({ force: false }),
            accountsStore.loadAllAccounts({ force: false }),
            services.getBudgetPlan({ year: requestedYear, month: requestedMonth }),
            services.getTransactionStatistics({
                startTime: getYearMonthFirstUnixTime({ year: requestedYear, month0base: requestedMonth - 1 }),
                endTime: getYearMonthLastUnixTime({ year: requestedYear, month0base: requestedMonth - 1 }),
                tagFilter: '',
                keyword: '',
                matchMode: KeywordMatchMode.Default.type,
                subscriptionFilter: 0,
                useTransactionTimezone: false
            })
        ]).then(([, , , planResponse, statisticsResponse]) => {
            // a month stepped through while these were in flight must not be overwritten by the
            // answer to the month that was left
            if (requestedYear !== year.value || requestedMonth !== month.value) {
                return;
            }

            const planData = planResponse.data;
            const statisticsData = statisticsResponse.data;

            if (!planData || !planData.success || !planData.result) {
                throw new Error('Unable to retrieve budget plan');
            }

            planItems.value = BudgetPlanItem.ofMulti(planData.result.items || []);
            planAdjustments.value = planData.result.adjustments || [];
            planExpectations.value = planData.result.expectations || [];
            actualItems.value = statisticsData && statisticsData.success && statisticsData.result ? (statisticsData.result.items || []) : [];
            planStateInvalid.value = false;
        }).catch(error => {
            logger.error('failed to retrieve budget plan', error);

            if (error.response && error.response.data && error.response.data.errorMessage) {
                return Promise.reject({ error: error.response.data });
            } else if (!error.processed) {
                return Promise.reject({ message: 'Unable to retrieve budget plan' });
            }

            return Promise.reject(error);
        });
    }

    function saveBudgetPlanItem({ item }: { item: BudgetPlanItem }): Promise<BudgetPlanItem> {
        const isNew = !item.id;
        const request = isNew ? services.addBudgetPlanItem(item.toCreateRequest()) : services.modifyBudgetPlanItem(item.toModifyRequest());

        return request.then(response => {
            const data = response.data;

            if (!data || !data.success || !data.result) {
                throw new Error(isNew ? 'Unable to add planned item' : 'Unable to save planned item');
            }

            const saved = BudgetPlanItem.of(data.result);

            if (isNew) {
                planItems.value.push(saved);
            } else {
                const index = planItems.value.findIndex(existing => existing.id === saved.id);

                if (index >= 0) {
                    planItems.value.splice(index, 1, saved);
                }
            }

            return saved;
        }).catch(error => {
            logger.error('failed to save budget plan item', error);

            if (error.response && error.response.data && error.response.data.errorMessage) {
                return Promise.reject({ error: error.response.data });
            } else if (!error.processed) {
                return Promise.reject({ message: isNew ? 'Unable to add planned item' : 'Unable to save planned item' });
            }

            return Promise.reject(error);
        });
    }

    function deleteBudgetPlanItem({ item }: { item: BudgetPlanItem }): Promise<void> {
        return services.deleteBudgetPlanItem({ id: item.id }).then(response => {
            const data = response.data;

            if (!data || !data.success || !data.result) {
                throw new Error('Unable to delete planned item');
            }

            planItems.value = planItems.value.filter(existing => existing.id !== item.id);
        }).catch(error => {
            logger.error('failed to delete budget plan item', error);

            if (error.response && error.response.data && error.response.data.errorMessage) {
                return Promise.reject({ error: error.response.data });
            } else if (!error.processed) {
                return Promise.reject({ message: 'Unable to delete planned item' });
            }

            return Promise.reject(error);
        });
    }

    function copyPreviousMonthItems(): Promise<number> {
        const previous = month.value > 1 ? { year: year.value, month: month.value - 1 } : { year: year.value - 1, month: 12 };

        return services.copyBudgetPlanItems({
            fromYear: previous.year,
            fromMonth: previous.month,
            toYear: year.value,
            toMonth: month.value
        }).then(response => {
            const data = response.data;

            if (!data || !data.success || !data.result) {
                throw new Error('Unable to copy the previous month');
            }

            return loadBudgetPlan({ force: false }).then(() => data.result);
        }).catch(error => {
            logger.error('failed to copy budget plan items', error);

            if (error.response && error.response.data && error.response.data.errorMessage) {
                return Promise.reject({ error: error.response.data });
            } else if (!error.processed) {
                return Promise.reject({ message: 'Unable to copy the previous month' });
            }

            return Promise.reject(error);
        });
    }

    // Setting an expectation is a write of one row and nothing else. Everything that follows from
    // it - the category's own figure, its parent's, the month's total, the bar at the top - is
    // derived, so the page moves the moment this resolves without anything being reloaded.
    function setCategoryExpectation({ categoryId, amount }: { categoryId: string, amount: number }): Promise<void> {
        return services.setBudgetPlanExpectation({
            year: year.value,
            month: month.value,
            categoryId: categoryId,
            amount: amount
        }).then(response => {
            const data = response.data;

            if (!data || !data.success) {
                throw new Error('Unable to set this expectation');
            }

            planExpectations.value = planExpectations.value.filter(expectation => expectation.categoryId !== categoryId);

            if (data.result) {
                planExpectations.value.push(data.result);
            }
        }).catch(error => {
            logger.error('failed to set budget plan expectation', error);

            if (error.response && error.response.data && error.response.data.errorMessage) {
                return Promise.reject({ error: error.response.data });
            } else if (!error.processed) {
                return Promise.reject({ message: 'Unable to set this expectation' });
            }

            return Promise.reject(error);
        });
    }

    function setScheduleAdjustment({ templateId, excluded, amount }: { templateId: string, excluded: boolean, amount?: number }): Promise<void> {
        return services.setBudgetPlanAdjustment({
            year: year.value,
            month: month.value,
            templateId: templateId,
            excluded: excluded,
            amount: amount
        }).then(response => {
            const data = response.data;

            if (!data || !data.success) {
                throw new Error('Unable to adjust this schedule');
            }

            planAdjustments.value = planAdjustments.value.filter(adjustment => adjustment.templateId !== templateId);

            if (data.result) {
                planAdjustments.value.push(data.result);
            }
        }).catch(error => {
            logger.error('failed to set budget plan adjustment', error);

            if (error.response && error.response.data && error.response.data.errorMessage) {
                return Promise.reject({ error: error.response.data });
            } else if (!error.processed) {
                return Promise.reject({ message: 'Unable to adjust this schedule' });
            }

            return Promise.reject(error);
        });
    }

    return {
        // states
        year,
        month,
        planItems,
        planAdjustments,
        planExpectations,
        planStateInvalid,
        // computed states
        defaultCurrency,
        scheduleLines,
        itemLines,
        allLines,
        incomeLines,
        expenseLines,
        plannedTotals,
        plannedLineTotals,
        committedExpense,
        actualTotals,
        incomeBasis,
        remainingToSpend,
        projectedExpense,
        monthNet,
        primaryCategories,
        categoryBudgetTree,
        // functions
        convertToDefaultCurrency,
        setMonth,
        loadBudgetPlan,
        saveBudgetPlanItem,
        deleteBudgetPlanItem,
        copyPreviousMonthItems,
        setScheduleAdjustment,
        setCategoryExpectation
    };
});

export { PlannedLineSource };
export type { PlannedLine, PlanTotals, CategoryBudgetNode };
