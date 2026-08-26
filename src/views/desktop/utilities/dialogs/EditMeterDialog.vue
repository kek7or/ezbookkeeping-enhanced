<template>
    <v-dialog width="700" v-model="showState">
        <v-card class="pa-sm-1 pa-md-2">
            <template #title>
                <h4 class="text-h4 text-wrap">{{ editingMeterId ? tt('Edit Meter') : tt('Add Meter') }}</h4>
            </template>
            <v-card-text>
                <p class="text-body-2 text-medium-emphasis mb-4">
                    {{ tt('A meter is read, not paid. What leaves your account is the monthly prepayment, and this page only says what it was worth.') }}
                </p>
                <v-row>
                    <v-col cols="12" md="6">
                        <v-text-field type="text" persistent-placeholder
                                      autofocus
                                      :disabled="submitting"
                                      :label="tt('Name')"
                                      :placeholder="tt('Electricity')"
                                      v-model="name"/>
                    </v-col>
                    <v-col cols="12" md="6">
                        <v-text-field type="text" persistent-placeholder
                                      :disabled="submitting"
                                      :label="tt('Meter Number')"
                                      :placeholder="tt('As printed on the dial')"
                                      v-model="meterNumber"/>
                    </v-col>
                    <v-col cols="12" md="4">
                        <v-select item-title="displayName" item-value="type"
                                  persistent-placeholder
                                  :disabled="submitting"
                                  :label="tt('Kind')"
                                  :items="allMeterKinds"
                                  v-model="kind"/>
                    </v-col>
                    <v-col cols="12" md="4">
                        <v-select item-title="displayName" item-value="type"
                                  persistent-placeholder
                                  :disabled="submitting"
                                  :label="tt('Meter Counts In')"
                                  :items="allMeterUnits"
                                  v-model="unit"/>
                    </v-col>
                    <v-col cols="12" md="4">
                        <currency-select :disabled="submitting"
                                         :label="tt('Currency')"
                                         v-model="currency"/>
                    </v-col>
                    <v-col cols="12" md="6">
                        <v-select item-title="displayName" item-value="type"
                                  persistent-placeholder
                                  :disabled="submitting"
                                  :label="tt('Billing Year Starts In')"
                                  :hint="tt('The month your supplier settles the year in')"
                                  :persistent-hint="true"
                                  :items="allMonths"
                                  v-model="billingYearStartMonth"/>
                    </v-col>
                    <v-col cols="12" md="6">
                        <v-text-field type="text" persistent-placeholder
                                      :disabled="submitting"
                                      :label="tt('Description')"
                                      :placeholder="tt('Optional')"
                                      v-model="comment"/>
                    </v-col>
                </v-row>

                <template v-if="!editingMeterId">
                    <v-divider class="my-4"/>
                    <h5 class="text-h6 mb-1">{{ tt('Tariff') }}</h5>
                    <p class="text-body-2 text-medium-emphasis mb-4">
                        {{ tt('What this meter costs from the day this tariff begins. You can add a later one whenever the price changes.') }}
                    </p>
                    <utility-tariff-fields :disabled="submitting"
                                           :currency="currency"
                                           :unit="unit"
                                           v-model="tariff"/>
                </template>
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
import CurrencySelect from '@/components/desktop/CurrencySelect.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';
import UtilityTariffFields from './UtilityTariffFields.vue';

import { ref, computed, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useUserStore } from '@/stores/user.ts';
import { useUtilityMetersStore } from '@/stores/utilityMeter.ts';

import type { UtilityMeterInfoResponse } from '@/models/utility_meter.ts';
import { UtilityMeterKind, UtilityMeterUnit } from '@/models/utility_meter.ts';

import type { UtilityTariffFormValue } from '@/views/base/UtilityMetersPageBase.ts';
import { newUtilityTariffFormValue, toUtilityTariffRequest } from '@/views/base/UtilityMetersPageBase.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt, getAllMonths } = useI18n();

const userStore = useUserStore();
const utilityMetersStore = useUtilityMetersStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

const showState = ref<boolean>(false);
const submitting = ref<boolean>(false);
const editingMeterId = ref<string>('');
const name = ref<string>('');
const meterNumber = ref<string>('');
const kind = ref<UtilityMeterKind>(UtilityMeterKind.Electricity);
const unit = ref<UtilityMeterUnit>(UtilityMeterUnit.KilowattHour);
const currency = ref<string>('');
const billingYearStartMonth = ref<number>(1);
const comment = ref<string>('');
const tariff = ref<UtilityTariffFormValue>(newUtilityTariffFormValue());

let resolveFunc: (() => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const allMeterKinds = computed(() => [
    { type: UtilityMeterKind.Electricity, displayName: tt('Electricity') },
    { type: UtilityMeterKind.Gas, displayName: tt('Gas') },
    { type: UtilityMeterKind.Water, displayName: tt('Water') },
    { type: UtilityMeterKind.Heating, displayName: tt('Heating') },
    { type: UtilityMeterKind.Other, displayName: tt('Other') }
]);

const allMeterUnits = computed(() => [
    { type: UtilityMeterUnit.KilowattHour, displayName: tt('Kilowatt hours (kWh)') },
    { type: UtilityMeterUnit.CubicMetre, displayName: tt('Cubic metres (m³)') },
    { type: UtilityMeterUnit.Litre, displayName: tt('Litres (l)') }
]);

const allMonths = computed(() => getAllMonths());

const isInputValid = computed<boolean>(() => !!name.value.trim() && !!currency.value && billingYearStartMonth.value >= 1 && billingYearStartMonth.value <= 12);

function open(meter?: UtilityMeterInfoResponse): Promise<void> {
    showState.value = true;
    submitting.value = false;
    editingMeterId.value = meter?.id ?? '';
    name.value = meter?.name ?? '';
    meterNumber.value = meter?.meterNumber ?? '';
    kind.value = meter?.kind ?? UtilityMeterKind.Electricity;
    unit.value = meter?.unit ?? UtilityMeterUnit.KilowattHour;
    currency.value = meter?.currency ?? userStore.currentUserDefaultCurrency;
    billingYearStartMonth.value = meter?.billingYearStartMonth ?? 1;
    comment.value = meter?.comment ?? '';
    tariff.value = newUtilityTariffFormValue();

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

    const request = {
        name: name.value.trim(),
        kind: kind.value,
        unit: unit.value,
        currency: currency.value,
        billingYearStartMonth: billingYearStartMonth.value,
        meterNumber: meterNumber.value.trim(),
        comment: comment.value.trim()
    };

    const promise = editingMeterId.value
        ? utilityMetersStore.modifyMeter({ id: editingMeterId.value, ...request })
        : utilityMetersStore.addMeter({ ...request, tariff: toUtilityTariffRequest(tariff.value) });

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
