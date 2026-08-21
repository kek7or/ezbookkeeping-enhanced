<template>
    <div class="transaction-owed-by mt-2">
        <div class="d-flex align-center px-2 pb-1">
            <v-select class="flex-grow-1" density="compact" hide-details persistent-placeholder
                      multiple chips closable-chips
                      item-title="name" item-value="id"
                      :disabled="loading || submitting"
                      :label="tt('Owed By')"
                      :placeholder="tt('Nobody')"
                      :items="allPeople"
                      v-model="selectedPersonIds">
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
                {{ getEntryChipText(entry) }}
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
                    {{ getEntryChipText(entry) }}
                </v-chip>
                <span class="ms-4 text-no-wrap">{{ getDisplayAmount(lineItem.amount) }}</span>
            </div>
        </template>

        <v-divider/>

        <div class="d-flex align-center px-2 pt-2" v-if="isSplit">
            <v-checkbox class="flex-grow-0" density="compact" hide-details
                        :disabled="loading || submitting"
                        :label="tt('Count me in the split')"
                        v-model="includeMyShare"></v-checkbox>
        </div>

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

        <div class="px-2 pt-1 text-caption text-medium-emphasis" v-if="hasSelection && sharePreview">
            {{ sharePreview }}
        </div>

        <div class="px-2 pt-2 text-caption text-medium-emphasis" v-if="!lineItems.length">
            {{ tt('This transaction has no positions yet. Itemize it on the Positions tab, and any one part of it can then be owed on its own.') }}
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
import type { DebtEntryCreateRequest, DebtEntryInfoResponse, DebtPersonInfoResponse } from '@/models/debt.ts';

import { splitAmountEvenly } from '@/models/debt.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';

import {
    mdiAccountPlusOutline
} from '@mdi/js';

type RenameDialogType = InstanceType<typeof RenameDialog>;

// AttachTarget is one thing that can be owed: the transaction as a whole, or one of its positions
interface AttachTarget {
    lineItemId?: string;
    amount: number;
}

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
const selectedPersonIds = ref<string[]>([]);
const wholeTransactionSelected = ref<boolean>(false);
const selectedLineItemIds = ref<string[]>([]);
// what everybody ate is not owed by everybody else alone - the one who paid had their share of it
// too, and counting them in is what stops the friends being charged for the whole dish
const includeMyShare = ref<boolean>(true);

const allPeople = computed<DebtPersonInfoResponse[]>(() => debtsStore.allPeople);

// more than one person can be on the same thing - a fare split three ways is three entries against
// one transaction, each for the share that person is to pay
const wholeTransactionEntries = computed<DebtEntryInfoResponse[]>(() => entries.value.filter(entry => !entry.lineItemId));

// a position without an id was never written by the server and cannot be pointed at, which is the
// case for a transaction still being imported
const attachableLineItems = computed<TransactionReceiptLineItem[]>(() => props.lineItems.filter(lineItem => !!lineItem.id));

const hasSelection = computed<boolean>(() => wholeTransactionSelected.value || selectedLineItemIds.value.length > 0);

// one person owes the whole of what is ticked; two or more share it
const isSplit = computed<boolean>(() => selectedPersonIds.value.length > 1);

const canAttach = computed<boolean>(() => hasSelection.value && selectedPersonIds.value.length > 0);

// shareCount counts the payer as well when they are in on it, so that a dish split with two friends
// is a third each rather than a half each
const shareCount = computed<number>(() => selectedPersonIds.value.length + (isSplit.value && includeMyShare.value ? 1 : 0));

// the things that were ticked, each with what it cost. A position is split on its own rather than
// thrown into one pot, so every share still says which article it is a share of.
const selectedTargets = computed<AttachTarget[]>(() => {
    const targets: AttachTarget[] = [];

    if (wholeTransactionSelected.value) {
        targets.push({ amount: props.amount });
    }

    for (const lineItem of attachableLineItems.value) {
        if (lineItem.id && selectedLineItemIds.value.indexOf(lineItem.id) >= 0) {
            targets.push({ lineItemId: lineItem.id, amount: lineItem.amount });
        }
    }

    return targets;
});

const selectedAmount = computed<number>(() => {
    let total = 0;

    for (const target of selectedTargets.value) {
        total += target.amount;
    }

    return total;
});

// payerTakesAShare says the one who paid ate some of this too, so their share is simply kept and
// never written down as owed by anybody
const payerTakesAShare = computed<boolean>(() => isSplit.value && includeMyShare.value);

// what each person ends up owing, and what the payer keeps, added over everything that was ticked
const shares = computed<{ people: Record<string, number>, mine: number }>(() => {
    const people: Record<string, number> = {};
    let mine = 0;

    for (const personId of selectedPersonIds.value) {
        people[personId] = 0;
    }

    for (const target of selectedTargets.value) {
        const targetShares = splitAmountEvenly(target.amount, shareCount.value);

        if (payerTakesAShare.value) {
            mine += targetShares[0] ?? 0;
        }

        for (let i = 0; i < selectedPersonIds.value.length; i++) {
            const personId = selectedPersonIds.value[i] as string;
            const share = targetShares[payerTakesAShare.value ? i + 1 : i] ?? 0;
            people[personId] = (people[personId] ?? 0) + share;
        }
    }

    return { people: people, mine: mine };
});

const sharePreview = computed<string>(() => {
    if (!hasSelection.value || !selectedPersonIds.value.length) {
        return '';
    }

    const parts: string[] = [];

    for (const personId of selectedPersonIds.value) {
        parts.push(`${getPersonName(personId)} ${getDisplayAmount(shares.value.people[personId] ?? 0)}`);
    }

    if (payerTakesAShare.value) {
        parts.push(tt('format.misc.debtYourShare', { amount: getDisplayAmount(shares.value.mine) }));
    }

    return parts.join(' · ');
});

function getDisplayAmount(amount: number): string {
    return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(amount), props.currency);
}

function getPersonName(personId: string): string {
    return debtsStore.allPeopleMap[personId]?.name ?? '';
}

// a chip carries the amount as well as the name, because once a thing is shared out the interesting
// part is not that somebody owes for it but how much of it they owe
function getEntryChipText(entry: DebtEntryInfoResponse): string {
    return `${getPersonName(entry.personId)} ${getDisplayAmount(entry.amount)}`;
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

        // preselected only when there is nobody else it could be
        if (!selectedPersonIds.value.length && allPeople.value.length === 1) {
            selectedPersonIds.value = [allPeople.value[0]?.id ?? ''];
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
            selectedPersonIds.value = selectedPersonIds.value.concat([person.id]);
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

    const newEntries: DebtEntryCreateRequest[] = [];

    for (const target of selectedTargets.value) {
        const targetShares = splitAmountEvenly(target.amount, shareCount.value);

        for (let i = 0; i < selectedPersonIds.value.length; i++) {
            const share = targetShares[payerTakesAShare.value ? i + 1 : i] ?? 0;

            // a share of nothing is nobody's debt, and sending it as a zero would be read as
            // asking for the whole amount
            if (share <= 0) {
                continue;
            }

            newEntries.push({
                personId: selectedPersonIds.value[i] as string,
                transactionId: props.transactionId,
                lineItemId: target.lineItemId,
                amount: share
            });
        }
    }

    if (!newEntries.length) {
        emit('message', 'There is nothing to attach');
        return;
    }

    const splitBetweenPeople = isSplit.value;

    submitting.value = true;

    debtsStore.attachEntries({
        entries: newEntries
    }).then(createdEntries => {
        submitting.value = false;
        entries.value = entries.value.concat(createdEntries);
        wholeTransactionSelected.value = false;
        selectedLineItemIds.value = [];

        emit('message', splitBetweenPeople ? 'This has been split between them' : 'This is now owed by this person');
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
