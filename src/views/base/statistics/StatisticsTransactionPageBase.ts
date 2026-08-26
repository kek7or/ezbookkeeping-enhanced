import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useSettingsStore } from '@/stores/setting.ts';
import { useUserStore } from '@/stores/user.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { type TransactionStatisticsFilter, useStatisticsStore } from '@/stores/statistics.ts';

import type { TypeAndDisplayName } from '@/core/base.ts';
import type { BigDecimal } from '@/core/numeral.ts';
import { type LocalizedDateRange, type WeekDayValue, DateRangeScene, DateRange } from '@/core/datetime.ts';
import type { ColorStyleValue } from '@/core/color.ts';
import type { PaycheckPeriod } from '@/core/paycheck.ts';
import {
    StatisticsAnalysisType,
    ChartDataType,
    ChartSortingType,
    ChartDateAggregationType,
    TrendChartType
} from '@/core/statistics.ts';

import { DISPLAY_HIDDEN_AMOUNT } from '@/consts/numeral.ts';

import type {
    TransactionCategoricalOverviewAnalysisData,
    TransactionCategoricalAnalysisData,
    TransactionCategoricalAnalysisDataItem,
    TransactionTrendsAnalysisData,
    TransactionAssetTrendsAnalysisData
} from '@/models/transaction.ts';

import { limitText, findNameByType, findDisplayNameByType } from '@/lib/common.ts';
import {
    parseDateTimeFromUnixTime,
    getYearMonthFirstUnixTime,
    getYearMonthLastUnixTime,
    getCurrentUnixTime
} from '@/lib/datetime.ts';
import { parseBigDecimal } from '@/lib/numeral.ts';
import { findPaycheckPeriodIndex, getPaycheckPeriodRemainingDays } from '@/lib/paycheck.ts';
import { getDisplayColor, getCategoryDisplayColor, getAccountDisplayColor } from '@/lib/color.ts';

export function useStatisticsTransactionPageBase() {
    const {
        tt,
        getAllDateRanges,
        getAllStatisticsSortingTypes,
        getAllStatisticsDateAggregationTypes,
        formatDateTimeToLongDate,
        formatDateTimeToLongDateTime,
        formatDateTimeToGregorianLikeLongYearMonth,
        formatDateRange,
        formatAmountToLocalizedNumeralsWithCurrency
    } = useI18n();

    const settingsStore = useSettingsStore();
    const userStore = useUserStore();
    const accountsStore = useAccountsStore();
    const statisticsStore = useStatisticsStore();

    const loading = ref<boolean>(true);
    const analysisType = ref<StatisticsAnalysisType>(StatisticsAnalysisType.CategoricalAnalysis);
    const trendDateAggregationType = ref<number>(ChartDateAggregationType.Default.type);
    const assetTrendsDateAggregationType = ref<number>(ChartDateAggregationType.Default.type);

    const showAccountBalance = computed<boolean>(() => settingsStore.showAccountBalance);
    const hideAmount = computed<boolean>(() => !settingsStore.showAmount);
    const defaultCurrency = computed<string>(() => userStore.currentUserDefaultCurrency);
    const firstDayOfWeek = computed<WeekDayValue>(() => userStore.currentUserFirstDayOfWeek);
    const fiscalYearStart = computed<number>(() => userStore.currentUserFiscalYearStart);

    const allDateRanges = computed<LocalizedDateRange[]>(() => {
        if (analysisType.value === StatisticsAnalysisType.CategoricalAnalysis) {
            return getAllDateRanges(DateRangeScene.Normal, { includeCustom: true });
        } else if (analysisType.value === StatisticsAnalysisType.TrendAnalysis) {
            return getAllDateRanges(DateRangeScene.TrendAnalysis, { includeCustom: true });
        } else if (analysisType.value === StatisticsAnalysisType.AssetTrends) {
            return getAllDateRanges(DateRangeScene.AssetTrends, { includeCustom: true });
        } else {
            // the paycheck analysis offers the pay periods read from the paycheck transactions
            // rather than calendar date ranges, so the page builds that menu itself
            return [];
        }
    });
    const allSortingTypes = computed<TypeAndDisplayName[]>(() => getAllStatisticsSortingTypes());
    const allTrendAnalysisDateAggregationTypes = computed<TypeAndDisplayName[]>(() => getAllStatisticsDateAggregationTypes(StatisticsAnalysisType.TrendAnalysis, false));
    const allAssetTrendsDateAggregationTypes = computed<TypeAndDisplayName[]>(() => getAllStatisticsDateAggregationTypes(StatisticsAnalysisType.AssetTrends, false));

    const query = computed<TransactionStatisticsFilter>(() => statisticsStore.transactionStatisticsFilter);

    const paycheckPeriods = computed<PaycheckPeriod[]>(() => statisticsStore.paycheckPeriods);

    const selectedPaycheckPeriodIndex = computed<number>(() => findPaycheckPeriodIndex(paycheckPeriods.value, query.value.paycheckChartStartTime, query.value.paycheckChartEndTime));

    const selectedPaycheckPeriod = computed<PaycheckPeriod | null>(() => paycheckPeriods.value[selectedPaycheckPeriodIndex.value] ?? null);

    const selectedPaycheckAmount = computed<string>(() => {
        const period = selectedPaycheckPeriod.value;

        if (!period) {
            return '';
        }

        const account = accountsStore.allAccountsMap[period.paycheckAccountId];
        return getDisplayAmount(parseBigDecimal(period.paycheckAmount), account ? account.currency : defaultCurrency.value);
    });

    const selectedPaycheckDate = computed<string>(() => {
        const period = selectedPaycheckPeriod.value;
        return period ? formatDateTimeToLongDate(parseDateTimeFromUnixTime(period.paycheckTime)) : '';
    });

    const paycheckPeriodRemainingDays = computed<number>(() => {
        const period = selectedPaycheckPeriod.value;
        return period ? getPaycheckPeriodRemainingDays(period, getCurrentUnixTime()) : 0;
    });

    const queryChartDataCategory = computed<string>(() => statisticsStore.categoricalAnalysisChartDataCategory);
    const queryDateType = computed<number | null>(() => {
        if (analysisType.value === StatisticsAnalysisType.CategoricalAnalysis) {
            return query.value.categoricalChartDateType;
        } else if (analysisType.value === StatisticsAnalysisType.TrendAnalysis) {
            return query.value.trendChartDateType;
        } else if (analysisType.value === StatisticsAnalysisType.AssetTrends) {
            return query.value.assetTrendsChartDateType;
        } else if (analysisType.value === StatisticsAnalysisType.PaycheckAnalysis) {
            return DateRange.Custom.type;
        } else {
            return null;
        }
    });

    const queryStartTime = computed<string>(() => {
        if (analysisType.value === StatisticsAnalysisType.CategoricalAnalysis) {
            return formatDateTimeToLongDateTime(parseDateTimeFromUnixTime(query.value.categoricalChartStartTime));
        } else if (analysisType.value === StatisticsAnalysisType.TrendAnalysis) {
            return formatDateTimeToGregorianLikeLongYearMonth(parseDateTimeFromUnixTime(getYearMonthFirstUnixTime(query.value.trendChartStartYearMonth)));
        } else if (analysisType.value === StatisticsAnalysisType.AssetTrends) {
            return formatDateTimeToLongDateTime(parseDateTimeFromUnixTime(query.value.assetTrendsChartStartTime));
        } else if (analysisType.value === StatisticsAnalysisType.PaycheckAnalysis) {
            return formatDateTimeToLongDateTime(parseDateTimeFromUnixTime(query.value.paycheckChartStartTime));
        } else {
            return '';
        }
    });

    const queryEndTime = computed<string>(() => {
        if (analysisType.value === StatisticsAnalysisType.CategoricalAnalysis) {
            return formatDateTimeToLongDateTime(parseDateTimeFromUnixTime(query.value.categoricalChartEndTime));
        } else if (analysisType.value === StatisticsAnalysisType.TrendAnalysis) {
            return formatDateTimeToGregorianLikeLongYearMonth(parseDateTimeFromUnixTime(getYearMonthLastUnixTime(query.value.trendChartEndYearMonth)));
        } else if (analysisType.value === StatisticsAnalysisType.AssetTrends) {
            return formatDateTimeToLongDateTime(parseDateTimeFromUnixTime(query.value.assetTrendsChartEndTime));
        } else if (analysisType.value === StatisticsAnalysisType.PaycheckAnalysis) {
            return formatDateTimeToLongDateTime(parseDateTimeFromUnixTime(query.value.paycheckChartEndTime));
        } else {
            return '';
        }
    });

    const queryDateRangeName = computed<string>(() => {
        if (analysisType.value === StatisticsAnalysisType.CategoricalAnalysis) {
            if (query.value.chartDataType === ChartDataType.AccountTotalAssets.type ||
                query.value.chartDataType === ChartDataType.AccountTotalLiabilities.type) {
                return tt(DateRange.All.name);
            }

            return formatDateRange(query.value.categoricalChartDateType, query.value.categoricalChartStartTime, query.value.categoricalChartEndTime);
        } else if (analysisType.value === StatisticsAnalysisType.TrendAnalysis) {
            return formatDateRange(query.value.trendChartDateType, getYearMonthFirstUnixTime(query.value.trendChartStartYearMonth), getYearMonthLastUnixTime(query.value.trendChartEndYearMonth));
        } else if (analysisType.value === StatisticsAnalysisType.AssetTrends) {
            return formatDateRange(query.value.assetTrendsChartDateType, query.value.assetTrendsChartStartTime, query.value.assetTrendsChartEndTime);
        } else if (analysisType.value === StatisticsAnalysisType.PaycheckAnalysis) {
            if (!query.value.paycheckChartStartTime || !query.value.paycheckChartEndTime) {
                return tt('Pay Period');
            }

            if (selectedPaycheckPeriod.value && selectedPaycheckPeriod.value.isCurrent) {
                return tt('Since Last Paycheck');
            }

            return formatDateRange(DateRange.Custom.type, query.value.paycheckChartStartTime, query.value.paycheckChartEndTime);
        } else {
            return '';
        }
    });

    const queryChartDataTypeName = computed<string>(() => {
        const queryChartDataTypeName = findNameByType(ChartDataType.values(), query.value.chartDataType) || 'Statistics';
        return tt(queryChartDataTypeName);
    });

    const querySortingTypeName = computed<string>(() => {
        const querySortingTypeName = findNameByType(ChartSortingType.values(), query.value.sortingType) || 'System Default';
        return tt(querySortingTypeName);
    });

    const queryTrendDateAggregationTypeName = computed<string>(() => findDisplayNameByType(allTrendAnalysisDateAggregationTypes.value, trendDateAggregationType.value) || '');
    const queryAssetTrendsDateAggregationTypeName = computed<string>(() => findDisplayNameByType(allAssetTrendsDateAggregationTypes.value, assetTrendsDateAggregationType.value) || '');

    const isQueryDateRangeChanged = computed<boolean>(() => {
        if (analysisType.value === StatisticsAnalysisType.CategoricalAnalysis) {
            if (query.value.chartDataType === ChartDataType.AccountTotalAssets.type ||
                query.value.chartDataType === ChartDataType.AccountTotalLiabilities.type) {
                return false;
            }

            if (query.value.categoricalChartDateType === settingsStore.appSettings.statistics.defaultCategoricalChartDataRangeType) {
                return false;
            }

            return !!query.value.categoricalChartStartTime || !!query.value.categoricalChartEndTime;
        } else if (analysisType.value === StatisticsAnalysisType.TrendAnalysis) {
            if (query.value.trendChartDateType === settingsStore.appSettings.statistics.defaultTrendChartDataRangeType) {
                return false;
            }

            return !!query.value.trendChartStartYearMonth || !!query.value.trendChartEndYearMonth;
        } else if (analysisType.value === StatisticsAnalysisType.AssetTrends) {
            if (query.value.assetTrendsChartDateType === settingsStore.appSettings.statistics.defaultAssetTrendsChartDataRangeType) {
                return false;
            }

            return !!query.value.assetTrendsChartStartTime || !!query.value.assetTrendsChartEndTime;
        } else if (analysisType.value === StatisticsAnalysisType.PaycheckAnalysis) {
            return !!selectedPaycheckPeriod.value && !selectedPaycheckPeriod.value.isCurrent;
        } else {
            return false;
        }
    });

    const canChangeDateRange = computed<boolean>(() => {
        if (analysisType.value === StatisticsAnalysisType.CategoricalAnalysis) {
            if (query.value.chartDataType === ChartDataType.AccountTotalAssets.type || query.value.chartDataType === ChartDataType.AccountTotalLiabilities.type) {
                return false;
            }

            return true;
        } else {
            return true;
        }
    });

    const canShiftDateRange = computed<boolean>(() => {
        if (!canChangeDateRange.value) {
            return false;
        }

        if (analysisType.value === StatisticsAnalysisType.CategoricalAnalysis) {
            return query.value.categoricalChartDateType !== DateRange.All.type;
        } else if (analysisType.value === StatisticsAnalysisType.TrendAnalysis) {
            return query.value.trendChartDateType !== DateRange.All.type;
        } else if (analysisType.value === StatisticsAnalysisType.AssetTrends) {
            return query.value.assetTrendsChartDateType !== DateRange.All.type;
        } else if (analysisType.value === StatisticsAnalysisType.PaycheckAnalysis) {
            return paycheckPeriods.value.length > 1 && selectedPaycheckPeriodIndex.value >= 0;
        } else {
            return false;
        }
    });

    const canUseCategoryFilter = computed<boolean>(() => {
        if (analysisType.value === StatisticsAnalysisType.CategoricalAnalysis) {
            if (query.value.chartDataType === ChartDataType.AccountTotalAssets.type || query.value.chartDataType === ChartDataType.AccountTotalLiabilities.type) {
                return false;
            }
        } else if (analysisType.value === StatisticsAnalysisType.AssetTrends) {
            return false;
        }

        return true;
    });

    const canUseServerCustomFilter = computed<boolean>(() => {
        if (analysisType.value === StatisticsAnalysisType.CategoricalAnalysis) {
            if (query.value.chartDataType === ChartDataType.AccountTotalAssets.type || query.value.chartDataType === ChartDataType.AccountTotalLiabilities.type) {
                return false;
            }
        } else if (analysisType.value === StatisticsAnalysisType.AssetTrends) {
            return false;
        }

        return true;
    });

    const canUseTagFilter = computed<boolean>(() => {
        return canUseServerCustomFilter.value;
    });

    const canUseKeywordFilter = computed<boolean>(() => {
        return canUseServerCustomFilter.value;
    });

    const showAmountInChart = computed<boolean>(() => {
        if (hideAmount.value) {
            return false;
        }

        if (!showAccountBalance.value) {
            if (analysisType.value === StatisticsAnalysisType.CategoricalAnalysis
                && (query.value.chartDataType === ChartDataType.AccountTotalAssets.type || query.value.chartDataType === ChartDataType.AccountTotalLiabilities.type)) {
                return false;
            }
        }

        return true;
    });

    const totalAmountName = computed<string>(() => {
        if (query.value.chartDataType === ChartDataType.InflowsByAccount.type) {
            return tt('Total Inflows');
        } else if (query.value.chartDataType === ChartDataType.OutflowsByAccount.type) {
            return tt('Total Outflows');
        } else if (query.value.chartDataType === ChartDataType.IncomeByAccount.type
            || query.value.chartDataType === ChartDataType.IncomeByPrimaryCategory.type
            || query.value.chartDataType === ChartDataType.IncomeBySecondaryCategory.type) {
            return tt('Total Income');
        } else if (query.value.chartDataType === ChartDataType.ExpenseByAccount.type
            || query.value.chartDataType === ChartDataType.ExpenseByPrimaryCategory.type
            || query.value.chartDataType === ChartDataType.ExpenseBySecondaryCategory.type) {
            return tt('Total Expense');
        } else if (query.value.chartDataType === ChartDataType.AccountTotalAssets.type) {
            return tt('Total Assets');
        } else if (query.value.chartDataType === ChartDataType.AccountTotalLiabilities.type) {
            return tt('Total Liabilities');
        }

        return tt('Total Amount');
    });

    const showPercentInCategoricalChart = computed<boolean>(() => {
        return query.value.chartDataType !== ChartDataType.OutflowsByAccount.type &&
            query.value.chartDataType !== ChartDataType.InflowsByAccount.type;
    });

    const showTotalAmountInTrendsChart = computed<boolean>(() => {
        return query.value.chartDataType !== ChartDataType.OutflowsByAccount.type &&
            query.value.chartDataType !== ChartDataType.InflowsByAccount.type &&
            query.value.chartDataType !== ChartDataType.TotalOutflows.type &&
            query.value.chartDataType !== ChartDataType.TotalExpense.type &&
            query.value.chartDataType !== ChartDataType.TotalInflows.type &&
            query.value.chartDataType !== ChartDataType.TotalIncome.type &&
            query.value.chartDataType !== ChartDataType.NetCashFlow.type &&
            query.value.chartDataType !== ChartDataType.NetIncome.type &&
            query.value.chartDataType !== ChartDataType.NetWorth.type;
    });

    const showStackedInTrendsChart = computed<boolean>(() => {
        return (query.value.trendChartType === TrendChartType.Area.type || query.value.trendChartType === TrendChartType.Column.type) &&
            query.value.chartDataType !== ChartDataType.OutflowsByAccount.type &&
            query.value.chartDataType !== ChartDataType.InflowsByAccount.type;
    });

    const translateNameInTrendsChart = computed<boolean>(() => {
        return query.value.chartDataType === ChartDataType.TotalOutflows.type ||
            query.value.chartDataType === ChartDataType.TotalExpense.type ||
            query.value.chartDataType === ChartDataType.TotalInflows.type ||
            query.value.chartDataType === ChartDataType.TotalIncome.type ||
            query.value.chartDataType === ChartDataType.NetCashFlow.type ||
            query.value.chartDataType === ChartDataType.NetIncome.type ||
            query.value.chartDataType === ChartDataType.NetWorth.type;
    });

    const categoricalOverviewAnalysisData = computed<TransactionCategoricalOverviewAnalysisData | null>(() => statisticsStore.categoricalOverviewAnalysisData);
    const categoricalAnalysisData = computed<TransactionCategoricalAnalysisData>(() => statisticsStore.categoricalAnalysisData);
    const trendsAnalysisData = computed<TransactionTrendsAnalysisData | null>(() => statisticsStore.trendsAnalysisData);
    const assetTrendsData = computed<TransactionAssetTrendsAnalysisData | null>(() => statisticsStore.assetTrendsData);

    function canShowCustomDateRange(dateRangeType: number): boolean {
        if (analysisType.value === StatisticsAnalysisType.CategoricalAnalysis) {
            return query.value.categoricalChartDateType === dateRangeType && !!query.value.categoricalChartStartTime && !!query.value.categoricalChartEndTime;
        } else if (analysisType.value === StatisticsAnalysisType.TrendAnalysis) {
            return query.value.trendChartDateType === dateRangeType && !!query.value.trendChartStartYearMonth && !!query.value.trendChartEndYearMonth;
        } else if (analysisType.value === StatisticsAnalysisType.AssetTrends) {
            return query.value.assetTrendsChartDateType === dateRangeType && !!query.value.assetTrendsChartStartTime && !!query.value.assetTrendsChartEndTime;
        } else {
            return false;
        }
    }

    function getPaycheckPeriodDisplayName(period: PaycheckPeriod): string {
        return formatDateRange(DateRange.Custom.type, period.startTime, period.endTime);
    }

    function getTransactionCategoricalAnalysisDataItemDisplayColor(item: TransactionCategoricalAnalysisDataItem): ColorStyleValue {
        if (item.type === 'category') {
            return getCategoryDisplayColor(item.color);
        } else if (item.type === 'account') {
            return getAccountDisplayColor(item.color);
        } else {
            return getDisplayColor(item.color);
        }
    }

    function getDisplayAmount(amount: BigDecimal, currency: string, textLimit?: number): string {
        if (hideAmount.value) {
            return formatAmountToLocalizedNumeralsWithCurrency(DISPLAY_HIDDEN_AMOUNT, currency);
        }

        const finalAmount = formatAmountToLocalizedNumeralsWithCurrency(amount, currency);

        if (!showAccountBalance.value) {
            if (analysisType.value === StatisticsAnalysisType.CategoricalAnalysis
                && (query.value.chartDataType === ChartDataType.AccountTotalAssets.type || query.value.chartDataType === ChartDataType.AccountTotalLiabilities.type)) {
                return DISPLAY_HIDDEN_AMOUNT;
            }
        }

        if (textLimit) {
            return limitText(finalAmount, textLimit);
        }

        return finalAmount;
    }

    return {
        // states
        loading,
        analysisType,
        trendDateAggregationType,
        assetTrendsDateAggregationType,
        // computed states
        showAccountBalance,
        defaultCurrency,
        firstDayOfWeek,
        fiscalYearStart,
        allDateRanges,
        allSortingTypes,
        allTrendAnalysisDateAggregationTypes,
        allAssetTrendsDateAggregationTypes,
        query,
        queryChartDataCategory,
        queryDateType,
        queryStartTime,
        queryEndTime,
        queryDateRangeName,
        queryChartDataTypeName,
        querySortingTypeName,
        queryTrendDateAggregationTypeName,
        queryAssetTrendsDateAggregationTypeName,
        isQueryDateRangeChanged,
        canChangeDateRange,
        canShiftDateRange,
        canUseCategoryFilter,
        canUseTagFilter,
        canUseKeywordFilter,
        showAmountInChart,
        totalAmountName,
        showPercentInCategoricalChart,
        showTotalAmountInTrendsChart,
        showStackedInTrendsChart,
        translateNameInTrendsChart,
        categoricalOverviewAnalysisData,
        categoricalAnalysisData,
        trendsAnalysisData,
        assetTrendsData,
        paycheckPeriods,
        selectedPaycheckPeriodIndex,
        selectedPaycheckPeriod,
        selectedPaycheckAmount,
        selectedPaycheckDate,
        paycheckPeriodRemainingDays,
        // functions
        canShowCustomDateRange,
        getPaycheckPeriodDisplayName,
        getTransactionCategoricalAnalysisDataItemDisplayColor,
        getDisplayAmount
    };
}
