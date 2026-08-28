<template>
    <v-dialog width="600" v-model="showState">
        <v-card class="pa-sm-1 pa-md-2">
            <template #title>
                <h4 class="text-h4 text-wrap">{{ tt('Adjust for This Month') }}</h4>
            </template>
            <v-card-text>
                <p class="text-body-2 text-medium-emphasis mb-4">
                    {{ tt('Says how this one month differs from the schedule. The schedule itself is left alone, so every other month is planned as before.') }}
                </p>
                <v-row>
                    <v-col cols="12">
                        <v-text-field type="text" persistent-placeholder readonly
                                      :label="tt('Scheduled Transaction')"
                                      :model-value="scheduleName"/>
                    </v-col>
                    <v-col cols="12" md="6">
                        <amount-input :currency="currency"
                                      :show-currency="true"
                                      :persistent-placeholder="true"
                                      :disabled="submitting || excluded"
                                      :label="tt('Amount This Month')"
                                      :enable-formula="true"
                                      v-model="amount"/>
                    </v-col>
                    <v-col cols="12" md="6">
                        <v-checkbox class="mt-2" density="comfortable"
                                    :disabled="submitting"
                                    :label="tt('Not happening this month')"
                                    v-model="excluded"/>
                    </v-col>
                </v-row>
            </v-card-text>
            <v-card-text>
                <div class="w-100 d-flex justify-center gap-4">
                    <v-btn color="primary" :disabled="submitting" @click="save">
                        {{ tt('Save') }}
                        <v-progress-circular indeterminate size="22" class="ms-2" v-if="submitting"></v-progress-circular>
                    </v-btn>
                    <v-btn color="secondary" variant="tonal" :disabled="submitting" @click="clear">
                        {{ tt('Use the Schedule') }}
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
import SnackBar from '@/components/desktop/SnackBar.vue';

import { ref, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useBudgetPlanStore } from '@/stores/budgetPlan.ts';

import type { PlannedLine } from '@/lib/budgetPlan.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt } = useI18n();

const budgetPlanStore = useBudgetPlanStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

const showState = ref<boolean>(false);
const submitting = ref<boolean>(false);
const templateId = ref<string>('');
const scheduleName = ref<string>('');
const currency = ref<string>('');
const amount = ref<number>(0);
const excluded = ref<boolean>(false);

let resolveFunc: (() => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

// The amount is what one occurrence costs, not what the month costs. A weekly schedule happening
// four times is adjusted by saying what one of those four is worth, which is the only figure the
// user has any opinion about.
function open(line: PlannedLine): Promise<void> {
    showState.value = true;
    submitting.value = false;
    templateId.value = line.id;
    scheduleName.value = line.name;
    currency.value = line.currency;
    amount.value = line.unitAmount;
    excluded.value = line.excluded;

    return new Promise((resolve, reject) => {
        resolveFunc = resolve;
        rejectFunc = reject;
    });
}

function save(): void {
    submit(excluded.value, amount.value);
}

// Clearing puts the schedule's own figure back in force for this month, which is not the same as
// setting the amount to what the schedule currently says: if the schedule later changes price, a
// month that was cleared follows it and a month that was set does not.
function clear(): void {
    submit(false, undefined);
}

function submit(newExcluded: boolean, newAmount: number | undefined): void {
    submitting.value = true;

    budgetPlanStore.setScheduleAdjustment({
        templateId: templateId.value,
        excluded: newExcluded,
        amount: newAmount
    }).then(() => {
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
