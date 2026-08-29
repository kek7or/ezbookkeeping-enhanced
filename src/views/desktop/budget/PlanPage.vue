<template>
    <v-row class="match-height">
        <v-col cols="12">
            <v-card>
                <template #title>
                    <div class="title-and-toolbar d-flex align-center">
                        <span>{{ tt('Budget Plan') }}</span>
                        <v-btn class="ms-3" density="comfortable" color="default" variant="text"
                               :disabled="loading" :icon="true" @click="stepMonth(-1)">
                            <v-icon :icon="mdiChevronLeft"/>
                            <v-tooltip activator="parent">{{ tt('Previous Month') }}</v-tooltip>
                        </v-btn>
                        <span class="budget-plan-month text-body-1">{{ displayMonth }}</span>
                        <v-btn density="comfortable" color="default" variant="text"
                               :disabled="loading" :icon="true" @click="stepMonth(1)">
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
                        <v-btn class="ms-3" color="primary" variant="tonal" :prepend-icon="mdiPlus"
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

                <v-card-text>
                    <v-row>
                        <v-col cols="12" md="4">
                            <div class="text-caption text-medium-emphasis">{{ overspent ? tt('Short By') : tt('Left of What You Earned') }}</div>
                            <div class="budget-plan-hero" :class="overspent ? 'text-expense' : 'text-income'">
                                <v-skeleton-loader type="heading" :loading="true" v-if="loading"/>
                                <span v-else>{{ displayAmount(overspent ? monthNet.negate() : monthNet) }}</span>
                            </div>
                            <div class="text-body-2 text-medium-emphasis">
                                {{ tt('format.misc.budgetOfComingIn', { amount: displayAmount(incomeBasis) }) }}
                            </div>
                        </v-col>
                        <v-col cols="12" md="8">
                            <!-- One bar, the three parts of the money: what has gone out, what
                                 is still to go out, and what survives the month. They do not
                                 overlap, so they add up to what is coming in. -->
                            <div class="budget-flow-bar-wrapper" v-if="!loading && flowSegments.length">
                                <div class="budget-flow-bar">
                                    <div class="budget-flow-segment"
                                         :key="segment.key"
                                         :style="{ flexGrow: segment.share, background: segment.color }"
                                         v-for="segment in flowSegments">
                                        <span class="budget-flow-inline-label" :style="{ color: segment.labelColor }"
                                              v-if="segment.showInlineLabel">{{ segment.label }}</span>
                                        <v-tooltip activator="parent" location="top">
                                            {{ segment.label }} — {{ segment.displayAmount }} ({{ segment.displayShare }})
                                        </v-tooltip>
                                    </div>
                                </div>
                                <!-- where the income runs out. It is only drawn when the month
                                     plans past it, because otherwise it sits on the bar's own end
                                     and marks nothing. -->
                                <div class="budget-flow-income-marker" :style="{ left: incomeMarkerLeft }"
                                     v-if="incomeMarkerLeft">
                                    <v-tooltip activator="parent" location="top">
                                        {{ tt('Coming In') }} — {{ displayAmount(incomeBasis) }}
                                    </v-tooltip>
                                </div>
                            </div>
                            <v-skeleton-loader type="text" :loading="true" v-else-if="loading"/>

                            <div class="budget-flow-legend" v-if="!loading">
                                <div class="budget-flow-legend-item" :key="segment.key" v-for="segment in flowSegments">
                                    <span class="budget-flow-swatch" :style="{ background: segment.color }"></span>
                                    <span class="text-body-2">{{ segment.label }}</span>
                                    <span class="text-body-2 text-medium-emphasis ms-2">{{ segment.displayAmount }}</span>
                                </div>
                            </div>

                            <!-- The three figures the hero is worked out from, in the order the
                                 month is read: what it will cost, what it has cost so far, and what
                                 has come in. -->
                            <div class="budget-plan-figures mt-4" v-if="!loading">
                                <div class="budget-plan-figure">
                                    <div class="text-caption text-medium-emphasis">{{ tt('Planned to Spend') }}</div>
                                    <div class="text-h6 font-weight-regular">{{ displayAmount(plannedTotals.expense) }}</div>
                                </div>
                                <div class="budget-plan-figure">
                                    <div class="text-caption text-medium-emphasis">{{ tt('Spent so far') }}</div>
                                    <div class="text-h6 font-weight-regular">{{ displayAmount(actualTotals.expense) }}</div>
                                </div>
                                <div class="budget-plan-figure">
                                    <div class="text-caption text-medium-emphasis">{{ tt('Still to Come') }}</div>
                                    <div class="text-h6 font-weight-regular">{{ displayAmount(remainingToSpend) }}</div>
                                </div>
                                <div class="budget-plan-figure">
                                    <div class="text-caption text-medium-emphasis">{{ tt('Earned so far') }}</div>
                                    <div class="text-h6 font-weight-regular">{{ displayAmount(actualTotals.income) }}</div>
                                </div>
                            </div>
                        </v-col>
                    </v-row>
                </v-card-text>
            </v-card>
        </v-col>

        <v-col cols="12">
            <v-card :title="tt('The Plan')">
                <v-card-text class="pt-0">
                    <span class="text-body-2 text-medium-emphasis">{{ tt('Click any amount to change it. Everything above recalculates as you type.') }}</span>
                </v-card-text>

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
                        <td colspan="6" class="py-6 text-center">
                            <v-icon class="mb-2" size="32" :icon="mdiCalendarBlankOutline" color="secondary"/>
                            <div class="text-body-1">{{ tt('Nothing is planned for this month yet') }}</div>
                            <div class="text-body-2 text-medium-emphasis mb-3">{{ tt('Scheduled transactions appear here on their own; anything else is added with Plan Something.') }}</div>
                            <v-btn color="primary" variant="tonal" :prepend-icon="mdiPlus" @click="addItem">{{ tt('Plan Something') }}</v-btn>
                        </td>
                    </tr>
                    </tbody>

                    <tbody v-else>
                    <template :key="group.key" v-for="group in lineGroups">
                        <tr class="budget-plan-group-row" v-if="group.lines.length" @click="toggleGroup(group.key)">
                            <td colspan="4">
                                <div class="d-flex align-center">
                                    <v-icon size="18" :icon="collapsedGroups[group.key] ? mdiChevronRight : mdiChevronDown"/>
                                    <span class="text-uppercase text-caption font-weight-medium ms-1">{{ group.title }}</span>
                                    <span class="text-caption text-medium-emphasis ms-2">{{ formatNumberToLocalizedNumerals(group.lines.length) }}</span>
                                </div>
                            </td>
                            <td class="text-end text-no-wrap text-caption font-weight-medium">{{ displayAmount(group.total) }}</td>
                            <td class="budget-plan-operation-column"></td>
                        </tr>

                        <tr class="budget-plan-line-row"
                            :key="lineKey(group.key, line)"
                            v-for="line in (collapsedGroups[group.key] ? [] : group.lines)"
                            @mouseenter="hoveredLineKey = lineKey(group.key, line)"
                            @mouseleave="hoveredLineKey = ''">
                            <td>
                                <div class="d-flex align-center">
                                    <v-icon size="20" start :icon="line.source === PlannedLineSource.Schedule ? mdiClockTimeNineOutline : mdiPencilOutline"/>
                                    <span :class="{ 'text-decoration-line-through text-medium-emphasis': line.excluded }">{{ line.name }}</span>
                                    <v-chip class="ms-2" size="x-small" color="secondary" variant="tonal" v-if="line.excluded">{{ tt('Skipped') }}</v-chip>
                                    <v-chip class="ms-2" size="x-small" color="primary" variant="tonal" v-else-if="line.adjusted">{{ tt('Adjusted') }}</v-chip>
                                </div>
                            </td>
                            <td class="text-truncate">{{ getCategoryName(line.categoryId) }}</td>
                            <td class="text-truncate">{{ getAccountName(line.accountId) }}</td>
                            <td class="text-end text-no-wrap text-medium-emphasis">{{ displayOccurrences(line.occurrences) }}</td>
                            <td class="text-end budget-plan-amount-cell">
                                <amount-input class="budget-plan-amount-input"
                                              density="compact" variant="outlined"
                                              :autofocus="true"
                                              :currency="line.currency"
                                              :show-currency="true"
                                              :disabled="savingLineKey === lineKey(group.key, line)"
                                              v-model="editingAmount"
                                              @enter="commitAmount(line)"
                                              v-if="editingLineKey === lineKey(group.key, line)"/>
                                <button class="budget-plan-amount-button text-no-wrap"
                                        :class="{ 'text-medium-emphasis': line.excluded }"
                                        :disabled="line.excluded"
                                        @click="startEditingAmount(group.key, line)"
                                        v-else>
                                    <span :class="{ 'text-decoration-line-through': line.excluded }">{{ displayLineAmount(line) }}</span>
                                    <v-icon class="budget-plan-amount-pencil ms-1" size="13" :icon="mdiPencilOutline"/>
                                </button>
                            </td>
                            <td class="text-end budget-plan-operation-column">
                                <div class="d-flex align-center justify-end">
                                    <div class="budget-plan-operation-buttons"
                                         :class="{ 'budget-plan-operation-buttons-shown': hoveredLineKey === lineKey(group.key, line) || editingLineKey === lineKey(group.key, line) }">
                                        <template v-if="editingLineKey === lineKey(group.key, line)">
                                            <v-btn class="px-2" color="primary" density="comfortable" variant="text"
                                                   :prepend-icon="mdiCheck" :loading="savingLineKey === lineKey(group.key, line)"
                                                   @click="commitAmount(line)">{{ tt('Save') }}</v-btn>
                                            <v-btn class="px-2" color="default" density="comfortable" variant="text"
                                                   :prepend-icon="mdiClose" @click="cancelEditingAmount">{{ tt('Cancel') }}</v-btn>
                                        </template>
                                        <template v-else-if="line.source === PlannedLineSource.Schedule">
                                            <v-btn class="px-2" color="default" density="comfortable" variant="text"
                                                   :prepend-icon="mdiTuneVariant" :disabled="loading"
                                                   @click="adjustSchedule(line)">{{ tt('Adjust') }}</v-btn>
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

                                    <!-- Skipping is the commonest thing a plan has to say about a
                                         schedule, so it is one always-visible click rather than
                                         something the row has to be hovered to reveal. -->
                                    <v-btn class="ms-1" density="comfortable" color="default" variant="text"
                                           :icon="true" :disabled="loading"
                                           @click="toggleExcluded(line)"
                                           v-if="line.source === PlannedLineSource.Schedule">
                                        <v-icon size="20" :icon="line.excluded ? mdiRestore : mdiCancel"/>
                                        <v-tooltip activator="parent">{{ line.excluded ? tt('Restore') : tt('Skip this month') }}</v-tooltip>
                                    </v-btn>
                                </div>
                            </td>
                        </tr>
                    </template>
                    </tbody>
                </v-table>
            </v-card>
        </v-col>

        <v-col cols="12">
            <v-card>
                <template #title>
                    <div class="title-and-toolbar d-flex align-center">
                        <span>{{ tt('Category Expectations') }}</span>
                        <v-spacer/>
                        <v-switch class="budget-category-switch" density="compact" color="primary"
                                  :hide-details="true" :disabled="loading"
                                  :label="tt('Show every category')"
                                  v-model="showAllCategories"/>
                    </div>
                </template>
                <v-card-text class="pt-0">
                    <span class="text-body-2 text-medium-emphasis">{{ tt('Say what a category should come to without listing what it is made of. A figure on a primary category is the ceiling for the whole branch; a figure on one beneath it divides that branch up.') }}</span>
                </v-card-text>
                <v-table class="budget-plan-table budget-category-table table-striped">
                    <thead>
                    <tr>
                        <th>{{ tt('Category') }}</th>
                        <th class="budget-meter-column">{{ tt('Progress') }}</th>
                        <th class="text-end">{{ tt('Expected') }}</th>
                        <th class="text-end">{{ tt('Planned') }}</th>
                        <th class="text-end">{{ tt('Actual') }}</th>
                        <th class="text-end">{{ tt('Left') }}</th>
                        <th class="text-end budget-plan-operation-column"></th>
                    </tr>
                    </thead>
                    <tbody v-if="loading">
                    <tr :key="itemIdx" v-for="itemIdx in [ 1, 2 ]">
                        <td class="px-0" colspan="7">
                            <v-skeleton-loader type="text" :loading="true"></v-skeleton-loader>
                        </td>
                    </tr>
                    </tbody>
                    <tbody v-else-if="!categorySections.length">
                    <tr>
                        <td colspan="7" class="py-6 text-center">
                            <div class="text-body-1">{{ tt('Nothing planned and nothing spent in this month.') }}</div>
                            <div class="text-body-2 text-medium-emphasis">{{ tt('Turn on Show every category to set a figure against one anyway.') }}</div>
                        </td>
                    </tr>
                    </tbody>
                    <template v-else>
                    <tbody :key="section.key" v-for="section in categorySections">
                    <!-- What is earned and what is spent are both worth a figure, but they are not
                         read the same way, so they are never mixed into one run of rows. -->
                    <tr class="budget-category-section-row" v-if="categorySections.length > 1">
                        <td colspan="7">
                            <span class="text-uppercase text-caption font-weight-medium">{{ section.title }}</span>
                        </td>
                    </tr>
                    <tr :key="row.categoryId"
                        :class="row.isPrimary ? 'budget-category-primary-row' : ''"
                        v-for="row in section.rows"
                        @mouseenter="hoveredCategoryId = row.categoryId"
                        @mouseleave="hoveredCategoryId = ''">
                        <td>
                            <div class="d-flex align-center">
                                <!-- the twisty keeps its space on a row that has none, so that the
                                     names of the primaries all start at the same place -->
                                <v-btn class="budget-category-twisty" density="compact" variant="text"
                                       color="default" size="24" :icon="true"
                                       :aria-label="getCategoryName(row.categoryId)"
                                       :aria-expanded="!collapsedCategories[row.categoryId]"
                                       @click="toggleCategory(row.categoryId)"
                                       v-if="row.isPrimary && row.hasChildren">
                                    <v-icon size="18" :icon="collapsedCategories[row.categoryId] ? mdiChevronRight : mdiChevronDown"/>
                                </v-btn>
                                <span class="budget-category-twisty" v-else-if="row.isPrimary"></span>
                                <span class="budget-category-indent" v-else></span>
                                <div class="d-flex flex-column">
                                    <div class="d-flex align-center">
                                        <span :class="row.isPrimary ? 'font-weight-medium' : ''">{{ getCategoryName(row.categoryId) }}</span>
                                        <v-chip class="ms-2" size="x-small" color="error" variant="tonal"
                                                v-if="row.node.overAllocated">{{ tt('Over-allocated') }}</v-chip>
                                    </div>
                                    <span class="text-caption text-medium-emphasis"
                                          v-if="row.isPrimary && row.node.unallocated.isPositive()">
                                        {{ tt('format.misc.budgetUnallocatedInBranch', { amount: displayAmount(row.node.unallocated) }) }}
                                    </span>
                                </div>
                            </div>
                        </td>
                        <td class="budget-meter-column">
                            <div class="d-flex align-center">
                                <div class="budget-meter" :style="{ background: row.trackColor }">
                                    <div class="budget-meter-fill" :style="{ width: row.width, background: row.color }"></div>
                                </div>
                                <!-- the state is named and iconed, never carried by the colour alone -->
                                <v-icon class="ms-2" size="16" :icon="row.icon" :style="{ color: row.color }" v-if="row.icon"/>
                                <span class="text-caption text-medium-emphasis ms-2 text-no-wrap">{{ row.statusText }}</span>
                            </div>
                        </td>
                        <td class="text-end budget-plan-amount-cell">
                            <amount-input class="budget-plan-amount-input"
                                          density="compact" variant="outlined"
                                          :autofocus="true"
                                          :currency="defaultCurrency"
                                          :show-currency="true"
                                          :disabled="savingCategoryId === row.categoryId"
                                          v-model="editingExpectation"
                                          @enter="commitExpectation(row.node)"
                                          v-if="editingCategoryId === row.categoryId"/>
                            <button class="budget-plan-amount-button text-no-wrap"
                                    @click="startEditingExpectation(row.node)"
                                    v-else>
                                <span v-if="row.node.expectation">{{ displayAmount(row.node.expectation) }}</span>
                                <span class="text-medium-emphasis" v-else>{{ tt('Set a figure') }}</span>
                                <v-icon class="budget-plan-amount-pencil ms-1" size="13" :icon="mdiPencilOutline"/>
                            </button>
                        </td>
                        <td class="text-end text-no-wrap">{{ displayAmount(row.node.planned) }}</td>
                        <td class="text-end text-no-wrap">{{ displayAmount(row.node.actual) }}</td>
                        <!-- a negative remainder is only bad news on the way out: an income
                             category past what was expected of it has earned more, not overspent -->
                        <td class="text-end text-no-wrap"
                            :class="!row.isIncome && row.node.remaining.isNegative() ? 'text-expense' : ''">{{ row.remainingText }}</td>
                        <td class="text-end budget-plan-operation-column">
                            <div class="budget-plan-operation-buttons"
                                 :class="{ 'budget-plan-operation-buttons-shown': hoveredCategoryId === row.categoryId || editingCategoryId === row.categoryId }">
                                <template v-if="editingCategoryId === row.categoryId">
                                    <v-btn class="px-2" color="primary" density="comfortable" variant="text"
                                           :prepend-icon="mdiCheck" :loading="savingCategoryId === row.categoryId"
                                           @click="commitExpectation(row.node)">{{ tt('Save') }}</v-btn>
                                    <v-btn class="px-2" color="default" density="comfortable" variant="text"
                                           :prepend-icon="mdiClose" @click="cancelEditingExpectation">{{ tt('Cancel') }}</v-btn>
                                </template>
                                <!-- clearing is its own button because the alternative is typing a
                                     zero, and a zero looks like a figure somebody meant -->
                                <v-btn class="px-2" color="default" density="comfortable" variant="text"
                                       :prepend-icon="mdiCloseCircleOutline" :disabled="loading"
                                       @click="clearExpectation(row.node)"
                                       v-else-if="row.node.expectation">{{ tt('Clear') }}</v-btn>
                            </div>
                        </td>
                    </tr>
                    </tbody>
                    </template>
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
import AmountInput from '@/components/desktop/AmountInput.vue';
import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';
import EditPlanItemDialog from './dialogs/EditPlanItemDialog.vue';
import AdjustScheduleDialog from './dialogs/AdjustScheduleDialog.vue';

import { ref, computed, useTemplateRef, onMounted } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useAccountsStore } from '@/stores/account.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { type CategoryBudgetNode, useBudgetPlanStore } from '@/stores/budgetPlan.ts';

import type { BigDecimal } from '@/core/numeral.ts';
import { TransactionType } from '@/core/transaction.ts';
import { BudgetPlanItem } from '@/models/budget_plan.ts';
import { type PlannedLine, PlannedLineSource, hasCategoryBudgetActivity } from '@/lib/budgetPlan.ts';
import { BIG_DECIMAL_ZERO } from '@/lib/numeral.ts';
import { parseDateTimeFromUnixTime, getYearMonthFirstUnixTime } from '@/lib/datetime.ts';

import {
    mdiRefresh,
    mdiPlus,
    mdiCheck,
    mdiClose,
    mdiChevronLeft,
    mdiChevronRight,
    mdiChevronDown,
    mdiDotsVertical,
    mdiContentCopy,
    mdiCalendarTodayOutline,
    mdiCalendarBlankOutline,
    mdiClockTimeNineOutline,
    mdiPencilOutline,
    mdiDeleteOutline,
    mdiTuneVariant,
    mdiCancel,
    mdiRestore,
    mdiCloseCircleOutline,
    mdiAlertCircleOutline,
    mdiAlertOutline
} from '@mdi/js';

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;
type EditPlanItemDialogType = InstanceType<typeof EditPlanItemDialog>;
type AdjustScheduleDialogType = InstanceType<typeof AdjustScheduleDialog>;

interface PlannedLineGroup {
    key: string;
    title: string;
    lines: PlannedLine[];
    total: BigDecimal;
}

interface FlowSegment {
    key: string;
    label: string;
    color: string;
    // labelColor is chosen against the fill rather than by position, so the one segment that can be
    // either a light neutral or a dark red never ends up with unreadable text on it
    labelColor: string;
    share: number;
    displayAmount: string;
    displayShare: string;
    showInlineLabel: boolean;
}

interface CategoryRow {
    categoryId: string;
    node: CategoryBudgetNode;
    isPrimary: boolean;
    isIncome: boolean;
    hasChildren: boolean;
    width: string;
    color: string;
    trackColor: string;
    statusText: string;
    // remainingText rather than an amount, because a category with nothing to measure against has
    // no remainder to state and a zero there would read as one
    remainingText: string;
    icon: string;
}

interface CategorySection {
    key: string;
    title: string;
    rows: CategoryRow[];
}

// The three parts of one income are told apart by identity, so they take the app's own two accent
// hues plus a neutral for the part that is not allocated at all. The status colours are left alone
// for the meters below, where they mean over and near-over rather than "the third series".
const FLOW_COLOR_SPENT = 'rgb(var(--v-theme-primary))';
const FLOW_COLOR_STILL_TO_COME = 'rgb(var(--v-theme-teal))';
const FLOW_COLOR_LEFT = 'rgba(var(--v-theme-on-surface), 0.14)';

const FLOW_LABEL_ON_FILL = 'rgb(var(--v-theme-on-primary))';
const FLOW_LABEL_ON_SURFACE = 'rgb(var(--v-theme-on-surface))';

const METER_COLOR_UNDER = 'rgb(var(--v-theme-primary))';
const METER_COLOR_NEAR = 'rgb(var(--v-theme-warning))';
const METER_COLOR_OVER = 'rgb(var(--v-theme-error))';

// the unfilled track is a lighter step of whatever the fill is, so the state reads across the whole
// bar rather than only across the filled part
const METER_TRACK_UNDER = 'rgba(var(--v-theme-primary), 0.16)';
const METER_TRACK_NEAR = 'rgba(var(--v-theme-warning), 0.16)';
const METER_TRACK_OVER = 'rgba(var(--v-theme-error), 0.16)';

const NEAR_PLAN_RATIO = 0.85;

const { tt, formatAmountToLocalizedNumeralsWithCurrency, formatNumberToLocalizedNumerals, formatPercentToLocalizedNumerals, formatDateTimeToGregorianLikeLongYearMonth } = useI18n();

const accountsStore = useAccountsStore();
const transactionCategoriesStore = useTransactionCategoriesStore();
const budgetPlanStore = useBudgetPlanStore();

const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');
const editPlanItemDialog = useTemplateRef<EditPlanItemDialogType>('editPlanItemDialog');
const adjustScheduleDialog = useTemplateRef<AdjustScheduleDialogType>('adjustScheduleDialog');

const loading = ref<boolean>(true);
const hoveredLineKey = ref<string>('');
const editingLineKey = ref<string>('');
const savingLineKey = ref<string>('');
const editingAmount = ref<number>(0);
const collapsedGroups = ref<Record<string, boolean>>({});
const collapsedCategories = ref<Record<string, boolean>>({});
const hoveredCategoryId = ref<string>('');
const editingCategoryId = ref<string>('');
const savingCategoryId = ref<string>('');
const editingExpectation = ref<number>(0);
const showAllCategories = ref<boolean>(false);

const allLines = computed<PlannedLine[]>(() => budgetPlanStore.allLines);
const plannedTotals = computed(() => budgetPlanStore.plannedTotals);
const incomeBasis = computed<BigDecimal>(() => budgetPlanStore.incomeBasis);
const remainingToSpend = computed<BigDecimal>(() => budgetPlanStore.remainingToSpend);
const projectedExpense = computed<BigDecimal>(() => budgetPlanStore.projectedExpense);
const monthNet = computed<BigDecimal>(() => budgetPlanStore.monthNet);
const actualTotals = computed(() => budgetPlanStore.actualTotals);
const defaultCurrency = computed<string>(() => budgetPlanStore.defaultCurrency);

const overspent = computed<boolean>(() => monthNet.value.isNegative());

const displayMonth = computed<string>(() => formatDateTimeToGregorianLikeLongYearMonth(parseDateTimeFromUnixTime(getYearMonthFirstUnixTime({ year: budgetPlanStore.year, month0base: budgetPlanStore.month - 1 }))));

// Income first, then what recurs, then what was decided for this month in particular. It is the
// order the month is actually reasoned about: what is coming in, what is already spoken for, and
// what is left to decide.
const lineGroups = computed<PlannedLineGroup[]>(() => [
    buildGroup('income', tt('Income'), line => line.type === TransactionType.Income),
    buildGroup('scheduled', tt('Scheduled'), line => line.type !== TransactionType.Income && line.source === PlannedLineSource.Schedule),
    buildGroup('planned', tt('Planned'), line => line.type !== TransactionType.Income && line.source === PlannedLineSource.Item)
]);

// The bar is one income, or one month's spending, whichever is the larger - and it holds only
// parts that do not overlap. What is spent past the income is deliberately NOT a fourth segment:
// the overspend is the same money already drawn as committed and planned, and adding it again
// would make the parts sum to more than the whole. It is the hero figure above, and the point on
// the bar where the income ran out is the marker.
const flowTotal = computed<BigDecimal>(() => {
    const income = incomeBasis.value;
    const expense = projectedExpense.value;
    return income.greaterThan(expense) ? income : expense;
});

const flowSegments = computed<FlowSegment[]>(() => {
    const total = flowTotal.value;

    if (!total.isPositive()) {
        return [];
    }

    // The bar reads left to right the way the month runs: money already gone, money still to go,
    // and what survives. Every part is money the income has to cover, and no part is counted twice,
    // so the three of them are the income - which is what makes the last one believable.
    const segments: FlowSegment[] = [
        buildSegment('spent', tt('Spent'), FLOW_COLOR_SPENT, FLOW_LABEL_ON_FILL, actualTotals.value.expense, total),
        buildSegment('still-to-come', tt('Still to Come'), FLOW_COLOR_STILL_TO_COME, FLOW_LABEL_ON_FILL, remainingToSpend.value, total),
        buildSegment('left', tt('Left Over'), FLOW_COLOR_LEFT, FLOW_LABEL_ON_SURFACE, monthNet.value, total)
    ];

    // a segment worth nothing is dropped rather than drawn at zero width, where it would still cost
    // the bar one of its gaps and read as a sliver that means something
    return segments.filter(segment => segment.share > 0);
});

// The marker is placed only when there is an overspend to mark and an income to mark it with: a
// month with no income planned at all has nothing to say here that the hero figure does not.
const incomeMarkerLeft = computed<string>(() => {
    if (!overspent.value || !incomeBasis.value.isPositive() || !flowTotal.value.isPositive()) {
        return '';
    }

    return `${(incomeBasis.value.toDoubleNumber() / flowTotal.value.toDoubleNumber()) * 100}%`;
});

// A primary is shown when anything under it is planned, spent or expected; a secondary only when it
// is itself. Turning on Show every category reveals the rest, which is how a figure gets set against
// a category that has nothing in it yet - the commonest way an expectation starts life.
const categoryRows = computed<CategoryRow[]>(() => {
    const rows: CategoryRow[] = [];

    for (const node of budgetPlanStore.categoryBudgetTree) {
        const children = node.children.filter(isCategoryShown);

        if (!isCategoryShown(node) && !children.length) {
            continue;
        }

        rows.push(buildCategoryRow(node, true, node.children.length > 0));

        if (collapsedCategories.value[node.categoryId]) {
            continue;
        }

        for (const child of children) {
            rows.push(buildCategoryRow(child, false, false));
        }
    }

    return rows;
});

// Spending first: it is what the page is mostly for. A section with no rows is left out entirely
// rather than shown empty, which is also what keeps the heading off a table that has only one.
const categorySections = computed<CategorySection[]>(() => {
    const spending = categoryRows.value.filter(row => !row.isIncome);
    const earning = categoryRows.value.filter(row => row.isIncome);
    const sections: CategorySection[] = [];

    if (spending.length) {
        sections.push({ key: 'spending', title: tt('Spending'), rows: spending });
    }

    if (earning.length) {
        sections.push({ key: 'earning', title: tt('Earning'), rows: earning });
    }

    return sections;
});

// A category hidden from the rest of the app stays hidden here too, unless something is planned or
// spent in it - in which case leaving it out would make the totals not add up.
function isCategoryShown(node: CategoryBudgetNode): boolean {
    if (hasCategoryBudgetActivity(node)) {
        return true;
    }

    return showAllCategories.value && !transactionCategoriesStore.allTransactionCategoriesMap[node.categoryId]?.hidden;
}

function buildCategoryRow(node: CategoryBudgetNode, isPrimary: boolean, hasChildren: boolean): CategoryRow {
    // the meter runs against what the category costs the month, which is its expectation where it
    // has one and what is listed under it where it has not
    const ratio = node.budget.isPositive() ? node.actual.toDoubleNumber() / node.budget.toDoubleNumber() : (node.actual.isPositive() ? Infinity : 0);

    const isIncome = node.type === TransactionType.Income;
    // an income category with nothing expected of it has nothing to measure against: the money
    // simply arrived. A bar filled to the end and a remainder of minus the whole salary would both
    // be saying something false about it.
    const unmeasured = isIncome && !node.budget.isPositive();

    let color = METER_COLOR_UNDER;
    let trackColor = METER_TRACK_UNDER;
    let icon = '';
    let statusText: string;

    // Only spending is warned about. Earning more than was expected, or from somewhere that was not
    // planned for at all, is not a problem to flag - and the status colours mean a problem.
    if (unmeasured) {
        statusText = tt('Received');
    } else if (isIncome) {
        statusText = formatPercentToLocalizedNumerals(Math.min(ratio, 1) * 100, 0, '<1');
    } else if (!node.budget.isPositive() && node.actual.isPositive()) {
        color = METER_COLOR_OVER;
        trackColor = METER_TRACK_OVER;
        icon = mdiAlertCircleOutline;
        statusText = tt('Unplanned');
    } else if (ratio > 1) {
        color = METER_COLOR_OVER;
        trackColor = METER_TRACK_OVER;
        icon = mdiAlertCircleOutline;
        statusText = tt('Over plan');
    } else if (ratio >= NEAR_PLAN_RATIO) {
        color = METER_COLOR_NEAR;
        trackColor = METER_TRACK_NEAR;
        icon = mdiAlertOutline;
        statusText = formatPercentToLocalizedNumerals(ratio * 100, 0, '<1');
    } else {
        statusText = formatPercentToLocalizedNumerals(ratio * 100, 0, '<1');
    }

    const meterRatio = unmeasured ? 0 : ratio;

    return {
        categoryId: node.categoryId,
        node: node,
        isPrimary: isPrimary,
        isIncome: isIncome,
        hasChildren: hasChildren,
        width: `${Math.max(Math.min(meterRatio, 1) * 100, meterRatio > 0 ? 3 : 0)}%`,
        color: color,
        trackColor: trackColor,
        statusText: statusText,
        remainingText: unmeasured ? '—' : displayAmount(node.remaining),
        icon: icon
    };
}

function buildGroup(key: string, title: string, matches: (line: PlannedLine) => boolean): PlannedLineGroup {
    const lines = allLines.value.filter(matches);
    let total = BIG_DECIMAL_ZERO;

    for (const line of lines) {
        if (line.excluded) {
            continue;
        }

        const converted = budgetPlanStore.convertToDefaultCurrency(line.amount, line.currency);

        if (converted) {
            total = total.add(converted);
        }
    }

    return { key: key, title: title, lines: lines, total: total };
}

function buildSegment(key: string, label: string, color: string, labelColor: string, amount: BigDecimal, total: BigDecimal): FlowSegment {
    const share = amount.isPositive() ? amount.toDoubleNumber() / total.toDoubleNumber() : 0;

    return {
        key: key,
        label: label,
        color: color,
        labelColor: labelColor,
        share: share,
        displayAmount: displayAmount(amount),
        displayShare: formatPercentToLocalizedNumerals(share * 100, 0, '<1'),
        // a label only goes inside a segment when the segment is wide enough to hold it without
        // being clipped; the legend and the tooltip carry the rest
        showInlineLabel: share >= 0.25
    };
}

function lineKey(groupKey: string, line: PlannedLine): string {
    return `${groupKey}-${line.source}-${line.id}`;
}

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
        return `×${formatNumberToLocalizedNumerals(occurrences)}`;
    }

    return `×${formatNumberToLocalizedNumerals(Math.round(occurrences * 100) / 100)}`;
}

function getCategoryName(categoryId: string): string {
    return transactionCategoriesStore.allTransactionCategoriesMap[categoryId]?.name ?? '';
}

function getAccountName(accountId: string): string {
    return accountsStore.allAccountsMap[accountId]?.name ?? '';
}

function toggleGroup(key: string): void {
    collapsedGroups.value = { ...collapsedGroups.value, [key]: !collapsedGroups.value[key] };
}

function toggleCategory(categoryId: string): void {
    collapsedCategories.value = { ...collapsedCategories.value, [categoryId]: !collapsedCategories.value[categoryId] };
}

// An expectation is typed in the default currency, because it is a figure about a category rather
// than about an account, and a category has no currency of its own.
function startEditingExpectation(node: CategoryBudgetNode): void {
    editingCategoryId.value = node.categoryId;
    editingExpectation.value = node.expectation ? node.expectation.toSafeIntegerNumber() : 0;
}

function cancelEditingExpectation(): void {
    editingCategoryId.value = '';
    editingExpectation.value = 0;
}

function commitExpectation(node: CategoryBudgetNode): void {
    const newAmount = editingExpectation.value;
    const oldAmount = node.expectation ? node.expectation.toSafeIntegerNumber() : 0;

    if (newAmount === oldAmount) {
        cancelEditingExpectation();
        return;
    }

    saveExpectation(node.categoryId, newAmount);
}

// Clearing is saving nothing: the server takes a zero as the removal it is, so there is one path
// through here rather than two.
function clearExpectation(node: CategoryBudgetNode): void {
    saveExpectation(node.categoryId, 0);
}

function saveExpectation(categoryId: string, amount: number): void {
    savingCategoryId.value = categoryId;

    budgetPlanStore.setCategoryExpectation({ categoryId: categoryId, amount: amount }).then(() => {
        savingCategoryId.value = '';
        cancelEditingExpectation();
    }).catch(error => {
        savingCategoryId.value = '';

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

// The amount edited in place is what one occurrence costs, not what the month costs - the same
// figure the adjust dialog asks for, so the two never disagree.
function startEditingAmount(groupKey: string, line: PlannedLine): void {
    if (line.excluded) {
        return;
    }

    editingLineKey.value = lineKey(groupKey, line);
    editingAmount.value = line.unitAmount;
}

function cancelEditingAmount(): void {
    editingLineKey.value = '';
    editingAmount.value = 0;
}

function commitAmount(line: PlannedLine): void {
    const key = editingLineKey.value;
    const newAmount = editingAmount.value;

    if (newAmount === line.unitAmount) {
        cancelEditingAmount();
        return;
    }

    savingLineKey.value = key;

    const saving = line.source === PlannedLineSource.Schedule
        ? budgetPlanStore.setScheduleAdjustment({ templateId: line.id, excluded: false, amount: newAmount })
        : saveItemAmount(line, newAmount);

    saving.then(() => {
        savingLineKey.value = '';
        cancelEditingAmount();
    }).catch(error => {
        savingLineKey.value = '';

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function saveItemAmount(line: PlannedLine, newAmount: number): Promise<unknown> {
    const item = budgetPlanStore.planItems.find(planItem => planItem.id === line.id);

    if (!item) {
        return Promise.resolve();
    }

    const updated = item.clone();
    updated.amount = newAmount;

    return budgetPlanStore.saveBudgetPlanItem({ item: updated });
}

function load(force: boolean): void {
    loading.value = true;
    cancelEditingAmount();
    cancelEditingExpectation();

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
.budget-plan-month {
    min-width: 130px;
    text-align: center;
}

.budget-plan-figures {
    display: flex;
    flex-wrap: wrap;
    gap: 8px 40px;
}

.budget-plan-figure {
    min-width: 120px;
}

.budget-category-section-row > td {
    padding-top: 14px;
    letter-spacing: 0.5px;
}

.budget-plan-hero {
    font-size: 48px;
    line-height: 1.1;
    font-weight: 600;
    letter-spacing: -0.5px;
}

/* One bar, its parts separated by the surface showing through rather than by a border drawn
   around each of them. */
.budget-flow-bar-wrapper {
    position: relative;
    margin-top: 8px;
}

.budget-flow-income-marker {
    position: absolute;
    top: -3px;
    bottom: -3px;
    width: 2px;
    margin-inline-start: -1px;
    background: rgb(var(--v-theme-on-surface));
    border-radius: 1px;
}

.budget-flow-bar {
    display: flex;
    gap: 2px;
    height: 20px;
    border-radius: 4px;
    background: rgba(var(--v-theme-on-surface), 0.06);
}

.budget-flow-segment {
    height: 100%;
    flex-basis: 0;
    flex-shrink: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: flex-grow 0.35s ease;
    cursor: default;
}

.budget-flow-segment:first-child {
    border-start-start-radius: 4px;
    border-end-start-radius: 4px;
}

.budget-flow-segment:last-child {
    border-start-end-radius: 4px;
    border-end-end-radius: 4px;
}

.budget-flow-inline-label {
    font-size: 11px;
    line-height: 1;
    white-space: nowrap;
    padding: 0 6px;
}

.budget-flow-legend {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 20px;
    margin-top: 10px;
}

.budget-flow-legend-item {
    display: flex;
    align-items: center;
}

.budget-flow-swatch {
    width: 10px;
    height: 10px;
    border-radius: 2px;
    margin-inline-end: 6px;
}

.budget-meter-column {
    width: 210px;
}

.budget-category-switch {
    flex: 0 0 auto;
}

/* The twisty and the spacer that stands in for it are the same width, so a primary with
   subcategories and one without line their names up in the same place; the indent is that width
   again plus the step that puts a subcategory under its parent. */
.budget-category-twisty {
    width: 24px;
    min-width: 24px;
    flex: 0 0 auto;
    margin-inline-end: 6px;
}

.budget-category-indent {
    width: 48px;
    min-width: 48px;
    flex: 0 0 auto;
}

.budget-category-table .budget-category-primary-row > td {
    border-top: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
}

.budget-category-table .budget-category-primary-row:first-child > td {
    border-top: none;
}

.budget-category-table tr:hover .budget-plan-amount-button .budget-plan-amount-pencil {
    opacity: 0.5;
}

.budget-meter {
    flex: 1 1 auto;
    min-width: 70px;
    height: 6px;
    border-radius: 3px;
    overflow: hidden;
}

.budget-meter-fill {
    height: 100%;
    border-radius: 3px;
    transition: width 0.35s ease;
}

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

.budget-plan-table .budget-plan-group-row {
    cursor: pointer;
    user-select: none;
}

.budget-plan-table .budget-plan-group-row > td {
    padding-top: 14px;
    letter-spacing: 0.5px;
}

.budget-plan-table .budget-plan-amount-cell {
    min-width: 160px;
}

.budget-plan-amount-button {
    background: none;
    border: none;
    padding: 2px 4px;
    border-radius: 4px;
    color: inherit;
    font: inherit;
    cursor: pointer;
}

.budget-plan-amount-button:hover:not(:disabled) {
    background: rgba(var(--v-theme-on-surface), 0.06);
}

.budget-plan-amount-button:disabled {
    cursor: default;
}

.budget-plan-amount-pencil {
    opacity: 0;
}

.budget-plan-line-row:hover .budget-plan-amount-button:not(:disabled) .budget-plan-amount-pencil {
    opacity: 0.5;
}

.budget-plan-amount-input {
    min-width: 150px;
}
</style>
