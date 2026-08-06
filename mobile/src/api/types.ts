/**
 * Wire types for the ezbookkeeping server API.
 *
 * IMPORTANT: every server-side int64 is serialised as a JSON *string*
 * (`json:"id,string"`). Snowflake ids exceed Number.MAX_SAFE_INTEGER, so these
 * stay `string` end-to-end and must never be passed through `Number()`.
 * Monetary amounts are the exception: they are plain JSON numbers holding
 * minor units (cents), and are safely below 2^53.
 */

export type ApiResponse<T> =
    | { success: true; result: T }
    | { success: false; errorCode: number; errorMessage: string; path: string };

// pkg/models/transaction.go
export const TRANSACTION_TYPE_MODIFY_BALANCE = 1;
export const TRANSACTION_TYPE_INCOME = 2;
export const TRANSACTION_TYPE_EXPENSE = 3;
export const TRANSACTION_TYPE_TRANSFER = 4;

export type TransactionType =
    | typeof TRANSACTION_TYPE_MODIFY_BALANCE
    | typeof TRANSACTION_TYPE_INCOME
    | typeof TRANSACTION_TYPE_EXPENSE
    | typeof TRANSACTION_TYPE_TRANSFER;

// pkg/models/transaction_category.go
export const CATEGORY_TYPE_INCOME = 1;
export const CATEGORY_TYPE_EXPENSE = 2;
export const CATEGORY_TYPE_TRANSFER = 3;

export type TransactionCategoryType =
    | typeof CATEGORY_TYPE_INCOME
    | typeof CATEGORY_TYPE_EXPENSE
    | typeof CATEGORY_TYPE_TRANSFER;

export interface UserBasicInfo {
    username: string;
    email: string;
    nickname: string;
    defaultCurrency: string;
    defaultAccountId: string;
}

export interface AuthResponse {
    token: string;
    need2FA: boolean;
    user: UserBasicInfo;
}

export interface TokenGenerateApiResponse {
    token: string;
    expiresAt: number;
}

export interface TransactionCategoryInfoResponse {
    id: string;
    name: string;
    parentId: string;
    type: TransactionCategoryType;
    icon: string;
    color: string;
    comment: string;
    displayOrder: number;
    hidden: boolean;
    excludeFromStatistics: boolean;
    subCategories?: TransactionCategoryInfoResponse[];
}

export interface AccountInfoResponse {
    id: string;
    name: string;
    parentId: string;
    category: number;
    type: number;
    icon: string;
    color: string;
    currency: string;
    balance: number;
    displayOrder: number;
    hidden: boolean;
    subAccounts?: AccountInfoResponse[];
}

export interface TransactionTagInfoResponse {
    id: string;
    name: string;
    displayOrder: number;
    hidden: boolean;
}

/** Mirrors pkg/models/transaction.go TransactionCreateRequest. */
export interface TransactionCreateRequest {
    type: TransactionType;
    categoryId: string;
    time: number; // unix millis
    utcOffset: number; // minutes, -720..840
    sourceAccountId: string;
    destinationAccountId?: string;
    sourceAmount: number; // minor units
    destinationAmount?: number; // minor units
    hideAmount?: boolean;
    tagIds?: string[];
    pictureIds?: string[];
    comment?: string;
    clientSessionId?: string;
}

/** Mirrors pkg/models/transaction.go TransactionImportRequest. */
export interface TransactionImportRequest {
    transactions: TransactionCreateRequest[];
    clientSessionId: string;
}

export interface TransactionPictureInfoResponse {
    pictureId: string;
    originalUrl: string;
}

/**
 * Mirrors pkg/models/large_language_model.go RecognizedTransactionResponse, as
 * returned by /llm/transactions/recognize_receipt_image.json.
 *
 * The server resolves the model's category/account *names* into real ids before
 * responding, so ids here are usable directly — but everything except `type` is
 * `omitempty` and the model routinely fails to identify some fields. An
 * unmatched name yields an absent id. Filling those gaps is exactly what the
 * review screen is for. Note there is no `utcOffset`: the server has already
 * interpreted the receipt's wall-clock time in the timezone we sent.
 */
export interface RecognizedTransactionResponse {
    type: TransactionType;
    time?: number;
    categoryId?: string;
    sourceAccountId?: string;
    destinationAccountId?: string;
    sourceAmount?: number;
    destinationAmount?: number;
    tagIds?: string[];
    comment?: string;
}

// --- receipt recognition queue ---
// pkg/models/receipt_recognition_job.go

export const RECEIPT_JOB_STATUS_PENDING = 0;
export const RECEIPT_JOB_STATUS_PROCESSING = 1;
export const RECEIPT_JOB_STATUS_COMPLETED = 2;
export const RECEIPT_JOB_STATUS_FAILED = 3;
export const RECEIPT_JOB_STATUS_RESOLVED = 4;

export type ReceiptJobStatus =
    | typeof RECEIPT_JOB_STATUS_PENDING
    | typeof RECEIPT_JOB_STATUS_PROCESSING
    | typeof RECEIPT_JOB_STATUS_COMPLETED
    | typeof RECEIPT_JOB_STATUS_FAILED
    | typeof RECEIPT_JOB_STATUS_RESOLVED;

export interface ReceiptJobSubmitResponse {
    jobId: string;
    pictureId: string;
}

export interface ReceiptJobInfoResponse {
    jobId: string;
    status: ReceiptJobStatus;
    pictureId: string;
    originalUrl: string;
    /** Present only once status is completed. */
    result?: RecognizedTransactionResponse;
    errorMessage?: string;
    createdTime: number;
    updatedTime: number;
}
