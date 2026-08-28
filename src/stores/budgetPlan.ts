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
import { type BudgetPlanAdjustment, BudgetPlanItem } from '@/models/budget_plan.ts';
import type { TransactionStatisticResponseItem } from '@/models/transaction.ts';

import {
    type PlannedLine,
    type PlanTotals,
    PlannedLineSource,
    buildScheduleLines,
    buildItemLines,
    sumPlannedLines,
    sumPlannedLinesByCategory
} from '@/lib/budgetPlan.ts';
import { BIG_DECIMAL_ZERO, parseBigDecimal } from '@/lib/numeral.ts';
import { getYearMonthFirstUnixTime, getYearMonthLastUnixTime, getCurrentDateTime } from '@/lib/datetime.ts';
import services from '@/lib/services.ts';
import logger from '@/lib/logger.ts';

// CategoryComparison is one row of the plan set against the ledger: what this category was planned
// to cost this month, and what it has actually cost so far.
export interface CategoryComparison {
    readonly categoryId: string;
    readonly type: number;
    readonly planned: BigDecimal;
    readonly actual: BigDecimal;
    // remaining is what is left of the plan. It goes negative when a category is overspent, which is
    // the number the page exists to show, so it is not clamped at zero.
    readonly remaining: BigDecimal;
}

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

    const plannedTotals = computed<PlanTotals>(() => sumPlannedLines(allLines.value, convertToDefaultCurrency));

    // The committed part is what recurs whether or not anything is decided this month. What is left
    // of the income after it is the figure a month is actually planned within.
    const committedExpense = computed<BigDecimal>(() => sumPlannedLines(scheduleLines.value, convertToDefaultCurrency).expense);

    const plannedByCategory = computed<Record<string, BigDecimal>>(() => sumPlannedLinesByCategory(allLines.value, convertToDefaultCurrency));

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

    // Every category named by either side appears, so a category that was planned for and not spent
    // is as visible as one that was spent without being planned for.
    const categoryComparisons = computed<CategoryComparison[]>(() => {
        const categoryIds = new Set<string>();

        for (const categoryId in plannedByCategory.value) {
            categoryIds.add(categoryId);
        }

        for (const categoryId in actualByCategory.value) {
            categoryIds.add(categoryId);
        }

        const comparisons: CategoryComparison[] = [];

        for (const categoryId of categoryIds) {
            const category = transactionCategoriesStore.allTransactionCategoriesMap[categoryId];
            const planned = plannedByCategory.value[categoryId] ?? BIG_DECIMAL_ZERO;
            const actual = actualByCategory.value[categoryId] ?? BIG_DECIMAL_ZERO;

            comparisons.push({
                categoryId: categoryId,
                type: category ? (category.type === CategoryType.Income ? TransactionType.Income : TransactionType.Expense) : TransactionType.Expense,
                planned: planned,
                actual: actual,
                remaining: planned.subtract(actual)
            });
        }

        return comparisons.sort((comparison1, comparison2) => comparison2.planned.compareTo(comparison1.planned));
    });

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
        planStateInvalid,
        // computed states
        defaultCurrency,
        scheduleLines,
        itemLines,
        allLines,
        incomeLines,
        expenseLines,
        plannedTotals,
        committedExpense,
        actualTotals,
        categoryComparisons,
        // functions
        setMonth,
        loadBudgetPlan,
        saveBudgetPlanItem,
        deleteBudgetPlanItem,
        copyPreviousMonthItems,
        setScheduleAdjustment
    };
});

export { PlannedLineSource };
export type { PlannedLine, PlanTotals };
