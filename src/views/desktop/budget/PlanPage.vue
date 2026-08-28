<template>
    <v-row class="match-height">
        <v-col cols="12">
            <v-card>
                <template #title>
                    <div class="title-and-toolbar d-flex align-center">
                        <span>{{ tt('Budget Plan') }}</span>
                        <v-btn class="ms-3" density="comfortable" color="default" variant="text"
                               :disabled="loading" :icon="mdiChevronLeft" @click="stepMonth(-1)">
                            <v-icon :icon="mdiChevronLeft"/>
                            <v-tooltip activator="parent">{{ tt('Previous Month') }}</v-tooltip>
                        </v-btn>
                        <span class="budget-plan-month text-body-1 mx-1">{{ displayMonth }}</span>
                        <v-btn density="comfortable" color="default" variant="text"
                               :disabled="loading" :icon="mdiChevronRight" @click="stepMonth(1)">
                            <v-icon :icon="mdiChevronRight"/>
                            <v-tooltip activator="parent">{{ tt('Next Month') }}</v-tooltip>
                        </v-btn>
                        <v-btn class="ms-2" density="compact" color="default" variant="text" size="24"
                               :icon="true" :disabled="loading" :loading="loading" @click="reload">
                            <template #loader>
                                <v-progress-circular indeterminate size="20"/>
                            </template>
                            <v-icon :icon="mdiRefresh" size="24"/>
                            <v-tooltip activator="parent">{{ tt('Refresh') }}</v-tooltip>
                        </v-btn>
                        <v-spacer/>
                        <v-btn class="ms-3" color="default" variant="outlined"
                               :disabled="loading" @click="addItem">{{ tt('Plan Something') }}</v-btn>
                        <v-btn density="comfortable" color="default" variant="text" class="ms-2"
                               :disabled="loading" :icon="true">
                            <v-icon :icon="mdiDotsVertical"/>
                            <v-menu activator="parent">
                                <v-list>
                                    <v-list-item :disabled="loading"
                                                 :prepend-icon="mdiContentCopy"
                                                 :title="tt('Copy Previous Month')"
                                                 @click="copyPreviousMonth"></v-list-item>
                                    <v-list-item :disabled="loading"
                                                 :prepend-icon="mdiCalendarTodayOutline"
                                                 :title="tt('Go to This Month')"
                                                 @click="goToThisMonth"></v-list-item>
                                </v-list>
                            </v-menu>
                        </v-btn>
                    </div>
                </template>

                <v-card-text class="budget-plan-summary">
                    <v-row>
                        <v-col cols="12" md="3">
                            <div class="text-caption text-medium-emphasis">{{ tt('Planned Income') }}</div>
                            <div class="text-h6 text-income">{{ displayAmount(plannedTotals.income) }}</div>
                            <div class="text-caption text-medium-emphasis">{{ tt('So far') }} {{ displayAmount(actualTotals.income) }}</div>
                        </v-col>
                        <v-col cols="12" md="3">
                            <div class="text-caption text-medium-emphasis">{{ tt('Planned Expense') }}</div>
                            <div class="text-h6 text-expense">{{ displayAmount(plannedTotals.expense) }}</div>
                            <div class="text-caption text-medium-emphasis">{{ tt('So far') }} {{ displayAmount(actualTotals.expense) }}</div>
                        </v-col>
                        <v-col cols="12" md="3">
                            <div class="text-caption text-medium-emphasis">{{ tt('Committed') }}</div>
                            <div class="text-h6">{{ displayAmount(committedExpense) }}</div>
                            <div class="text-caption text-medium-emphasis">{{ tt('What the schedules cost whatever else is decided') }}</div>
                        </v-col>
                        <v-col cols="12" md="3">
                            <div class="text-caption text-medium-emphasis">{{ tt('Left Over') }}</div>
                            <div class="text-h6" :class="plannedTotals.net.isNegative() ? 'text-expense' : 'text-income'">{{ displayAmount(plannedTotals.net) }}</div>
                            <div class="text-caption text-medium-emphasis">{{ tt('Planned income less everything planned to leave') }}</div>
                        </v-col>
                    </v-row>
                </v-card-text>
            </v-card>
        </v-col>

        <v-col cols="12">
            <v-card :title="tt('The Plan')">
                <v-table class="budget-plan-table table-striped" :hover="!loading">
                    <thead>
                    <tr>
                        <th>{{ tt('Name') }}</th>
                        <th>{{ tt('Category') }}</th>
                        <th>{{ tt('Account') }}</th>
                        <th class="text-end">{{ tt('Times') }}</th>
                        <th class="text-end">{{ tt('Amount') }}</th>
                        <th class="text-end budget-plan-operation-column">{{ tt('Operation') }}</th>
                    </tr>
                    </thead>

                    <tbody v-if="loading">
                    <tr :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]">
                        <td class="px-0" colspan="6">
                            <v-skeleton-loader type="text" :loading="true"></v-skeleton-loader>
                        </td>
                    </tr>
                    </tbody>

                    <tbody v-else-if="!allLines.length">
                    <tr>
                        <td colspan="6">{{ tt('Nothing is planned for this month yet. Scheduled transactions appear here on their own; anything else is added with Plan Something.') }}</td>
                    </tr>
                    </tbody>

                    <tbody v-else>
                    <template :key="group.title" v-for="group in lineGroups">
                        <tr class="budget-plan-group-row" v-if="group.lines.length">
                            <td colspan="6" class="text-uppercase text-caption text-medium-emphasis">{{ group.title }}</td>
                        </tr>
                        <tr class="budget-plan-line-row" :key="group.title + '-' + line.source + '-' + line.id"
                            v-for="line in group.lines"
                            @mouseenter="hoveredLineKey = group.title + '-' + line.source + '-' + line.id"
                            @mouseleave="hoveredLineKey = ''">
                            <td>
                                <div class="d-flex align-center">
                                    <v-icon size="20" start :icon="line.source === PlannedLineSource.Schedule ? mdiClockTimeNineOutline : mdiPencilOutline"/>
                                    <span :class="{ 'text-decoration-line-through text-medium-emphasis': line.excluded }">{{ line.name }}</span>
                                </div>
                            </td>
                            <td class="text-truncate">{{ getCategoryName(line.categoryId) }}</td>
                            <td class="text-truncate">{{ getAccountName(line.accountId) }}</td>
                            <td class="text-end text-no-wrap">{{ displayOccurrences(line.occurrences) }}</td>
                            <td class="text-end text-no-wrap" :class="{ 'text-medium-emphasis': line.excluded }">
                                <span :class="{ 'text-decoration-line-through': line.excluded }">{{ displayLineAmount(line) }}</span>
                                <v-icon class="ms-1" size="14" :icon="mdiPencilOutline" v-if="line.adjusted">
                                    <v-tooltip activator="parent">{{ tt('Adjusted for this month') }}</v-tooltip>
                                </v-icon>
                            </td>
                            <td class="text-end budget-plan-operation-column">
                                <div class="budget-plan-operation-buttons"
                                     :class="{ 'budget-plan-operation-buttons-shown': hoveredLineKey === group.title + '-' + line.source + '-' + line.id }">
                                    <template v-if="line.source === PlannedLineSource.Schedule">
                                        <v-btn class="px-2" color="default" density="comfortable" variant="text"
                                               :prepend-icon="mdiTuneVariant" :disabled="loading"
                                               @click="adjustSchedule(line)">{{ tt('Adjust') }}</v-btn>
                                        <v-btn class="px-2" color="default" density="comfortable" variant="text"
                                               :prepend-icon="line.excluded ? mdiRestore : mdiCancel" :disabled="loading"
                                               @click="toggleExcluded(line)">{{ line.excluded ? tt('Restore') : tt('Skip') }}</v-btn>
                                    </template>
                                    <template v-else>
                                        <v-btn class="px-2" color="default" density="comfortable" variant="text"
                                               :prepend-icon="mdiPencilOutline" :disabled="loading"
                                               @click="editItem(line)">{{ tt('Edit') }}</v-btn>
                                        <v-btn class="px-2" color="default" density="comfortable" variant="text"
                                               :prepend-icon="mdiDeleteOutline" :disabled="loading"
                                               @click="removeItem(line)">{{ tt('Delete') }}</v-btn>
                                    </template>
                                </div>
                            </td>
                        </tr>
                    </template>
                    </tbody>
                </v-table>
            </v-card>
        </v-col>

        <v-col cols="12">
            <v-card :title="tt('Planned Against Actual')">
                <v-card-text class="pt-0">
                    <span class="text-body-2 text-medium-emphasis">{{ tt('What each category was planned to cost this month, and what it has cost so far.') }}</span>
                </v-card-text>
                <v-table class="budget-plan-table table-striped" :hover="!loading">
                    <thead>
                    <tr>
                        <th>{{ tt('Category') }}</th>
                        <th class="text-end">{{ tt('Planned') }}</th>
                        <th class="text-end">{{ tt('Actual') }}</th>
                        <th class="text-end">{{ tt('Left') }}</th>
                    </tr>
                    </thead>
                    <tbody v-if="!loading && !categoryComparisons.length">
                    <tr>
                        <td colspan="4">{{ tt('Nothing planned and nothing spent in this month.') }}</td>
                    </tr>
                    </tbody>
                    <tbody v-else-if="!loading">
                    <tr :key="comparison.categoryId" v-for="comparison in categoryComparisons">
                        <td class="text-truncate">{{ getCategoryName(comparison.categoryId) }}</td>
                        <td class="text-end text-no-wrap">{{ displayAmount(comparison.planned) }}</td>
                        <td class="text-end text-no-wrap">{{ displayAmount(comparison.actual) }}</td>
                        <td class="text-end text-no-wrap"
                            :class="comparison.remaining.isNegative() ? 'text-expense' : ''">{{ displayAmount(comparison.remaining) }}</td>
                    </tr>
                    </tbody>
                </v-table>
            </v-card>
        </v-col>
    </v-row>

    <edit-plan-item-dialog ref="editPlanItemDialog" />
    <adjust-schedule-dialog ref="adjustScheduleDialog" />
    <confirm-dialog ref="confirmDialog"/>
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';
import EditPlanItemDialog from './dialogs/EditPlanItemDialog.vue';
import AdjustScheduleDialog from './dialogs/AdjustScheduleDialog.vue';

import { ref, computed, useTemplateRef, onMounted } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useAccountsStore } from '@/stores/account.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { useBudgetPlanStore } from '@/stores/budgetPlan.ts';

import type { BigDecimal } from '@/core/numeral.ts';
import { TransactionType } from '@/core/transaction.ts';
import { BudgetPlanItem } from '@/models/budget_plan.ts';
import { type PlannedLine, PlannedLineSource } from '@/lib/budgetPlan.ts';
import { parseDateTimeFromUnixTime, getYearMonthFirstUnixTime } from '@/lib/datetime.ts';

import {
    mdiRefresh,
    mdiChevronLeft,
    mdiChevronRight,
    mdiDotsVertical,
    mdiContentCopy,
    mdiCalendarTodayOutline,
    mdiClockTimeNineOutline,
    mdiPencilOutline,
    mdiDeleteOutline,
    mdiTuneVariant,
    mdiCancel,
    mdiRestore
} from '@mdi/js';

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;
type EditPlanItemDialogType = InstanceType<typeof EditPlanItemDialog>;
type AdjustScheduleDialogType = InstanceType<typeof AdjustScheduleDialog>;

interface PlannedLineGroup {
    title: string;
    lines: PlannedLine[];
}

const { tt, formatAmountToLocalizedNumeralsWithCurrency, formatNumberToLocalizedNumerals, formatDateTimeToGregorianLikeLongYearMonth } = useI18n();

const accountsStore = useAccountsStore();
const transactionCategoriesStore = useTransactionCategoriesStore();
const budgetPlanStore = useBudgetPlanStore();

const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');
const editPlanItemDialog = useTemplateRef<EditPlanItemDialogType>('editPlanItemDialog');
const adjustScheduleDialog = useTemplateRef<AdjustScheduleDialogType>('adjustScheduleDialog');

const loading = ref<boolean>(true);
const hoveredLineKey = ref<string>('');

const allLines = computed<PlannedLine[]>(() => budgetPlanStore.allLines);
const plannedTotals = computed(() => budgetPlanStore.plannedTotals);
const actualTotals = computed(() => budgetPlanStore.actualTotals);
const committedExpense = computed<BigDecimal>(() => budgetPlanStore.committedExpense);
const categoryComparisons = computed(() => budgetPlanStore.categoryComparisons);
const defaultCurrency = computed<string>(() => budgetPlanStore.defaultCurrency);

const displayMonth = computed<string>(() => formatDateTimeToGregorianLikeLongYearMonth(parseDateTimeFromUnixTime(getYearMonthFirstUnixTime({ year: budgetPlanStore.year, month0base: budgetPlanStore.month - 1 }))));

// Income first, then what recurs, then what was decided for this month in particular. It is the
// order the month is actually reasoned about: what is coming in, what is already spoken for, and
// what is left to decide.
const lineGroups = computed<PlannedLineGroup[]>(() => [
    { title: tt('Income'), lines: allLines.value.filter(line => line.type === TransactionType.Income) },
    { title: tt('Scheduled'), lines: allLines.value.filter(line => line.type !== TransactionType.Income && line.source === PlannedLineSource.Schedule) },
    { title: tt('Planned'), lines: allLines.value.filter(line => line.type !== TransactionType.Income && line.source === PlannedLineSource.Item) }
]);

function displayAmount(amount: BigDecimal): string {
    return formatAmountToLocalizedNumeralsWithCurrency(amount.truncate(), defaultCurrency.value);
}

function displayLineAmount(line: PlannedLine): string {
    return formatAmountToLocalizedNumeralsWithCurrency(line.amount.truncate(), line.currency);
}

// A whole number of occurrences is shown as it is. A fraction only ever comes from a recurrence with
// no day to land on, and is shown to two places rather than rounded away, so that a twelfth of a
// yearly bill does not read as if it were nothing.
function displayOccurrences(occurrences: number): string {
    if (Number.isInteger(occurrences)) {
        return formatNumberToLocalizedNumerals(occurrences);
    }

    return formatNumberToLocalizedNumerals(Math.round(occurrences * 100) / 100);
}

function getCategoryName(categoryId: string): string {
    return transactionCategoriesStore.allTransactionCategoriesMap[categoryId]?.name ?? '';
}

function getAccountName(accountId: string): string {
    return accountsStore.allAccountsMap[accountId]?.name ?? '';
}

function load(force: boolean): void {
    loading.value = true;

    budgetPlanStore.loadBudgetPlan({ force: force }).then(() => {
        loading.value = false;
    }).catch(error => {
        loading.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function reload(): void {
    load(true);
}

function stepMonth(offset: number): void {
    let year = budgetPlanStore.year;
    let month = budgetPlanStore.month + offset;

    while (month < 1) {
        month += 12;
        year -= 1;
    }

    while (month > 12) {
        month -= 12;
        year += 1;
    }

    budgetPlanStore.setMonth(year, month);
    load(false);
}

function goToThisMonth(): void {
    const now = new Date();
    budgetPlanStore.setMonth(now.getFullYear(), now.getMonth() + 1);
    load(false);
}

function addItem(): void {
    editPlanItemDialog.value?.open(BudgetPlanItem.createNew(budgetPlanStore.year, budgetPlanStore.month, TransactionType.Expense)).then(() => {
        snackbar.value?.showMessage('You have planned a new item');
    }).catch(() => {
        // dismissed
    });
}

function editItem(line: PlannedLine): void {
    const item = budgetPlanStore.planItems.find(planItem => planItem.id === line.id);

    if (!item) {
        return;
    }

    editPlanItemDialog.value?.open(item.clone()).then(() => {
        snackbar.value?.showMessage('You have saved this planned item');
    }).catch(() => {
        // dismissed
    });
}

function removeItem(line: PlannedLine): void {
    const item = budgetPlanStore.planItems.find(planItem => planItem.id === line.id);

    if (!item) {
        return;
    }

    confirmDialog.value?.open('Are you sure you want to remove this planned item?').then(() => {
        budgetPlanStore.deleteBudgetPlanItem({ item: item }).then(() => {
            snackbar.value?.showMessage('You have removed this planned item');
        }).catch(error => {
            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    }).catch(() => {
        // dismissed
    });
}

function adjustSchedule(line: PlannedLine): void {
    adjustScheduleDialog.value?.open(line).then(() => {
        snackbar.value?.showMessage('You have adjusted this schedule for this month');
    }).catch(() => {
        // dismissed
    });
}

// Skipping is one click rather than a dialog, because a subscription paused for a month is the
// commonest thing a plan has to say and the amount is not in question when it is said.
function toggleExcluded(line: PlannedLine): void {
    budgetPlanStore.setScheduleAdjustment({
        templateId: line.id,
        excluded: !line.excluded,
        amount: line.adjusted ? line.unitAmount : undefined
    }).catch(error => {
        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function copyPreviousMonth(): void {
    budgetPlanStore.copyPreviousMonthItems().then(() => {
        snackbar.value?.showMessage('The previous month has been copied into this one');
    }).catch(error => {
        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

onMounted(() => {
    load(false);
});
</script>

<style>
.budget-plan-table .budget-plan-operation-column {
    width: 1%;
    white-space: nowrap;
}

.budget-plan-table .budget-plan-operation-buttons {
    visibility: hidden;
    display: flex;
    align-items: center;
    justify-content: flex-end;
}

.budget-plan-table .budget-plan-operation-buttons-shown {
    visibility: visible;
}

.budget-plan-table .budget-plan-group-row > td {
    padding-top: 12px;
    letter-spacing: 0.5px;
}

.budget-plan-month {
    min-width: 130px;
    text-align: center;
}
</style>
