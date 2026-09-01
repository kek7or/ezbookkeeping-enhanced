package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// utcPlusTwo is the kind of offset that makes a schedule set to midnight local sit on the previous
// UTC day, which is where the day a template recurs on and the UTC day it is reckoned from part ways
var utcPlusTwo = time.FixedZone("UTC+2", 2*60*60)

func newScheduledTemplate(frequencyType models.TransactionScheduleFrequencyType, scheduledAtMinuteInUtc int16, utcOffsetMinutes int16) *models.TransactionTemplate {
	return &models.TransactionTemplate{
		TemplateId:                 1,
		Uid:                        1,
		TemplateType:               models.TRANSACTION_TEMPLATE_TYPE_SCHEDULE,
		Type:                       models.TRANSACTION_TYPE_EXPENSE,
		ScheduledFrequencyType:     frequencyType,
		ScheduledAt:                scheduledAtMinuteInUtc,
		ScheduledTimezoneUtcOffset: utcOffsetMinutes,
	}
}

func TestIsScheduledTemplateDueAt_Monthly(t *testing.T) {
	template := newScheduledTemplate(models.TRANSACTION_SCHEDULE_FREQUENCY_TYPE_MONTHLY, 1320, 120)

	assert.True(t, isScheduledTemplateDueAt(template, []int64{1}, time.Date(2026, 9, 1, 0, 0, 0, 0, utcPlusTwo)))
	assert.False(t, isScheduledTemplateDueAt(template, []int64{1}, time.Date(2026, 8, 31, 0, 0, 0, 0, utcPlusTwo)))
	assert.True(t, isScheduledTemplateDueAt(template, []int64{1, 15}, time.Date(2026, 9, 15, 0, 0, 0, 0, utcPlusTwo)))
}

// A negative day counts back from the end of the month, and which day that is depends on the month
// the occurrence falls in rather than on the month the server happens to be in when it is posted
func TestIsScheduledTemplateDueAt_MonthlyCountingBackFromEndOfMonth(t *testing.T) {
	template := newScheduledTemplate(models.TRANSACTION_SCHEDULE_FREQUENCY_TYPE_MONTHLY, 1320, 120)

	assert.True(t, isScheduledTemplateDueAt(template, []int64{-1}, time.Date(2026, 2, 28, 0, 0, 0, 0, utcPlusTwo)))
	assert.False(t, isScheduledTemplateDueAt(template, []int64{-1}, time.Date(2026, 2, 27, 0, 0, 0, 0, utcPlusTwo)))
	assert.True(t, isScheduledTemplateDueAt(template, []int64{-1}, time.Date(2026, 3, 31, 0, 0, 0, 0, utcPlusTwo)))
	assert.False(t, isScheduledTemplateDueAt(template, []int64{-1}, time.Date(2026, 3, 28, 0, 0, 0, 0, utcPlusTwo)))
	assert.True(t, isScheduledTemplateDueAt(template, []int64{-1}, time.Date(2024, 2, 29, 0, 0, 0, 0, utcPlusTwo)))
}

func TestIsScheduledTemplateDueAt_WeeklyDailyAndYearly(t *testing.T) {
	weekly := newScheduledTemplate(models.TRANSACTION_SCHEDULE_FREQUENCY_TYPE_WEEKLY, 1320, 120)
	assert.True(t, isScheduledTemplateDueAt(weekly, []int64{int64(time.Tuesday)}, time.Date(2026, 9, 1, 0, 0, 0, 0, utcPlusTwo)))
	assert.False(t, isScheduledTemplateDueAt(weekly, []int64{int64(time.Monday)}, time.Date(2026, 9, 1, 0, 0, 0, 0, utcPlusTwo)))

	daily := newScheduledTemplate(models.TRANSACTION_SCHEDULE_FREQUENCY_TYPE_DAILY, 1320, 120)
	assert.True(t, isScheduledTemplateDueAt(daily, []int64{0}, time.Date(2026, 9, 1, 0, 0, 0, 0, utcPlusTwo)))
	assert.True(t, isScheduledTemplateDueAt(daily, []int64{0}, time.Date(2026, 9, 2, 0, 0, 0, 0, utcPlusTwo)))

	yearly := newScheduledTemplate(models.TRANSACTION_SCHEDULE_FREQUENCY_TYPE_YEARLY, 1320, 120)
	assert.True(t, isScheduledTemplateDueAt(yearly, []int64{901}, time.Date(2026, 9, 1, 0, 0, 0, 0, utcPlusTwo)))
	assert.False(t, isScheduledTemplateDueAt(yearly, []int64{901}, time.Date(2026, 9, 2, 0, 0, 0, 0, utcPlusTwo)))
}

func TestIsScheduledTemplateDueAt_EveryNDays(t *testing.T) {
	template := newScheduledTemplate(models.TRANSACTION_SCHEDULE_FREQUENCY_TYPE_EVERY_N_DAYS, 1320, 120)
	startTime := time.Date(2026, 8, 1, 0, 0, 0, 0, utcPlusTwo).Unix()
	template.ScheduledStartTime = &startTime

	assert.True(t, isScheduledTemplateDueAt(template, []int64{10}, time.Date(2026, 8, 11, 0, 0, 0, 0, utcPlusTwo)))
	assert.True(t, isScheduledTemplateDueAt(template, []int64{10}, time.Date(2026, 8, 21, 0, 0, 0, 0, utcPlusTwo)))
	assert.False(t, isScheduledTemplateDueAt(template, []int64{10}, time.Date(2026, 8, 12, 0, 0, 0, 0, utcPlusTwo)))
	assert.False(t, isScheduledTemplateDueAt(template, []int64{10}, time.Date(2026, 7, 22, 0, 0, 0, 0, utcPlusTwo)))

	// An interval with nothing to count from, or no interval at all, recurs on no day
	withoutStart := newScheduledTemplate(models.TRANSACTION_SCHEDULE_FREQUENCY_TYPE_EVERY_N_DAYS, 1320, 120)
	assert.False(t, isScheduledTemplateDueAt(withoutStart, []int64{10}, time.Date(2026, 8, 11, 0, 0, 0, 0, utcPlusTwo)))
	assert.False(t, isScheduledTemplateDueAt(template, []int64{0}, time.Date(2026, 8, 11, 0, 0, 0, 0, utcPlusTwo)))
}

func TestIsScheduledTemplateDueAt_DisabledRecursOnNoDay(t *testing.T) {
	template := newScheduledTemplate(models.TRANSACTION_SCHEDULE_FREQUENCY_TYPE_DISABLED, 1320, 120)

	assert.False(t, isScheduledTemplateDueAt(template, []int64{1}, time.Date(2026, 9, 1, 0, 0, 0, 0, utcPlusTwo)))
}

// The case the button exists for: a monthly schedule due on the first, noticed on the evening of the
// first because the server was down when it should have posted
func TestFindMostRecentScheduledOccurrence_MissedToday(t *testing.T) {
	template := newScheduledTemplate(models.TRANSACTION_SCHEDULE_FREQUENCY_TYPE_MONTHLY, 1320, 120)
	now := time.Date(2026, 9, 1, 19, 40, 0, 0, utcPlusTwo).Unix()

	occurrence, found := findMostRecentScheduledOccurrence(template, []int64{1}, false, now)

	assert.True(t, found)
	assert.Equal(t, time.Date(2026, 9, 1, 0, 0, 0, 0, utcPlusTwo).Unix(), occurrence)
}

// Noticed two days late, it still lands on the day the money moved rather than on the day it was noticed
func TestFindMostRecentScheduledOccurrence_MissedDaysAgo(t *testing.T) {
	template := newScheduledTemplate(models.TRANSACTION_SCHEDULE_FREQUENCY_TYPE_MONTHLY, 1320, 120)
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, utcPlusTwo).Unix()

	occurrence, found := findMostRecentScheduledOccurrence(template, []int64{1}, false, now)

	assert.True(t, found)
	assert.Equal(t, time.Date(2026, 9, 1, 0, 0, 0, 0, utcPlusTwo).Unix(), occurrence)
}

// Pressed before the day's occurrence has come round, the one before it is the most recent
func TestFindMostRecentScheduledOccurrence_TodaysOccurrenceNotYetDue(t *testing.T) {
	template := newScheduledTemplate(models.TRANSACTION_SCHEDULE_FREQUENCY_TYPE_DAILY, 1320, 120)
	now := time.Date(2026, 9, 3, 0, 30, 0, 0, utcPlusTwo).Unix()

	occurrence, found := findMostRecentScheduledOccurrence(template, []int64{0}, false, now)

	assert.True(t, found)
	assert.Equal(t, time.Date(2026, 9, 3, 0, 0, 0, 0, utcPlusTwo).Unix(), occurrence)

	// half an hour earlier, the same schedule has not come round today
	occurrence, found = findMostRecentScheduledOccurrence(template, []int64{0}, false, time.Date(2026, 9, 2, 23, 30, 0, 0, utcPlusTwo).Unix())

	assert.True(t, found)
	assert.Equal(t, time.Date(2026, 9, 2, 0, 0, 0, 0, utcPlusTwo).Unix(), occurrence)
}

// A yearly schedule is found however far back in the year it sits
func TestFindMostRecentScheduledOccurrence_Yearly(t *testing.T) {
	template := newScheduledTemplate(models.TRANSACTION_SCHEDULE_FREQUENCY_TYPE_YEARLY, 1320, 120)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, utcPlusTwo).Unix()

	occurrence, found := findMostRecentScheduledOccurrence(template, []int64{1015}, false, now)

	assert.True(t, found)
	assert.Equal(t, time.Date(2025, 10, 15, 0, 0, 0, 0, utcPlusTwo).Unix(), occurrence)
}

// A template whose day is not fixed is due today, because somebody has seen the money move
func TestFindMostRecentScheduledOccurrence_DayNotFixed(t *testing.T) {
	template := newScheduledTemplate(models.TRANSACTION_SCHEDULE_FREQUENCY_TYPE_MONTHLY, 1320, 120)
	now := time.Date(2026, 9, 17, 19, 40, 0, 0, utcPlusTwo).Unix()

	occurrence, found := findMostRecentScheduledOccurrence(template, nil, true, now)

	assert.True(t, found)
	assert.Equal(t, time.Date(2026, 9, 17, 0, 0, 0, 0, utcPlusTwo).Unix(), occurrence)
}

func TestFindMostRecentScheduledOccurrence_OutsideStartAndEnd(t *testing.T) {
	template := newScheduledTemplate(models.TRANSACTION_SCHEDULE_FREQUENCY_TYPE_MONTHLY, 1320, 120)
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, utcPlusTwo).Unix()

	// nothing has been due yet, the schedule starts next year
	startTime := time.Date(2027, 1, 1, 0, 0, 0, 0, utcPlusTwo).Unix()
	template.ScheduledStartTime = &startTime
	_, found := findMostRecentScheduledOccurrence(template, []int64{1}, false, now)
	assert.False(t, found)

	// the schedule ended in July, so July is the most recent occurrence rather than September
	template.ScheduledStartTime = nil
	endTime := time.Date(2026, 7, 31, 0, 0, 0, 0, utcPlusTwo).Unix()
	template.ScheduledEndTime = &endTime
	occurrence, found := findMostRecentScheduledOccurrence(template, []int64{1}, false, now)
	assert.True(t, found)
	assert.Equal(t, time.Date(2026, 7, 1, 0, 0, 0, 0, utcPlusTwo).Unix(), occurrence)
}

// The transaction the button writes has to be the one the cron would have written
func TestNewTransactionFromScheduledTemplate(t *testing.T) {
	template := newScheduledTemplate(models.TRANSACTION_SCHEDULE_FREQUENCY_TYPE_MONTHLY, 1320, 120)
	template.CategoryId = 20
	template.AccountId = 30
	template.Amount = 12345
	template.IsSubscription = true
	template.Comment = "Rent"

	transactionTime := time.Date(2026, 9, 1, 0, 0, 0, 0, utcPlusTwo)
	transaction, err := newTransactionFromScheduledTemplate(template, transactionTime, "127.0.0.1")

	assert.Nil(t, err)
	assert.Equal(t, models.TRANSACTION_DB_TYPE_EXPENSE, transaction.Type)
	assert.Equal(t, int64(20), transaction.CategoryId)
	assert.Equal(t, int64(30), transaction.AccountId)
	assert.Equal(t, int64(12345), transaction.Amount)
	assert.Equal(t, int16(120), transaction.TimezoneUtcOffset)
	assert.Equal(t, transactionTime.Unix()*1000, transaction.TransactionTime)
	assert.True(t, transaction.IsSubscription)
	assert.True(t, transaction.ScheduledCreated)
	assert.Equal(t, template.TemplateId, transaction.ScheduledTemplateId)
}
