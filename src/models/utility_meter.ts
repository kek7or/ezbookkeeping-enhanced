// The scales the server keeps the numbers of a meter at. Every number below crosses the wire as the
// whole number it is stored as, because a tariff sent as a float stops being the number the bill
// printed, and these numbers are checked against a bill.
export const UTILITY_READING_VALUE_SCALE: number = 1000;
export const UTILITY_UNIT_PRICE_SCALE: number = 1000000;
export const UTILITY_CONVERSION_FACTOR_SCALE: number = 10000;

// How many decimals each of them is worth showing. A reading is shown to three because a water
// meter counts litres, a price to four because that is how a German tariff is quoted, and a
// Brennwert to four because that is how it is printed.
export const UTILITY_READING_VALUE_PRECISION: number = 3;
export const UTILITY_UNIT_PRICE_PRECISION: number = 4;
export const UTILITY_CONVERSION_FACTOR_PRECISION: number = 4;

// UtilityMeterKind is what a meter measures. It decides how the meter is labelled and nothing about
// what it costs.
export enum UtilityMeterKind {
    Electricity = 1,
    Gas = 2,
    Water = 3,
    Heating = 4,
    Other = 5
}

// UtilityMeterUnit is what the dial of a meter counts in
export enum UtilityMeterUnit {
    KilowattHour = 1,
    CubicMetre = 2,
    Litre = 3
}

export interface UtilityTariffInfoResponse {
    readonly id: string;
    readonly meterId: string;
    // startDate is the first day this price applies to, as a YYYYMMDD number
    readonly startDate: number;
    readonly unitPrice: number;
    readonly baseFeeAnnual: number;
    readonly calorificValue?: number;
    readonly stateNumber?: number;
    readonly monthlyPrepayment: number;
    readonly comment?: string;
}

export interface UtilityReadingInfoResponse {
    readonly id: string;
    readonly meterId: string;
    readonly readingDate: number;
    readonly value: number;
    readonly estimated?: boolean;
    readonly comment?: string;
}

// UtilityPeriodResponse is what happened between two readings, as the server worked it out
export interface UtilityPeriodResponse {
    readonly startDate: number;
    readonly endDate: number;
    readonly days: number;
    readonly startValue: number;
    readonly endValue: number;
    readonly usedUnits: number;
    readonly usedEnergy?: number;
    readonly energyCost: number;
    readonly baseFee: number;
    readonly cost: number;
    readonly prepaid: number;
    // difference is what was paid less what it cost: a credit while it is positive, and money owed
    // while it is not
    readonly difference: number;
    readonly estimated?: boolean;
    // partial says this row is a piece of a reading interval, cut where the billing year ended
    readonly partial?: boolean;
    readonly priceChanged?: boolean;
}

export interface UtilityBillingYearResponse {
    readonly label: string;
    readonly startDate: number;
    readonly endDate: number;
    readonly complete: boolean;
    readonly periods: UtilityPeriodResponse[];
    readonly usedUnits: number;
    readonly usedEnergy?: number;
    readonly cost: number;
    readonly prepaid: number;
    readonly difference: number;
    readonly daysCovered: number;
    readonly daysInYear: number;
    readonly projectedCost: number;
    readonly projectedPrepaid: number;
    readonly projectedDifference: number;
}

export interface UtilityMeterInfoResponse {
    readonly id: string;
    readonly name: string;
    readonly kind: UtilityMeterKind;
    readonly unit: UtilityMeterUnit;
    readonly currency: string;
    readonly billingYearStartMonth: number;
    readonly meterNumber?: string;
    readonly comment?: string;
    readonly displayOrder: number;
    readonly tariffs: UtilityTariffInfoResponse[];
    readonly readings: UtilityReadingInfoResponse[];
    readonly billingYears: UtilityBillingYearResponse[];
}

export interface UtilityTariffCreateRequest {
    readonly meterId?: string;
    readonly startDate: number;
    readonly unitPrice: number;
    readonly baseFeeAnnual: number;
    readonly calorificValue: number;
    readonly stateNumber: number;
    readonly monthlyPrepayment: number;
    readonly comment: string;
}

export interface UtilityTariffModifyRequest extends UtilityTariffCreateRequest {
    readonly id: string;
}

export interface UtilityMeterCreateRequest {
    readonly name: string;
    readonly kind: UtilityMeterKind;
    readonly unit: UtilityMeterUnit;
    readonly currency: string;
    readonly billingYearStartMonth: number;
    readonly meterNumber: string;
    readonly comment: string;
    readonly tariff?: UtilityTariffCreateRequest;
}

export interface UtilityMeterModifyRequest {
    readonly id: string;
    readonly name: string;
    readonly kind: UtilityMeterKind;
    readonly unit: UtilityMeterUnit;
    readonly currency: string;
    readonly billingYearStartMonth: number;
    readonly meterNumber: string;
    readonly comment: string;
}

export interface UtilityReadingCreateRequest {
    readonly meterId: string;
    readonly readingDate: number;
    readonly value: number;
    readonly estimated: boolean;
    readonly comment: string;
}

export interface UtilityReadingModifyRequest {
    readonly id: string;
    readonly readingDate: number;
    readonly value: number;
    readonly estimated: boolean;
    readonly comment: string;
}

export interface UtilityIdRequest {
    readonly id: string;
}

// isUtilityMeterConverted says whether what this meter counts has to be turned into energy before a
// price can be put on it - a gas meter counts cubic metres and gas is sold by the kilowatt hour
export function isUtilityMeterConverted(meter: UtilityMeterInfoResponse): boolean {
    return meter.unit !== UtilityMeterUnit.KilowattHour;
}

// getUtilityMeterUnitName returns the unit a meter counts in, as it is written beside a number
export function getUtilityMeterUnitName(unit: UtilityMeterUnit): string {
    switch (unit) {
        case UtilityMeterUnit.CubicMetre:
            return 'm³';
        case UtilityMeterUnit.Litre:
            return 'l';
        default:
            return 'kWh';
    }
}

// scaledValueToNumber turns one of the whole numbers the server stores back into the number it
// stands for
export function scaledValueToNumber(value: number | undefined, scale: number): number {
    if (!value) {
        return 0;
    }

    return value / scale;
}

// numberToScaledValue turns a number the user typed into the whole number the server stores.
//
// It rounds rather than truncates, because a price typed with more decimals than are kept should
// become the nearest price that can be kept and not the one below it.
export function numberToScaledValue(value: number | undefined, scale: number): number {
    if (!value || !isFinite(value)) {
        return 0;
    }

    return Math.round(value * scale);
}

// getNumericDateFromDateString turns the YYYY-MM-DD a date field holds into the YYYYMMDD number the
// server speaks in, and returns zero for anything that is not a date
export function getNumericDateFromDateString(dateString: string): number {
    const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(dateString);

    if (!match) {
        return 0;
    }

    return parseInt(match[1] as string, 10) * 10000 + parseInt(match[2] as string, 10) * 100 + parseInt(match[3] as string, 10);
}

// getDateStringFromNumericDate turns a YYYYMMDD number into the YYYY-MM-DD a date field holds
export function getDateStringFromNumericDate(numericDate: number): string {
    if (!numericDate) {
        return '';
    }

    const year = Math.floor(numericDate / 10000);
    const month = Math.floor((numericDate % 10000) / 100);
    const day = numericDate % 100;

    return `${year.toString().padStart(4, '0')}-${month.toString().padStart(2, '0')}-${day.toString().padStart(2, '0')}`;
}

// getUnixTimeFromNumericDate turns a YYYYMMDD number into the instant the day began, which is what
// the date formatters of this application take.
//
// The day is built in the local timezone rather than UTC, because it is the day itself that is
// being shown and a midnight converted out of UTC can land on the day before.
export function getUnixTimeFromNumericDate(numericDate: number): number {
    if (!numericDate) {
        return 0;
    }

    const year = Math.floor(numericDate / 10000);
    const month = Math.floor((numericDate % 10000) / 100);
    const day = numericDate % 100;

    return new Date(year, month - 1, day).getTime() / 1000;
}

// getLatestUtilityReading returns the most recent reading of a meter, which the readings arrive
// sorted newest first for
export function getLatestUtilityReading(meter: UtilityMeterInfoResponse): UtilityReadingInfoResponse | null {
    return meter.readings.length > 0 ? (meter.readings[0] as UtilityReadingInfoResponse) : null;
}

// getCurrentUtilityTariff returns the tariff of a meter that is in force today, which is the newest
// one that has already begun. A meter whose only tariff starts in the future shows that one, since
// a price nobody is paying yet is still the only price on file.
export function getCurrentUtilityTariff(meter: UtilityMeterInfoResponse): UtilityTariffInfoResponse | null {
    if (meter.tariffs.length < 1) {
        return null;
    }

    const now = new Date();
    const today = now.getFullYear() * 10000 + (now.getMonth() + 1) * 100 + now.getDate();

    for (const tariff of meter.tariffs) {
        if (tariff.startDate <= today) {
            return tariff;
        }
    }

    return meter.tariffs[meter.tariffs.length - 1] as UtilityTariffInfoResponse;
}

// getCurrentUtilityBillingYear returns the billing year a meter is in the middle of, which is the
// newest one it has periods for
export function getCurrentUtilityBillingYear(meter: UtilityMeterInfoResponse): UtilityBillingYearResponse | null {
    return meter.billingYears.length > 0 ? (meter.billingYears[0] as UtilityBillingYearResponse) : null;
}
