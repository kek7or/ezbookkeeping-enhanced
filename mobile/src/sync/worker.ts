import { ApiError, NetworkError } from '../api/client';
import { RECEIPT_JOB_STATUS_COMPLETED, RECEIPT_JOB_STATUS_FAILED } from '../api/types';
import {
    clearPhotoJobId,
    listPhotos,
    listTransactions,
    markPhotoState,
    markTransactionsState,
    replaceReferenceData,
    getPhoto,
    setMeta
} from '../db/repo';

import type { ApiClient } from '../api/client';
import type { LocalTransaction } from '../db/repo';
import type { TransactionCreateRequest } from '../api/types';

/**
 * The sync worker. This is the only component in the app that touches the
 * network. Screens never call the API directly — they write rows and let this
 * drain them, which is what makes every failure mode collapse into "the row
 * kept its pending state, try again later".
 *
 * The whole run is resumable. Nothing here assumes it will be allowed to finish.
 */

export type SyncStage =
    | 'idle'
    | 'pulling'
    | 'uploading-pictures'
    | 'pushing'
    | 'submitting-receipts'
    | 'collecting'
    | 'done';

export interface SyncProgress {
    stage: SyncStage;
    message: string;
    /** 0-1 within the current stage, or null when it cannot be known. */
    fraction: number | null;
}

export interface SyncResult {
    transactionsPushed: number;
    /** Receipts handed to the server's queue during this run. */
    photosSubmitted: number;
    /** Receipts the server has finished with, now waiting to be reviewed. */
    photosReady: number;
    /** Receipts the server is still working on. Nothing to wait for. */
    stillProcessing: number;
    /** Rows the server refused for reasons retrying will not fix. */
    rejected: { transactionId: number; reason: string }[];
    errors: string[];
}

export type ProgressCallback = (progress: SyncProgress) => void;

const BATCH_SESSION_META_KEY = 'pending_batch_session_id';
const MAX_PHOTO_ATTEMPTS = 3;

/**
 * Session ids only need to be unique per user, and exist to let the server
 * recognise a resend of the same batch. Math.random is sufficient for that;
 * this is not a security boundary.
 */
function newClientSessionId(): string {
    const random = Math.random().toString(16).slice(2, 12);
    return `mobile-${Date.now().toString(16)}-${random}`;
}

function describeError(error: unknown): string {
    if (error instanceof ApiError || error instanceof NetworkError) {
        return error.message;
    }

    return error instanceof Error ? error.message : String(error);
}

function toCreateRequest(transaction: LocalTransaction, pictureIds: string[]): TransactionCreateRequest {
    return {
        type: transaction.type,
        categoryId: transaction.categoryId,
        time: transaction.time,
        utcOffset: transaction.utcOffset,
        sourceAccountId: transaction.sourceAccountId,
        destinationAccountId: transaction.destinationAccountId,
        sourceAmount: transaction.sourceAmount,
        destinationAmount: transaction.destinationAmount,
        tagIds: transaction.tagIds,
        pictureIds,
        comment: transaction.comment
    };
}

export async function runSync(client: ApiClient, onProgress: ProgressCallback): Promise<SyncResult> {
    const result: SyncResult = {
        transactionsPushed: 0,
        photosSubmitted: 0,
        photosReady: 0,
        stillProcessing: 0,
        rejected: [],
        errors: []
    };

    // 1. Refresh reference data first. Category and account ids are what the
    // queued transactions point at, so stale ones are the most likely cause of
    // a server-side rejection below.
    onProgress({ stage: 'pulling', message: 'Getting categories and accounts', fraction: null });

    try {
        const [categories, accounts, tags] = await Promise.all([
            client.listCategories(),
            client.listAccounts(),
            client.listTags()
        ]);
        await replaceReferenceData(categories, accounts, tags);
    } catch (error) {
        // A failed pull is not fatal: the queued rows already carry their ids,
        // so pushing can still succeed on a cached catalogue.
        result.errors.push(`Could not refresh categories: ${describeError(error)}`);

        if (error instanceof ApiError && error.isAuthFailure) {
            throw error;
        }
    }

    await recoverInFlightBatch(client, result, onProgress);
    await pushTransactions(client, result, onProgress);

    // Collect first, so results the server finished since the last run are shown
    // straight away rather than waiting for another round trip.
    await collectFinishedJobs(client, result, onProgress);
    await submitPhotos(client, result, onProgress);
    await closeResolvedJobs(client, result);

    onProgress({ stage: 'done', message: 'Finished', fraction: 1 });
    return result;
}

/**
 * Handles rows left in `syncing` by a previous run that died mid-flight — app
 * killed, phone rebooted, connection dropped after the server had already
 * committed. We cannot tell from the client whether those landed, so we ask.
 *
 * The progress endpoint is keyed by the same clientSessionId we sent, so a
 * `finished` result means the batch committed and must NOT be resent.
 */
async function recoverInFlightBatch(
    client: ApiClient,
    result: SyncResult,
    onProgress: ProgressCallback
): Promise<void> {
    const inFlight = await listTransactions(['syncing']);

    if (!inFlight.length) {
        return;
    }

    onProgress({ stage: 'pushing', message: 'Checking an interrupted upload', fraction: null });

    const sessionId = inFlight[0].batchSessionId;
    const ids = inFlight.map((transaction) => transaction.id);

    if (!sessionId) {
        // Should not happen, but an unknown session id is unrecoverable —
        // re-queue and let the normal path resend under a fresh id.
        await markTransactionsState(ids, 'pending');
        return;
    }

    try {
        const progress = await client.getImportProgress(sessionId);

        if (progress === 100) {
            await markTransactionsState(ids, 'synced', { batchSessionId: sessionId });
            result.transactionsPushed += ids.length;
            return;
        }

        if (progress === null) {
            // The server has no record of this session. Either it never arrived
            // or the server restarted (the duplicate checker is in-memory).
            // Re-queueing risks a duplicate in the restart case, but losing the
            // transactions outright is the worse failure.
            await markTransactionsState(ids, 'pending');
            return;
        }

        // Still processing server-side. Leave it alone; next run will re-check.
        result.errors.push('A previous upload is still being processed by the server.');
    } catch (error) {
        result.errors.push(`Could not check the interrupted upload: ${describeError(error)}`);
    }
}

/** Resolves the server picture ids a transaction needs, uploading any that are missing. */
async function resolvePictureIds(
    client: ApiClient,
    transaction: LocalTransaction,
    result: SyncResult
): Promise<string[]> {
    if (!transaction.photoId) {
        return [];
    }

    const photo = await getPhoto(transaction.photoId);

    if (!photo) {
        return [];
    }

    if (photo.serverPictureId) {
        return [photo.serverPictureId];
    }

    try {
        const uploaded = await client.uploadPicture(photo.fileUri, photo.fileName);
        await markPhotoState(photo.id, photo.syncState, { serverPictureId: uploaded.pictureId });
        return [uploaded.pictureId];
    } catch (error) {
        // Losing the attachment is much better than losing the transaction, so
        // this is reported but does not block the push.
        result.errors.push(`Could not attach the receipt photo: ${describeError(error)}`);
        return [];
    }
}

async function pushTransactions(
    client: ApiClient,
    result: SyncResult,
    onProgress: ProgressCallback
): Promise<void> {
    const pending = await listTransactions(['pending']);

    if (!pending.length) {
        return;
    }

    onProgress({ stage: 'uploading-pictures', message: 'Uploading receipt photos', fraction: null });

    const requests: TransactionCreateRequest[] = [];

    for (let i = 0; i < pending.length; i++) {
        const pictureIds = await resolvePictureIds(client, pending[i], result);
        requests.push(toCreateRequest(pending[i], pictureIds));
        onProgress({
            stage: 'uploading-pictures',
            message: 'Uploading receipt photos',
            fraction: (i + 1) / pending.length
        });
    }

    const ids = pending.map((transaction) => transaction.id);
    const sessionId = newClientSessionId();

    // Record the session id and claim the rows *before* the request goes out.
    // If the app dies mid-request, recoverInFlightBatch has what it needs.
    await setMeta(BATCH_SESSION_META_KEY, sessionId);
    await markTransactionsState(ids, 'syncing', { batchSessionId: sessionId });

    onProgress({
        stage: 'pushing',
        message: `Sending ${pending.length} transaction${pending.length === 1 ? '' : 's'}`,
        fraction: null
    });

    try {
        await client.importTransactions({ transactions: requests, clientSessionId: sessionId });
        await markTransactionsState(ids, 'synced', { batchSessionId: sessionId });
        result.transactionsPushed += ids.length;
        return;
    } catch (error) {
        if (error instanceof NetworkError) {
            // Never arrived. Safe to re-queue wholesale.
            await markTransactionsState(ids, 'pending');
            result.errors.push(`No connection: ${error.message}`);
            return;
        }

        if (error instanceof ApiError && error.isRetryable) {
            await markTransactionsState(ids, 'pending');
            result.errors.push(`Server error, will retry: ${error.message}`);
            return;
        }

        // A validation failure. import.json checks the whole batch up front and
        // rejects all of it if any single row is bad, so one stale category id
        // would otherwise block every other transaction with one opaque error.
        // Fall back to sending them individually to isolate the offender.
        result.errors.push(`The server rejected the batch, retrying one at a time: ${describeError(error)}`);
        await pushTransactionsIndividually(client, pending, result, onProgress);
    }
}

async function pushTransactionsIndividually(
    client: ApiClient,
    transactions: LocalTransaction[],
    result: SyncResult,
    onProgress: ProgressCallback
): Promise<void> {
    for (let i = 0; i < transactions.length; i++) {
        const transaction = transactions[i];
        const sessionId = newClientSessionId();

        onProgress({
            stage: 'pushing',
            message: `Sending transaction ${i + 1} of ${transactions.length}`,
            fraction: (i + 1) / transactions.length
        });

        const pictureIds = await resolvePictureIds(client, transaction, result);
        await markTransactionsState([transaction.id], 'syncing', { batchSessionId: sessionId });

        try {
            await client.importTransactions({
                transactions: [toCreateRequest(transaction, pictureIds)],
                clientSessionId: sessionId
            });
            await markTransactionsState([transaction.id], 'synced', { batchSessionId: sessionId });
            result.transactionsPushed += 1;
        } catch (error) {
            if (error instanceof NetworkError || (error instanceof ApiError && error.isRetryable)) {
                await markTransactionsState([transaction.id], 'pending', { lastError: describeError(error) });
                continue;
            }

            // This specific row is the problem. Park it for the user to fix
            // rather than retrying it forever.
            const reason = describeError(error);
            await markTransactionsState([transaction.id], 'failed', { lastError: reason });
            result.rejected.push({ transactionId: transaction.id, reason });
        }
    }
}

/**
 * Hands every captured receipt to the server's recognition queue.
 *
 * This is the whole of the app's involvement in recognition. Submitting costs
 * only the upload; the model round-trip happens later inside the server's
 * queue, so the user is never left watching a spinner for it.
 */
async function submitPhotos(client: ApiClient, result: SyncResult, onProgress: ProgressCallback): Promise<void> {
    const pending = await listPhotos(['pending']);

    if (!pending.length) {
        return;
    }

    for (let i = 0; i < pending.length; i++) {
        const photo = pending[i];

        onProgress({
            stage: 'submitting-receipts',
            message: `Sending receipt ${i + 1} of ${pending.length}`,
            fraction: (i + 1) / pending.length
        });

        try {
            const submitted = await client.submitReceiptJob(photo.fileUri, photo.fileName);
            await markPhotoState(photo.id, 'submitted', {
                serverJobId: submitted.jobId,
                serverPictureId: submitted.pictureId,
                incrementAttempts: true
            });
            result.photosSubmitted += 1;
        } catch (error) {
            const reason = describeError(error);
            const retryable = error instanceof NetworkError || (error instanceof ApiError && error.isRetryable);
            const exhausted = photo.attempts + 1 >= MAX_PHOTO_ATTEMPTS;

            // A receipt that cannot be submitted still has to reach the user, so
            // once attempts run out it is parked for manual entry rather than
            // retried forever or silently dropped.
            if (retryable && !exhausted) {
                await markPhotoState(photo.id, 'pending', { lastError: reason, incrementAttempts: true });
            } else {
                await markPhotoState(photo.id, 'failed', { lastError: reason, incrementAttempts: true });
            }

            result.errors.push(`Receipt ${i + 1}: ${reason}`);
        }
    }
}

/**
 * Reconciles local photos against the server's queue.
 *
 * The server is authoritative here — the app is only mirroring what the queue
 * says. Nothing waits: whatever is still processing is simply left alone and
 * picked up on the next sync.
 */
async function collectFinishedJobs(
    client: ApiClient,
    result: SyncResult,
    onProgress: ProgressCallback
): Promise<void> {
    const submitted = await listPhotos(['submitted']);

    if (!submitted.length) {
        return;
    }

    onProgress({ stage: 'collecting', message: 'Checking for finished receipts', fraction: null });

    let jobs;

    try {
        jobs = await client.listReceiptJobs();
    } catch (error) {
        result.errors.push(`Could not check the receipt queue: ${describeError(error)}`);
        return;
    }

    const jobsById = new Map(jobs.map((job) => [job.jobId, job]));

    for (const photo of submitted) {
        if (!photo.serverJobId) {
            continue;
        }

        const job = jobsById.get(photo.serverJobId);

        if (!job) {
            // The server no longer knows about this job. Re-queue the photo
            // rather than leave it stuck waiting on a result that never comes.
            await markPhotoState(photo.id, 'pending', { lastError: 'the server lost track of this receipt' });
            continue;
        }

        if (job.status === RECEIPT_JOB_STATUS_COMPLETED && job.result) {
            await markPhotoState(photo.id, 'needs_review', { recognized: job.result });
            result.photosReady += 1;
        } else if (job.status === RECEIPT_JOB_STATUS_FAILED) {
            // Still surfaced to the user, just with nothing filled in.
            await markPhotoState(photo.id, 'needs_review', {
                lastError: job.errorMessage ?? 'the server could not read this receipt'
            });
            result.photosReady += 1;
        }

        // Pending or processing: the server is still on it. Nothing to wait for.
    }

    const stillWorking = submitted.length - result.photosReady;

    if (stillWorking > 0) {
        result.stillProcessing = stillWorking;
    }
}

/**
 * Tells the server about receipts the user has already dealt with.
 *
 * Resolving happens locally and offline, so the server is only informed on the
 * next sync. Without this the server's queue would return the same finished jobs
 * forever and grow without bound.
 */
async function closeResolvedJobs(client: ApiClient, result: SyncResult): Promise<void> {
    const resolved = await listPhotos(['resolved']);
    const needsClosing = resolved.filter((photo) => photo.serverJobId);

    for (const photo of needsClosing) {
        try {
            await client.resolveReceiptJob(photo.serverJobId as string);
            await clearPhotoJobId(photo.id);
        } catch (error) {
            if (error instanceof ApiError && !error.isRetryable) {
                // The server already considers it gone. Stop asking.
                await clearPhotoJobId(photo.id);
                continue;
            }

            // Purely bookkeeping between app and server — not worth reporting to
            // the user, and it will be retried on the next sync.
            result.errors.push(`Could not close a finished receipt: ${describeError(error)}`);
        }
    }
}
