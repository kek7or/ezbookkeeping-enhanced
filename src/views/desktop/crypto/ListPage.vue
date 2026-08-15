<template>
    <v-row class="match-height">
        <v-col cols="12">
            <v-card :class="{ 'disabled': loading }">
                <template #title>
                    <div class="title-and-toolbar d-flex align-center">
                        <span>{{ tt('Crypto Assets') }}</span>
                        <v-btn density="compact" color="default" variant="text" size="24"
                               class="ms-2" :icon="true" :loading="loading" @click="reload()">
                            <template #loader>
                                <v-progress-circular indeterminate size="20"/>
                            </template>
                            <v-icon :icon="mdiRefresh" size="24" />
                            <v-tooltip activator="parent">{{ tt('Reload Page') }}</v-tooltip>
                        </v-btn>
                    </div>
                </template>

                <v-card-text>
                    <v-row>
                        <v-col cols="12" md="4">
                            <span class="text-subtitle-2">{{ tt('Total Crypto Value') }}</span>
                            <p class="text-h5 mt-1 mb-0">
                                <span v-if="!loading">{{ displayTotalValue }}</span>
                                <v-skeleton-loader class="skeleton-no-margin mt-3 mb-1" type="text" :loading="true" v-else-if="loading"></v-skeleton-loader>
                            </p>
                        </v-col>
                        <v-col cols="12" md="4">
                            <span class="text-subtitle-2">{{ tt('Change in 24 Hours') }}</span>
                            <p class="text-h5 mt-1 mb-0" :class="getPriceChangeColor(totalPriceChange1d)">
                                <span v-if="!loading">{{ getDisplayPriceChange(totalPriceChange1d) || '-' }}</span>
                                <v-skeleton-loader class="skeleton-no-margin mt-3 mb-1" type="text" :loading="true" v-else-if="loading"></v-skeleton-loader>
                            </p>
                        </v-col>
                        <v-col cols="12" md="4">
                            <span class="text-subtitle-2">{{ tt('Portfolio Last Updated') }}</span>
                            <p class="text-body-1 mt-1 mb-0 d-flex align-center flex-wrap">
                                <span v-if="!loading">{{ displayPricesUpdateTime }}</span>
                                <v-skeleton-loader class="skeleton-no-margin mt-3 mb-1" type="text" :loading="true" v-else-if="loading"></v-skeleton-loader>
                                <v-btn class="ms-2" density="comfortable" color="default" variant="text"
                                       :prepend-icon="mdiCloudDownloadOutline"
                                       :loading="refreshing"
                                       :disabled="loading || refreshing || !canRefreshPortfolio"
                                       @click="refreshPortfolio" v-if="!loading">
                                    {{ tt('Update Now') }}
                                    <v-tooltip activator="parent" v-if="refreshTooltip">{{ refreshTooltip }}</v-tooltip>
                                </v-btn>
                            </p>
                        </v-col>
                    </v-row>
                </v-card-text>

                <v-card-text class="pt-0" v-if="!loading && !dataSourceConfigured">
                    <v-alert type="info" variant="tonal" density="compact">
                        {{ tt('The crypto data source needs an api key and a portfolio share token before it can be read') }}
                    </v-alert>
                </v-card-text>

                <v-card-text class="pt-0" v-if="!loading && lastErrorMessage">
                    <v-alert type="warning" variant="tonal" density="compact">
                        {{ tt('format.misc.cryptoPortfolioRefreshFailed', { message: lastErrorMessage }) }}
                    </v-alert>
                </v-card-text>

                <v-table class="crypto-portfolio-table table-striped" :hover="!loading">
                    <thead>
                    <tr>
                        <th>{{ tt('Coin') }}</th>
                        <th class="text-right">{{ tt('Quantity') }}</th>
                        <th class="text-right">{{ tt('Unit Price') }}</th>
                        <th class="text-right">{{ tt('24H') }}</th>
                        <th class="text-right">{{ tt('Share') }}</th>
                        <th class="text-right">{{ tt('Value') }}</th>
                    </tr>
                    </thead>

                    <tbody>
                    <tr :key="itemIdx" v-for="itemIdx in (loading && allCoins.length < 1 ? [ 1, 2, 3 ] : [])">
                        <td colspan="6" class="px-0">
                            <v-skeleton-loader type="text" :loading="true"></v-skeleton-loader>
                        </td>
                    </tr>

                    <tr v-if="!loading && allCoins.length < 1">
                        <td colspan="6">{{ tt('No crypto assets') }}</td>
                    </tr>

                    <tr :key="coin.coinId" v-for="coin in allCoins">
                        <td>
                            <div class="d-flex align-center">
                                <v-avatar size="24" class="me-2" v-if="coin.iconUrl">
                                    <v-img :src="coin.iconUrl" :alt="coin.symbol" />
                                </v-avatar>
                                <div class="d-flex flex-column">
                                    <span>{{ coin.name }}</span>
                                    <small class="text-caption">{{ coin.symbol }}</small>
                                </div>
                            </div>
                        </td>
                        <td class="text-right">{{ formatCryptoCount(coin.count) }}</td>
                        <td class="text-right">{{ coin.price || '-' }}</td>
                        <td class="text-right" :class="getPriceChangeColor(coin.priceChange1d)">{{ getDisplayPriceChange(coin.priceChange1d) || '-' }}</td>
                        <td class="text-right">{{ getDisplayShare(coin) || '-' }}</td>
                        <td class="text-right">{{ getDisplayValue(coin.value) || '-' }}</td>
                    </tr>
                    </tbody>
                </v-table>
            </v-card>
        </v-col>
    </v-row>

    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import SnackBar from '@/components/desktop/SnackBar.vue';

import { ref, computed, onMounted, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { useCryptoAssetsPageBase } from '@/views/base/CryptoAssetsPageBase.ts';

import { useCryptoAssetsStore } from '@/stores/cryptoAsset.ts';
import { useExchangeRatesStore } from '@/stores/exchangeRates.ts';

import { formatCryptoCount } from '@/models/crypto_asset.ts';

import { getCurrentUnixTime } from '@/lib/datetime.ts';

import {
    mdiRefresh,
    mdiCloudDownloadOutline
} from '@mdi/js';

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt, formatNumberToLocalizedNumerals } = useI18n();
const {
    allCoins,
    displayTotalValue,
    displayPricesUpdateTime,
    getDisplayValue,
    getDisplayPriceChange,
    getPriceChangeColor,
    getDisplayShare
} = useCryptoAssetsPageBase();

const cryptoAssetsStore = useCryptoAssetsStore();
const exchangeRatesStore = useExchangeRatesStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

const loading = ref<boolean>(true);
const refreshing = ref<boolean>(false);

const totalPriceChange1d = computed<string>(() => cryptoAssetsStore.totalPriceChange1d);
const lastErrorMessage = computed<string>(() => cryptoAssetsStore.lastErrorMessage);
const dataSourceConfigured = computed<boolean>(() => cryptoAssetsStore.dataSourceConfigured);

// the button is only disabled as a courtesy - the server is what enforces the interval and the
// daily allowance, so a stale clock here can never turn into a request it would have refused
const canRefreshPortfolio = computed<boolean>(() => {
    if (!dataSourceConfigured.value || cryptoAssetsStore.requestsRemainingToday <= 0) {
        return false;
    }

    return getCurrentUnixTime() >= cryptoAssetsStore.nextRefreshAvailableTime;
});

const refreshTooltip = computed<string>(() => {
    if (!dataSourceConfigured.value) {
        return tt('The crypto data source needs an api key and a portfolio share token before it can be read');
    }

    if (cryptoAssetsStore.requestsRemainingToday <= 0) {
        return tt('Today\'s portfolio update limit has been reached');
    }

    if (getCurrentUnixTime() < cryptoAssetsStore.nextRefreshAvailableTime) {
        return tt('The portfolio has just been updated, please try again later');
    }

    return tt('format.misc.cryptoPortfolioUpdatesRemaining', { count: formatNumberToLocalizedNumerals(cryptoAssetsStore.requestsRemainingToday) });
});

function reload(): void {
    loading.value = true;

    Promise.all([
        cryptoAssetsStore.loadCryptoPortfolio({ force: true }),
        exchangeRatesStore.getLatestExchangeRates({ silent: true, force: false })
    ]).then(() => {
        loading.value = false;
    }).catch(error => {
        loading.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function refreshPortfolio(): void {
    refreshing.value = true;

    cryptoAssetsStore.refreshCryptoPortfolio().then(() => {
        refreshing.value = false;
        snackbar.value?.showMessage('The crypto portfolio has been updated');
    }).catch(error => {
        refreshing.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

onMounted(() => {
    reload();
});
</script>
