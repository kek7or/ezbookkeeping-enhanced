import { computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useUtilityMetersStore } from '@/stores/utilityMeter.ts';

import type {
    UtilityMeterInfoResponse,
    UtilityTariffInfoResponse,
    UtilityTariffCreateRequest
} from '@/models/utility_meter.ts';

import {
    UtilityMeterKind,
    UtilityMeterUnit,
    UTILITY_READING_VALUE_SCALE,
    UTILITY_UNIT_PRICE_SCALE,
    UTILITY_CONVERSION_FACTOR_SCALE,
    UTILITY_READING_VALUE_PRECISION,
    UTILITY_UNIT_PRICE_PRECISION,
    UTILITY_CONVERSION_FACTOR_PRECISION,
    getUtilityMeterUnitName,
    scaledValueToNumber,
    numberToScaledValue,
    getNumericDateFromDateString,
    getDateStringFromNumericDate,
    getUnixTimeFromNumericDate
} from '@/models/utility_meter.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { parseDateTimeFromUnixTime } from '@/lib/datetime.ts';

// UtilityTariffFormValue is a tariff as the fields of a form hold it: the numbers as they are
// written on the bill rather than as the server scales them, and the base fee still carrying which
// period it was quoted for
export interface UtilityTariffFormValue {
    startDate: string;
    unitPrice: number;
    baseFee: number;
    baseFeePerYear: boolean;
    calorificValue: number;
    stateNumber: number;
    monthlyPrepayment: number;
    comment: string;
}

// newUtilityTariffFormValue returns the tariff a form opens on: empty, beginning on the first of
// this month, with the base fee quoted the way a German bill quotes it
export function newUtilityTariffFormValue(): UtilityTariffFormValue {
    const now = new Date();
    const firstOfThisMonth = `${now.getFullYear()}-${(now.getMonth() + 1).toString().padStart(2, '0')}-01`;

    return {
        startDate: firstOfThisMonth,
        unitPrice: 0,
        baseFee: 0,
        baseFeePerYear: false,
        calorificValue: 0,
        stateNumber: 0,
        monthlyPrepayment: 0,
        comment: ''
    };
}

// toUtilityTariffFormValue returns a stored tariff as the form holds it.
//
// The base fee is stored for a year and nothing else, but it comes back per month whenever the year
// divides evenly into twelve - which is exactly when it was entered per month in the first place,
// off a bill that quotes it that way. It is the same money either way; this is only about handing
// back the figure the user typed rather than one they would have to divide to recognise.
export function toUtilityTariffFormValue(tariff: UtilityTariffInfoResponse): UtilityTariffFormValue {
    const perYear = tariff.baseFeeAnnual % 12 !== 0;

    return {
        startDate: getDateStringFromNumericDate(tariff.startDate),
        unitPrice: scaledValueToNumber(tariff.unitPrice, UTILITY_UNIT_PRICE_SCALE),
        baseFee: perYear ? tariff.baseFeeAnnual : tariff.baseFeeAnnual / 12,
        baseFeePerYear: perYear,
        calorificValue: scaledValueToNumber(tariff.calorificValue, UTILITY_CONVERSION_FACTOR_SCALE),
        stateNumber: scaledValueToNumber(tariff.stateNumber, UTILITY_CONVERSION_FACTOR_SCALE),
        monthlyPrepayment: tariff.monthlyPrepayment,
        comment: tariff.comment ?? ''
    };
}

// toUtilityTariffRequest returns what a form holds as the request the server takes
export function toUtilityTariffRequest(value: UtilityTariffFormValue): UtilityTariffCreateRequest {
    return {
        startDate: getNumericDateFromDateString(value.startDate),
        unitPrice: numberToScaledValue(value.unitPrice, UTILITY_UNIT_PRICE_SCALE),
        baseFeeAnnual: value.baseFeePerYear ? Math.round(value.baseFee) : Math.round(value.baseFee) * 12,
        calorificValue: numberToScaledValue(value.calorificValue, UTILITY_CONVERSION_FACTOR_SCALE),
        // a Zustandszahl with no Brennwert beside it converts nothing, and the server refuses the
        // pair rather than silently dropping one of them
        stateNumber: value.calorificValue > 0 ? numberToScaledValue(value.stateNumber, UTILITY_CONVERSION_FACTOR_SCALE) : 0,
        monthlyPrepayment: Math.round(value.monthlyPrepayment),
        comment: value.comment.trim()
    };
}

// isUtilityTariffFormValid says whether a tariff can be saved. A price of zero is allowed - a meter
// read for the use alone, before anybody has looked up what it costs, is still worth keeping - but
// a start date that is not a date is not.
export function isUtilityTariffFormValid(value: UtilityTariffFormValue): boolean {
    if (getNumericDateFromDateString(value.startDate) <= 0) {
        return false;
    }

    if (value.unitPrice < 0 || value.baseFee < 0 || value.monthlyPrepayment < 0) {
        return false;
    }

    return !(value.calorificValue <= 0 && value.stateNumber > 0);
}

export function useUtilityMetersPageBase() {
    const { tt, formatAmountToLocalizedNumeralsWithCurrency, formatNumberToLocalizedNumerals, formatDateTimeToLongDate } = useI18n();

    const utilityMetersStore = useUtilityMetersStore();

    const allMeters = computed<UtilityMeterInfoResponse[]>(() => utilityMetersStore.allMeters);

    function getDisplayAmount(amount: number, currency: string): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(amount), currency);
    }

    // a difference is shown with the sign it carries, because "you are 65.00 up" and "you owe
    // 65.00" are the two answers this page exists to tell apart
    function getDisplayDifference(difference: number, currency: string): string {
        const formatted = getDisplayAmount(Math.abs(difference), currency);
        return difference < 0 ? `-${formatted}` : `+${formatted}`;
    }

    function getDifferenceColor(difference: number): string {
        if (difference > 0) {
            return 'text-success';
        } else if (difference < 0) {
            return 'text-error';
        }

        return '';
    }

    // a reading and a use are shown to as many decimals as they actually carry, so that a meter
    // counting whole kilowatt hours does not print three zeros after every number
    function getDisplayScaledNumber(value: number, scale: number, precision: number): string {
        const number = scaledValueToNumber(value, scale);
        let text = number.toFixed(precision);

        if (text.indexOf('.') >= 0) {
            text = text.replace(/0+$/, '').replace(/\.$/, '');
        }

        const separatorIndex = text.indexOf('.');
        const decimals = separatorIndex >= 0 ? text.length - separatorIndex - 1 : 0;

        return formatNumberToLocalizedNumerals(number, decimals);
    }

    function getDisplayUsage(value: number, unit: UtilityMeterUnit): string {
        return `${getDisplayScaledNumber(value, UTILITY_READING_VALUE_SCALE, UTILITY_READING_VALUE_PRECISION)} ${getUtilityMeterUnitName(unit)}`;
    }

    function getDisplayEnergy(value: number): string {
        return `${getDisplayScaledNumber(value, UTILITY_READING_VALUE_SCALE, UTILITY_READING_VALUE_PRECISION)} kWh`;
    }

    function getDisplayReading(value: number, unit: UtilityMeterUnit): string {
        return getDisplayUsage(value, unit);
    }

    // the price of one unit needs more decimals than money is kept in - 0.3280 per kWh is four of
    // them, and a price rounded to the cent would be a fifth of a cent out on every kilowatt hour
    function getDisplayUnitPrice(tariff: UtilityTariffInfoResponse): string {
        return getDisplayScaledNumber(tariff.unitPrice, UTILITY_UNIT_PRICE_SCALE, UTILITY_UNIT_PRICE_PRECISION);
    }

    function getDisplayConversionFactor(value: number | undefined): string {
        return getDisplayScaledNumber(value ?? 0, UTILITY_CONVERSION_FACTOR_SCALE, UTILITY_CONVERSION_FACTOR_PRECISION);
    }

    function getDisplayDate(numericDate: number): string {
        if (!numericDate) {
            return '';
        }

        return formatDateTimeToLongDate(parseDateTimeFromUnixTime(getUnixTimeFromNumericDate(numericDate)));
    }

    function getMeterKindName(kind: UtilityMeterKind): string {
        switch (kind) {
            case UtilityMeterKind.Electricity:
                return tt('Electricity');
            case UtilityMeterKind.Gas:
                return tt('Gas');
            case UtilityMeterKind.Water:
                return tt('Water');
            case UtilityMeterKind.Heating:
                return tt('Heating');
            default:
                return tt('Other');
        }
    }

    return {
        allMeters,
        getDisplayAmount,
        getDisplayDifference,
        getDifferenceColor,
        getDisplayUsage,
        getDisplayEnergy,
        getDisplayReading,
        getDisplayUnitPrice,
        getDisplayConversionFactor,
        getDisplayDate,
        getMeterKindName
    };
}
