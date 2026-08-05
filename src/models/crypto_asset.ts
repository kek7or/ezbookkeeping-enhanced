// CryptoPortfolioCoinResponse is one coin of the CoinStats portfolio, as the last snapshot read it
export interface CryptoPortfolioCoinResponse {
    readonly coinId: string;
    readonly symbol: string;
    readonly name: string;
    readonly iconUrl?: string;
    readonly count: string;
    readonly price?: string;
    readonly priceChange1d?: string;
    // value is in minor units of the value currency, and is null when the coin has no usable
    // price, so that a coin nobody priced is never read as a coin that is worth nothing
    readonly value: number | null;
}

// CryptoPortfolioResponse is everything the crypto page shows: the coins of the portfolio, what
// they add up to, and the state of the snapshot they were read from
export interface CryptoPortfolioResponse {
    readonly coins: CryptoPortfolioCoinResponse[];
    readonly valueCurrency: string;
    readonly totalValue: number;
    readonly totalValueIncomplete?: boolean;
    readonly totalPriceChange1d?: string;
    readonly pricesUpdateTime: number;
    readonly nextRefreshAvailableTime: number;
    readonly requestsRemainingToday: number;
    readonly lastErrorMessage?: string;
    readonly dataSourceConfigured: boolean;
}

// getCryptoCoinShare returns what fraction of the portfolio one coin is, as a percentage
//
// It is computed from the values the server already rounded to minor units, so the shares shown
// beside the rows are the shares of the numbers the reader can see and add up.
export function getCryptoCoinShare(value: number | null, totalValue: number): number | null {
    if (value === null || totalValue <= 0) {
        return null;
    }

    return value / totalValue * 100;
}

// formatCryptoCount returns a coin quantity without the trailing zeros the data source pads it
// with, so that "44.49870000" reads as the "44.4987" the portfolio actually holds
export function formatCryptoCount(count: string): string {
    if (count.indexOf('.') < 0) {
        return count;
    }

    return count.replace(/0+$/, '').replace(/\.$/, '');
}
