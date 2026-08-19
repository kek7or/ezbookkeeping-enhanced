<template>
    <v-dialog width="500" :persistent="true" v-model="showState">
        <v-card class="pa-sm-1 pa-md-2">
            <template #title>
                <h4 class="text-h4 text-wrap">{{ tt('Rename Receipt') }}</h4>
            </template>
            <v-card-text class="pb-2 text-body-2 text-medium-emphasis">
                {{ tt('What this shopping is called in the transaction list') }}
            </v-card-text>
            <v-card-text class="w-100 d-flex justify-center">
                <div class="w-100">
                    <v-text-field
                        autofocus
                        variant="underlined"
                        :disabled="saving"
                        :maxlength="TRANSACTION_RECEIPT_MAX_MERCHANT_NAME_LENGTH"
                        :placeholder="tt('Name this receipt')"
                        v-model="merchantName"
                        @keyup.enter="confirm"
                    />
                </div>
            </v-card-text>
            <v-card-text>
                <div class="w-100 d-flex justify-center flex-wrap mt-sm-1 mt-md-2 gap-4">
                    <v-btn color="primary" :disabled="saving" @click="confirm">
                        {{ tt('Save') }}
                        <v-progress-circular indeterminate size="22" class="ms-2" v-if="saving"></v-progress-circular>
                    </v-btn>
                    <v-btn color="secondary" variant="tonal" :disabled="saving" @click="cancel">{{ tt('Cancel') }}</v-btn>
                </div>
            </v-card-text>
        </v-card>
    </v-dialog>

    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import SnackBar from '@/components/desktop/SnackBar.vue';

import { ref, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useTransactionsStore } from '@/stores/transaction.ts';

import { TRANSACTION_RECEIPT_MAX_MERCHANT_NAME_LENGTH } from '@/consts/transaction.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt } = useI18n();

const transactionsStore = useTransactionsStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

const showState = ref<boolean>(false);
const saving = ref<boolean>(false);
const receiptId = ref<string>('');
const merchantName = ref<string>('');
const originalMerchantName = ref<string>('');

let resolveFunc: ((renamed: boolean) => void) | null = null;
let rejectFunc: (() => void) | null = null;

function open(options: { receiptId: string, merchantName: string }): Promise<boolean> {
    receiptId.value = options.receiptId;
    merchantName.value = options.merchantName;
    originalMerchantName.value = options.merchantName;
    saving.value = false;
    showState.value = true;

    return new Promise((resolve, reject) => {
        resolveFunc = resolve;
        rejectFunc = reject;
    });
}

function confirm(): void {
    const newMerchantName = merchantName.value.trim();

    // saving a name that is already the name is a request that would change nothing, so it is closed
    // as what it is - the user having decided to leave it alone
    if (newMerchantName === originalMerchantName.value) {
        showState.value = false;
        resolveFunc?.(false);
        return;
    }

    saving.value = true;

    transactionsStore.modifyTransactionReceipt({
        receiptId: receiptId.value,
        merchantName: newMerchantName
    }).then(() => {
        saving.value = false;
        showState.value = false;
        resolveFunc?.(true);
    }).catch(error => {
        saving.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function cancel(): void {
    rejectFunc?.();
    showState.value = false;
}

defineExpose({
    open
});
</script>
