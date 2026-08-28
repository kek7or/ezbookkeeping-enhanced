<template>
    <v-row class="match-height">
        <v-col cols="12">
            <v-card>
                <template #title>
                    <div class="title-and-toolbar d-flex align-center">
                        <span>{{ templateType === TemplateType.Schedule.type ? tt('Scheduled Transactions') : tt('Transaction Templates') }}</span>
                        <v-btn class="ms-3" color="default" variant="outlined"
                               :disabled="loading || updating" @click="add">{{ tt('Add') }}</v-btn>
                        <v-btn class="ms-3" color="primary" variant="tonal"
                               :disabled="loading || updating" @click="saveSortResult"
                               v-if="displayOrderModified">{{ tt('Save Display Order') }}</v-btn>
                        <v-btn density="compact" color="default" variant="text" size="24"
                               class="ms-2" :icon="true" :disabled="loading || updating"
                               :loading="loading" @click="reload">
                            <template #loader>
                                <v-progress-circular indeterminate size="20"/>
                            </template>
                            <v-icon :icon="mdiRefresh" size="24" />
                            <v-tooltip activator="parent">{{ tt('Refresh') }}</v-tooltip>
                        </v-btn>
                        <v-spacer/>
                        <v-btn density="comfortable" color="default" variant="text" class="ms-2"
                               :disabled="loading || updating" :icon="true">
                            <v-icon :icon="mdiDotsVertical" />
                            <v-menu activator="parent">
                                <v-list>
                                    <v-list-item :prepend-icon="mdiEyeOutline"
                                                 :title="tt('Show Hidden Transaction Templates')"
                                                 v-if="!showHidden" @click="showHidden = true"></v-list-item>
                                    <v-list-item :prepend-icon="mdiEyeOffOutline"
                                                 :title="tt('Hide Hidden Transaction Templates')"
                                                 v-if="showHidden" @click="showHidden = false"></v-list-item>
                                </v-list>
                            </v-menu>
                        </v-btn>
                    </div>
                </template>

                <v-table class="transaction-templates-table table-striped" :hover="!loading">
                    <thead>
                    <tr>
                        <th>{{ tt('Template Name') }}</th>
                        <template v-if="isScheduleList">
                            <th>{{ tt('Category') }}</th>
                            <th>{{ tt('Account') }}</th>
                            <th>{{ tt('Frequency') }}</th>
                            <th class="text-end">{{ tt('Amount') }}</th>
                        </template>
                        <th class="text-end template-operation-column">{{ tt('Operation') }}</th>
                    </tr>
                    </thead>

                    <tbody v-if="loading && noAvailableTemplate">
                    <tr :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]">
                        <td class="px-0" :colspan="columnCount">
                            <v-skeleton-loader type="text" :loading="true"></v-skeleton-loader>
                        </td>
                    </tr>
                    </tbody>

                    <tbody v-if="!loading && noAvailableTemplate">
                    <tr>
                        <td :colspan="columnCount" v-if="templateType === TemplateType.Normal.type">{{ tt('No available template. Once you add templates, you can quickly add a new transaction using the dropdown menu of the Add button on the transaction list page') }}</td>
                        <td :colspan="columnCount" v-else-if="templateType === TemplateType.Schedule.type">{{ tt('No available scheduled transactions') }}</td>
                        <td :colspan="columnCount" v-else>{{ tt('No available template') }}</td>
                    </tr>
                    </tbody>

                    <draggable-list tag="tbody"
                                    item-key="id"
                                    handle=".drag-handle"
                                    ghost-class="dragging-item"
                                    :disabled="noAvailableTemplate"
                                    v-model="templates"
                                    @change="onMove">
                        <template #item="{ element }">
                            <tr class="transaction-templates-table-row" v-if="showHidden || !element.hidden"
                                @mouseenter="hoveredTemplateId = element.id" @mouseleave="hoveredTemplateId = ''">
                                <td>
                                    <div class="d-flex align-center">
                                        <v-badge class="right-bottom-icon" color="secondary"
                                                 location="bottom right" offset-x="8" :icon="mdiEyeOffOutline"
                                                 v-if="element.hidden">
                                            <v-icon size="20" start :icon="getTemplateIcon(element)"/>
                                        </v-badge>
                                        <v-icon size="20" start :icon="getTemplateIcon(element)" v-else-if="!element.hidden">
                                            <v-tooltip activator="parent" v-if="element.isSubscription">{{ tt('This is a subscription') }}</v-tooltip>
                                        </v-icon>
                                        <span class="transaction-template-name">{{ element.name }}</span>
                                    </div>
                                </td>

                                <template v-if="isScheduleList">
                                    <td class="text-truncate">{{ getDisplayCategoryName(element) }}</td>
                                    <td class="text-truncate">{{ getDisplayAccountName(element) }}</td>
                                    <td class="text-truncate">{{ getDisplayFrequency(element) }}</td>
                                    <td class="text-end text-no-wrap">{{ getDisplayAmount(element) }}</td>
                                </template>

                                <td class="text-end template-operation-column">
                                    <div class="d-flex align-center justify-end">
                                        <div class="template-operation-buttons d-flex align-center"
                                             :class="{ 'template-operation-buttons-shown': hoveredTemplateId === element.id && !loading }">
                                            <v-btn class="px-2 ms-2" color="default"
                                                   density="comfortable" variant="text"
                                                   :prepend-icon="element.hidden ? mdiEyeOutline : mdiEyeOffOutline"
                                                   :loading="templateHiding[element.id]"
                                                   :disabled="loading || updating"
                                                   @click="hide(element, !element.hidden)">
                                                <template #loader>
                                                    <v-progress-circular indeterminate size="20" width="2"/>
                                                </template>
                                                {{ element.hidden ? tt('Show') : tt('Hide') }}
                                            </v-btn>
                                            <v-btn class="px-2" color="default"
                                                   density="comfortable" variant="text"
                                                   :prepend-icon="mdiPencilOutline"
                                                   :disabled="loading || updating"
                                                   @click="edit(element)">
                                                <template #loader>
                                                    <v-progress-circular indeterminate size="20" width="2"/>
                                                </template>
                                                {{ tt('Edit') }}
                                            </v-btn>
                                            <v-btn class="px-2" color="default"
                                                   density="comfortable" variant="text"
                                                   :prepend-icon="mdiDeleteOutline"
                                                   :loading="templateRemoving[element.id]"
                                                   :disabled="loading || updating"
                                                   @click="remove(element)">
                                                <template #loader>
                                                    <v-progress-circular indeterminate size="20" width="2"/>
                                                </template>
                                                {{ tt('Delete') }}
                                            </v-btn>
                                        </div>

                                        <span class="ms-2">
                                            <v-icon :class="!loading && !updating && availableTemplateCount > 1 ? 'drag-handle' : 'disabled'"
                                                    :icon="mdiDrag"/>
                                            <v-tooltip activator="parent" v-if="!loading && !updating && availableTemplateCount > 1 && hoveredTemplateId === element.id">{{ tt('Drag to Reorder') }}</v-tooltip>
                                        </span>
                                    </div>
                                </td>
                            </tr>
                        </template>
                    </draggable-list>

                    <tfoot v-if="isScheduleList && !noAvailableTemplate && scheduleTotalRows.length">
                    <tr class="schedule-total-row" :key="row.label" v-for="row in scheduleTotalRows">
                        <td class="text-end font-weight-medium" :colspan="columnCount - 2">{{ row.label }}</td>
                        <td class="text-end text-no-wrap font-weight-medium">
                            <div>{{ row.amount }}</div>
                            <div class="text-caption text-medium-emphasis font-weight-regular"
                                 v-if="row.subscriptionAmount">{{ tt('Of which subscriptions') }} {{ row.subscriptionAmount }}</div>
                        </td>
                        <td></td>
                    </tr>
                    </tfoot>
                </v-table>
            </v-card>
        </v-col>
    </v-row>

    <edit-dialog ref="editDialog" :type="TransactionEditPageType.Template" />

    <confirm-dialog ref="confirmDialog"/>
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';
import EditDialog from '@/views/desktop/transactions/list/dialogs/EditDialog.vue';
import { TransactionEditPageType } from '@/views/base/transactions/TransactionEditPageBase.ts';

import { ref, computed, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useUserStore } from '@/stores/user.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { useTransactionTemplatesStore } from '@/stores/transactionTemplate.ts';

import { TemplateType } from '@/core/template.ts';
import { type WeekDayValue } from '@/core/datetime.ts';
import { DISPLAY_HIDDEN_AMOUNT } from '@/consts/numeral.ts';
import { TransactionTemplate } from '@/models/transaction_template.ts';

import { type BigDecimal } from '@/core/numeral.ts';
import { parseBigDecimal } from '@/lib/numeral.ts';
import {
    type ScheduledCostTotal,
    isNoAvailableTemplate,
    getAvailableTemplateCount,
    sumScheduledCostByCurrency
} from '@/lib/template.ts';

import {
    mdiRefresh,
    mdiPencilOutline,
    mdiEyeOffOutline,
    mdiEyeOutline,
    mdiDeleteOutline,
    mdiDrag,
    mdiDotsVertical,
    mdiTextBoxOutline,
    mdiClockTimeNineOutline,
    mdiAutorenew
} from '@mdi/js';

interface ScheduleTotalRow {
    label: string;
    amount: string;
    subscriptionAmount: string;
}

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;
type EditDialogType = InstanceType<typeof EditDialog>;

const props = defineProps<{
    initType: number;
}>();

const { tt, getScheduleFrequencyDisplayName, formatAmountToLocalizedNumeralsWithCurrency } = useI18n();

const userStore = useUserStore();
const accountsStore = useAccountsStore();
const transactionCategoriesStore = useTransactionCategoriesStore();
const transactionTemplatesStore = useTransactionTemplatesStore();

const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');
const editDialog = useTemplateRef<EditDialogType>('editDialog');

const templateType = ref<number>(TemplateType.Normal.type);
const loading = ref<boolean>(true);
const updating = ref<boolean>(false);
const hoveredTemplateId = ref<string>('');
const templateHiding = ref<Record<string, boolean>>({});
const templateRemoving = ref<Record<string, boolean>>({});
const displayOrderModified = ref<boolean>(false);
const showHidden = ref<boolean>(false);

const templates = computed<TransactionTemplate[]>(() => transactionTemplatesStore.allTransactionTemplates[templateType.value] || []);
const noAvailableTemplate = computed<boolean>(() => isNoAvailableTemplate(templates.value, showHidden.value));
const availableTemplateCount = computed<number>(() => getAvailableTemplateCount(templates.value, showHidden.value));

const isScheduleList = computed<boolean>(() => templateType.value === TemplateType.Schedule.type);
const columnCount = computed<number>(() => isScheduleList.value ? 6 : 2);
const firstDayOfWeek = computed<WeekDayValue>(() => userStore.currentUserFirstDayOfWeek);

const scheduleTotals = computed<ScheduledCostTotal[]>(() => sumScheduledCostByCurrency(templates.value, showHidden.value, getTemplateCurrency));

// The subscription figure is a share of the expense figure above it, not a third total beside it.
// It is what the page is asked for once the rent and the Abschlag are taken as given: of everything
// committed, how much is the part that could be cancelled tomorrow.
const subscriptionTotals = computed<ScheduledCostTotal[]>(() => sumScheduledCostByCurrency(templates.value.filter(template => template.isSubscription), showHidden.value, getTemplateCurrency));

// The footer says what a month and a year of the listed schedules come to. Expense and income are
// named only when both are present: a page of nothing but outgoings does not need to be told which
// it is looking at, and a page that has both must never let the two be read as one figure.
const scheduleTotalRows = computed<ScheduleTotalRow[]>(() => {
    const totals = scheduleTotals.value;
    const rows: ScheduleTotalRow[] = [];

    const hasExpense = totals.some(total => !total.yearlyExpense.isZero());
    const hasIncome = totals.some(total => !total.yearlyIncome.isZero());

    for (const [label, pick, present, subscriptions] of [
        [tt('Expense'), (total: ScheduledCostTotal) => total.yearlyExpense, hasExpense, subscriptionTotals.value] as const,
        [tt('Income'), (total: ScheduledCostTotal) => total.yearlyIncome, hasIncome, [] as ScheduledCostTotal[]] as const
    ]) {
        if (!present) {
            continue;
        }

        const named = hasExpense && hasIncome;
        const perMonth = (total: ScheduledCostTotal): BigDecimal => pick(total).divide(12);

        rows.push({
            label: named ? `${label} · ${tt('Per Month')}` : tt('Per Month'),
            amount: formatTotals(totals, perMonth),
            subscriptionAmount: formatTotals(subscriptions, perMonth)
        });
        rows.push({
            label: named ? `${label} · ${tt('Per Year')}` : tt('Per Year'),
            amount: formatTotals(totals, pick),
            subscriptionAmount: formatTotals(subscriptions, pick)
        });
    }

    return rows;
});

function getTemplateIcon(template: TransactionTemplate): string {
    if (templateType.value !== TemplateType.Schedule.type) {
        return mdiTextBoxOutline;
    }

    return template.isSubscription ? mdiAutorenew : mdiClockTimeNineOutline;
}

// Totals of different currencies are joined rather than summed, because there is no honest single
// number for them - see sumScheduledCostByCurrency.
function formatTotals(totals: ScheduledCostTotal[], pick: (total: ScheduledCostTotal) => BigDecimal): string {
    return totals
        .filter(total => !pick(total).isZero())
        .map(total => formatAmountToLocalizedNumeralsWithCurrency(pick(total).truncate(), total.currency))
        .join(' + ');
}

function getTemplateCurrency(template: TransactionTemplate): string | undefined {
    return accountsStore.allAccountsMap[template.sourceAccountId]?.currency;
}

function getDisplayCategoryName(template: TransactionTemplate): string {
    return transactionCategoriesStore.allTransactionCategoriesMap[template.categoryId]?.name ?? '';
}

function getDisplayAccountName(template: TransactionTemplate): string {
    return accountsStore.allAccountsMap[template.sourceAccountId]?.name ?? '';
}

function getDisplayFrequency(template: TransactionTemplate): string {
    return getScheduleFrequencyDisplayName(template.scheduledFrequencyType ?? 0, template.scheduledFrequency ?? '', firstDayOfWeek.value);
}

function getDisplayAmount(template: TransactionTemplate): string {
    const currency = getTemplateCurrency(template) ?? userStore.currentUserDefaultCurrency;

    if (template.hideAmount) {
        return formatAmountToLocalizedNumeralsWithCurrency(DISPLAY_HIDDEN_AMOUNT, currency);
    }

    return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(template.sourceAmount), currency);
}

function init(): void {
    templateType.value = props.initType;
    loading.value = true;

    // The schedule list names a category, an account and an amount in that account's currency, none
    // of which travels with the template - it carries ids. Both caches are asked for unforced, so a
    // page reached from anywhere else in the app costs nothing.
    Promise.all([
        transactionTemplatesStore.loadAllTemplates({
            templateType: templateType.value,
            force: false
        }),
        transactionCategoriesStore.loadAllCategories({ force: false }),
        accountsStore.loadAllAccounts({ force: false })
    ]).then(() => {
        loading.value = false;
    }).catch(error => {
        loading.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function reload(): void {
    loading.value = true;

    transactionTemplatesStore.loadAllTemplates({
        templateType: templateType.value,
        force: true
    }).then(() => {
        loading.value = false;
        displayOrderModified.value = false;

        snackbar.value?.showMessage('Template list has been updated');
    }).catch(error => {
        loading.value = false;

        if (error && error.isUpToDate) {
            displayOrderModified.value = false;
        }

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function add(): void {
    editDialog.value?.open({
        templateType: templateType.value
    }).then(result => {
        if (result && result.message) {
            snackbar.value?.showMessage(result.message);
        }
    }).catch(error => {
        if (error) {
            snackbar.value?.showError(error);
        }
    });
}

function edit(template: TransactionTemplate): void {
    editDialog.value?.open({
        id: template.id,
        currentTemplate: template
    }).then(result => {
        if (result && result.message) {
            snackbar.value?.showMessage(result.message);
        }
    }).catch(error => {
        if (error) {
            snackbar.value?.showError(error);
        }
    });
}

function hide(template: TransactionTemplate, hidden: boolean): void {
    updating.value = true;
    templateHiding.value[template.id] = true;

    transactionTemplatesStore.hideTemplate({
        template: template,
        hidden: hidden
    }).then(() => {
        updating.value = false;
        templateHiding.value[template.id] = false;
    }).catch(error => {
        updating.value = false;
        templateHiding.value[template.id] = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function remove(template: TransactionTemplate): void {
    confirmDialog.value?.open('Are you sure you want to delete this template?').then(() => {
        updating.value = true;
        templateRemoving.value[template.id] = true;

        transactionTemplatesStore.deleteTemplate({
            template: template
        }).then(() => {
            updating.value = false;
            templateRemoving.value[template.id] = false;
        }).catch(error => {
            updating.value = false;
            templateRemoving.value[template.id] = false;

            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    });
}

function saveSortResult(): void {
    if (!displayOrderModified.value) {
        return;
    }

    loading.value = true;

    transactionTemplatesStore.updateTemplateDisplayOrders({
        templateType: templateType.value
    }).then(() => {
        loading.value = false;
        displayOrderModified.value = false;
    }).catch(error => {
        loading.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function onMove(event: { moved: { element: { id: string }, oldIndex: number, newIndex: number } }): void {
    if (!event || !event.moved) {
        return;
    }

    const moveEvent = event.moved;

    if (!moveEvent.element || !moveEvent.element.id) {
        snackbar.value?.showMessage('Unable to move template');
        return;
    }

    transactionTemplatesStore.changeTemplateDisplayOrder({
        templateType: templateType.value,
        templateId: moveEvent.element.id,
        from: moveEvent.oldIndex,
        to: moveEvent.newIndex
    }).then(() => {
        displayOrderModified.value = true;
    }).catch(error => {
        snackbar.value?.showError(error);
    });
}

init();
</script>

<style>
.transaction-templates-table .template-operation-column {
    width: 1%;
    white-space: nowrap;
}

.transaction-templates-table .template-operation-buttons {
    visibility: hidden;
}

.transaction-templates-table .template-operation-buttons-shown {
    visibility: visible;
}

.transaction-templates-table tr:not(:last-child) > td > div {
    padding-bottom: 1px;
}

.transaction-templates-table .has-bottom-border tr:last-child > td > div {
    padding-bottom: 1px;
}

.transaction-templates-table tr.transaction-templates-table-row .right-bottom-icon .v-badge__badge {
    padding-bottom: 1px;
}

.transaction-templates-table .v-text-field .v-input__prepend {
    margin-inline-end: 0;
    color: rgba(var(--v-theme-on-surface));
}

.transaction-templates-table .v-text-field .v-input__prepend .v-badge > .v-badge__wrapper > .v-icon {
    opacity: var(--v-medium-emphasis-opacity);
}

.transaction-templates-table .v-text-field.v-input--plain-underlined .v-input__prepend {
    padding-top: 10px;
}

.transaction-templates-table .v-text-field .v-field__input {
    padding-top: 0;
    color: rgba(var(--v-theme-on-surface));
}

.transaction-templates-table tr .v-text-field .v-field__input {
    padding-bottom: 1px;
}
</style>
