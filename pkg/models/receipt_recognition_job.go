package models

// ReceiptRecognitionJobStatus represents the status of a receipt recognition job
type ReceiptRecognitionJobStatus byte

// Receipt recognition job statuses
const (
	RECEIPT_RECOGNITION_JOB_STATUS_PENDING    ReceiptRecognitionJobStatus = 0
	RECEIPT_RECOGNITION_JOB_STATUS_PROCESSING ReceiptRecognitionJobStatus = 1
	RECEIPT_RECOGNITION_JOB_STATUS_COMPLETED  ReceiptRecognitionJobStatus = 2
	RECEIPT_RECOGNITION_JOB_STATUS_FAILED     ReceiptRecognitionJobStatus = 3
	// RECEIPT_RECOGNITION_JOB_STATUS_RESOLVED means the user has dealt with the
	// result, either by creating a transaction from it or discarding it. Jobs are
	// kept in this state rather than deleted so a client that uploaded but never
	// saw the response does not resurrect them.
	RECEIPT_RECOGNITION_JOB_STATUS_RESOLVED ReceiptRecognitionJobStatus = 4
)

// MaxReceiptRecognitionJobAttempts is how many times a job is retried before it is
// left as failed for the user to handle by hand
const MaxReceiptRecognitionJobAttempts = 3

// ReceiptRecognitionJob represents a queued receipt image awaiting recognition.
//
// The image itself is stored as an ordinary transaction picture, so this table
// only holds the job state. That means the picture is already in the right place
// to be attached to whatever transaction the user creates from the result.
type ReceiptRecognitionJob struct {
	JobId     int64                       `xorm:"PK"`
	Uid       int64                       `xorm:"INDEX(IDX_receipt_recognition_job_uid_status) NOT NULL"`
	Status    ReceiptRecognitionJobStatus `xorm:"INDEX(IDX_receipt_recognition_job_uid_status) INDEX(IDX_receipt_recognition_job_status) NOT NULL"`
	PictureId int64                       `xorm:"NOT NULL"`
	// PictureExtension is copied from the stored picture so that the job's image
	// url can be built without a second lookup per job when listing.
	PictureExtension string `xorm:"VARCHAR(10) NOT NULL"`
	// UtcOffset is the submitting client's timezone offset in minutes. Receipts
	// print wall-clock times, so the offset has to travel with the job — a job
	// picked up hours later must resolve dates the same way it would have at
	// submission time.
	UtcOffset int16 `xorm:"NOT NULL"`
	Attempts  int32 `xorm:"NOT NULL"`
	// RecognizedResult is a serialised RecognizedTransactionResponse, stored
	// verbatim so that adding fields to that struct does not need a migration.
	RecognizedResult string `xorm:"TEXT"`
	ErrorMessage     string `xorm:"VARCHAR(255)"`
	CreatedUnixTime  int64
	UpdatedUnixTime  int64
}

// ReceiptRecognitionJobSubmitResponse is returned the moment a job is queued,
// before any recognition has happened
type ReceiptRecognitionJobSubmitResponse struct {
	JobId     int64 `json:"jobId,string"`
	PictureId int64 `json:"pictureId,string"`
}

// ReceiptRecognitionJobInfoResponse represents a view-object of a recognition job
type ReceiptRecognitionJobInfoResponse struct {
	JobId       int64                       `json:"jobId,string"`
	Status      ReceiptRecognitionJobStatus `json:"status"`
	PictureId   int64                       `json:"pictureId,string"`
	OriginalUrl string                      `json:"originalUrl"`
	// Result is present only once Status is completed.
	Result          *RecognizedTransactionResponse `json:"result,omitempty"`
	ErrorMessage    string                         `json:"errorMessage,omitempty"`
	CreatedUnixTime int64                          `json:"createdTime"`
	UpdatedUnixTime int64                          `json:"updatedTime"`
}

// ReceiptRecognitionJobListRequest represents all parameters of a job list request
type ReceiptRecognitionJobListRequest struct {
	// IncludeResolved returns jobs the user has already dealt with. Off by
	// default, so the common case is "what still needs my attention".
	IncludeResolved bool `form:"include_resolved"`
}

// ReceiptRecognitionJobResolveRequest represents all parameters of a job resolve request
type ReceiptRecognitionJobResolveRequest struct {
	Id int64 `json:"id,string" binding:"required,min=1"`
}

// ReceiptRecognitionJobInfoResponseSlice represents the slice data structure of
// ReceiptRecognitionJobInfoResponse
type ReceiptRecognitionJobInfoResponseSlice []*ReceiptRecognitionJobInfoResponse

func (s ReceiptRecognitionJobInfoResponseSlice) Len() int {
	return len(s)
}

func (s ReceiptRecognitionJobInfoResponseSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s ReceiptRecognitionJobInfoResponseSlice) Less(i, j int) bool {
	return s[i].CreatedUnixTime < s[j].CreatedUnixTime
}
