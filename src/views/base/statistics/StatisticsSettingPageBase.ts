import { computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useSettingsStore } from '@/stores/setting.ts';
import { useStatisticsStore } from '@/stores/statistics.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';

import type { TypeAndDisplayName } from '@/core/base.ts';
import { type LocalizedDateRange, DateRangeScene } from '@/core/datetime.ts';
import { StatisticsAnalysisType } from '@/core/statistics.ts';
import { CategoryType } from '@/core/category.ts';

import { objectFieldToArrayItem, arrayItemToObjectField } from '@/lib/common.ts';

export interface PaycheckCategoryOption {
    readonly id: string;
    readonly displayName: string;
}

export function useStatisticsSettingPageBase() {
    const {
        getAllDateRanges,
        getAllTimezoneTypesUsedForStatistics,
        getAllKeywordMatchModes,
        getAllCategoricalChartTypes,
        getAllTrendChartTypes,
        getAllStatisticsChartDataTypes,
        getAllStatisticsSortingTypes
    } = useI18n();

    const settingsStore = useSettingsStore();
    const statisticsStore = useStatisticsStore();
    const transactionCategoriesStore = useTransactionCategoriesStore();

    const allChartDataTypes = computed<TypeAndDisplayName[]>(() => getAllStatisticsChartDataTypes(StatisticsAnalysisType.CategoricalAnalysis));
    const allTimezoneTypesUsedForStatistics = computed<TypeAndDisplayName[]>(() => getAllTimezoneTypesUsedForStatistics());
    const allKeywordMatchModes = computed<TypeAndDisplayName[]>(() => getAllKeywordMatchModes());
    const allSortingTypes = computed<TypeAndDisplayName[]>(() => getAllStatisticsSortingTypes());
    const allCategoricalChartTypes = computed<TypeAndDisplayName[]>(() => getAllCategoricalChartTypes());
    const allCategoricalChartDateRanges = computed<LocalizedDateRange[]>(() => getAllDateRanges(DateRangeScene.Normal, {}));
    const allTrendChartTypes = computed<TypeAndDisplayName[]>(() => getAllTrendChartTypes());
    const allTrendChartDateRanges = computed<LocalizedDateRange[]>(() => getAllDateRanges(DateRangeScene.TrendAnalysis, {}));
    const allAssetTrendsChartDateRanges = computed<LocalizedDateRange[]>(() => getAllDateRanges(DateRangeScene.AssetTrends, {}));

    // a paycheck is recorded against the category it was booked in, which is a secondary category
    // whenever the primary one has any, so only the categories transactions can use are offered here
    const allPaycheckCategories = computed<PaycheckCategoryOption[]>(() => {
        const allOptions: PaycheckCategoryOption[] = [];
        const incomeCategories = transactionCategoriesStore.allTransactionCategories[CategoryType.Income] || [];

        for (const category of incomeCategories) {
            if (category.subCategories && category.subCategories.length) {
                for (const subCategory of category.subCategories) {
                    allOptions.push({
                        id: subCategory.id,
                        displayName: `${category.name} / ${subCategory.name}`
                    });
                }
            } else {
                allOptions.push({
                    id: category.id,
                    displayName: category.name
                });
            }
        }

        return allOptions;
    });

    const paycheckCategoryIds = computed<string[]>({
        get: () => objectFieldToArrayItem(settingsStore.appSettings.statistics.paycheckCategoryIds || {}),
        set: (value: string[]) => {
            settingsStore.setStatisticsPaycheckCategoryIds(arrayItemToObjectField(value, true));
            // the pay periods were built from the categories that were selected until now
            statisticsStore.updatePaycheckPeriodsInvalidState(true);
            statisticsStore.updateTransactionStatisticsInvalidState(true);
        }
    });

    const defaultChartDataType = computed<number>({
        get: () => settingsStore.appSettings.statistics.defaultChartDataType,
        set: (value: number) => settingsStore.setStatisticsDefaultChartDataType(value)
    });

    const defaultTimezoneType = computed<number>({
        get: () => settingsStore.appSettings.statistics.defaultTimezoneType,
        set: (value: number) => settingsStore.setStatisticsDefaultTimezoneType(value)
    });

    const defaultKeywordMatchMode = computed<number>({
        get: () => settingsStore.appSettings.statistics.defaultKeywordMatchMode,
        set: (value: number) => settingsStore.setStatisticsDefaultKeywordMatchMode(value)
    });

    const defaultSortingType = computed<number>({
        get: () => settingsStore.appSettings.statistics.defaultSortingType,
        set: (value: number) => settingsStore.setStatisticsSortingType(value)
    });

    const defaultCategoricalChartType = computed<number>({
        get: () => settingsStore.appSettings.statistics.defaultCategoricalChartType,
        set: (value: number) => settingsStore.setStatisticsDefaultCategoricalChartType(value)
    });

    const defaultCategoricalChartDateRange = computed<number>({
        get: () => settingsStore.appSettings.statistics.defaultCategoricalChartDataRangeType,
        set: (value: number) => settingsStore.setStatisticsDefaultCategoricalChartDateRange(value)
    });

    const defaultTrendChartType = computed<number>({
        get: () => settingsStore.appSettings.statistics.defaultTrendChartType,
        set: (value: number) => settingsStore.setStatisticsDefaultTrendChartType(value)
    });

    const defaultTrendChartDateRange = computed<number>({
        get: () => settingsStore.appSettings.statistics.defaultTrendChartDataRangeType,
        set: (value: number) => settingsStore.setStatisticsDefaultTrendChartDateRange(value)
    });

    const defaultAssetTrendsChartType = computed<number>({
        get: () => settingsStore.appSettings.statistics.defaultAssetTrendsChartType,
        set: (value: number) => settingsStore.setStatisticsDefaultAssetTrendsChartType(value)
    });

    const defaultAssetTrendsChartDateRange = computed<number>({
        get: () => settingsStore.appSettings.statistics.defaultAssetTrendsChartDataRangeType,
        set: (value: number) => settingsStore.setStatisticsDefaultAssetTrendsChartDateRange(value)
    });

    return {
        // computed states
        allChartDataTypes,
        allTimezoneTypesUsedForStatistics,
        allKeywordMatchModes,
        allSortingTypes,
        allCategoricalChartTypes,
        allCategoricalChartDateRanges,
        allTrendChartTypes,
        allTrendChartDateRanges,
        allAssetTrendsChartDateRanges,
        allPaycheckCategories,
        paycheckCategoryIds,
        defaultChartDataType,
        defaultTimezoneType,
        defaultKeywordMatchMode,
        defaultSortingType,
        defaultCategoricalChartType,
        defaultCategoricalChartDateRange,
        defaultTrendChartType,
        defaultTrendChartDateRange,
        defaultAssetTrendsChartType,
        defaultAssetTrendsChartDateRange
    };
}
