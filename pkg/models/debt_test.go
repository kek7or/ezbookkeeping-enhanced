package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitAmountEvenly(t *testing.T) {
	assert.Equal(t, []int64{1000, 1000, 1000}, SplitAmountEvenly(3000, 3))
	assert.Equal(t, []int64{334, 333, 333}, SplitAmountEvenly(1000, 3))
	assert.Equal(t, []int64{334, 334, 333}, SplitAmountEvenly(1001, 3))
	assert.Equal(t, []int64{699}, SplitAmountEvenly(699, 1))
	assert.Equal(t, []int64{0, 0, 0}, SplitAmountEvenly(0, 3))
	assert.Equal(t, []int64{}, SplitAmountEvenly(3000, 0))
}

func TestSplitAmountEvenly_SharesAddUpToWhatWasDivided(t *testing.T) {
	for amount := int64(0); amount < 200; amount++ {
		for shareCount := 1; shareCount <= 7; shareCount++ {
			shares := SplitAmountEvenly(amount, shareCount)
			total := int64(0)

			for i := 0; i < len(shares); i++ {
				total += shares[i]
			}

			assert.Equal(t, shareCount, len(shares), "amount %d over %d shares", amount, shareCount)
			assert.Equal(t, amount, total, "amount %d over %d shares", amount, shareCount)
		}
	}
}

func TestResplitEvenly_DividedAmongThePeopleAlone(t *testing.T) {
	// 30,00 owed by three people, the one who paid ate none of it. One of them off it leaves 15,00
	// each rather than the 10,00 they were told before.
	newShares, isEvenSplit := ResplitEvenly(3000, []int64{1000, 1000, 1000}, 2)

	assert.True(t, isEvenSplit)
	assert.Equal(t, []int64{1500, 1500}, newShares)
}

func TestResplitEvenly_DividedWithTheOneWhoPaid(t *testing.T) {
	// 30,00 shared by three friends and the one who paid, 7,50 apiece. One friend off it is three
	// heads left, the payer among them, so the friends owe 10,00 each and the payer keeps 10,00.
	newShares, isEvenSplit := ResplitEvenly(3000, []int64{750, 750, 750}, 2)

	assert.True(t, isEvenSplit)
	assert.Equal(t, []int64{1000, 1000}, newShares)
}

func TestResplitEvenly_LeavesTheRoundingCentWithTheOneWhoPaid(t *testing.T) {
	// 10,00 between three friends and the payer is 250 each; two friends left is 334 for the payer
	// and 333 apiece for them
	newShares, isEvenSplit := ResplitEvenly(1000, []int64{250, 250, 250}, 2)

	assert.True(t, isEvenSplit)
	assert.Equal(t, []int64{333, 333}, newShares)
}

func TestResplitEvenly_RecognizesASplitThatDidNotDivideEvenly(t *testing.T) {
	// 10,00 between three people alone is 334, 333, 333 - shares that differ by a cent are still an
	// even division of it
	newShares, isEvenSplit := ResplitEvenly(1000, []int64{334, 333, 333}, 2)

	assert.True(t, isEvenSplit)
	assert.Equal(t, []int64{500, 500}, newShares)
}

func TestResplitEvenly_LastOneLeftOwesTheWholeThing(t *testing.T) {
	newShares, isEvenSplit := ResplitEvenly(3000, []int64{1500, 1500}, 1)

	assert.True(t, isEvenSplit)
	assert.Equal(t, []int64{3000}, newShares)

	// the payer was counted in, so the one left owes half of it and the payer keeps the other half
	newShares, isEvenSplit = ResplitEvenly(3000, []int64{1000, 1000}, 1)

	assert.True(t, isEvenSplit)
	assert.Equal(t, []int64{1500}, newShares)
}

func TestResplitEvenly_SeveralSharesGoneAtOnce(t *testing.T) {
	newShares, isEvenSplit := ResplitEvenly(3000, []int64{750, 750, 750}, 1)

	assert.True(t, isEvenSplit)
	assert.Equal(t, []int64{1500}, newShares)
}

func TestResplitEvenly_LeavesAmountsSetByHandAlone(t *testing.T) {
	// two people owing 5,00 apiece of a 30,00 transaction were never sharing it out evenly - the
	// numbers were put there by hand and say what they were meant to say
	_, isEvenSplit := ResplitEvenly(3000, []int64{500, 500}, 1)

	assert.False(t, isEvenSplit)

	// the same thing attached whole to two people is two people each owing all of it, not two
	// shares of it
	_, isEvenSplit = ResplitEvenly(3000, []int64{3000, 3000}, 1)

	assert.False(t, isEvenSplit)
}

func TestResplitEvenly_NothingToDivide(t *testing.T) {
	// nobody left on it
	_, isEvenSplit := ResplitEvenly(3000, []int64{1500, 1500}, 0)

	assert.False(t, isEvenSplit)

	// nothing was taken off it
	_, isEvenSplit = ResplitEvenly(3000, []int64{1500, 1500}, 2)

	assert.False(t, isEvenSplit)

	// a transaction that says it cost nothing cannot be divided by anybody
	_, isEvenSplit = ResplitEvenly(0, []int64{0, 0}, 1)

	assert.False(t, isEvenSplit)
}

func TestResplitEvenly_KeepsTheSharesAddingUpToTheThing(t *testing.T) {
	for totalAmount := int64(1); totalAmount < 200; totalAmount++ {
		for shareCount := 2; shareCount <= 6; shareCount++ {
			for payerShares := 0; payerShares <= 1; payerShares++ {
				oldShares := SplitAmountEvenly(totalAmount, shareCount+payerShares)[payerShares:]
				newShares, isEvenSplit := ResplitEvenly(totalAmount, oldShares, shareCount-1)

				assert.True(t, isEvenSplit, "%d over %d shares, payer %d", totalAmount, shareCount, payerShares)

				owed := int64(0)

				for i := 0; i < len(newShares); i++ {
					owed += newShares[i]
				}

				// what the people owe plus what the payer keeps is the whole thing, and the payer
				// keeps a share only where they were counted in to begin with
				expectedKept := int64(0)

				if payerShares > 0 {
					expectedKept = SplitAmountEvenly(totalAmount, shareCount-1+payerShares)[0]
				}

				assert.Equal(t, shareCount-1, len(newShares), "%d over %d shares, payer %d", totalAmount, shareCount, payerShares)
				assert.Equal(t, totalAmount-expectedKept, owed, "%d over %d shares, payer %d", totalAmount, shareCount, payerShares)
			}
		}
	}
}
