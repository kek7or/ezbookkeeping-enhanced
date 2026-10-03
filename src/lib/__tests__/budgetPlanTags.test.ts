import { describe, expect, test } from 'vitest';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { sumAmountsByTag, sumAmountsByTagSet } from '@/lib/budgetPlan.ts';

describe('sumAmountsByTag', () => {
    test('counts a transaction in full under every tag it carries', () => {
        const totals = sumAmountsByTag([
            { tagIds: ['locked', 'anna'], amount: parseBigDecimal('63') },
            { tagIds: ['anna', 'anna'], amount: parseBigDecimal('10') },
            { tagIds: [], amount: parseBigDecimal('5') },
            { tagIds: ['anna'], amount: null }
        ]);

        expect(totals['locked']?.toString()).toBe('63');
        expect(totals['anna']?.toString()).toBe('73');
        expect(Object.keys(totals).length).toBe(2);
    });
});

describe('sumAmountsByTagSet', () => {
    test('keeps each combination of tags apart, whatever order the tags were put on in', () => {
        const totals = sumAmountsByTagSet([
            { tagIds: ['locked', 'anna'], amount: parseBigDecimal('63') },
            { tagIds: ['anna', 'locked'], amount: parseBigDecimal('72') },
            { tagIds: ['anna'], amount: parseBigDecimal('10') },
            { tagIds: ['locked', 'deleted-tag'], amount: parseBigDecimal('950') },
            { tagIds: [], amount: parseBigDecimal('5') }
        ], ['anna', 'locked']);

        expect(totals['anna,locked']?.toString()).toBe('135');
        expect(totals['anna']?.toString()).toBe('10');
        expect(totals['locked']?.toString()).toBe('950');
        expect(Object.keys(totals).length).toBe(3);
    });
});
