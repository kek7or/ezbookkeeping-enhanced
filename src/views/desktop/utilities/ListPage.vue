<template>
    <v-row class="match-height">
        <v-col cols="12">
            <v-card :class="{ 'disabled': loading }">
                <template #title>
                    <div class="title-and-toolbar d-flex align-center">
                        <span>{{ tt('Utility Meters') }}</span>
                        <v-btn density="compact" color="default" variant="text" size="24"
                               class="ms-2" :icon="true" :loading="loading" @click="reload()">
                            <template #loader>
                                <v-progress-circular indeterminate size="20"/>
                            </template>
                            <v-icon :icon="mdiRefresh" size="24" />
                            <v-tooltip activator="parent">{{ tt('Reload Page') }}</v-tooltip>
                        </v-btn>
                        <v-btn class="ms-auto" density="comfortable" color="default" variant="text"
                               :prepend-icon="mdiPlus" :disabled="loading || updating"
                               @click="addMeter">
                            {{ tt('Add Meter') }}
                        </v-btn>
                    </div>
                </template>

                <v-card-text>
                    <p class="text-body-2 text-medium-emphasis mb-0">
                        {{ tt('What your meters have used, and what that costs. Nothing here is written to your ledger.') }}
                    </p>
                </v-card-text>

                <v-card-text class="pt-0" v-if="loading && allMeters.length < 1">
                    <v-skeleton-loader type="text" :loading="true"></v-skeleton-loader>
                </v-card-text>

                <v-card-text class="pt-0" v-else-if="!loading && allMeters.length < 1">
                    <v-alert type="info" variant="tonal" density="compact">
                        {{ tt('No meters yet. Add one with the price you pay, then enter a reading whenever you take one.') }}
                    </v-alert>
                </v-card-text>
            </v-card>
        </v-col>

        <v-col cols="12" :key="meter.id" v-for="meter in allMeters">
            <v-card :class="{ 'disabled': updating }">
                <template #title>
                    <div class="title-and-toolbar d-flex align-center">
                        <v-icon class="me-2" :icon="getMeterIcon(meter.kind)" />
                        <div class="d-flex flex-column">
                            <span>{{ meter.name }}</span>
                            <small class="text-caption">{{ getMeterSubtitle(meter) }}</small>
                        </div>
                        <v-btn class="ms-auto" density="comfortable" color="default" variant="text"
                               :prepend-icon="mdiGaugeFull" :disabled="updating"
                               @click="addReading(meter)">
                            {{ tt('Add Reading') }}
                        </v-btn>
                        <v-btn density="comfortable" color="default" variant="text" :icon="true" :disabled="updating">
                            <v-icon :icon="mdiDotsVertical" />
                            <v-menu activator="parent">
                                <v-list>
                                    <v-list-item :prepend-icon="mdiPencilOutline"
                                                 :title="tt('Edit Meter')"
                                                 @click="editMeter(meter)"></v-list-item>
                                    <v-list-item :prepend-icon="mdiCurrencyUsd"
                                                 :title="tt('Add Tariff')"
                                                 @click="addTariff(meter)"></v-list-item>
                                    <v-list-item class="text-error"
                                                 :prepend-icon="mdiDeleteOutline"
                                                 :title="tt('Delete Meter')"
                                                 @click="removeMeter(meter)"></v-list-item>
                                </v-list>
                            </v-menu>
                        </v-btn>
                    </div>
                </template>

                <v-card-text v-if="meter.tariffs.length < 1">
                    <v-alert type="warning" variant="tonal" density="compact">
                        {{ tt('This meter has no price on file, so it can only say what it used. Add a tariff to see what that costs.') }}
                    </v-alert>
                </v-card-text>

                <v-card-text v-if="meter.billingYears.length < 1">
                    <v-alert type="info" variant="tonal" density="compact">
                        {{ tt('Two readings are needed before anything can be worked out. Enter the reading you started from and the one you have now.') }}
                    </v-alert>
                </v-card-text>

                <template v-else>
                    <v-card-text class="pb-0" v-if="meter.billingYears.length > 1">
                        <v-select density="compact" hide-details
                                  item-title="label" item-value="label"
                                  style="max-width: 260px"
                                  :label="tt('Billing Year')"
                                  :items="meter.billingYears"
                                  :model-value="getSelectedYearLabel(meter)"
                                  @update:model-value="selectYear(meter, $event)"/>
                    </v-card-text>

                    <v-card-text v-if="getSelectedYear(meter)">
                        <v-row>
                            <v-col cols="12" md="6">
                                <span class="text-subtitle-2">{{ tt('Used') }}</span>
                                <p class="text-h6 mt-1 mb-0">{{ getDisplayUsage(getSelectedYear(meter)!.usedUnits, meter.unit) }}</p>
                                <small class="text-caption" v-if="getSelectedYear(meter)!.usedEnergy">
                                    {{ getDisplayEnergy(getSelectedYear(meter)!.usedEnergy!) }}
                                </small>
                            </v-col>
                            <v-col cols="12" md="6">
                                <span class="text-subtitle-2">{{ tt('Cost So Far') }}</span>
                                <p class="text-h6 mt-1 mb-0">{{ getDisplayAmount(getSelectedYear(meter)!.cost, meter.currency) }}</p>
                            </v-col>
                        </v-row>
                    </v-card-text>

                    <v-table class="utility-period-table table-striped" :hover="!updating">
                        <thead>
                        <tr>
                            <th>{{ tt('Period') }}</th>
                            <th class="text-right">{{ tt('Days') }}</th>
                            <th class="text-right">{{ tt('From') }}</th>
                            <th class="text-right">{{ tt('To') }}</th>
                            <th class="text-right">{{ tt('Used') }}</th>
                            <th class="text-right">{{ tt('Cost') }}</th>
                        </tr>
                        </thead>

                        <tbody>
                        <tr :key="period.startDate" v-for="period in (getSelectedYear(meter)?.periods ?? [])">
                            <td class="text-no-wrap">
                                <div class="d-flex align-center">
                                    <span>{{ getDisplayPeriod(period.startDate, period.endDate) }}</span>
                                    <v-icon class="ms-1" size="16" :icon="mdiHelpCircleOutline"
                                            v-if="period.estimated || period.partial || period.priceChanged">
                                        <v-tooltip activator="parent">{{ getPeriodNote(period) }}</v-tooltip>
                                    </v-icon>
                                </div>
                            </td>
                            <td class="text-right">{{ period.days }}</td>
                            <td class="text-right text-no-wrap">{{ getDisplayReading(period.startValue, meter.unit) }}</td>
                            <td class="text-right text-no-wrap">{{ getDisplayReading(period.endValue, meter.unit) }}</td>
                            <td class="text-right text-no-wrap">
                                <div>{{ getDisplayUsage(period.usedUnits, meter.unit) }}</div>
                                <small class="text-caption" v-if="period.usedEnergy">{{ getDisplayEnergy(period.usedEnergy) }}</small>
                            </td>
                            <td class="text-right text-no-wrap">{{ getDisplayAmount(period.cost, meter.currency) }}</td>
                        </tr>
                        </tbody>
                    </v-table>
                </template>

                <v-expansion-panels class="mt-2" variant="accordion" multiple>
                    <v-expansion-panel :title="tt('Readings')" v-if="meter.readings.length > 0">
                        <v-expansion-panel-text>
                            <v-table class="table-striped" density="compact">
                                <tbody>
                                <tr :key="reading.id" v-for="reading in meter.readings">
                                    <td class="text-no-wrap">{{ getDisplayDate(reading.readingDate) }}</td>
                                    <td class="text-no-wrap">{{ getDisplayReading(reading.value, meter.unit) }}</td>
                                    <td>
                                        <span class="text-caption">{{ reading.comment }}</span>
                                        <span class="text-caption text-medium-emphasis" v-if="reading.estimated">{{ tt('estimated') }}</span>
                                    </td>
                                    <td class="text-right text-no-wrap">
                                        <v-btn density="compact" color="default" variant="text" size="24" :icon="true"
                                               :disabled="updating" @click="editReading(meter, reading)">
                                            <v-icon :icon="mdiPencilOutline" size="20" />
                                        </v-btn>
                                        <v-btn class="ms-1" density="compact" color="error" variant="text" size="24" :icon="true"
                                               :disabled="updating" @click="removeReading(meter, reading)">
                                            <v-icon :icon="mdiDeleteOutline" size="20" />
                                        </v-btn>
                                    </td>
                                </tr>
                                </tbody>
                            </v-table>
                        </v-expansion-panel-text>
                    </v-expansion-panel>

                    <v-expansion-panel :title="tt('Tariffs')" v-if="meter.tariffs.length > 0">
                        <v-expansion-panel-text>
                            <v-table class="table-striped" density="compact">
                                <tbody>
                                <tr :key="tariff.id" v-for="tariff in meter.tariffs">
                                    <td class="text-no-wrap">{{ tt('from') }} {{ getDisplayDate(tariff.startDate) }}</td>
                                    <td class="text-no-wrap">{{ getTariffSummary(tariff, meter) }}</td>
                                    <td class="text-right text-no-wrap">
                                        <v-btn density="compact" color="default" variant="text" size="24" :icon="true"
                                               :disabled="updating" @click="editTariff(meter, tariff)">
                                            <v-icon :icon="mdiPencilOutline" size="20" />
                                        </v-btn>
                                        <v-btn class="ms-1" density="compact" color="error" variant="text" size="24" :icon="true"
                                               :disabled="updating || meter.tariffs.length < 2" @click="removeTariff(tariff)">
                                            <v-icon :icon="mdiDeleteOutline" size="20" />
                                        </v-btn>
                                    </td>
                                </tr>
                                </tbody>
                            </v-table>
                        </v-expansion-panel-text>
                    </v-expansion-panel>
                </v-expansion-panels>
            </v-card>
        </v-col>
    </v-row>

    <edit-meter-dialog ref="editMeterDialog" />
    <edit-tariff-dialog ref="editTariffDialog" />
    <edit-reading-dialog ref="editReadingDialog" />
    <confirm-dialog ref="confirmDialog" />
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';
import EditMeterDialog from './dialogs/EditMeterDialog.vue';
import EditTariffDialog from './dialogs/EditTariffDialog.vue';
import EditReadingDialog from './dialogs/EditReadingDialog.vue';

import { ref, onMounted, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { useUtilityMetersPageBase } from '@/views/base/UtilityMetersPageBase.ts';

import { useUtilityMetersStore } from '@/stores/utilityMeter.ts';

import type {
    UtilityMeterInfoResponse,
    UtilityTariffInfoResponse,
    UtilityReadingInfoResponse,
    UtilityBillingYearResponse,
    UtilityPeriodResponse
} from '@/models/utility_meter.ts';
import { UtilityMeterKind, UtilityMeterUnit, getUtilityMeterUnitName } from '@/models/utility_meter.ts';

import {
    mdiRefresh,
    mdiPlus,
    mdiDotsVertical,
    mdiPencilOutline,
    mdiDeleteOutline,
    mdiFlash,
    mdiFire,
    mdiWater,
    mdiRadiator,
    mdiGauge,
    mdiGaugeFull,
    mdiCurrencyUsd,
    mdiHelpCircleOutline
} from '@mdi/js';

type EditMeterDialogType = InstanceType<typeof EditMeterDialog>;
type EditTariffDialogType = InstanceType<typeof EditTariffDialog>;
type EditReadingDialogType = InstanceType<typeof EditReadingDialog>;
type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;

const { tt } = useI18n();
const {
    allMeters,
    getDisplayAmount,
    getDisplayUsage,
    getDisplayEnergy,
    getDisplayReading,
    getDisplayUnitPrice,
    getDisplayConversionFactor,
    getDisplayDate,
    getDisplayPeriod,
    getMeterKindName
} = useUtilityMetersPageBase();

const utilityMetersStore = useUtilityMetersStore();

const editMeterDialog = useTemplateRef<EditMeterDialogType>('editMeterDialog');
const editTariffDialog = useTemplateRef<EditTariffDialogType>('editTariffDialog');
const editReadingDialog = useTemplateRef<EditReadingDialogType>('editReadingDialog');
const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');

const loading = ref<boolean>(true);
const updating = ref<boolean>(false);

// which billing year each meter is showing. A meter the user has not chosen a year for shows its
// newest one, which is the year they are living in.
const selectedYearLabels = ref<Record<string, string>>({});

function getSelectedYearLabel(meter: UtilityMeterInfoResponse): string {
    const selected = selectedYearLabels.value[meter.id];

    if (selected && meter.billingYears.some(year => year.label === selected)) {
        return selected;
    }

    return meter.billingYears.length > 0 ? (meter.billingYears[0] as UtilityBillingYearResponse).label : '';
}

function getSelectedYear(meter: UtilityMeterInfoResponse): UtilityBillingYearResponse | null {
    const label = getSelectedYearLabel(meter);
    return meter.billingYears.find(year => year.label === label) ?? null;
}

function selectYear(meter: UtilityMeterInfoResponse, label: unknown): void {
    selectedYearLabels.value[meter.id] = `${label}`;
}

function getMeterIcon(kind: UtilityMeterKind): string {
    switch (kind) {
        case UtilityMeterKind.Electricity:
            return mdiFlash;
        case UtilityMeterKind.Gas:
            return mdiFire;
        case UtilityMeterKind.Water:
            return mdiWater;
        case UtilityMeterKind.Heating:
            return mdiRadiator;
        default:
            return mdiGauge;
    }
}

function getMeterSubtitle(meter: UtilityMeterInfoResponse): string {
    const parts = [getMeterKindName(meter.kind), getUtilityMeterUnitName(meter.unit)];

    if (meter.meterNumber) {
        parts.push(meter.meterNumber);
    }

    if (meter.comment) {
        parts.push(meter.comment);
    }

    return parts.join(' · ');
}

function getTariffSummary(tariff: UtilityTariffInfoResponse, meter: UtilityMeterInfoResponse): string {
    const parts = [
        tt('format.misc.utilityPricePerUnit', {
            price: getDisplayUnitPrice(tariff),
            currency: meter.currency,
            unit: meter.unit === UtilityMeterUnit.KilowattHour ? getUtilityMeterUnitName(meter.unit) : 'kWh'
        }),
        tt('format.misc.utilityBaseFeePerYear', { amount: getDisplayAmount(tariff.baseFeeAnnual, meter.currency) }),
        tt('format.misc.utilityPrepaymentPerMonth', { amount: getDisplayAmount(tariff.monthlyPrepayment, meter.currency) })
    ];

    if (tariff.calorificValue) {
        parts.push(tt('format.misc.utilityConversionFactors', {
            calorificValue: getDisplayConversionFactor(tariff.calorificValue),
            stateNumber: getDisplayConversionFactor(tariff.stateNumber)
        }));
    }

    if (tariff.comment) {
        parts.push(tariff.comment);
    }

    return parts.join(' · ');
}

function getPeriodNote(period: UtilityPeriodResponse): string {
    const notes: string[] = [];

    if (period.partial) {
        notes.push(tt('Part of a longer reading interval, cut where the billing year ends.'));
    }

    if (period.estimated) {
        notes.push(tt('One of the readings this sits between was estimated.'));
    }

    if (period.priceChanged) {
        notes.push(tt('The price changed during these days, so the use was split between the tariffs by the days each covered.'));
    }

    return notes.join(' ');
}

function reload(force?: boolean): void {
    loading.value = true;

    utilityMetersStore.loadAllMeters({ force: force !== false }).then(() => {
        loading.value = false;
    }).catch(error => {
        loading.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function addMeter(): void {
    editMeterDialog.value?.open().then(() => {
        snackbar.value?.showMessage('The meter has been added');
    }).catch(() => {
        // the dialog was closed without saving
    });
}

function editMeter(meter: UtilityMeterInfoResponse): void {
    editMeterDialog.value?.open(meter).then(() => {
        snackbar.value?.showMessage('The meter has been saved');
    }).catch(() => {
        // the dialog was closed without saving
    });
}

function removeMeter(meter: UtilityMeterInfoResponse): void {
    confirmDialog.value?.open('Are you sure you want to delete this meter? Its readings and its tariffs go with it, and no transaction is touched.').then(() => {
        updating.value = true;

        utilityMetersStore.deleteMeter({ id: meter.id }).then(() => {
            updating.value = false;
            snackbar.value?.showMessage('The meter has been deleted');
        }).catch(error => {
            updating.value = false;

            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    });
}

function addTariff(meter: UtilityMeterInfoResponse): void {
    editTariffDialog.value?.open(meter).then(() => {
        snackbar.value?.showMessage('The tariff has been added');
    }).catch(() => {
        // the dialog was closed without saving
    });
}

function editTariff(meter: UtilityMeterInfoResponse, tariff: UtilityTariffInfoResponse): void {
    editTariffDialog.value?.open(meter, tariff).then(() => {
        snackbar.value?.showMessage('The tariff has been saved');
    }).catch(() => {
        // the dialog was closed without saving
    });
}

function removeTariff(tariff: UtilityTariffInfoResponse): void {
    confirmDialog.value?.open('Are you sure you want to delete this tariff? The days it covered will be priced at whichever tariff covers them once it is gone.').then(() => {
        updating.value = true;

        utilityMetersStore.deleteTariff({ id: tariff.id }).then(() => {
            updating.value = false;
            snackbar.value?.showMessage('The tariff has been deleted');
        }).catch(error => {
            updating.value = false;

            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    });
}

function addReading(meter: UtilityMeterInfoResponse): void {
    editReadingDialog.value?.open(meter).then(() => {
        snackbar.value?.showMessage('The reading has been saved');
    }).catch(() => {
        // the dialog was closed without saving
    });
}

function editReading(meter: UtilityMeterInfoResponse, reading: UtilityReadingInfoResponse): void {
    editReadingDialog.value?.open(meter, reading).then(() => {
        snackbar.value?.showMessage('The reading has been saved');
    }).catch(() => {
        // the dialog was closed without saving
    });
}

function removeReading(meter: UtilityMeterInfoResponse, reading: UtilityReadingInfoResponse): void {
    confirmDialog.value?.open('Are you sure you want to delete this reading? The periods either side of it become one.').then(() => {
        updating.value = true;

        utilityMetersStore.deleteReading({ id: reading.id }).then(() => {
            updating.value = false;
            snackbar.value?.showMessage('The reading has been deleted');
        }).catch(error => {
            updating.value = false;

            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    });
}

onMounted(() => {
    reload(false);
});
</script>

<style>
.utility-period-table tr td {
    white-space: nowrap;
}
</style>
