<template>
    <v-row class="match-height">
        <v-col cols="12" md="4">
            <v-card :class="{ 'disabled': loadingPeople }">
                <template #title>
                    <div class="title-and-toolbar d-flex align-center">
                        <span>{{ tt('People') }}</span>
                        <v-btn class="ms-2" density="comfortable" color="default" variant="text" size="24"
                               :icon="true" :loading="loadingPeople" @click="reload(true)">
                            <template #loader>
                                <v-progress-circular indeterminate size="20"/>
                            </template>
                            <v-icon :icon="mdiRefresh" size="24" />
                            <v-tooltip activator="parent">{{ tt('Reload Page') }}</v-tooltip>
                        </v-btn>
                        <v-spacer/>
                        <v-btn density="comfortable" :prepend-icon="mdiPlus"
                               :disabled="loadingPeople || updating" @click="addPerson">
                            {{ tt('Add') }}
                        </v-btn>
                    </div>
                </template>

                <v-card-text class="pb-0">
                    <span class="text-subtitle-2">{{ tt('Total Owed to You') }}</span>
                    <p class="text-h5 mt-1 mb-0">
                        <span v-if="totalOwed.length">{{ getDisplayAmounts(totalOwed) }}</span>
                        <span v-else-if="!totalOwed.length">{{ tt('Nothing') }}</span>
                    </p>
                </v-card-text>

                <v-card-text>
                    <v-list class="pt-0" density="comfortable" v-if="allPeople.length">
                        <v-list-item class="px-3" rounded
                                     :key="person.id" v-for="person in allPeople"
                                     :active="person.id === selectedPersonId"
                                     @click="selectPerson(person.id)">
                            <template #prepend>
                                <v-icon class="me-3" :icon="mdiAccountOutline"/>
                            </template>
                            <v-list-item-title>{{ person.name }}</v-list-item-title>
                            <v-list-item-subtitle>
                                <span v-if="person.openCount">{{ getDisplayOpenAmounts(person) }}</span>
                                <span v-else-if="!person.openCount">{{ tt('Owes you nothing') }}</span>
                            </v-list-item-subtitle>
                            <template #append>
                                <v-btn density="comfortable" color="default" variant="text" :icon="true"
                                       :disabled="updating">
                                    <v-icon :icon="mdiDotsVertical"/>
                                    <v-menu activator="parent">
                                        <v-list>
                                            <v-list-item :prepend-icon="mdiPencilOutline"
                                                         :title="tt('Rename')"
                                                         @click="renamePerson(person)"></v-list-item>
                                            <v-list-item class="text-error" :prepend-icon="mdiDeleteOutline"
                                                         :title="tt('Delete')"
                                                         @click="removePerson(person)"></v-list-item>
                                        </v-list>
                                    </v-menu>
                                </v-btn>
                            </template>
                        </v-list-item>
                    </v-list>

                    <span class="text-body-1" v-else-if="!allPeople.length && !loadingPeople">
                        {{ tt('Nobody owes you anything yet. Add somebody, then attach a transaction or one of its positions to them.') }}
                    </span>

                    <v-skeleton-loader type="paragraph" :loading="true" v-else-if="!allPeople.length && loadingPeople"></v-skeleton-loader>
                </v-card-text>
            </v-card>
        </v-col>

        <v-col cols="12" md="8">
            <v-card :class="{ 'disabled': loadingEntries }" v-if="selectedPerson">
                <template #title>
                    <div class="title-and-toolbar d-flex align-center">
                        <span>{{ selectedPerson.name }}</span>
                        <v-spacer/>
                        <v-btn density="comfortable" variant="text" color="default"
                               :prepend-icon="showSettled ? mdiEyeOffOutline : mdiEyeOutline"
                               :disabled="loadingEntries" @click="toggleShowSettled">
                            {{ showSettled ? tt('Hide Settled') : tt('Show Settled') }}
                        </v-btn>
                        <v-btn class="ms-2" density="comfortable" variant="text" color="default"
                               :prepend-icon="mdiFileExcelOutline" :loading="exportingReceipt"
                               :disabled="loadingEntries || updating || !openEntries.length"
                               @click="makeReceipt">
                            <template #loader>
                                <v-progress-circular indeterminate size="20"/>
                            </template>
                            {{ tt('Make a Receipt') }}
                            <v-tooltip activator="parent" open-delay="500">{{ tt('A spreadsheet of everything still owed, to hand to this person') }}</v-tooltip>
                        </v-btn>
                        <v-btn class="ms-2" density="comfortable" :prepend-icon="mdiHandCoinOutline"
                               :disabled="loadingEntries || updating" @click="addLoan">
                            {{ tt('Record a Loan') }}
                        </v-btn>
                    </div>
                </template>

                <v-card-text>
                    <span class="text-subtitle-2">{{ tt('Still Owed') }}</span>
                    <p class="text-h5 mt-1 mb-0">
                        <span v-if="openEntries.length">{{ getDisplayAmounts(openTotals) }}</span>
                        <span v-else-if="!openEntries.length">{{ tt('Nothing') }}</span>
                    </p>
                </v-card-text>

                <v-card-text class="pt-0">
                    <v-table density="comfortable" v-if="visibleEntries.length">
                        <thead>
                            <tr>
                                <th class="debt-entry-select">
                                    <v-checkbox density="compact" hide-details
                                                :disabled="updating || !selectableEntries.length"
                                                :indeterminate="someSelected"
                                                v-model="allSelected">
                                        <v-tooltip activator="parent">{{ allSelected ? tt('Select None') : tt('Select All') }}</v-tooltip>
                                    </v-checkbox>
                                </th>
                                <th>{{ tt('Description') }}</th>
                                <th>{{ tt('Transaction Time') }}</th>
                                <th class="text-end">{{ tt('Amount') }}</th>
                                <th></th>
                            </tr>
                        </thead>
                        <tbody>
                            <template :key="row.key" v-for="row in visibleRows">
                            <tr class="debt-entry-group-row cursor-pointer"
                                :class="`debt-entry-depth-${row.depth}`"
                                v-if="row.group"
                                @click="toggleGroup(row.group)">
                                <td class="debt-entry-select">
                                    <v-checkbox density="compact" hide-details
                                                :disabled="updating || !row.group.openEntryIds.length"
                                                :indeterminate="isGroupPartlySelected(row.group)"
                                                :model-value="isGroupSelected(row.group)"
                                                @click.stop
                                                @update:model-value="(selected: boolean | null) => selectGroup(row.group!, !!selected)"></v-checkbox>
                                </td>
                                <td>
                                    <div class="d-flex align-center">
                                        <v-icon size="20" :icon="isGroupExpanded(row.group) ? mdiChevronDown : mdiChevronRight"></v-icon>
                                        <v-icon class="ms-1" size="20" :icon="row.group.kind === 'receipt' ? mdiReceiptTextOutline : mdiFormatListBulletedSquare"></v-icon>
                                        <span class="ms-2 font-weight-medium"
                                              :class="{ 'text-medium-emphasis': !isGroupNamed(row) }">
                                            {{ getGroupTitle(row) }}
                                        </span>
                                        <v-chip class="ms-2" size="x-small" label>{{ getGroupCount(row) }}</v-chip>
                                    </div>
                                    <div class="text-caption text-medium-emphasis" v-if="getGroupContext(row)">{{ getGroupContext(row) }}</div>
                                </td>
                                <td class="text-no-wrap">{{ getDisplayTime(row.entry.time) }}</td>
                                <td class="text-end text-no-wrap font-weight-medium">{{ getDisplayAmount(row.group.totalAmount, row.group.currency) }}</td>
                                <td></td>
                            </tr>
                            <tr :class="[`debt-entry-depth-${row.depth}`, { 'text-medium-emphasis': row.entry.settled }]"
                                v-else-if="!row.group">
                                <td class="debt-entry-select">
                                    <v-checkbox density="compact" hide-details
                                                :disabled="updating"
                                                :value="row.entry.id"
                                                v-model="selectedEntryIds"></v-checkbox>
                                </td>
                                <td>
                                    <div class="d-flex align-center">
                                        <span :class="{ 'cursor-pointer': !row.entry.manual }" @click="showTransaction(row.entry)">{{ getEntryDescription(row.entry) }}</span>
                                        <v-chip class="ms-2" size="x-small" label v-if="row.entry.manual">{{ tt('By Hand') }}</v-chip>
                                        <v-chip class="ms-2" size="x-small" label v-if="row.entry.settled">{{ tt('Settled') }}</v-chip>
                                        <v-chip class="ms-2" size="x-small" label color="warning" v-if="row.entry.missing">{{ tt('Transaction Deleted') }}</v-chip>
                                    </div>
                                    <div class="text-caption text-medium-emphasis" v-if="getEntryContext(row)">{{ getEntryContext(row) }}</div>
                                </td>
                                <td class="text-no-wrap">{{ getDisplayTime(row.entry.time) }}</td>
                                <td class="text-end text-no-wrap">{{ getDisplayAmount(row.entry.amount, row.entry.currency) }}</td>
                                <td class="text-end">
                                    <v-btn density="comfortable" color="default" variant="text" :icon="true"
                                           :disabled="updating">
                                        <v-icon :icon="mdiDotsVertical"/>
                                        <v-menu activator="parent">
                                            <v-list>
                                                <v-list-item :prepend-icon="mdiPencilOutline"
                                                             :title="tt('Change Amount Owed')"
                                                             v-if="!row.entry.settled"
                                                             @click="changeAmount(row.entry)"></v-list-item>
                                                <v-list-item :prepend-icon="mdiRenameOutline"
                                                             :title="tt('Rename')"
                                                             v-if="row.entry.manual && !row.entry.settled"
                                                             @click="renameEntry(row.entry)"></v-list-item>
                                                <v-list-item :prepend-icon="mdiUndoVariant"
                                                             :title="tt('Put Back on the Bill')"
                                                             v-if="row.entry.settled"
                                                             @click="reopen(row.entry)"></v-list-item>
                                                <v-list-item class="text-error" :prepend-icon="mdiDeleteOutline"
                                                             :title="tt('Detach')"
                                                             @click="detach(row.entry)"></v-list-item>
                                            </v-list>
                                        </v-menu>
                                    </v-btn>
                                </td>
                            </tr>
                            </template>
                        </tbody>
                    </v-table>

                    <span class="text-body-1" v-else-if="!visibleEntries.length && !loadingEntries">
                        {{ tt('This person owes you nothing. Open a transaction and attach it, or one of its positions, on its Owed By tab.') }}
                    </span>

                    <v-skeleton-loader type="paragraph" :loading="true" v-else-if="!visibleEntries.length && loadingEntries"></v-skeleton-loader>
                </v-card-text>

                <v-card-text class="pt-0" v-if="visibleEntries.length">
                    <div class="d-flex align-center flex-wrap gap-4">
                        <span class="text-body-1" v-if="selectedOpenEntries.length">
                            {{ tt('format.misc.debtSelectedCountAndAmount', { count: displaySelectedCount, amount: getDisplayAmounts(selectedTotals) }) }}
                        </span>
                        <span class="text-body-1 text-medium-emphasis" v-else-if="!selectedOpenEntries.length">
                            {{ tt('Tick what has been paid back, or what is no longer owed') }}
                        </span>
                        <v-spacer/>
                        <v-btn color="error" variant="tonal"
                               :disabled="!selectedEntries.length || updating" @click="detachSelected">
                            {{ tt('Detach') }}
                            <v-tooltip activator="parent" open-delay="500">{{ tt('Stop these being owed, without recording any payment') }}</v-tooltip>
                        </v-btn>
                        <v-btn :disabled="!selectedOpenEntries.length || updating" @click="recordRepayment">
                            {{ tt('Record Repayment') }}
                            <v-progress-circular indeterminate size="20" class="ms-2" v-if="updating"></v-progress-circular>
                        </v-btn>
                    </div>
                </v-card-text>
            </v-card>

            <v-card v-else-if="!selectedPerson">
                <v-card-text>
                    <span class="text-body-1">{{ tt('Select somebody to see what they owe you.') }}</span>
                </v-card-text>
            </v-card>
        </v-col>
    </v-row>

    <edit-dialog ref="editDialog" :type="TransactionEditPageType.Transaction" />
    <add-loan-dialog ref="addLoanDialog" />
    <rename-dialog ref="renameDialog" />
    <amount-input-dialog ref="amountInputDialog" />
    <confirm-dialog ref="confirmDialog" />
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import AddLoanDialog from './dialogs/AddLoanDialog.vue';
import EditDialog from '@/views/desktop/transactions/list/dialogs/EditDialog.vue';
import RenameDialog from '@/components/desktop/RenameDialog.vue';
import AmountInputDialog from '@/components/desktop/AmountInputDialog.vue';
import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';

import { ref, computed, onMounted, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useDebtsStore } from '@/stores/debt.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';

import { TransactionType } from '@/core/transaction.ts';
import { TransactionEditPageType } from '@/views/base/transactions/TransactionEditPageBase.ts';

import type { DebtAmount, DebtEntryGroup, DebtEntryGroupKind, DebtEntryInfoResponse, DebtEntryRow, DebtPersonInfoResponse } from '@/models/debt.ts';
import { sumDebtAmountsByCurrency, groupDebtEntries } from '@/models/debt.ts';

import { KnownFileType } from '@/core/file.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { parseDateTimeFromUnixTime } from '@/lib/datetime.ts';
import { startDownloadFile } from '@/lib/ui/common.ts';

import {
    mdiRefresh,
    mdiPlus,
    mdiAccountOutline,
    mdiDotsVertical,
    mdiPencilOutline,
    mdiDeleteOutline,
    mdiUndoVariant,
    mdiRenameOutline,
    mdiHandCoinOutline,
    mdiEyeOutline,
    mdiEyeOffOutline,
    mdiChevronDown,
    mdiChevronRight,
    mdiReceiptTextOutline,
    mdiFormatListBulletedSquare,
    mdiFileExcelOutline
} from '@mdi/js';

type AddLoanDialogType = InstanceType<typeof AddLoanDialog>;
type EditDialogType = InstanceType<typeof EditDialog>;
type RenameDialogType = InstanceType<typeof RenameDialog>;
type AmountInputDialogType = InstanceType<typeof AmountInputDialog>;
type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;

// DebtEntryVisibleRow is one row as the table draws it: a row of the grouped list, told how deep
// it sits so that it can be indented, and what it sits under so that it does not repeat what the
// row above already says
interface DebtEntryVisibleRow extends DebtEntryRow {
    readonly depth: number;
    readonly parentKind?: DebtEntryGroupKind;
}

const { tt, formatNumberToLocalizedNumerals, formatAmountToLocalizedNumeralsWithCurrency, formatDateTimeToLongDateTime } = useI18n();

const debtsStore = useDebtsStore();
const transactionCategoriesStore = useTransactionCategoriesStore();

const addLoanDialog = useTemplateRef<AddLoanDialogType>('addLoanDialog');
const editDialog = useTemplateRef<EditDialogType>('editDialog');
const renameDialog = useTemplateRef<RenameDialogType>('renameDialog');
const amountInputDialog = useTemplateRef<AmountInputDialogType>('amountInputDialog');
const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');

const loadingPeople = ref<boolean>(true);
const loadingEntries = ref<boolean>(false);
const updating = ref<boolean>(false);
const exportingReceipt = ref<boolean>(false);
const showSettled = ref<boolean>(false);
const selectedPersonId = ref<string>('');
const selectedEntryIds = ref<string[]>([]);

const allPeople = computed<DebtPersonInfoResponse[]>(() => debtsStore.allPeople);
const selectedPerson = computed<DebtPersonInfoResponse | undefined>(() => debtsStore.allPeopleMap[selectedPersonId.value]);

const openEntries = computed<DebtEntryInfoResponse[]>(() => debtsStore.openEntries);
const visibleEntries = computed<DebtEntryInfoResponse[]>(() => showSettled.value ? debtsStore.currentEntries : debtsStore.openEntries);

const openTotals = computed<DebtAmount[]>(() => sumDebtAmountsByCurrency(openEntries.value));

// what everybody together still owes, added from the standing totals the server keeps for each
// person, so it counts the people whose debts are not on screen as well as the one who is
const totalOwed = computed<DebtAmount[]>(() => {
    const allOpenAmounts: DebtAmount[] = [];

    for (const person of allPeople.value) {
        for (const openAmount of person.openAmounts ?? []) {
            allOpenAmounts.push(openAmount);
        }
    }

    return sumDebtAmountsByCurrency(allOpenAmounts);
});

// only what is still open can be paid back, so a ticked settled row is ignored rather than counted
const selectedOpenEntries = computed<DebtEntryInfoResponse[]>(() => openEntries.value.filter(entry => selectedEntryIds.value.indexOf(entry.id) >= 0));

// everything ticked, whether it is still owed or already paid back. Detaching works on all of it,
// because a row put there by mistake is a mistake either way.
const selectedEntries = computed<DebtEntryInfoResponse[]>(() => visibleEntries.value.filter(entry => selectedEntryIds.value.indexOf(entry.id) >= 0));

// ticking the whole column means everything a repayment could cover, which is what is still open -
// a settled row is shown for the record and there is nothing left to do to it
const selectableEntries = computed<DebtEntryInfoResponse[]>(() => visibleEntries.value.filter(entry => !entry.settled));

const allSelected = computed<boolean>({
    get: () => selectableEntries.value.length > 0 && selectedOpenEntries.value.length >= selectableEntries.value.length,
    set: (selectAll: boolean) => {
        selectedEntryIds.value = selectAll ? selectableEntries.value.map(entry => entry.id) : [];
    }
});

const someSelected = computed<boolean>(() => selectedOpenEntries.value.length > 0 && !allSelected.value);

// what somebody owes is read the way it was bought: the positions picked out of one transaction
// stand under that transaction, and everything owed off one shopping trip stands under that trip,
// so a receipt of a dozen articles is one row until it is opened rather than filling the page
const entryRows = computed<DebtEntryRow[]>(() => groupDebtEntries(visibleEntries.value));

// the rows actually on screen, a group contributing what it opens to only while it is open. The
// flattening is done here rather than in the template because the groups nest and a table does not.
const visibleRows = computed<DebtEntryVisibleRow[]>(() => flattenRows(entryRows.value, 0, undefined));

const expandedGroupKeys = ref<Record<string, boolean>>({});

function flattenRows(rows: DebtEntryRow[], depth: number, parentKind: DebtEntryGroupKind | undefined): DebtEntryVisibleRow[] {
    const rowsOnScreen: DebtEntryVisibleRow[] = [];

    for (const row of rows) {
        rowsOnScreen.push({ ...row, depth: depth, parentKind: parentKind });

        if (row.group && isGroupExpanded(row.group)) {
            rowsOnScreen.push(...flattenRows(row.group.rows, depth + 1, row.group.kind));
        }
    }

    return rowsOnScreen;
}

function getGroupKey(group: DebtEntryGroup): string {
    return `${group.kind}_${group.id}`;
}

function isGroupExpanded(group: DebtEntryGroup): boolean {
    return !!expandedGroupKeys.value[getGroupKey(group)];
}

function toggleGroup(group: DebtEntryGroup): void {
    const key = getGroupKey(group);
    expandedGroupKeys.value[key] = !expandedGroupKeys.value[key];
}

function isGroupSelected(group: DebtEntryGroup): boolean {
    if (!group.openEntryIds.length) {
        return false;
    }

    return group.openEntryIds.every(entryId => selectedEntryIds.value.indexOf(entryId) >= 0);
}

function isGroupPartlySelected(group: DebtEntryGroup): boolean {
    return !isGroupSelected(group) && group.openEntryIds.some(entryId => selectedEntryIds.value.indexOf(entryId) >= 0);
}

// ticking a group ticks everything still open under it, however deeply - a trip is paid back as the
// trip it was, and a transaction as all of the articles of it somebody is to pay for
function selectGroup(group: DebtEntryGroup, selected: boolean): void {
    if (selected) {
        const missingIds = group.openEntryIds.filter(entryId => selectedEntryIds.value.indexOf(entryId) < 0);
        selectedEntryIds.value = selectedEntryIds.value.concat(missingIds);
    } else {
        selectedEntryIds.value = selectedEntryIds.value.filter(entryId => group.openEntryIds.indexOf(entryId) < 0);
    }
}
const selectedTotals = computed<DebtAmount[]>(() => sumDebtAmountsByCurrency(selectedOpenEntries.value));
const displaySelectedCount = computed<string>(() => formatNumberToLocalizedNumerals(selectedOpenEntries.value.length));

function getDisplayAmount(amount: number, currency: string): string {
    return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(amount), currency);
}

function getDisplayAmounts(amounts: DebtAmount[]): string {
    return amounts.map(amount => getDisplayAmount(amount.amount, amount.currency)).join(' + ');
}

function getDisplayOpenAmounts(person: DebtPersonInfoResponse): string {
    return getDisplayAmounts(person.openAmounts ?? []);
}

function getDisplayTime(unixTime: number): string {
    return formatDateTimeToLongDateTime(parseDateTimeFromUnixTime(unixTime));
}

// a whole transaction is named by its category, which is what it is called everywhere else, and a
// transaction whose category has left the ledger by what it is
function getCategoryName(categoryId: string | undefined): string {
    if (categoryId) {
        const category = transactionCategoriesStore.allTransactionCategoriesMap[categoryId];

        if (category) {
            return category.name;
        }
    }

    return tt('Transaction');
}

// a position is named by what the receipt called it, and a whole transaction by its category - the
// same two names they are shown under everywhere else
function getEntryDescription(entry: DebtEntryInfoResponse): string {
    if (entry.name) {
        return entry.name;
    }

    return getCategoryName(entry.categoryId);
}

// a thing owed says where it was bought and what the transaction was for, unless the row it stands
// under has just said so - the positions of one transaction must not each repeat its article list
function getEntryContext(row: DebtEntryVisibleRow): string {
    if (row.parentKind === 'transaction') {
        return '';
    }

    const context: string[] = [];

    if (row.entry.merchantName) {
        context.push(row.entry.merchantName);
    }

    if (row.entry.comment) {
        context.push(row.entry.comment);
    }

    return context.join(' · ');
}

// a trip is called after the shop it was to, and the positions of one transaction after the
// category they were summed into
function getGroupTitle(row: DebtEntryVisibleRow): string {
    if (!row.group) {
        return '';
    }

    if (row.group.kind === 'receipt') {
        return row.group.merchantName || tt('Receipt');
    }

    return getCategoryName(row.entry.categoryId);
}

// a group that has nothing of its own to be called by is shown faintly, so that the word standing
// in for the name does not read as one
function isGroupNamed(row: DebtEntryVisibleRow): boolean {
    if (!row.group) {
        return false;
    }

    if (row.group.kind === 'receipt') {
        return !!row.group.merchantName;
    }

    return !!row.entry.categoryId;
}

// the chip counts the things owed under the group, however deeply they are nested, because that is
// what the group's total is the sum of
function getGroupCount(row: DebtEntryVisibleRow): string {
    if (!row.group) {
        return '';
    }

    const count = formatNumberToLocalizedNumerals(row.group.entries.length);

    if (row.group.kind === 'receipt') {
        return tt('format.misc.receiptLineItemCount', { count: count });
    }

    return tt('format.misc.debtPositionCount', { count: count });
}

// a group of positions standing on its own says where it was bought, because there is no trip above
// it to say so. Inside a trip that would only repeat the row above.
function getGroupContext(row: DebtEntryVisibleRow): string {
    if (!row.group || row.group.kind !== 'transaction' || row.parentKind) {
        return '';
    }

    return row.entry.merchantName ?? '';
}

// makeReceipt downloads what this person still owes as a spreadsheet, so that they can be handed a
// bill that says what they are paying for and when it was bought.
//
// Only what is still open goes on it, whether or not the page is showing what has been settled - a
// receipt is what is left to pay, and a paid row on one is an invitation to pay it twice.
function makeReceipt(): void {
    const person = selectedPerson.value;

    if (!person || exportingReceipt.value) {
        return;
    }

    exportingReceipt.value = true;

    debtsStore.exportReceipt({ personId: person.id }).then(receipt => {
        exportingReceipt.value = false;

        startDownloadFile(receipt.fileName || KnownFileType.XLSX.formatFileName(tt('Receipt')), receipt.content);
    }).catch(error => {
        exportingReceipt.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function reload(force: boolean): void {
    loadingPeople.value = true;

    Promise.all([
        debtsStore.loadAllPeople({ force: force }),
        transactionCategoriesStore.loadAllCategories({ force: false })
    ]).then(() => {
        loadingPeople.value = false;

        if (!selectedPersonId.value && allPeople.value.length) {
            selectPerson(allPeople.value[0]?.id ?? '');
        } else if (selectedPersonId.value) {
            loadEntries();
        }
    }).catch(error => {
        loadingPeople.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function loadEntries(): void {
    if (!selectedPersonId.value) {
        return;
    }

    loadingEntries.value = true;

    debtsStore.loadEntries({
        personId: selectedPersonId.value,
        includeSettled: showSettled.value
    }).then(() => {
        loadingEntries.value = false;
    }).catch(error => {
        loadingEntries.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function selectPerson(personId: string): void {
    selectedPersonId.value = personId;
    selectedEntryIds.value = [];
    loadEntries();
}

function toggleShowSettled(): void {
    showSettled.value = !showSettled.value;
    loadEntries();
}

function addPerson(): void {
    renameDialog.value?.open('', tt('New Person Name')).then((newName: string) => {
        updating.value = true;

        debtsStore.addPerson({ name: newName }).then(person => {
            updating.value = false;
            selectPerson(person.id);
        }).catch(error => {
            updating.value = false;

            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    });
}

function renamePerson(person: DebtPersonInfoResponse): void {
    renameDialog.value?.open(person.name, tt('Rename')).then((newName: string) => {
        updating.value = true;

        debtsStore.renamePerson({ id: person.id, name: newName }).then(() => {
            updating.value = false;
            reload(true);
        }).catch(error => {
            updating.value = false;

            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    });
}

function removePerson(person: DebtPersonInfoResponse): void {
    confirmDialog.value?.open('Are you sure you want to delete this person? Everything they owe will be forgotten, but no transaction will be deleted.').then(() => {
        updating.value = true;

        debtsStore.deletePerson({ id: person.id }).then(() => {
            updating.value = false;

            if (selectedPersonId.value === person.id) {
                selectedPersonId.value = '';
                selectedEntryIds.value = [];
            }
        }).catch(error => {
            updating.value = false;

            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    });
}

function changeAmount(entry: DebtEntryInfoResponse): void {
    amountInputDialog.value?.open({
        title: 'Change Amount Owed',
        currency: entry.currency,
        initAmount: entry.amount
    }).then((amount: number) => {
        if (amount === entry.amount) {
            return;
        }

        updating.value = true;

        debtsStore.modifyEntry({ id: entry.id, amount: amount }).then(() => {
            updating.value = false;
            reload(true);
        }).catch(error => {
            updating.value = false;

            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    });
}

function addLoan(): void {
    if (!selectedPersonId.value) {
        return;
    }

    addLoanDialog.value?.open(selectedPersonId.value).then(() => {
        reload(true);
    }).catch(() => {
        // the dialog rejects when it is closed without saving, which is not an error here
    });
}

function renameEntry(entry: DebtEntryInfoResponse): void {
    renameDialog.value?.open(entry.name ?? '', tt('Rename')).then((newName: string) => {
        updating.value = true;

        debtsStore.modifyEntry({ id: entry.id, amount: entry.amount, description: newName }).then(() => {
            updating.value = false;
            reload(true);
        }).catch(error => {
            updating.value = false;

            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    });
}

// detachSelected takes things off somebody's bill without any money changing hands - bought for
// them and then not, counted twice, or simply forgiven. The transactions stay exactly as they are.
function detachSelected(): void {
    if (!selectedEntries.value.length) {
        return;
    }

    const entryIds = selectedEntries.value.map(entry => entry.id);

    confirmDialog.value?.open('Are you sure you want to detach the ticked things? They will no longer be owed, and the transactions themselves stay as they are.').then(() => {
        updating.value = true;

        debtsStore.deleteEntries({ ids: entryIds }).then(() => {
            updating.value = false;
            selectedEntryIds.value = [];
            reload(true);

            snackbar.value?.showMessage('These are no longer owed');
        }).catch(error => {
            updating.value = false;

            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    });
}

function detach(entry: DebtEntryInfoResponse): void {
    updating.value = true;

    debtsStore.deleteEntries({ ids: [entry.id] }).then(() => {
        updating.value = false;
        selectedEntryIds.value = selectedEntryIds.value.filter(id => id !== entry.id);
        reload(true);
    }).catch(error => {
        updating.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function reopen(entry: DebtEntryInfoResponse): void {
    updating.value = true;

    debtsStore.reopenEntries({ ids: [entry.id] }).then(() => {
        updating.value = false;
        reload(true);
    }).catch(error => {
        updating.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function showTransaction(entry: DebtEntryInfoResponse): void {
    // a debt entered by hand has no transaction to show - it is all there is of itself
    if (entry.manual || entry.missing) {
        return;
    }

    editDialog.value?.open({
        id: entry.transactionId
    }).catch(() => {
        // the dialog rejects when it is closed without saving, which is not an error here
    });
}

// recordRepayment opens the ordinary transaction editor with the money coming back already filled
// in, and only marks the ticked things paid once that transaction has actually been written.
function recordRepayment(): void {
    if (!selectedOpenEntries.value.length || !selectedPerson.value) {
        return;
    }

    if (selectedTotals.value.length > 1) {
        snackbar.value?.showMessage('A repayment covers one currency at a time. Tick things owed in the same currency.');
        return;
    }

    const total = selectedTotals.value[0];

    if (!total) {
        return;
    }

    const settledEntryIds = selectedOpenEntries.value.map(entry => entry.id);
    const personName = selectedPerson.value.name;

    editDialog.value?.open({
        type: TransactionType.Income,
        amount: total.amount,
        comment: tt('format.misc.debtRepaymentComment', { name: personName }),
        noTransactionDraft: true
    }).then(result => {
        if (!result || !result.transactionId) {
            return;
        }

        updating.value = true;

        debtsStore.settleEntries({
            ids: settledEntryIds,
            settlementTransactionId: result.transactionId
        }).then(() => {
            updating.value = false;
            selectedEntryIds.value = [];
            reload(true);

            snackbar.value?.showMessage('This has been paid back');
        }).catch(error => {
            updating.value = false;

            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    }).catch(() => {
        // the editor rejects when it is closed without saving, which simply means no repayment was
        // recorded and nothing is to be settled
    });
}

onMounted(() => {
    reload(false);
});
</script>

<style>
.debt-entry-select {
    width: 40px;
}

.debt-entry-select .v-selection-control {
    min-height: unset;
}

.v-table .debt-entry-depth-1 > td:nth-child(2) {
    padding-inline-start: 32px;
}

.v-table .debt-entry-depth-2 > td:nth-child(2) {
    padding-inline-start: 64px;
}
</style>
