<template>
    <v-dialog width="700" v-model="showState">
        <v-card class="pa-sm-1 pa-md-2">
            <template #title>
                <h4 class="text-h4 text-wrap">{{ tt('Grab from Budget') }}</h4>
            </template>
            <template #subtitle>
                <div class="text-body-2 text-wrap">{{ tt('Take something planned out of its month and back onto the wishlist. It stays attached to that month, uncounted.') }}</div>
            </template>
            <v-card-text>
                <div class="d-flex align-center justify-center mb-2">
                    <v-btn density="comfortable" variant="text" :icon="mdiChevronLeft" :disabled="loading" @click="stepMonth(-1)"/>
                    <span class="text-h6 mx-4">{{ displayMonth }}</span>
                    <v-btn density="comfortable" variant="text" :icon="mdiChevronRight" :disabled="loading" @click="stepMonth(1)"/>
                </div>
                <v-table density="comfortable">
                    <tbody v-if="loading">
                    <tr><td><v-skeleton-loader type="text" :loading="true"/></td></tr>
                    </tbody>
                    <tbody v-else-if="!items.length">
                    <tr><td class="text-center text-medium-emphasis py-4">{{ tt('Nothing planned by hand in this month') }}</td></tr>
                    </tbody>
                    <tbody v-else>
                    <tr :key="item.id" v-for="item in items">
                        <td>
                            <div class="text-body-1">{{ item.name }}</div>
                            <div class="text-caption text-medium-emphasis" v-if="item.comment">{{ item.comment }}</div>
                        </td>
                        <td class="text-end text-no-wrap">{{ displayAmount(item) }}</td>
                        <td class="text-end">
                            <v-btn density="comfortable" color="primary" variant="tonal"
                                   :prepend-icon="mdiHeartOutline"
                                   :loading="grabbingId === item.id" :disabled="!!grabbingId"
                                   @click="grab(item)">{{ tt('Move to Wishlist') }}</v-btn>
                        </td>
                    </tr>
                    </tbody>
                </v-table>
            </v-card-text>
            <v-card-text>
                <div class="w-100 d-flex justify-center">
                    <v-btn color="secondary" variant="tonal" @click="showState = false">{{ tt('Close') }}</v-btn>
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

import { useUserStore } from '@/stores/user.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { useWishlistStore } from '@/stores/wishlist.ts';

import { TransactionType } from '@/core/transaction.ts';
import { BudgetPlanItem } from '@/models/budget_plan.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { getCurrentDateTime, parseDateTimeFromUnixTime, getYearMonthFirstUnixTime } from '@/lib/datetime.ts';
import services from '@/lib/services.ts';

import { mdiChevronLeft, mdiChevronRight, mdiHeartOutline } from '@mdi/js';

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt, formatAmountToLocalizedNumeralsWithCurrency, formatDateTimeToGregorianLikeLongYearMonth } = useI18n();

const userStore = useUserStore();
const accountsStore = useAccountsStore();
const wishlistStore = useWishlistStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

const showState = ref<boolean>(false);
const loading = ref<boolean>(false);
const grabbingId = ref<string>('');
const year = ref<number>(0);
const month = ref<number>(0);
const items = ref<BudgetPlanItem[]>([]);

const displayMonth = computed<string>(() => year.value ? formatDateTimeToGregorianLikeLongYearMonth(parseDateTimeFromUnixTime(getYearMonthFirstUnixTime({ year: year.value, month0base: month.value - 1 }))) : '');

function displayAmount(item: BudgetPlanItem): string {
    const currency = accountsStore.allAccountsMap[item.accountId]?.currency ?? userStore.currentUserDefaultCurrency;
    return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(item.amount).truncate(), currency);
}

function load(): void {
    const requestedYear = year.value;
    const requestedMonth = month.value;

    loading.value = true;

    services.getBudgetPlan({ year: requestedYear, month: requestedMonth }).then(response => {
        if (requestedYear !== year.value || requestedMonth !== month.value) {
            return;
        }

        const data = response.data;
        // only expenses go on a wishlist
        items.value = data && data.success && data.result ? BudgetPlanItem.ofMulti(data.result.items || []).filter(item => item.type === TransactionType.Expense) : [];
        loading.value = false;
    }).catch(error => {
        loading.value = false;

        if (!error.processed) {
            snackbar.value?.showError('Unable to retrieve budget plan');
        }
    });
}

function stepMonth(delta: number): void {
    const index = year.value * 12 + (month.value - 1) + delta;
    year.value = Math.floor(index / 12);
    month.value = index % 12 + 1;
    load();
}

function open(): void {
    const now = getCurrentDateTime();

    showState.value = true;
    grabbingId.value = '';
    year.value = now.getGregorianCalendarYear();
    month.value = now.getGregorianCalendarMonth();
    items.value = [];
    load();
}

function grab(item: BudgetPlanItem): void {
    grabbingId.value = item.id;

    wishlistStore.grabPlannedItem({ item: item }).then(() => {
        grabbingId.value = '';
        items.value = items.value.filter(existing => existing.id !== item.id);
        snackbar.value?.showMessage('You have returned this to the wishlist');
    }).catch(error => {
        grabbingId.value = '';

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

defineExpose({
    open
});
</script>
