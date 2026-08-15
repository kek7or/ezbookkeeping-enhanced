package cron

import (
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/services"
)

// RemoveExpiredTokensJob represents the cron job which periodically remove expired user tokens from the database
var RemoveExpiredTokensJob = &CronJob{
	Name:        "RemoveExpiredTokens",
	Description: "Periodically remove expired user tokens from the database.",
	Period: CronJobFixedHourPeriod{
		Hour: 0,
	},
	Run: func(c *core.CronContext) error {
		return services.Tokens.DeleteAllExpiredTokens(c)
	},
}

// CreateScheduledTransactionJob represents the cron job which periodically create transaction by scheduled transaction template
var CreateScheduledTransactionJob = &CronJob{
	Name:        "CreateScheduledTransaction",
	Description: "Periodically create transaction by scheduled transaction template.",
	Period: CronJobEvery15MinutesPeriod{
		Second: 0,
	},
	Run: func(c *core.CronContext) error {
		return services.Transactions.CreateScheduledTransactions(c, time.Now().Unix(), c.GetInterval())
	},
}

// ProcessReceiptRecognitionJobsJob represents the cron job which drains the receipt recognition queue
//
// The interval is short because a user who has just uploaded receipts is waiting
// on these results. The scheduler already prevents overlapping runs, so a pass
// that takes longer than the interval simply delays the next one.
var ProcessReceiptRecognitionJobsJob = &CronJob{
	Name:        "ProcessReceiptRecognitionJobs",
	Description: "Periodically recognize queued receipt images.",
	Period: CronJobIntervalPeriod{
		Interval: 15 * time.Second,
	},
	Run: func(c *core.CronContext) error {
		return services.ReceiptRecognitionJobs.ProcessPendingReceiptRecognitionJobs(c)
	},
}
