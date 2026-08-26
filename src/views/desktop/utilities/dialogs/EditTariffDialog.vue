<template>
    <v-dialog width="700" v-model="showState">
        <v-card class="pa-sm-1 pa-md-2">
            <template #title>
                <h4 class="text-h4 text-wrap">{{ editingTariffId ? tt('Edit Tariff') : tt('Add Tariff') }}</h4>
            </template>
            <v-card-text>
                <p class="text-body-2 text-medium-emphasis mb-4">
                    {{ tt('A tariff holds from the day it begins until the next one starts. Add a new one when the price changes instead of editing the old one, so that what came before is still priced the way it was paid for.') }}
                </p>
                <utility-tariff-fields :disabled="submitting"
                                       :currency="currency"
                                       :unit="unit"
                                       v-model="tariff"/>
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
import SnackBar from '@/components/desktop/SnackBar.vue';
import UtilityTariffFields from './UtilityTariffFields.vue';

import { ref, computed, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useUtilityMetersStore } from '@/stores/utilityMeter.ts';

import type { UtilityMeterInfoResponse, UtilityTariffInfoResponse } from '@/models/utility_meter.ts';
import { UtilityMeterUnit } from '@/models/utility_meter.ts';

import type { UtilityTariffFormValue } from '@/views/base/UtilityMetersPageBase.ts';
import {
    newUtilityTariffFormValue,
    toUtilityTariffFormValue,
    toUtilityTariffRequest,
    isUtilityTariffFormValid
} from '@/views/base/UtilityMetersPageBase.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt } = useI18n();

const utilityMetersStore = useUtilityMetersStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

const showState = ref<boolean>(false);
const submitting = ref<boolean>(false);
const meterId = ref<string>('');
const editingTariffId = ref<string>('');
const currency = ref<string>('');
const unit = ref<UtilityMeterUnit>(UtilityMeterUnit.KilowattHour);
const tariff = ref<UtilityTariffFormValue>(newUtilityTariffFormValue());

let resolveFunc: (() => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const isInputValid = computed<boolean>(() => isUtilityTariffFormValid(tariff.value));

// a new tariff opens on what the meter is charged now, because a price change is usually one field
// of the old price and not a contract nobody has ever seen before
function open(meter: UtilityMeterInfoResponse, editingTariff?: UtilityTariffInfoResponse): Promise<void> {
    showState.value = true;
    submitting.value = false;
    meterId.value = meter.id;
    editingTariffId.value = editingTariff?.id ?? '';
    currency.value = meter.currency;
    unit.value = meter.unit;

    if (editingTariff) {
        tariff.value = toUtilityTariffFormValue(editingTariff);
    } else if (meter.tariffs.length > 0) {
        tariff.value = {
            ...toUtilityTariffFormValue(meter.tariffs[0] as UtilityTariffInfoResponse),
            ...{ startDate: newUtilityTariffFormValue().startDate, comment: '' }
        };
    } else {
        tariff.value = newUtilityTariffFormValue();
    }

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

    const request = toUtilityTariffRequest(tariff.value);
    const promise = editingTariffId.value
        ? utilityMetersStore.modifyTariff({ id: editingTariffId.value, ...request })
        : utilityMetersStore.addTariff({ meterId: meterId.value, ...request });

    promise.then(() => {
        submitting.value = false;
        showState.value = false;

        if (resolveFunc) {
            resolveFunc();
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

defineExpose({
    open
});
</script>
