package services

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// maxJobsPerRun caps how much the worker takes on in a single pass. Each job is a
// live model round-trip, so a large backlog is drained over several passes rather
// than held in one very long-running invocation.
const maxJobsPerRun = 10

// ProcessPendingReceiptRecognitionJobs drains the receipt recognition queue.
//
// Called on a short interval by the cron scheduler, which already guarantees that
// only one invocation runs at a time. Job claiming is still done with a
// conditional update, so a multi-instance deployment stays correct too.
func (s *ReceiptRecognitionJobService) ProcessPendingReceiptRecognitionJobs(c core.Context) error {
	config := ReceiptRecognitions.CurrentConfig()

	if config.ReceiptImageRecognitionLLMConfig == nil || config.ReceiptImageRecognitionLLMConfig.LLMProvider == "" || !config.TransactionFromAIImageRecognition {
		return nil
	}

	jobs, err := s.GetClaimableJobs(c, maxJobsPerRun)

	if err != nil {
		return err
	}

	if len(jobs) < 1 {
		return nil
	}

	processed := 0

	for i := 0; i < len(jobs) && processed < maxJobsPerRun; i++ {
		job := jobs[i]
		claimed, err := s.ClaimJob(c, job)

		if err != nil {
			log.Errorf(c, "[receipt_recognition_worker.ProcessPendingReceiptRecognitionJobs] failed to claim job \"id:%d\", because %s", job.JobId, err.Error())
			continue
		}

		if !claimed {
			// Another worker got there first.
			continue
		}

		processed++
		s.processOneJob(c, job)
	}

	if processed > 0 {
		log.Infof(c, "[receipt_recognition_worker.ProcessPendingReceiptRecognitionJobs] processed %d receipt recognition job(s)", processed)
	}

	return nil
}

// processOneJob runs a single claimed job to completion. It never returns an
// error: one user's unreadable receipt must not abort the whole queue pass, so
// failures are recorded on the job itself.
func (s *ReceiptRecognitionJobService) processOneJob(c core.Context, job *models.ReceiptRecognitionJob) {
	contentType := utils.GetImageContentType(job.PictureExtension)

	if contentType == "" {
		s.failJob(c, job, "unsupported image type")
		return
	}

	imageData, err := TransactionPictures.GetPictureByPictureId(c, job.Uid, job.PictureId, job.PictureExtension)

	if err != nil {
		s.failJob(c, job, "could not read the stored image: "+err.Error())
		return
	}

	essentialData, err := ReceiptRecognitions.GetUserEssentialData(c, job.Uid)

	if err != nil {
		s.failJob(c, job, "could not load categories and accounts: "+err.Error())
		return
	}

	// Reconstruct the submitting client's timezone so a receipt's printed
	// wall-clock time resolves the same way it would have at submission.
	clientTimezone := time.FixedZone("Client Fixed Timezone", int(job.UtcOffset)*60)

	result, err := ReceiptRecognitions.RecognizeReceiptImage(c, job.Uid, clientTimezone, imageData, contentType, essentialData)

	if err != nil {
		// A receipt the model cannot find a transaction in will never succeed on
		// retry, so it goes straight to failed instead of burning attempts.
		if errors.Is(err, errs.ErrNoTransactionInformation) {
			s.failJobPermanently(c, job, "no transaction information could be read from this receipt")
			return
		}

		s.failJob(c, job, err.Error())
		return
	}

	serialised, err := json.Marshal(result)

	if err != nil {
		s.failJob(c, job, "could not store the recognition result: "+err.Error())
		return
	}

	err = s.CompleteJob(c, job.Uid, job.JobId, string(serialised))

	if err != nil {
		log.Errorf(c, "[receipt_recognition_worker.processOneJob] failed to save result of job \"id:%d\" for user \"uid:%d\", because %s", job.JobId, job.Uid, err.Error())
		return
	}

	log.Infof(c, "[receipt_recognition_worker.processOneJob] completed recognition job \"id:%d\" for user \"uid:%d\"", job.JobId, job.Uid)
}

func (s *ReceiptRecognitionJobService) failJob(c core.Context, job *models.ReceiptRecognitionJob, message string) {
	log.Warnf(c, "[receipt_recognition_worker.failJob] recognition job \"id:%d\" for user \"uid:%d\" failed on attempt %d, because %s", job.JobId, job.Uid, job.Attempts, message)

	if err := s.FailJob(c, job.Uid, job.JobId, job.Attempts, message); err != nil {
		log.Errorf(c, "[receipt_recognition_worker.failJob] failed to record failure of job \"id:%d\", because %s", job.JobId, err.Error())
	}
}

// failJobPermanently records a failure that retrying cannot fix.
func (s *ReceiptRecognitionJobService) failJobPermanently(c core.Context, job *models.ReceiptRecognitionJob, message string) {
	log.Warnf(c, "[receipt_recognition_worker.failJobPermanently] recognition job \"id:%d\" for user \"uid:%d\" will not be retried, because %s", job.JobId, job.Uid, message)

	if err := s.FailJob(c, job.Uid, job.JobId, models.MaxReceiptRecognitionJobAttempts, message); err != nil {
		log.Errorf(c, "[receipt_recognition_worker.failJobPermanently] failed to record failure of job \"id:%d\", because %s", job.JobId, err.Error())
	}
}
