<template>
    <v-dialog width="600" v-model="showState">
        <v-card class="pa-sm-1 pa-md-2">
            <template #title>
                <h4 class="text-h4 text-wrap">{{ tt('Record a Loan') }}</h4>
            </template>
            <v-card-text>
                <p class="text-body-2 text-medium-emphasis mb-4">
                    {{ tt('For money that never passed through the ledger, such as cash you handed over.') }}
                </p>
                <v-row>
                    <v-col cols="12">
                        <v-text-field type="text" persistent-placeholder
                                      autofocus
                                      :disabled="submitting"
                                      :label="tt('Description')"
                                      :placeholder="tt('What was lent')"
                                      v-model="description"/>
                    </v-col>
                    <v-col cols="12" md="6">
                        <amount-input :currency="currency"
                                      :show-currency="true"
                                      :persistent-placeholder="true"
                                      :disabled="submitting"
                                      :label="tt('Amount')"
                                      :enable-formula="true"
                                      v-model="amount"/>
                    </v-col>
                    <v-col cols="12" md="6">
                        <currency-select :disabled="submitting"
                                         :label="tt('Currency')"
                                         v-model="currency"/>
                    </v-col>
                    <v-col cols="12">
                        <date-time-select :disabled="submitting"
                                          :label="tt('Time')"
                                          :timezone-utc-offset="utcOffset"
                                          v-model="time"
                                          @error="onDateTimeError"/>
                    </v-col>
                </v-row>
            </v-card-text>
            <v-card-text>
                <div class="w-100 d-flex justify-center gap-4">
                    <v-btn color="primary" :disabled="!isInputValid || submitting" @click="save">
                        {{ tt('Save') }}
                        <v-progress-circular indeterminate size="22" class="ms-2" v-if="submitting"></v-progress-circular>
                    </v-btn>
                    <v-btn color="secondary" variant="tonal" :disabled="submitting" @click="cancel">
                        {{ tt('Cancel') }}
                    </v-btn>
                </div>
            </v-card-text>
        </v-card>

        <snack-bar ref="snackbar" />
    </v-dialog>
</template>

<script setup lang="ts">
import AmountInput from '@/components/desktop/AmountInput.vue';
import CurrencySelect from '@/components/desktop/CurrencySelect.vue';
import DateTimeSelect from '@/components/desktop/DateTimeSelect.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';

import { ref, computed, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useUserStore } from '@/stores/user.ts';
import { useSettingsStore } from '@/stores/setting.ts';
import { useDebtsStore } from '@/stores/debt.ts';

import type { DebtEntryInfoResponse } from '@/models/debt.ts';

import { getCurrentUnixTime, getTimezoneOffsetMinutes } from '@/lib/datetime.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt } = useI18n();

const userStore = useUserStore();
const settingsStore = useSettingsStore();
const debtsStore = useDebtsStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

const showState = ref<boolean>(false);
const submitting = ref<boolean>(false);
const personId = ref<string>('');
const description = ref<string>('');
const amount = ref<number>(0);
const currency = ref<string>('');
const time = ref<number>(0);
const utcOffset = ref<number>(0);

let resolveFunc: ((entry: DebtEntryInfoResponse) => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const isInputValid = computed<boolean>(() => !!description.value.trim() && amount.value > 0 && !!currency.value && time.value > 0);

// a loan is opened with the currency the user keeps their books in, because that is the one they are
// most likely to have handed over
function open(openPersonId: string): Promise<DebtEntryInfoResponse> {
    const now = getCurrentUnixTime();
    const timezone = settingsStore.appSettings.timeZone;

    showState.value = true;
    submitting.value = false;
    personId.value = openPersonId;
    description.value = '';
    amount.value = 0;
    currency.value = userStore.currentUserDefaultCurrency;
    time.value = now;
    utcOffset.value = getTimezoneOffsetMinutes(now, timezone);

    return new Promise((resolve, reject) => {
        resolveFunc = resolve;
        rejectFunc = reject;
    });
}

function save(): void {
    if (!isInputValid.value) {
        return;
    }

    submitting.value = true;

    debtsStore.addManualEntry({
        personId: personId.value,
        description: description.value.trim(),
        amount: amount.value,
        currency: currency.value,
        time: time.value
    }).then(entry => {
        submitting.value = false;
        showState.value = false;

        if (resolveFunc) {
            resolveFunc(entry);
        }
    }).catch(error => {
        submitting.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function cancel(): void {
    showState.value = false;

    if (rejectFunc) {
        rejectFunc();
    }
}

function onDateTimeError(message: string): void {
    snackbar.value?.showMessage(message);
}

defineExpose({
    open
});
</script>
