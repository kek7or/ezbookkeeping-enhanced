/**
 * Money handling. The server stores amounts as integer minor units and so do
 * we — a float never touches a monetary value anywhere in this app, because
 * 0.1 + 0.2 problems in a bookkeeping tool are silent and permanent.
 *
 * Two decimal places is assumed. ezbookkeeping supports zero-decimal currencies
 * (JPY, KRW) and three-decimal ones (KWD, BHD); supporting those means pulling
 * the server's per-currency decimal table, which v1 does not do.
 */

const MINOR_UNITS_PER_MAJOR = 100;

/**
 * Parses user input into minor units. Accepts "12", "12.3", "12.34" and the
 * comma decimal separator used across most of Europe.
 *
 * Returns null for anything it cannot parse exactly, so callers can refuse to
 * save rather than silently storing a wrong number.
 */
export function parseAmountToMinorUnits(input: string): number | null {
    const trimmed = input.trim().replace(',', '.');

    if (!trimmed) {
        return null;
    }

    if (!/^-?\d*(\.\d{0,2})?$/.test(trimmed) || trimmed === '.' || trimmed === '-') {
        return null;
    }

    const negative = trimmed.startsWith('-');
    const unsigned = negative ? trimmed.slice(1) : trimmed;
    const [whole, fraction = ''] = unsigned.split('.');

    // Built by string manipulation rather than `Math.round(value * 100)`, which
    // is exactly the float rounding this module exists to avoid.
    const wholePart = whole === '' ? 0 : Number.parseInt(whole, 10);
    const fractionPart = Number.parseInt(fraction.padEnd(2, '0'), 10);

    if (!Number.isFinite(wholePart) || !Number.isFinite(fractionPart)) {
        return null;
    }

    const total = wholePart * MINOR_UNITS_PER_MAJOR + fractionPart;
    return negative ? -total : total;
}

/** Formats minor units for display, e.g. 1234 -> "12.34". */
export function formatMinorUnits(amount: number): string {
    const negative = amount < 0;
    const absolute = Math.abs(amount);
    const whole = Math.floor(absolute / MINOR_UNITS_PER_MAJOR);
    const fraction = absolute % MINOR_UNITS_PER_MAJOR;

    return `${negative ? '-' : ''}${whole}.${String(fraction).padStart(2, '0')}`;
}

export function formatAmountWithCurrency(amount: number, currency: string): string {
    return currency ? `${formatMinorUnits(amount)} ${currency}` : formatMinorUnits(amount);
}
