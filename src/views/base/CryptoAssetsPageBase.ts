import { computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useUserStore } from '@/stores/user.ts';
import { useExchangeRatesStore } from '@/stores/exchangeRates.ts';
import { useCryptoAssetsStore } from '@/stores/cryptoAsset.ts';

import type { BigDecimal, BigDecimalWithSuffix } from '@/core/numeral.ts';
import { INCOMPLETE_AMOUNT_SUFFIX } from '@/consts/numeral.ts';

import type { CryptoPortfolioCoinResponse } from '@/models/crypto_asset.ts';
import { getCryptoCoinShare } from '@/models/crypto_asset.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { parseDateTimeFromUnixTime } from '@/lib/datetime.ts';

export function useCryptoAssetsPageBase() {
    const { tt, formatAmountToLocalizedNumeralsWithCurrency, formatPercentToLocalizedNumerals, formatDateTimeToLongDateTime } = useI18n();

    const userStore = useUserStore();
    const exchangeRatesStore = useExchangeRatesStore();
    const cryptoAssetsStore = useCryptoAssetsStore();

    const defaultCurrency = computed<string>(() => userStore.currentUserDefaultCurrency);
    const valueCurrency = computed<string>(() => cryptoAssetsStore.valueCurrency);
    const allCoins = computed<CryptoPortfolioCoinResponse[]>(() => cryptoAssetsStore.allCryptoCoins);

    // a crypto value is quoted in the currency the portfolio was read in, and reaches the user's
    // default currency through the same conversion an account in a foreign currency goes through
    function getExchangedValue(value: number): BigDecimal | null {
        const amount = parseBigDecimal(value);

        if (valueCurrency.value === defaultCurrency.value) {
            return amount;
        }

        const exchangedAmount = exchangeRatesStore.getExchangedAmount(amount, valueCurrency.value, defaultCurrency.value);

        return exchangedAmount ? exchangedAmount.truncate() : null;
    }

    function getDisplayValue(value: number | null, incomplete?: boolean): string {
        if (value === null) {
            return '';
        }

        const exchangedValue = getExchangedValue(value);

        // a value that could not be converted is shown in the currency it was priced in rather
        // than dropped, so that the number is never missing just because a rate is
        if (exchangedValue === null) {
            return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(value), valueCurrency.value);
        }

        if (incomplete) {
            const valueWithSuffix: BigDecimalWithSuffix = {
                value: exchangedValue,
                suffix: INCOMPLETE_AMOUNT_SUFFIX
            };

            return formatAmountToLocalizedNumeralsWithCurrency(valueWithSuffix, defaultCurrency.value);
        }

        return formatAmountToLocalizedNumeralsWithCurrency(exchangedValue, defaultCurrency.value);
    }

    const displayTotalValue = computed<string>(() => getDisplayValue(cryptoAssetsStore.totalValue, cryptoAssetsStore.totalValueIncomplete));

    const displayPricesUpdateTime = computed<string>(() => {
        if (!cryptoAssetsStore.pricesUpdateTime) {
            return tt('Never');
        }

        return formatDateTimeToLongDateTime(parseDateTimeFromUnixTime(cryptoAssetsStore.pricesUpdateTime));
    });

    function getDisplayPriceChange(priceChange?: string): string {
        if (!priceChange) {
            return '';
        }

        const value = parseFloat(priceChange);

        if (isNaN(value)) {
            return '';
        }

        return (value > 0 ? '+' : '') + formatPercentToLocalizedNumerals(value, 2, '<0.01');
    }

    function getPriceChangeColor(priceChange?: string): string {
        if (!priceChange) {
            return '';
        }

        const value = parseFloat(priceChange);

        if (isNaN(value) || value === 0) {
            return '';
        }

        return value > 0 ? 'text-success' : 'text-error';
    }

    function getDisplayShare(coin: CryptoPortfolioCoinResponse): string {
        const share = getCryptoCoinShare(coin.value, cryptoAssetsStore.totalValue);

        if (share === null) {
            return '';
        }

        return formatPercentToLocalizedNumerals(share, 2, '<0.01');
    }

    return {
        // computed states
        defaultCurrency,
        valueCurrency,
        allCoins,
        displayTotalValue,
        displayPricesUpdateTime,
        // functions
        getDisplayValue,
        getDisplayPriceChange,
        getPriceChangeColor,
        getDisplayShare
    };
}
