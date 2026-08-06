import { openDatabase } from './schema';

import type {
    AccountInfoResponse,
    RecognizedTransactionResponse,
    TransactionCategoryInfoResponse,
    TransactionTagInfoResponse,
    TransactionType
} from '../api/types';

export type TransactionSyncState = 'pending' | 'syncing' | 'synced' | 'failed';

/**
 * Photo lifecycle. The app only ever owns the first transition; everything from
 * `submitted` onwards mirrors what the server's recognition queue reports.
 *
 *   pending      — captured, not yet sent
 *   submitted    — handed to the server's queue; the server owns it now
 *   needs_review — the server finished; waiting for the user to confirm
 *   resolved     — the user turned it into a transaction, or discarded it
 *   failed       — the server gave up on it; enter it by hand
 */
export type PhotoSyncState = 'pending' | 'submitted' | 'needs_review' | 'resolved' | 'failed';

export interface Category {
    id: string;
    name: string;
    type: number;
    parentId: string;
    displayOrder: number;
    hidden: boolean;
}

export interface Account {
    id: string;
    name: string;
    currency: string;
    parentId: string;
    displayOrder: number;
    hidden: boolean;
}

export interface Tag {
    id: string;
    name: string;
    displayOrder: number;
    hidden: boolean;
}

export interface LocalTransaction {
    id: number;
    type: TransactionType;
    categoryId: string;
    sourceAccountId: string;
    destinationAccountId: string;
    sourceAmount: number;
    destinationAmount: number;
    time: number;
    utcOffset: number;
    comment: string;
    tagIds: string[];
    photoId: number | null;
    syncState: TransactionSyncState;
    batchSessionId: string | null;
    lastError: string | null;
    createdAt: number;
}

export interface Photo {
    id: number;
    fileUri: string;
    fileName: string;
    capturedAt: number;
    syncState: PhotoSyncState;
    attempts: number;
    lastError: string | null;
    recognized: RecognizedTransactionResponse | null;
    serverPictureId: string | null;
    /** The server-side queue job, once submitted. Used to match results back. */
    serverJobId: string | null;
}

export type NewLocalTransaction = Omit<
    LocalTransaction,
    'id' | 'syncState' | 'batchSessionId' | 'lastError' | 'createdAt'
>;

interface CategoryRow {
    id: string;
    name: string;
    type: number;
    parent_id: string;
    display_order: number;
    hidden: number;
}

interface AccountRow {
    id: string;
    name: string;
    currency: string;
    parent_id: string;
    display_order: number;
    hidden: number;
}

interface TagRow {
    id: string;
    name: string;
    display_order: number;
    hidden: number;
}

interface LocalTransactionRow {
    id: number;
    type: number;
    category_id: string;
    source_account_id: string;
    destination_account_id: string;
    source_amount: number;
    destination_amount: number;
    time: number;
    utc_offset: number;
    comment: string;
    tag_ids: string;
    photo_id: number | null;
    sync_state: string;
    batch_session_id: string | null;
    last_error: string | null;
    created_at: number;
}

interface PhotoRow {
    id: number;
    file_uri: string;
    file_name: string;
    captured_at: number;
    sync_state: string;
    attempts: number;
    last_error: string | null;
    recognized_json: string | null;
    server_picture_id: string | null;
    server_job_id: string | null;
}

function parseJsonColumn<T>(value: string | null, fallback: T): T {
    if (!value) {
        return fallback;
    }

    try {
        return JSON.parse(value) as T;
    } catch {
        return fallback;
    }
}

function toLocalTransaction(row: LocalTransactionRow): LocalTransaction {
    return {
        id: row.id,
        type: row.type as TransactionType,
        categoryId: row.category_id,
        sourceAccountId: row.source_account_id,
        destinationAccountId: row.destination_account_id,
        sourceAmount: row.source_amount,
        destinationAmount: row.destination_amount,
        time: row.time,
        utcOffset: row.utc_offset,
        comment: row.comment,
        tagIds: parseJsonColumn<string[]>(row.tag_ids, []),
        photoId: row.photo_id,
        syncState: row.sync_state as TransactionSyncState,
        batchSessionId: row.batch_session_id,
        lastError: row.last_error,
        createdAt: row.created_at
    };
}

function toPhoto(row: PhotoRow): Photo {
    return {
        id: row.id,
        fileUri: row.file_uri,
        fileName: row.file_name,
        capturedAt: row.captured_at,
        syncState: row.sync_state as PhotoSyncState,
        attempts: row.attempts,
        lastError: row.last_error,
        recognized: parseJsonColumn<RecognizedTransactionResponse | null>(row.recognized_json, null),
        serverPictureId: row.server_picture_id,
        serverJobId: row.server_job_id
    };
}

// --- reference data (pull) ---

/**
 * Replaces the cached reference data wholesale. These lists are small and we
 * have no change-tracking on the server, so a full refresh inside one
 * transaction is both simpler and less error-prone than reconciling deltas.
 * Sub-categories and sub-accounts are flattened, since a transaction always
 * refers to a leaf.
 */
export async function replaceReferenceData(
    categories: TransactionCategoryInfoResponse[],
    accounts: AccountInfoResponse[],
    tags: TransactionTagInfoResponse[]
): Promise<void> {
    const db = await openDatabase();

    const flatCategories: TransactionCategoryInfoResponse[] = [];
    const collectCategories = (items: TransactionCategoryInfoResponse[]): void => {
        for (const item of items) {
            flatCategories.push(item);

            if (item.subCategories?.length) {
                collectCategories(item.subCategories);
            }
        }
    };
    collectCategories(categories);

    const flatAccounts: AccountInfoResponse[] = [];
    const collectAccounts = (items: AccountInfoResponse[]): void => {
        for (const item of items) {
            flatAccounts.push(item);

            if (item.subAccounts?.length) {
                collectAccounts(item.subAccounts);
            }
        }
    };
    collectAccounts(accounts);

    await db.withTransactionAsync(async () => {
        await db.execAsync('DELETE FROM categories; DELETE FROM accounts; DELETE FROM tags;');

        for (const category of flatCategories) {
            await db.runAsync(
                `INSERT INTO categories (id, name, type, parent_id, display_order, hidden)
                 VALUES (?, ?, ?, ?, ?, ?)`,
                category.id,
                category.name,
                category.type,
                category.parentId,
                category.displayOrder,
                category.hidden ? 1 : 0
            );
        }

        for (const account of flatAccounts) {
            await db.runAsync(
                `INSERT INTO accounts (id, name, currency, parent_id, display_order, hidden)
                 VALUES (?, ?, ?, ?, ?, ?)`,
                account.id,
                account.name,
                account.currency,
                account.parentId,
                account.displayOrder,
                account.hidden ? 1 : 0
            );
        }

        for (const tag of tags) {
            await db.runAsync(
                `INSERT INTO tags (id, name, display_order, hidden) VALUES (?, ?, ?, ?)`,
                tag.id,
                tag.name,
                tag.displayOrder,
                tag.hidden ? 1 : 0
            );
        }
    });

    await setMeta('last_pull_at', String(Date.now()));
}

export async function listCategories(type?: number): Promise<Category[]> {
    const db = await openDatabase();
    const rows = type
        ? await db.getAllAsync<CategoryRow>(
              'SELECT * FROM categories WHERE hidden = 0 AND type = ? ORDER BY display_order, name',
              type
          )
        : await db.getAllAsync<CategoryRow>(
              'SELECT * FROM categories WHERE hidden = 0 ORDER BY type, display_order, name'
          );

    return rows.map((row) => ({
        id: row.id,
        name: row.name,
        type: row.type,
        parentId: row.parent_id,
        displayOrder: row.display_order,
        hidden: row.hidden !== 0
    }));
}

export async function listAccounts(): Promise<Account[]> {
    const db = await openDatabase();
    const rows = await db.getAllAsync<AccountRow>(
        'SELECT * FROM accounts WHERE hidden = 0 ORDER BY display_order, name'
    );

    return rows.map((row) => ({
        id: row.id,
        name: row.name,
        currency: row.currency,
        parentId: row.parent_id,
        displayOrder: row.display_order,
        hidden: row.hidden !== 0
    }));
}

export async function listTags(): Promise<Tag[]> {
    const db = await openDatabase();
    const rows = await db.getAllAsync<TagRow>('SELECT * FROM tags WHERE hidden = 0 ORDER BY display_order, name');

    return rows.map((row) => ({
        id: row.id,
        name: row.name,
        displayOrder: row.display_order,
        hidden: row.hidden !== 0
    }));
}

// --- transactions (outbox) ---

export async function insertTransaction(transaction: NewLocalTransaction): Promise<number> {
    const db = await openDatabase();
    const result = await db.runAsync(
        `INSERT INTO local_transactions (
            type, category_id, source_account_id, destination_account_id,
            source_amount, destination_amount, time, utc_offset, comment,
            tag_ids, photo_id, sync_state, created_at
         ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?)`,
        transaction.type,
        transaction.categoryId,
        transaction.sourceAccountId,
        transaction.destinationAccountId,
        transaction.sourceAmount,
        transaction.destinationAmount,
        transaction.time,
        transaction.utcOffset,
        transaction.comment,
        JSON.stringify(transaction.tagIds),
        transaction.photoId,
        Date.now()
    );

    return result.lastInsertRowId;
}

export async function listTransactions(states?: TransactionSyncState[]): Promise<LocalTransaction[]> {
    const db = await openDatabase();

    if (!states?.length) {
        const rows = await db.getAllAsync<LocalTransactionRow>(
            'SELECT * FROM local_transactions ORDER BY time DESC, id DESC'
        );
        return rows.map(toLocalTransaction);
    }

    const placeholders = states.map(() => '?').join(', ');
    const rows = await db.getAllAsync<LocalTransactionRow>(
        `SELECT * FROM local_transactions WHERE sync_state IN (${placeholders}) ORDER BY time ASC, id ASC`,
        ...states
    );

    return rows.map(toLocalTransaction);
}

export async function countTransactionsByState(state: TransactionSyncState): Promise<number> {
    const db = await openDatabase();
    const row = await db.getFirstAsync<{ count: number }>(
        'SELECT COUNT(*) AS count FROM local_transactions WHERE sync_state = ?',
        state
    );

    return row?.count ?? 0;
}

export async function markTransactionsState(
    ids: number[],
    state: TransactionSyncState,
    options: { batchSessionId?: string | null; lastError?: string | null } = {}
): Promise<void> {
    if (!ids.length) {
        return;
    }

    const db = await openDatabase();
    const placeholders = ids.map(() => '?').join(', ');

    await db.runAsync(
        `UPDATE local_transactions
            SET sync_state = ?, batch_session_id = ?, last_error = ?
          WHERE id IN (${placeholders})`,
        state,
        options.batchSessionId ?? null,
        options.lastError ?? null,
        ...ids
    );
}

export async function deleteTransaction(id: number): Promise<void> {
    const db = await openDatabase();
    await db.runAsync('DELETE FROM local_transactions WHERE id = ?', id);
}

/** Clears out successfully-synced rows so the local list stays a to-do list. */
export async function purgeSyncedTransactions(): Promise<void> {
    const db = await openDatabase();
    await db.runAsync("DELETE FROM local_transactions WHERE sync_state = 'synced'");
}

// --- photos ---

export async function insertPhoto(fileUri: string, fileName: string): Promise<number> {
    const db = await openDatabase();
    const result = await db.runAsync(
        `INSERT INTO photos (file_uri, file_name, captured_at, sync_state) VALUES (?, ?, ?, 'pending')`,
        fileUri,
        fileName,
        Date.now()
    );

    return result.lastInsertRowId;
}

export async function listPhotos(states?: PhotoSyncState[]): Promise<Photo[]> {
    const db = await openDatabase();

    if (!states?.length) {
        const rows = await db.getAllAsync<PhotoRow>('SELECT * FROM photos ORDER BY captured_at DESC, id DESC');
        return rows.map(toPhoto);
    }

    const placeholders = states.map(() => '?').join(', ');
    const rows = await db.getAllAsync<PhotoRow>(
        `SELECT * FROM photos WHERE sync_state IN (${placeholders}) ORDER BY captured_at ASC, id ASC`,
        ...states
    );

    return rows.map(toPhoto);
}

export async function countPhotosByState(state: PhotoSyncState): Promise<number> {
    const db = await openDatabase();
    const row = await db.getFirstAsync<{ count: number }>(
        'SELECT COUNT(*) AS count FROM photos WHERE sync_state = ?',
        state
    );

    return row?.count ?? 0;
}

/**
 * Updates a photo's state. Optional fields use COALESCE so that omitting one
 * leaves the stored value alone — a later update must not blank out the picture
 * or job id recorded by an earlier one.
 */
export async function markPhotoState(
    id: number,
    state: PhotoSyncState,
    options: {
        recognized?: RecognizedTransactionResponse | null;
        serverPictureId?: string | null;
        serverJobId?: string | null;
        lastError?: string | null;
        incrementAttempts?: boolean;
    } = {}
): Promise<void> {
    const db = await openDatabase();

    await db.runAsync(
        `UPDATE photos
            SET sync_state = ?,
                recognized_json = COALESCE(?, recognized_json),
                server_picture_id = COALESCE(?, server_picture_id),
                server_job_id = COALESCE(?, server_job_id),
                last_error = ?,
                attempts = attempts + ?
          WHERE id = ?`,
        state,
        options.recognized === undefined ? null : JSON.stringify(options.recognized),
        options.serverPictureId ?? null,
        options.serverJobId ?? null,
        options.lastError ?? null,
        options.incrementAttempts ? 1 : 0,
        id
    );
}

/** Finds the local photo matching a server-side queue job. */
export async function getPhotoByJobId(jobId: string): Promise<Photo | null> {
    const db = await openDatabase();
    const row = await db.getFirstAsync<PhotoRow>('SELECT * FROM photos WHERE server_job_id = ?', jobId);

    return row ? toPhoto(row) : null;
}

/**
 * Forgets the server-side job for a photo, marking it closed on the server too.
 * A resolved photo that still carries a job id is one the server has not been
 * told about yet, which is how the sync worker finds them.
 */
export async function clearPhotoJobId(id: number): Promise<void> {
    const db = await openDatabase();
    await db.runAsync('UPDATE photos SET server_job_id = NULL WHERE id = ?', id);
}

export async function getPhoto(id: number): Promise<Photo | null> {
    const db = await openDatabase();
    const row = await db.getFirstAsync<PhotoRow>('SELECT * FROM photos WHERE id = ?', id);

    return row ? toPhoto(row) : null;
}

export async function deletePhoto(id: number): Promise<void> {
    const db = await openDatabase();
    await db.runAsync('DELETE FROM photos WHERE id = ?', id);
}

// --- meta ---

export async function getMeta(key: string): Promise<string | null> {
    const db = await openDatabase();
    const row = await db.getFirstAsync<{ value: string }>('SELECT value FROM meta WHERE key = ?', key);

    return row?.value ?? null;
}

export async function setMeta(key: string, value: string): Promise<void> {
    const db = await openDatabase();
    await db.runAsync(
        'INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value',
        key,
        value
    );
}
