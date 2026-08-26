<template>
    <v-row>
        <v-col cols="12" md="6">
            <v-text-field type="date" persistent-placeholder
                          :disabled="disabled"
                          :label="tt('In Force From')"
                          :hint="tt('The first day this price applies to')"
                          :persistent-hint="true"
                          :model-value="modelValue.startDate"
                          @update:model-value="update({ startDate: $event })"/>
        </v-col>
        <v-col cols="12" md="6">
            <v-text-field type="number" persistent-placeholder
                          step="0.0001" min="0"
                          :disabled="disabled"
                          :label="tt('Price per kWh')"
                          :suffix="pricePerUnitSuffix"
                          :hint="tt('The Arbeitspreis on your bill, per kilowatt hour')"
                          :persistent-hint="true"
                          :model-value="modelValue.unitPrice"
                          @update:model-value="update({ unitPrice: toNumber($event) })"/>
        </v-col>
        <v-col cols="12" md="8">
            <amount-input :currency="currency"
                          :show-currency="true"
                          :persistent-placeholder="true"
                          :disabled="disabled"
                          :label="tt('Base Fee')"
                          :model-value="modelValue.baseFee"
                          @update:model-value="update({ baseFee: $event })"/>
        </v-col>
        <v-col cols="12" md="4">
            <v-select item-title="displayName" item-value="type"
                      persistent-placeholder
                      :disabled="disabled"
                      :label="tt('Quoted')"
                      :items="allBaseFeePeriods"
                      :model-value="modelValue.baseFeePerYear"
                      @update:model-value="update({ baseFeePerYear: $event })"/>
        </v-col>
        <v-col cols="12" md="6">
            <amount-input :currency="currency"
                          :show-currency="true"
                          :persistent-placeholder="true"
                          :disabled="disabled"
                          :label="tt('Monthly Prepayment')"
                          :model-value="modelValue.monthlyPrepayment"
                          @update:model-value="update({ monthlyPrepayment: $event })"/>
        </v-col>
        <v-col cols="12" md="6">
            <v-text-field type="text" persistent-placeholder
                          :disabled="disabled"
                          :label="tt('Description')"
                          :placeholder="tt('Optional')"
                          :model-value="modelValue.comment"
                          @update:model-value="update({ comment: $event })"/>
        </v-col>

        <template v-if="isConverted">
            <v-col cols="12">
                <v-alert type="info" variant="tonal" density="compact">
                    {{ tt('Gas is sold by the kilowatt hour but the meter counts volume. Both numbers below are printed on your bill.') }}
                </v-alert>
            </v-col>
            <v-col cols="12" md="6">
                <v-text-field type="number" persistent-placeholder
                              step="0.0001" min="0"
                              :disabled="disabled"
                              :label="tt('Calorific Value')"
                              :suffix="calorificValueSuffix"
                              :hint="tt('The Brennwert, usually between 10 and 13.1 for H-Gas')"
                              :persistent-hint="true"
                              :model-value="modelValue.calorificValue"
                              @update:model-value="update({ calorificValue: toNumber($event) })"/>
            </v-col>
            <v-col cols="12" md="6">
                <v-text-field type="number" persistent-placeholder
                              step="0.0001" min="0"
                              :disabled="disabled"
                              :label="tt('State Number')"
                              :hint="tt('The Zustandszahl, usually close to 0.95')"
                              :persistent-hint="true"
                              :model-value="modelValue.stateNumber"
                              @update:model-value="update({ stateNumber: toNumber($event) })"/>
            </v-col>
            <v-col cols="12" v-if="conversionExample">
                <p class="text-body-2 text-medium-emphasis mb-0">{{ conversionExample }}</p>
            </v-col>
        </template>
    </v-row>
</template>

<script setup lang="ts">
import AmountInput from '@/components/desktop/AmountInput.vue';

import { computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { UtilityMeterUnit, getUtilityMeterUnitName } from '@/models/utility_meter.ts';

import type { UtilityTariffFormValue } from '@/views/base/UtilityMetersPageBase.ts';

interface UtilityTariffFieldsProps {
    modelValue: UtilityTariffFormValue;
    currency: string;
    unit: UtilityMeterUnit;
    disabled?: boolean;
}

const props = defineProps<UtilityTariffFieldsProps>();
const emit = defineEmits<{
    (e: 'update:modelValue', value: UtilityTariffFormValue): void;
}>();

const { tt, formatNumberToLocalizedNumerals } = useI18n();

// a meter that counts volume has to have what it counted turned into energy before the price can be
// put on it, and one that counts kilowatt hours already holds the thing being priced
const isConverted = computed<boolean>(() => props.unit !== UtilityMeterUnit.KilowattHour);

const pricePerUnitSuffix = computed<string>(() => `/ ${isConverted.value ? 'kWh' : getUtilityMeterUnitName(props.unit)}`);
const calorificValueSuffix = computed<string>(() => `kWh / ${getUtilityMeterUnitName(props.unit)}`);

const allBaseFeePeriods = computed(() => [
    { type: true, displayName: tt('per year') },
    { type: false, displayName: tt('per month') }
]);

// what one unit off this dial will be counted as, spelled out, because a Brennwert entered in the
// wrong field is otherwise only visible months later in a total that is fifteen percent out
const conversionExample = computed<string>(() => {
    if (!isConverted.value || props.modelValue.calorificValue <= 0) {
        return '';
    }

    const stateNumber = props.modelValue.stateNumber > 0 ? props.modelValue.stateNumber : 1;
    const energy = props.modelValue.calorificValue * stateNumber;

    return tt('format.misc.utilityConversionExample', {
        unit: getUtilityMeterUnitName(props.unit),
        energy: formatNumberToLocalizedNumerals(Math.round(energy * 10000) / 10000, 4)
    });
});

function toNumber(value: unknown): number {
    const parsed = parseFloat(`${value}`);
    return isFinite(parsed) ? parsed : 0;
}

function update(patch: Partial<UtilityTariffFormValue>): void {
    emit('update:modelValue', { ...props.modelValue, ...patch });
}
</script>
