import { ref, computed } from 'vue';
import { defineStore } from 'pinia';

import type { CryptoPortfolioCoinResponse, CryptoPortfolioResponse } from '@/models/crypto_asset.ts';

import logger from '@/lib/logger.ts';
import services from '@/lib/services.ts';

export const useCryptoAssetsStore = defineStore('cryptoAssets', () => {
    const cryptoPortfolio = ref<CryptoPortfolioResponse | null>(null);
    const cryptoPortfolioStateInvalid = ref<boolean>(true);

    const allCryptoCoins = computed<CryptoPortfolioCoinResponse[]>(() => cryptoPortfolio.value?.coins ?? []);
    const valueCurrency = computed<string>(() => cryptoPortfolio.value?.valueCurrency ?? 'USD');
    const totalValue = computed<number>(() => cryptoPortfolio.value?.totalValue ?? 0);
    const totalValueIncomplete = computed<boolean>(() => cryptoPortfolio.value?.totalValueIncomplete ?? false);
    const totalPriceChange1d = computed<string>(() => cryptoPortfolio.value?.totalPriceChange1d ?? '');
    const pricesUpdateTime = computed<number>(() => cryptoPortfolio.value?.pricesUpdateTime ?? 0);
    const nextRefreshAvailableTime = computed<number>(() => cryptoPortfolio.value?.nextRefreshAvailableTime ?? 0);
    const requestsRemainingToday = computed<number>(() => cryptoPortfolio.value?.requestsRemainingToday ?? 0);
    const lastErrorMessage = computed<string>(() => cryptoPortfolio.value?.lastErrorMessage ?? '');
    const dataSourceConfigured = computed<boolean>(() => cryptoPortfolio.value?.dataSourceConfigured ?? false);

    function resetCryptoAssets(): void {
        cryptoPortfolio.value = null;
        cryptoPortfolioStateInvalid.value = true;
    }

    function setCryptoPortfolio(result: CryptoPortfolioResponse): void {
        cryptoPortfolio.value = result;
        cryptoPortfolioStateInvalid.value = false;
    }

    function loadCryptoPortfolio({ force }: { force: boolean }): Promise<CryptoPortfolioResponse> {
        if (!force && !cryptoPortfolioStateInvalid.value && cryptoPortfolio.value) {
            return Promise.resolve(cryptoPortfolio.value);
        }

        return new Promise((resolve, reject) => {
            services.getCryptoPortfolio().then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to retrieve crypto portfolio' });
                    return;
                }

                setCryptoPortfolio(data.result);
                resolve(data.result);
            }).catch(error => {
                logger.error('failed to load crypto portfolio', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to retrieve crypto portfolio' });
                } else {
                    reject(error);
                }
            });
        });
    }

    // refreshCryptoPortfolio asks the server to spend one request on the crypto data source. The
    // server is what enforces the interval and the daily allowance, so a rejection here is an
    // answer and not a failure to retry.
    function refreshCryptoPortfolio(): Promise<CryptoPortfolioResponse> {
        return new Promise((resolve, reject) => {
            services.refreshCryptoPortfolio().then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to refresh crypto portfolio' });
                    return;
                }

                setCryptoPortfolio(data.result);
                resolve(data.result);
            }).catch(error => {
                logger.error('failed to refresh crypto portfolio', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to refresh crypto portfolio' });
                } else {
                    reject(error);
                }
            });
        });
    }

    return {
        // states
        cryptoPortfolio,
        cryptoPortfolioStateInvalid,
        // computed states
        allCryptoCoins,
        valueCurrency,
        totalValue,
        totalValueIncomplete,
        totalPriceChange1d,
        pricesUpdateTime,
        nextRefreshAvailableTime,
        requestsRemainingToday,
        lastErrorMessage,
        dataSourceConfigured,
        // functions
        resetCryptoAssets,
        loadCryptoPortfolio,
        refreshCryptoPortfolio
    };
});
