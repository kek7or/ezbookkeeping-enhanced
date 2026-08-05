import { describe, expect, test } from 'vitest';

import type { CryptoPortfolioCoinResponse } from '@/models/crypto_asset.ts';
import { getCryptoCoinShare, formatCryptoCount } from '@/models/crypto_asset.ts';

function createCoin(coinId: string, count: string, value: number | null): CryptoPortfolioCoinResponse {
    return {
        coinId: coinId,
        symbol: coinId.toUpperCase(),
        name: coinId,
        count: count,
        value: value
    };
}

describe('getCryptoCoinShare', () => {
    test('reports what fraction of the portfolio a coin is', () => {
        const coins = [
            createCoin('bitcoin', '0.3421', 1479608),
            createCoin('ethereum', '2', 493203)
        ];

        const totalValue = 1479608 + 493203;

        expect(getCryptoCoinShare(coins[0]!.value, totalValue)).toBeCloseTo(75.0, 1);
        expect(getCryptoCoinShare(coins[1]!.value, totalValue)).toBeCloseTo(25.0, 1);
    });

    test('a coin with no price has no share rather than a share of zero', () => {
        const coin = createCoin('unpriced-coin', '10', null);

        expect(getCryptoCoinShare(coin.value, 1479608)).toBeNull();
    });

    test('an empty portfolio has no shares', () => {
        expect(getCryptoCoinShare(0, 0)).toBeNull();
    });
});

describe('formatCryptoCount', () => {
    test('drops the padding zeros the data source adds', () => {
        expect(formatCryptoCount('44.49870000')).toBe('44.4987');
        expect(formatCryptoCount('1.00000000')).toBe('1');
        expect(formatCryptoCount('0.10')).toBe('0.1');
    });

    test('leaves a whole quantity alone', () => {
        expect(formatCryptoCount('1000000000')).toBe('1000000000');
        expect(formatCryptoCount('0')).toBe('0');
    });

    test('keeps every significant digit of a tiny quantity', () => {
        expect(formatCryptoCount('0.00000001')).toBe('0.00000001');
        expect(formatCryptoCount('0.123456789012345678')).toBe('0.123456789012345678');
    });
});
