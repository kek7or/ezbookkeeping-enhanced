package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

const testCryptoNow = int64(1785269520) // 2026-07-28 in UTC
const testCryptoToday = int32(20260728)

func testCryptoConfig() *settings.Config {
	return &settings.Config{
		CryptoPriceCacheLifetime: 86400,
		CryptoMinRefreshInterval: 300,
		CryptoMaxRequestsPerDay:  4,
	}
}

func TestDecideCryptoRefresh_FreshCacheIsNotRefreshed(t *testing.T) {
	state := &models.CryptoDataSourceState{
		LastPriceRefreshTime: testCryptoNow - 7200, // 2 hours old
		LastAttemptTime:      testCryptoNow - 7200,
		RequestCountDate:     testCryptoToday,
		RequestCountToday:    1,
	}

	assert.Equal(t, cryptoRefreshNotNeeded, decideCryptoRefresh(state, testCryptoNow, testCryptoToday, false, false, testCryptoConfig()))
}

func TestDecideCryptoRefresh_ExpiredCacheIsRefreshed(t *testing.T) {
	state := &models.CryptoDataSourceState{
		LastPriceRefreshTime: testCryptoNow - 90000, // 25 hours old
		LastAttemptTime:      testCryptoNow - 90000,
		RequestCountDate:     testCryptoToday,
		RequestCountToday:    1,
	}

	assert.Equal(t, cryptoRefreshAllowed, decideCryptoRefresh(state, testCryptoNow, testCryptoToday, false, false, testCryptoConfig()))
}

func TestDecideCryptoRefresh_MissingSnapshotIsRefreshedBeforeTheCacheExpires(t *testing.T) {
	// on the very first visit there is nothing cached to show, and waiting until tomorrow to
	// fetch the portfolio would make the page look broken
	state := &models.CryptoDataSourceState{
		LastPriceRefreshTime: testCryptoNow - 7200,
		LastAttemptTime:      testCryptoNow - 7200,
		RequestCountDate:     testCryptoToday,
		RequestCountToday:    1,
	}

	assert.Equal(t, cryptoRefreshAllowed, decideCryptoRefresh(state, testCryptoNow, testCryptoToday, false, true, testCryptoConfig()))
}

func TestDecideCryptoRefresh_RecentAttemptBlocksEveryRefresh(t *testing.T) {
	state := &models.CryptoDataSourceState{
		LastPriceRefreshTime: testCryptoNow - 90000,
		LastAttemptTime:      testCryptoNow - 60, // one minute ago
		RequestCountDate:     testCryptoToday,
		RequestCountToday:    1,
	}

	// the manual button, the expired cache and a missing snapshot all stand down
	assert.Equal(t, cryptoRefreshTooFrequent, decideCryptoRefresh(state, testCryptoNow, testCryptoToday, true, false, testCryptoConfig()))
	assert.Equal(t, cryptoRefreshTooFrequent, decideCryptoRefresh(state, testCryptoNow, testCryptoToday, false, false, testCryptoConfig()))
	assert.Equal(t, cryptoRefreshTooFrequent, decideCryptoRefresh(state, testCryptoNow, testCryptoToday, false, true, testCryptoConfig()))
}

func TestDecideCryptoRefresh_DailyLimitBlocksTheManualButton(t *testing.T) {
	state := &models.CryptoDataSourceState{
		LastPriceRefreshTime: testCryptoNow - 90000,
		LastAttemptTime:      testCryptoNow - 90000,
		RequestCountDate:     testCryptoToday,
		RequestCountToday:    4,
	}

	assert.Equal(t, cryptoRefreshLimitExceeded, decideCryptoRefresh(state, testCryptoNow, testCryptoToday, true, false, testCryptoConfig()))
	assert.Equal(t, cryptoRefreshLimitExceeded, decideCryptoRefresh(state, testCryptoNow, testCryptoToday, false, false, testCryptoConfig()))
}

func TestDecideCryptoRefresh_FreshCacheIsNotAnErrorWhenTheLimitIsExhausted(t *testing.T) {
	// nothing is wanted, so nothing is reported - opening the page after four refreshes must not
	// look like a failure
	state := &models.CryptoDataSourceState{
		LastPriceRefreshTime: testCryptoNow - 60,
		LastAttemptTime:      testCryptoNow - 60,
		RequestCountDate:     testCryptoToday,
		RequestCountToday:    4,
	}

	assert.Equal(t, cryptoRefreshNotNeeded, decideCryptoRefresh(state, testCryptoNow, testCryptoToday, false, false, testCryptoConfig()))
}

func TestDecideCryptoRefresh_CountFromAnotherDayIsNotCounted(t *testing.T) {
	state := &models.CryptoDataSourceState{
		LastPriceRefreshTime: testCryptoNow - 90000,
		LastAttemptTime:      testCryptoNow - 90000,
		RequestCountDate:     testCryptoToday - 1,
		RequestCountToday:    4,
	}

	assert.Equal(t, cryptoRefreshAllowed, decideCryptoRefresh(state, testCryptoNow, testCryptoToday, true, false, testCryptoConfig()))
}

func TestDecideCryptoRefresh_FirstEverRefresh(t *testing.T) {
	state := &models.CryptoDataSourceState{Id: models.CryptoDataSourceStateId}

	assert.Equal(t, cryptoRefreshAllowed, decideCryptoRefresh(state, testCryptoNow, testCryptoToday, false, false, testCryptoConfig()))
}

func TestCryptoRequestsRemainingToday_CountsOnlyToday(t *testing.T) {
	today := cryptoUtcDate(1785269520)

	state := &models.CryptoDataSourceState{
		RequestCountDate:  today,
		RequestCountToday: 3,
	}

	assert.Equal(t, int32(1), cryptoRequestsRemainingTodayAt(state, today, 4))

	state.RequestCountDate = today - 1
	assert.Equal(t, int32(4), cryptoRequestsRemainingTodayAt(state, today, 4))

	state.RequestCountDate = today
	state.RequestCountToday = 9
	assert.Equal(t, int32(0), cryptoRequestsRemainingTodayAt(state, today, 4))
}

func TestCryptoUtcDate(t *testing.T) {
	assert.Equal(t, int32(20260728), cryptoUtcDate(1785269520))
	assert.Equal(t, int32(19700101), cryptoUtcDate(0))
}
