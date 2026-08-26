<template>
    <v-dialog width="600" v-model="showState">
        <v-card class="pa-sm-1 pa-md-2">
            <template #title>
                <h4 class="text-h4 text-wrap">{{ editingReadingId ? tt('Edit Reading') : tt('Add Reading') }}</h4>
            </template>
            <v-card-text>
                <v-row>
                    <v-col cols="12" md="6">
                        <v-text-field type="date" persistent-placeholder
                                      :disabled="submitting"
                                      :label="tt('Read On')"
                                      v-model="readingDate"/>
                    </v-col>
                    <v-col cols="12" md="6">
                        <v-text-field type="number" persistent-placeholder
                                      autofocus
                                      step="0.001" min="0"
                                      :disabled="submitting"
                                      :label="tt('Meter Reading')"
                                      :suffix="unitName"
                                      :hint="tt('What the dial shows, not what was used')"
                                      :persistent-hint="true"
                                      v-model="value"/>
                    </v-col>
                    <v-col cols="12">
                        <v-text-field type="text" persistent-placeholder
                                      :disabled="submitting"
                                      :label="tt('Description')"
                                      :placeholder="tt('Optional')"
                                      v-model="comment"/>
                    </v-col>
                    <v-col cols="12">
                        <v-checkbox density="compact"
                                    :disabled="submitting"
                                    :label="tt('This number was estimated, not read')"
                                    v-model="estimated"/>
                    </v-col>
                </v-row>

                <v-alert type="info" variant="tonal" density="compact" v-if="usageSincePrevious">
                    {{ usageSincePrevious }}
                </v-alert>
                <v-alert type="warning" variant="tonal" density="compact" v-else-if="warning">
                    {{ warning }}
                </v-alert>
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

import { ref, computed, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useUtilityMetersStore } from '@/stores/utilityMeter.ts';

import type { UtilityMeterInfoResponse, UtilityReadingInfoResponse } from '@/models/utility_meter.ts';
import {
    UtilityMeterUnit,
    UTILITY_READING_VALUE_SCALE,
    getUtilityMeterUnitName,
    scaledValueToNumber,
    numberToScaledValue,
    getNumericDateFromDateString,
    getDateStringFromNumericDate
} from '@/models/utility_meter.ts';

import { useUtilityMetersPageBase } from '@/views/base/UtilityMetersPageBase.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt } = useI18n();
const { getDisplayUsage, getDisplayDate } = useUtilityMetersPageBase();

const utilityMetersStore = useUtilityMetersStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

const showState = ref<boolean>(false);
const submitting = ref<boolean>(false);
const meterId = ref<string>('');
const editingReadingId = ref<string>('');
const unit = ref<UtilityMeterUnit>(UtilityMeterUnit.KilowattHour);
const readings = ref<UtilityReadingInfoResponse[]>([]);
const readingDate = ref<string>('');
const value = ref<number | string>('');
const estimated = ref<boolean>(false);
const comment = ref<string>('');

let resolveFunc: (() => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const unitName = computed<string>(() => getUtilityMeterUnitName(unit.value));
const numericDate = computed<number>(() => getNumericDateFromDateString(readingDate.value));
const scaledValue = computed<number>(() => numberToScaledValue(typeof value.value === 'string' ? parseFloat(value.value) : value.value, UTILITY_READING_VALUE_SCALE));

const isInputValid = computed<boolean>(() => numericDate.value > 0 && `${value.value}` !== '' && scaledValue.value >= 0);

// the reading this one would follow, which is what says how much was used and whether the number
// just typed is possible at all
const previousReading = computed<UtilityReadingInfoResponse | null>(() => {
    let previous: UtilityReadingInfoResponse | null = null;

    for (const reading of readings.value) {
        if (reading.id === editingReadingId.value || reading.readingDate >= numericDate.value) {
            continue;
        }

        if (!previous || reading.readingDate > previous.readingDate) {
            previous = reading;
        }
    }

    return previous;
});

// what this reading would say was used, shown while it is being typed. A digit dropped off a meter
// reading is invisible in the number itself and obvious in what it claims was used in a month.
const usageSincePrevious = computed<string>(() => {
    const previous = previousReading.value;

    if (!previous || !isInputValid.value || scaledValue.value < previous.value) {
        return '';
    }

    return tt('format.misc.utilityUsedSinceReading', {
        amount: getDisplayUsage(scaledValue.value - previous.value, unit.value),
        date: getDisplayDate(previous.readingDate)
    });
});

const warning = computed<string>(() => {
    const previous = previousReading.value;

    if (!previous || !isInputValid.value || scaledValue.value >= previous.value) {
        return '';
    }

    return tt('format.misc.utilityReadingBelowPrevious', {
        amount: getDisplayUsage(previous.value, unit.value),
        date: getDisplayDate(previous.readingDate)
    });
});

function open(meter: UtilityMeterInfoResponse, editingReading?: UtilityReadingInfoResponse): Promise<void> {
    const today = new Date();

    showState.value = true;
    submitting.value = false;
    meterId.value = meter.id;
    editingReadingId.value = editingReading?.id ?? '';
    unit.value = meter.unit;
    readings.value = meter.readings;
    readingDate.value = editingReading ? getDateStringFromNumericDate(editingReading.readingDate) : `${today.getFullYear()}-${(today.getMonth() + 1).toString().padStart(2, '0')}-${today.getDate().toString().padStart(2, '0')}`;
    value.value = editingReading ? scaledValueToNumber(editingReading.value, UTILITY_READING_VALUE_SCALE) : '';
    estimated.value = editingReading?.estimated ?? false;
    comment.value = editingReading?.comment ?? '';

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
        readingDate: numericDate.value,
        value: scaledValue.value,
        estimated: estimated.value,
        comment: comment.value.trim()
    };

    const promise = editingReadingId.value
        ? utilityMetersStore.modifyReading({ id: editingReadingId.value, ...request })
        : utilityMetersStore.addReading({ meterId: meterId.value, ...request });

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
