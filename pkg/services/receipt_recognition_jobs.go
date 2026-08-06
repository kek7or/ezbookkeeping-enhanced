package services

import (
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

// ReceiptRecognitionJobService owns the receipt recognition queue.
//
// The queue exists so that uploading a receipt costs a client nothing but the
// upload itself. Recognition is a live LLM round-trip measured in tens of
// seconds, which is not something a phone on mobile data should hold a
// connection open for.
type ReceiptRecognitionJobService struct {
	ServiceUsingDB
	ServiceUsingUuid
}

// Initialize a receipt recognition job service singleton instance
var (
	ReceiptRecognitionJobs = &ReceiptRecognitionJobService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
		ServiceUsingUuid: ServiceUsingUuid{
			container: uuid.Container,
		},
	}
)

// staleProcessingTimeout is how long a job may sit in the processing state before
// it is assumed to belong to a worker that died and is made available again.
// Comfortably longer than the slowest plausible LLM round-trip.
const staleProcessingTimeout = 10 * time.Minute

// CreateJob queues a receipt image for recognition and returns immediately
func (s *ReceiptRecognitionJobService) CreateJob(c core.Context, uid int64, pictureId int64, pictureExtension string, utcOffset int16) (*models.ReceiptRecognitionJob, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	now := time.Now().Unix()

	job := &models.ReceiptRecognitionJob{
		JobId:            s.GenerateUuid(uuid.UUID_TYPE_DEFAULT),
		Uid:              uid,
		Status:           models.RECEIPT_RECOGNITION_JOB_STATUS_PENDING,
		PictureId:        pictureId,
		PictureExtension: pictureExtension,
		UtcOffset:        utcOffset,
		CreatedUnixTime:  now,
		UpdatedUnixTime:  now,
	}

	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Insert(job)
		return err
	})

	if err != nil {
		return nil, err
	}

	return job, nil
}

// GetJobsByUid returns the recognition jobs of the specified user
func (s *ReceiptRecognitionJobService) GetJobsByUid(c core.Context, uid int64, includeResolved bool) ([]*models.ReceiptRecognitionJob, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var jobs []*models.ReceiptRecognitionJob
	session := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid)

	if !includeResolved {
		session = session.And("status<>?", models.RECEIPT_RECOGNITION_JOB_STATUS_RESOLVED)
	}

	err := session.OrderBy("created_unix_time asc").Find(&jobs)

	if err != nil {
		return nil, err
	}

	return jobs, nil
}

// GetJobByJobId returns the recognition job with the specified id
func (s *ReceiptRecognitionJobService) GetJobByJobId(c core.Context, uid int64, jobId int64) (*models.ReceiptRecognitionJob, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if jobId <= 0 {
		return nil, errs.ErrReceiptRecognitionJobIdInvalid
	}

	job := &models.ReceiptRecognitionJob{}
	has, err := s.UserDataDB(uid).NewSession(c).ID(jobId).Where("uid=?", uid).Get(job)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, errs.ErrReceiptRecognitionJobNotFound
	}

	return job, nil
}

// MarkJobResolved records that the user has dealt with the job's result
func (s *ReceiptRecognitionJobService) MarkJobResolved(c core.Context, uid int64, jobId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if jobId <= 0 {
		return errs.ErrReceiptRecognitionJobIdInvalid
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		updated, err := sess.ID(jobId).Where("uid=?", uid).Cols("status", "updated_unix_time").Update(&models.ReceiptRecognitionJob{
			Status:          models.RECEIPT_RECOGNITION_JOB_STATUS_RESOLVED,
			UpdatedUnixTime: time.Now().Unix(),
		})

		if err != nil {
			return err
		}

		if updated < 1 {
			return errs.ErrReceiptRecognitionJobNotFound
		}

		return nil
	})
}

// GetClaimableJobs returns jobs waiting for a worker, across every user data shard.
//
// A job counts as claimable when it is pending, or when it has been sitting in
// processing longer than staleProcessingTimeout — the latter belongs to a worker
// that died mid-recognition and would otherwise be stuck forever.
func (s *ReceiptRecognitionJobService) GetClaimableJobs(c core.Context, maxCountPerShard int) ([]*models.ReceiptRecognitionJob, error) {
	var allJobs []*models.ReceiptRecognitionJob
	staleBefore := time.Now().Add(-staleProcessingTimeout).Unix()

	for i := 0; i < s.UserDataDBCount(); i++ {
		var jobs []*models.ReceiptRecognitionJob
		err := s.UserDataDBByIndex(i).NewSession(c).
			Where("status=? OR (status=? AND updated_unix_time<?)",
				models.RECEIPT_RECOGNITION_JOB_STATUS_PENDING,
				models.RECEIPT_RECOGNITION_JOB_STATUS_PROCESSING,
				staleBefore).
			OrderBy("created_unix_time asc").
			Limit(maxCountPerShard).
			Find(&jobs)

		if err != nil {
			return nil, err
		}

		allJobs = append(allJobs, jobs...)
	}

	return allJobs, nil
}

// ClaimJob atomically takes ownership of a job, reporting whether this caller won it.
//
// The claim is a conditional update matching the status and timestamp the job was
// read with, so two workers racing for the same row cannot both succeed — the
// loser's update matches nothing and it moves on to the next job.
func (s *ReceiptRecognitionJobService) ClaimJob(c core.Context, job *models.ReceiptRecognitionJob) (bool, error) {
	if job.Uid <= 0 {
		return false, errs.ErrUserIdInvalid
	}

	now := time.Now().Unix()
	claimed := false

	err := s.UserDataDB(job.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		updated, err := sess.ID(job.JobId).
			Where("uid=? AND status=? AND updated_unix_time=?", job.Uid, job.Status, job.UpdatedUnixTime).
			Cols("status", "attempts", "updated_unix_time").
			Update(&models.ReceiptRecognitionJob{
				Status:          models.RECEIPT_RECOGNITION_JOB_STATUS_PROCESSING,
				Attempts:        job.Attempts + 1,
				UpdatedUnixTime: now,
			})

		if err != nil {
			return err
		}

		claimed = updated > 0
		return nil
	})

	if err != nil {
		return false, err
	}

	if claimed {
		job.Status = models.RECEIPT_RECOGNITION_JOB_STATUS_PROCESSING
		job.Attempts = job.Attempts + 1
		job.UpdatedUnixTime = now
	}

	return claimed, nil
}

// CompleteJob stores a successful recognition result
func (s *ReceiptRecognitionJobService) CompleteJob(c core.Context, uid int64, jobId int64, serialisedResult string) error {
	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.ID(jobId).Where("uid=?", uid).Cols("status", "recognized_result", "error_message", "updated_unix_time").Update(&models.ReceiptRecognitionJob{
			Status:           models.RECEIPT_RECOGNITION_JOB_STATUS_COMPLETED,
			RecognizedResult: serialisedResult,
			ErrorMessage:     "",
			UpdatedUnixTime:  time.Now().Unix(),
		})

		return err
	})
}

// FailJob records a failed attempt. The job returns to pending while it still has
// attempts left, so a transient model or network failure is retried without the
// user having to do anything.
func (s *ReceiptRecognitionJobService) FailJob(c core.Context, uid int64, jobId int64, attempts int32, errorMessage string) error {
	status := models.RECEIPT_RECOGNITION_JOB_STATUS_PENDING

	if attempts >= models.MaxReceiptRecognitionJobAttempts {
		status = models.RECEIPT_RECOGNITION_JOB_STATUS_FAILED
	}

	if len(errorMessage) > 255 {
		errorMessage = errorMessage[:255]
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.ID(jobId).Where("uid=?", uid).Cols("status", "error_message", "updated_unix_time").Update(&models.ReceiptRecognitionJob{
			Status:          status,
			ErrorMessage:    errorMessage,
			UpdatedUnixTime: time.Now().Unix(),
		})

		return err
	})
}
