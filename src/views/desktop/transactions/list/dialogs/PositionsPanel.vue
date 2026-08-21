<template>
    <div class="transaction-positions mt-2">
        <div class="d-flex align-center px-2 pb-1 text-caption text-medium-emphasis">
            <span class="flex-grow-1">{{ tt('Positions') }}</span>
            <span class="ms-4 text-no-wrap" v-if="positions.length">
                {{ tt('format.misc.receiptLineItemCount', { count: formatNumberToLocalizedNumerals(positions.length) }) }}
            </span>
        </div>

        <v-divider/>

        <div class="transaction-position d-flex align-center px-2 py-1"
             :key="position.key" v-for="position in positions">
            <v-text-field class="flex-grow-1" type="text" density="compact" hide-details
                          maxlength="255"
                          :disabled="submitting"
                          :placeholder="tt('What was bought')"
                          v-model="position.name"/>
            <amount-input class="transaction-position-amount ms-2" density="compact"
                          :currency="currency"
                          :show-currency="true"
                          :disabled="submitting"
                          :enable-formula="true"
                          v-model="position.amount"/>
            <v-btn class="ms-1" density="comfortable" color="default" variant="text" :icon="true"
                   :disabled="submitting" @click="removePosition(position)">
                <v-icon :icon="mdiDeleteOutline"/>
                <v-tooltip activator="parent">{{ tt('Remove') }}</v-tooltip>
            </v-btn>
        </div>

        <div class="d-flex align-center px-2 py-1">
            <v-btn density="comfortable" variant="text" :prepend-icon="mdiPlus"
                   :disabled="submitting" @click="addPosition">
                {{ tt('Add a Position') }}
            </v-btn>
        </div>

        <v-divider v-if="positions.length"/>

        <div class="d-flex align-center px-2 pt-1 font-weight-medium" v-if="positions.length">
            <span class="flex-grow-1">{{ tt('Total Amount') }}</span>
            <span class="ms-4 text-no-wrap">{{ getDisplayAmount(totalAmount) }}</span>
        </div>

        <div class="px-2 pt-1 text-caption text-error" v-if="positions.length && !matchesTransactionAmount">
            {{ tt('These positions do not add up to the amount of this transaction.') }}
        </div>

        <div class="px-2 pt-2 text-caption text-medium-emphasis" v-if="!positions.length">
            {{ tt('Say what this transaction was made up of, and any one part of it can then be owed on its own.') }}
        </div>

        <div class="d-flex align-center px-2 pt-2">
            <v-spacer/>
            <v-btn density="comfortable" variant="tonal" color="default"
                   :disabled="!dirty || submitting" @click="revert">
                {{ tt('Revert') }}
            </v-btn>
            <v-btn class="ms-2" density="comfortable"
                   :disabled="!dirty || !everyPositionNamed || submitting" @click="save">
                {{ tt('Save') }}
                <v-progress-circular indeterminate size="20" class="ms-2" v-if="submitting"></v-progress-circular>
            </v-btn>
        </div>

        <div class="px-2 pt-1 text-caption text-medium-emphasis text-end" v-if="dirty && !everyPositionNamed">
            {{ tt('Every position needs a name.') }}
        </div>
    </div>
</template>

<script setup lang="ts">
import AmountInput from '@/components/desktop/AmountInput.vue';

import { ref, computed, watch, onMounted } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useTransactionsStore } from '@/stores/transaction.ts';

import type { ErrorResponse } from '@/core/api.ts';
import type { TransactionReceiptLineItem, TransactionReceiptLineItemModifyItem } from '@/models/transaction.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';

import {
    mdiPlus,
    mdiDeleteOutline
} from '@mdi/js';

// EditablePosition is one position as it is being written. The key is what keeps a row identified
// while rows are added and removed around it, which an id cannot do: a position being added has no
// id until it has been saved, and two unsaved ones would otherwise be indistinguishable.
interface EditablePosition {
    key: number;
    id?: string;
    name: string;
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
    (e: 'saved', lineItems: TransactionReceiptLineItem[]): void;
}>();

const { tt, formatNumberToLocalizedNumerals, formatAmountToLocalizedNumeralsWithCurrency } = useI18n();

const transactionsStore = useTransactionsStore();

const positions = ref<EditablePosition[]>([]);
const submitting = ref<boolean>(false);

let nextPositionKey = 0;

const totalAmount = computed<number>(() => {
    let total = 0;

    for (const position of positions.value) {
        total += position.amount;
    }

    return total;
});

// what the positions come to is shown against the transaction rather than made to agree with it. An
// itemization written by hand is short of the total until the last line is in, and saying so is more
// use than refusing to save it.
const matchesTransactionAmount = computed<boolean>(() => totalAmount.value === props.amount);

const everyPositionNamed = computed<boolean>(() => positions.value.every(position => !!position.name.trim()));

// dirty compares what is on screen with what was last read back from the server, so that Save is
// offered only when there is something to save and Revert only when there is something to undo
const dirty = computed<boolean>(() => {
    if (positions.value.length !== props.lineItems.length) {
        return true;
    }

    for (let i = 0; i < positions.value.length; i++) {
        const position = positions.value[i] as EditablePosition;
        const lineItem = props.lineItems[i] as TransactionReceiptLineItem;

        if (position.id !== lineItem.id || position.name !== lineItem.name || position.amount !== lineItem.amount) {
            return true;
        }
    }

    return false;
});

function getDisplayAmount(amount: number): string {
    return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(amount), props.currency);
}

function toPositions(lineItems: readonly TransactionReceiptLineItem[]): EditablePosition[] {
    return lineItems.map(lineItem => ({
        key: nextPositionKey++,
        id: lineItem.id,
        name: lineItem.name,
        amount: lineItem.amount
    }));
}

function addPosition(): void {
    // a new position starts at what the transaction is still short of, because the last article of
    // an itemization is nearly always exactly that, and at nothing once the total is accounted for
    const remainingAmount = props.amount - totalAmount.value;

    positions.value.push({
        key: nextPositionKey++,
        name: '',
        amount: remainingAmount > 0 ? remainingAmount : 0
    });
}

function removePosition(position: EditablePosition): void {
    positions.value = positions.value.filter(existedPosition => existedPosition.key !== position.key);
}

function revert(): void {
    positions.value = toPositions(props.lineItems);
}

function save(): void {
    if (!dirty.value || !everyPositionNamed.value || submitting.value) {
        return;
    }

    const lineItems: TransactionReceiptLineItemModifyItem[] = positions.value.map(position => ({
        id: position.id,
        name: position.name.trim(),
        amount: position.amount
    }));

    submitting.value = true;

    transactionsStore.modifyTransactionLineItems({
        transactionId: props.transactionId,
        lineItems: lineItems
    }).then(savedLineItems => {
        submitting.value = false;
        positions.value = toPositions(savedLineItems);

        emit('saved', savedLineItems);
        emit('message', 'The positions have been saved');
    }).catch(error => {
        submitting.value = false;

        if (!error.processed) {
            emit('error', error);
        }
    });
}

watch(() => props.lineItems, () => {
    positions.value = toPositions(props.lineItems);
});

onMounted(() => {
    positions.value = toPositions(props.lineItems);
});
</script>

<style>
.transaction-position-amount {
    max-width: 160px;
}
</style>
