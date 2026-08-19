<template>
    <div class="transaction-owed-by mt-2">
        <div class="d-flex align-center px-2 pb-1">
            <v-select class="flex-grow-1" density="compact" hide-details persistent-placeholder
                      item-title="name" item-value="id"
                      :disabled="loading || submitting"
                      :label="tt('Owed By')"
                      :placeholder="tt('Nobody')"
                      :items="allPeople"
                      v-model="selectedPersonId">
            </v-select>
            <v-btn class="ms-2" density="comfortable" color="default" variant="text" :icon="true"
                   :disabled="loading || submitting" @click="addPerson">
                <v-icon :icon="mdiAccountPlusOutline"/>
                <v-tooltip activator="parent">{{ tt('Add Person') }}</v-tooltip>
            </v-btn>
        </div>

        <v-divider/>

        <div class="transaction-owed-by-row d-flex align-center px-2 py-1">
            <v-checkbox class="flex-grow-0" density="compact" hide-details
                        :disabled="loading || submitting"
                        v-model="wholeTransactionSelected"></v-checkbox>
            <span class="text-truncate flex-grow-1">{{ tt('The whole transaction') }}</span>
            <v-chip class="ms-2" size="small" label
                    :key="entry.id" v-for="entry in wholeTransactionEntries"
                    :closable="!submitting" @click:close="detach(entry)">
                {{ getPersonName(entry.personId) }}
            </v-chip>
            <span class="ms-4 text-no-wrap">{{ getDisplayAmount(amount) }}</span>
        </div>

        <template v-if="lineItems.length">
            <v-divider/>
            <div class="d-flex align-center px-2 py-1 text-caption text-medium-emphasis">
                <span class="flex-grow-1">{{ tt('Positions') }}</span>
            </div>
            <div class="transaction-owed-by-row d-flex align-center px-2 py-1"
                 :key="lineItem.id" v-for="lineItem in attachableLineItems">
                <v-checkbox class="flex-grow-0" density="compact" hide-details
                            :disabled="loading || submitting"
                            :value="lineItem.id"
                            v-model="selectedLineItemIds"></v-checkbox>
                <span class="text-truncate flex-grow-1">
                    {{ lineItem.name }}
                    <v-tooltip activator="parent" open-delay="500">{{ lineItem.name }}</v-tooltip>
                </span>
                <v-chip class="ms-2" size="small" label
                        :key="entry.id" v-for="entry in getLineItemEntries(lineItem.id)"
                        :closable="!submitting" @click:close="detach(entry)">
                    {{ getPersonName(entry.personId) }}
                </v-chip>
                <span class="ms-4 text-no-wrap">{{ getDisplayAmount(lineItem.amount) }}</span>
            </div>
        </template>

        <v-divider/>

        <div class="d-flex align-center px-2 pt-2">
            <span class="text-caption text-medium-emphasis flex-grow-1" v-if="!hasSelection">
                {{ tt('Tick what somebody else is to pay for') }}
            </span>
            <span class="text-caption text-medium-emphasis flex-grow-1" v-else-if="hasSelection">
                {{ tt('format.misc.debtSelectedAmount', { amount: getDisplayAmount(selectedAmount) }) }}
            </span>
            <v-btn density="comfortable" :disabled="!canAttach || loading || submitting" @click="attach">
                {{ tt('Attach') }}
                <v-progress-circular indeterminate size="20" class="ms-2" v-if="submitting"></v-progress-circular>
            </v-btn>
        </div>

        <div class="px-2 pt-2 text-caption text-medium-emphasis" v-if="!lineItems.length">
            {{ tt('This transaction has no positions, so only the whole of it can be owed.') }}
        </div>

        <rename-dialog ref="renameDialog" />
    </div>
</template>

<script setup lang="ts">
import RenameDialog from '@/components/desktop/RenameDialog.vue';

import { ref, computed, watch, onMounted, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useDebtsStore } from '@/stores/debt.ts';

import type { ErrorResponse } from '@/core/api.ts';
import type { TransactionReceiptLineItem } from '@/models/transaction.ts';
import type { DebtEntryInfoResponse, DebtPersonInfoResponse } from '@/models/debt.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';

import {
    mdiAccountPlusOutline
} from '@mdi/js';

type RenameDialogType = InstanceType<typeof RenameDialog>;

const props = defineProps<{
    transactionId: string;
    amount: number;
    currency: string;
    lineItems: TransactionReceiptLineItem[];
}>();

const emit = defineEmits<{
    (e: 'error', error: string | { message: string } | { error: ErrorResponse }): void;
    (e: 'message', message: string): void;
}>();

const { tt, formatAmountToLocalizedNumeralsWithCurrency } = useI18n();

const debtsStore = useDebtsStore();

const renameDialog = useTemplateRef<RenameDialogType>('renameDialog');

const loading = ref<boolean>(true);
const submitting = ref<boolean>(false);
const entries = ref<DebtEntryInfoResponse[]>([]);
const selectedPersonId = ref<string>('');
const wholeTransactionSelected = ref<boolean>(false);
const selectedLineItemIds = ref<string[]>([]);

const allPeople = computed<DebtPersonInfoResponse[]>(() => debtsStore.allPeople);

// more than one person can be on the same thing - a fare split three ways is three entries against
// one transaction, each for the share that person is to pay
const wholeTransactionEntries = computed<DebtEntryInfoResponse[]>(() => entries.value.filter(entry => !entry.lineItemId));

// a position without an id was never written by the server and cannot be pointed at, which is the
// case for a transaction still being imported
const attachableLineItems = computed<TransactionReceiptLineItem[]>(() => props.lineItems.filter(lineItem => !!lineItem.id));

const hasSelection = computed<boolean>(() => wholeTransactionSelected.value || selectedLineItemIds.value.length > 0);

const canAttach = computed<boolean>(() => hasSelection.value && !!selectedPersonId.value);

const selectedAmount = computed<number>(() => {
    if (wholeTransactionSelected.value) {
        return props.amount;
    }

    let total = 0;

    for (const lineItem of attachableLineItems.value) {
        if (lineItem.id && selectedLineItemIds.value.indexOf(lineItem.id) >= 0) {
            total += lineItem.amount;
        }
    }

    return total;
});

function getDisplayAmount(amount: number): string {
    return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(amount), props.currency);
}

function getPersonName(personId: string): string {
    return debtsStore.allPeopleMap[personId]?.name ?? '';
}

function getLineItemEntries(lineItemId?: string): DebtEntryInfoResponse[] {
    if (!lineItemId) {
        return [];
    }

    return entries.value.filter(entry => entry.lineItemId === lineItemId);
}

function reload(): void {
    if (!props.transactionId) {
        return;
    }

    loading.value = true;

    Promise.all([
        debtsStore.loadAllPeople({ force: false }),
        debtsStore.loadEntriesOfTransaction({ transactionId: props.transactionId })
    ]).then(([, transactionEntries]) => {
        entries.value = transactionEntries;
        loading.value = false;

        if (!selectedPersonId.value && allPeople.value.length) {
            selectedPersonId.value = allPeople.value[0]?.id ?? '';
        }
    }).catch(error => {
        loading.value = false;

        if (!error.processed) {
            emit('error', error);
        }
    });
}

function addPerson(): void {
    renameDialog.value?.open('', tt('New Person Name')).then((newName: string) => {
        submitting.value = true;

        debtsStore.addPerson({ name: newName }).then(person => {
            submitting.value = false;
            selectedPersonId.value = person.id;
        }).catch(error => {
            submitting.value = false;

            if (!error.processed) {
                emit('error', error);
            }
        });
    });
}

function attach(): void {
    if (!canAttach.value) {
        return;
    }

    const newEntries = [];

    if (wholeTransactionSelected.value) {
        newEntries.push({
            transactionId: props.transactionId
        });
    }

    for (const lineItemId of selectedLineItemIds.value) {
        newEntries.push({
            transactionId: props.transactionId,
            lineItemId: lineItemId
        });
    }

    submitting.value = true;

    debtsStore.attachEntries({
        personId: selectedPersonId.value,
        entries: newEntries
    }).then(createdEntries => {
        submitting.value = false;
        entries.value = entries.value.concat(createdEntries);
        wholeTransactionSelected.value = false;
        selectedLineItemIds.value = [];

        emit('message', 'This is now owed by this person');
    }).catch(error => {
        submitting.value = false;

        if (!error.processed) {
            emit('error', error);
        }
    });
}

function detach(entry: DebtEntryInfoResponse): void {
    submitting.value = true;

    debtsStore.deleteEntries({ ids: [entry.id] }).then(() => {
        submitting.value = false;
        entries.value = entries.value.filter(existedEntry => existedEntry.id !== entry.id);
    }).catch(error => {
        submitting.value = false;

        if (!error.processed) {
            emit('error', error);
        }
    });
}

watch(() => props.transactionId, () => {
    entries.value = [];
    wholeTransactionSelected.value = false;
    selectedLineItemIds.value = [];
    reload();
});

onMounted(() => {
    reload();
});
</script>

<style>
.transaction-owed-by-row .v-selection-control {
    min-height: unset;
}
</style>
