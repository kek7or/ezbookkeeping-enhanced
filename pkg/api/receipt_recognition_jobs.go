package api

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// ReceiptRecognitionJobsApi represents the receipt recognition job queue api
type ReceiptRecognitionJobsApi struct {
	ApiUsingConfig
	users    *services.UserService
	pictures *services.TransactionPictureService
	jobs     *services.ReceiptRecognitionJobService
}

// Initialize a receipt recognition jobs api singleton instance
var (
	ReceiptRecognitionJobs = &ReceiptRecognitionJobsApi{
		ApiUsingConfig: ApiUsingConfig{
			container: settings.Container,
		},
		users:    services.Users,
		pictures: services.TransactionPictures,
		jobs:     services.ReceiptRecognitionJobs,
	}
)

// ReceiptRecognitionJobSubmitHandler queues a receipt image for recognition and
// returns as soon as it is stored.
//
// The response deliberately carries no recognition result. The whole point of the
// queue is that the client — typically a phone that has just come back into
// signal — pays only for the upload and never waits on a model round-trip.
func (a *ReceiptRecognitionJobsApi) ReceiptRecognitionJobSubmitHandler(c *core.WebContext) (any, *errs.Error) {
	if a.CurrentConfig().ReceiptImageRecognitionLLMConfig == nil || a.CurrentConfig().ReceiptImageRecognitionLLMConfig.LLMProvider == "" || !a.CurrentConfig().TransactionFromAIImageRecognition {
		return nil, errs.ErrLargeLanguageModelProviderNotEnabled
	}

	uid := c.GetCurrentUid()
	user, err := a.users.GetUserById(c, uid)

	if err != nil {
		if !errs.IsCustomError(err) {
			log.Warnf(c, "[receipt_recognition_jobs.ReceiptRecognitionJobSubmitHandler] failed to get user for user \"uid:%d\", because %s", uid, err.Error())
		}

		return nil, errs.ErrUserNotFound
	}

	if user.FeatureRestriction.Contains(core.USER_FEATURE_RESTRICTION_TYPE_CREATE_TRANSACTION_FROM_AI_IMAGE_RECOGNITION) {
		return nil, errs.ErrNotPermittedToPerformThisAction
	}

	// The timezone has to be captured now, not when the worker runs: a receipt
	// prints a wall-clock time, and the worker has no client to ask.
	clientTimezone, err := c.GetClientTimezone()

	if err != nil {
		log.Warnf(c, "[receipt_recognition_jobs.ReceiptRecognitionJobSubmitHandler] cannot get client timezone, because %s", err.Error())
		return nil, errs.ErrClientTimezoneOffsetInvalid
	}

	_, offsetSeconds := time.Now().In(clientTimezone).Zone()
	utcOffset := int16(offsetSeconds / 60)

	form, err := c.MultipartForm()

	if err != nil {
		log.Errorf(c, "[receipt_recognition_jobs.ReceiptRecognitionJobSubmitHandler] failed to get multi-part form data for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.ErrParameterInvalid
	}

	imageFiles := form.File["image"]

	if len(imageFiles) < 1 {
		log.Warnf(c, "[receipt_recognition_jobs.ReceiptRecognitionJobSubmitHandler] there is no image in request for user \"uid:%d\"", uid)
		return nil, errs.ErrNoAIRecognitionImage
	}

	if imageFiles[0].Size < 1 {
		log.Warnf(c, "[receipt_recognition_jobs.ReceiptRecognitionJobSubmitHandler] the size of image in request is zero for user \"uid:%d\"", uid)
		return nil, errs.ErrAIRecognitionImageIsEmpty
	}

	if imageFiles[0].Size > int64(a.CurrentConfig().MaxAIRecognitionPictureFileSize) {
		log.Warnf(c, "[receipt_recognition_jobs.ReceiptRecognitionJobSubmitHandler] the upload file size \"%d\" exceeds the maximum size \"%d\" of image for user \"uid:%d\"", imageFiles[0].Size, a.CurrentConfig().MaxAIRecognitionPictureFileSize, uid)
		return nil, errs.ErrExceedMaxAIRecognitionImageFileSize
	}

	fileExtension := utils.GetFileNameExtension(imageFiles[0].Filename)

	if utils.GetImageContentType(fileExtension) == "" {
		log.Warnf(c, "[receipt_recognition_jobs.ReceiptRecognitionJobSubmitHandler] the file extension \"%s\" of image in request is not supported for user \"uid:%d\"", fileExtension, uid)
		return nil, errs.ErrImageTypeNotSupported
	}

	imageFile, err := imageFiles[0].Open()

	if err != nil {
		log.Errorf(c, "[receipt_recognition_jobs.ReceiptRecognitionJobSubmitHandler] failed to get image file from request for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.ErrOperationFailed
	}

	defer imageFile.Close()

	// Stored as an ordinary transaction picture, which means the image is already
	// in the right place to be attached to whatever transaction the user ends up
	// creating from the result, and is covered by the existing unused-picture
	// cleanup rather than needing its own.
	pictureInfo := &models.TransactionPictureInfo{
		Uid:              uid,
		TransactionId:    models.TransactionPictureNewPictureTransactionId,
		PictureExtension: fileExtension,
		CreatedIp:        c.ClientIP(),
	}

	err = a.pictures.UploadPicture(c, pictureInfo, imageFile)

	if err != nil {
		log.Errorf(c, "[receipt_recognition_jobs.ReceiptRecognitionJobSubmitHandler] failed to store image for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	job, err := a.jobs.CreateJob(c, uid, pictureInfo.PictureId, pictureInfo.PictureExtension, utcOffset)

	if err != nil {
		log.Errorf(c, "[receipt_recognition_jobs.ReceiptRecognitionJobSubmitHandler] failed to queue recognition job for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[receipt_recognition_jobs.ReceiptRecognitionJobSubmitHandler] user \"uid:%d\" queued recognition job \"id:%d\" for picture \"id:%d\"", uid, job.JobId, pictureInfo.PictureId)

	return &models.ReceiptRecognitionJobSubmitResponse{
		JobId:     job.JobId,
		PictureId: pictureInfo.PictureId,
	}, nil
}

// ReceiptRecognitionJobListHandler returns the current user's recognition jobs
func (a *ReceiptRecognitionJobsApi) ReceiptRecognitionJobListHandler(c *core.WebContext) (any, *errs.Error) {
	var listReq models.ReceiptRecognitionJobListRequest
	err := c.ShouldBindQuery(&listReq)

	if err != nil {
		log.Warnf(c, "[receipt_recognition_jobs.ReceiptRecognitionJobListHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	jobs, err := a.jobs.GetJobsByUid(c, uid, listReq.IncludeResolved)

	if err != nil {
		log.Errorf(c, "[receipt_recognition_jobs.ReceiptRecognitionJobListHandler] failed to get jobs for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	responses := make(models.ReceiptRecognitionJobInfoResponseSlice, 0, len(jobs))

	for i := 0; i < len(jobs); i++ {
		responses = append(responses, a.toJobInfoResponse(c, jobs[i]))
	}

	sort.Sort(responses)

	return responses, nil
}

// ReceiptRecognitionJobResolveHandler marks a job as dealt with.
//
// Creating the transaction itself goes through the normal transaction endpoints,
// so this only closes the job. That keeps one code path for creating
// transactions instead of a second one that happens to start from a receipt.
func (a *ReceiptRecognitionJobsApi) ReceiptRecognitionJobResolveHandler(c *core.WebContext) (any, *errs.Error) {
	var resolveReq models.ReceiptRecognitionJobResolveRequest
	err := c.ShouldBindJSON(&resolveReq)

	if err != nil {
		log.Warnf(c, "[receipt_recognition_jobs.ReceiptRecognitionJobResolveHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.jobs.MarkJobResolved(c, uid, resolveReq.Id)

	if err != nil {
		if !errs.IsCustomError(err) {
			log.Errorf(c, "[receipt_recognition_jobs.ReceiptRecognitionJobResolveHandler] failed to resolve job \"id:%d\" for user \"uid:%d\", because %s", resolveReq.Id, uid, err.Error())
		}

		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return true, nil
}

func (a *ReceiptRecognitionJobsApi) toJobInfoResponse(c *core.WebContext, job *models.ReceiptRecognitionJob) *models.ReceiptRecognitionJobInfoResponse {
	response := &models.ReceiptRecognitionJobInfoResponse{
		JobId:           job.JobId,
		Status:          job.Status,
		PictureId:       job.PictureId,
		ErrorMessage:    job.ErrorMessage,
		CreatedUnixTime: job.CreatedUnixTime,
		UpdatedUnixTime: job.UpdatedUnixTime,
	}

	response.OriginalUrl = fmt.Sprintf(internalTransactionPictureUrlFormat, a.CurrentConfig().RootUrl, job.PictureId, job.PictureExtension)

	if job.Status == models.RECEIPT_RECOGNITION_JOB_STATUS_COMPLETED && len(job.RecognizedResult) > 0 {
		var result models.RecognizedTransactionResponse

		if err := json.Unmarshal([]byte(job.RecognizedResult), &result); err != nil {
			// The stored result is unreadable, which the client cannot act on.
			// Surface it as an error rather than an empty success.
			log.Errorf(c, "[receipt_recognition_jobs.toJobInfoResponse] failed to unmarshal stored result of job \"id:%d\", because %s", job.JobId, err.Error())
			response.Status = models.RECEIPT_RECOGNITION_JOB_STATUS_FAILED
			response.ErrorMessage = "stored recognition result could not be read"
		} else {
			response.Result = &result
		}
	}

	return response
}
