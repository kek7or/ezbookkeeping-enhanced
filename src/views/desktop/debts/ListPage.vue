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

                <v-card-text class="pt-0">
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
                                <th class="debt-entry-select"></th>
                                <th>{{ tt('Description') }}</th>
                                <th>{{ tt('Transaction Time') }}</th>
                                <th class="text-end">{{ tt('Amount') }}</th>
                                <th></th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr :key="entry.id" v-for="entry in visibleEntries" :class="{ 'text-medium-emphasis': entry.settled }">
                                <td class="debt-entry-select">
                                    <v-checkbox density="compact" hide-details
                                                :disabled="updating"
                                                :value="entry.id"
                                                v-model="selectedEntryIds"></v-checkbox>
                                </td>
                                <td>
                                    <div class="d-flex align-center">
                                        <span :class="{ 'cursor-pointer': !entry.manual }" @click="showTransaction(entry)">{{ getEntryDescription(entry) }}</span>
                                        <v-chip class="ms-2" size="x-small" label v-if="entry.manual">{{ tt('By Hand') }}</v-chip>
                                        <v-chip class="ms-2" size="x-small" label v-if="entry.settled">{{ tt('Settled') }}</v-chip>
                                        <v-chip class="ms-2" size="x-small" label color="warning" v-if="entry.missing">{{ tt('Transaction Deleted') }}</v-chip>
                                    </div>
                                    <div class="text-caption text-medium-emphasis" v-if="getEntryContext(entry)">{{ getEntryContext(entry) }}</div>
                                </td>
                                <td class="text-no-wrap">{{ getDisplayTime(entry.time) }}</td>
                                <td class="text-end text-no-wrap">{{ getDisplayAmount(entry.amount, entry.currency) }}</td>
                                <td class="text-end">
                                    <v-btn density="comfortable" color="default" variant="text" :icon="true"
                                           :disabled="updating">
                                        <v-icon :icon="mdiDotsVertical"/>
                                        <v-menu activator="parent">
                                            <v-list>
                                                <v-list-item :prepend-icon="mdiPencilOutline"
                                                             :title="tt('Change Amount Owed')"
                                                             v-if="!entry.settled"
                                                             @click="changeAmount(entry)"></v-list-item>
                                                <v-list-item :prepend-icon="mdiRenameOutline"
                                                             :title="tt('Rename')"
                                                             v-if="entry.manual && !entry.settled"
                                                             @click="renameEntry(entry)"></v-list-item>
                                                <v-list-item :prepend-icon="mdiUndoVariant"
                                                             :title="tt('Put Back on the Bill')"
                                                             v-if="entry.settled"
                                                             @click="reopen(entry)"></v-list-item>
                                                <v-list-item class="text-error" :prepend-icon="mdiDeleteOutline"
                                                             :title="tt('Detach')"
                                                             @click="detach(entry)"></v-list-item>
                                            </v-list>
                                        </v-menu>
                                    </v-btn>
                                </td>
                            </tr>
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
                            {{ tt('Tick what has been paid back') }}
                        </span>
                        <v-spacer/>
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

import type { DebtAmount, DebtEntryInfoResponse, DebtPersonInfoResponse } from '@/models/debt.ts';
import { sumDebtAmountsByCurrency } from '@/models/debt.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { parseDateTimeFromUnixTime } from '@/lib/datetime.ts';

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
    mdiEyeOffOutline
} from '@mdi/js';

type AddLoanDialogType = InstanceType<typeof AddLoanDialog>;
type EditDialogType = InstanceType<typeof EditDialog>;
type RenameDialogType = InstanceType<typeof RenameDialog>;
type AmountInputDialogType = InstanceType<typeof AmountInputDialog>;
type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;

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
const showSettled = ref<boolean>(false);
const selectedPersonId = ref<string>('');
const selectedEntryIds = ref<string[]>([]);

const allPeople = computed<DebtPersonInfoResponse[]>(() => debtsStore.allPeople);
const selectedPerson = computed<DebtPersonInfoResponse | undefined>(() => debtsStore.allPeopleMap[selectedPersonId.value]);

const openEntries = computed<DebtEntryInfoResponse[]>(() => debtsStore.openEntries);
const visibleEntries = computed<DebtEntryInfoResponse[]>(() => showSettled.value ? debtsStore.currentEntries : debtsStore.openEntries);

const openTotals = computed<DebtAmount[]>(() => sumDebtAmountsByCurrency(openEntries.value));

// only what is still open can be paid back, so a ticked settled row is ignored rather than counted
const selectedOpenEntries = computed<DebtEntryInfoResponse[]>(() => openEntries.value.filter(entry => selectedEntryIds.value.indexOf(entry.id) >= 0));
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

// a position is named by what the receipt called it, and a whole transaction by its category - the
// same two names they are shown under everywhere else
function getEntryDescription(entry: DebtEntryInfoResponse): string {
    if (entry.name) {
        return entry.name;
    }

    if (entry.categoryId) {
        const category = transactionCategoriesStore.allTransactionCategoriesMap[entry.categoryId];

        if (category) {
            return category.name;
        }
    }

    return tt('Transaction');
}

function getEntryContext(entry: DebtEntryInfoResponse): string {
    const context: string[] = [];

    if (entry.merchantName) {
        context.push(entry.merchantName);
    }

    if (entry.comment) {
        context.push(entry.comment);
    }

    return context.join(' · ');
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
</style>
