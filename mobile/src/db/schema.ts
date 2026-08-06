import * as SQLite from 'expo-sqlite';

/**
 * Local database. This is the app's source of truth: every screen reads and
 * writes here only, and the sync worker is the sole component that touches the
 * network. That split is what makes the app work with no signal — a failed
 * upload is just a row that kept its `pending` state.
 *
 * `local_transactions` and `photos` are the outbox. They carry real typed
 * columns rather than opaque payload blobs so the UI can list and edit queued
 * items without deserialising, and so a schema mistake surfaces as a SQL error
 * rather than a malformed request the server rejects.
 */

const DATABASE_NAME = 'ezbookkeeping.db';

/** Bump alongside a new entry in MIGRATIONS. */
const SCHEMA_VERSION = 1;

const MIGRATIONS: string[] = [
    // v1 — initial schema
    `
    CREATE TABLE IF NOT EXISTS categories (
        id            TEXT    PRIMARY KEY NOT NULL,
        name          TEXT    NOT NULL,
        type          INTEGER NOT NULL,
        parent_id     TEXT    NOT NULL DEFAULT '0',
        display_order INTEGER NOT NULL DEFAULT 0,
        hidden        INTEGER NOT NULL DEFAULT 0
    );

    CREATE TABLE IF NOT EXISTS accounts (
        id            TEXT    PRIMARY KEY NOT NULL,
        name          TEXT    NOT NULL,
        currency      TEXT    NOT NULL DEFAULT '',
        parent_id     TEXT    NOT NULL DEFAULT '0',
        display_order INTEGER NOT NULL DEFAULT 0,
        hidden        INTEGER NOT NULL DEFAULT 0
    );

    CREATE TABLE IF NOT EXISTS tags (
        id            TEXT    PRIMARY KEY NOT NULL,
        name          TEXT    NOT NULL,
        display_order INTEGER NOT NULL DEFAULT 0,
        hidden        INTEGER NOT NULL DEFAULT 0
    );

    -- Once a photo is submitted the server owns the slow part: recognition runs
    -- in its queue, and this row just mirrors what the server reports back.
    CREATE TABLE IF NOT EXISTS photos (
        id                INTEGER PRIMARY KEY AUTOINCREMENT,
        file_uri          TEXT    NOT NULL,
        file_name         TEXT    NOT NULL,
        captured_at       INTEGER NOT NULL,
        sync_state        TEXT    NOT NULL DEFAULT 'pending',
        attempts          INTEGER NOT NULL DEFAULT 0,
        last_error        TEXT,
        recognized_json   TEXT,
        server_picture_id TEXT,
        server_job_id     TEXT
    );

    CREATE INDEX IF NOT EXISTS idx_photos_sync_state
        ON photos (sync_state);

    -- Amounts are integer minor units, mirroring the server. No floats touch a
    -- monetary value anywhere in this app.
    CREATE TABLE IF NOT EXISTS local_transactions (
        id                     INTEGER PRIMARY KEY AUTOINCREMENT,
        type                   INTEGER NOT NULL,
        category_id            TEXT    NOT NULL DEFAULT '0',
        source_account_id      TEXT    NOT NULL,
        destination_account_id TEXT    NOT NULL DEFAULT '0',
        source_amount          INTEGER NOT NULL,
        destination_amount     INTEGER NOT NULL DEFAULT 0,
        time                   INTEGER NOT NULL,
        utc_offset             INTEGER NOT NULL,
        comment                TEXT    NOT NULL DEFAULT '',
        tag_ids                TEXT    NOT NULL DEFAULT '[]',
        photo_id               INTEGER REFERENCES photos(id) ON DELETE SET NULL,
        sync_state             TEXT    NOT NULL DEFAULT 'pending',
        batch_session_id       TEXT,
        last_error             TEXT,
        created_at             INTEGER NOT NULL
    );

    CREATE INDEX IF NOT EXISTS idx_local_transactions_sync_state
        ON local_transactions (sync_state);

    CREATE TABLE IF NOT EXISTS meta (
        key   TEXT PRIMARY KEY NOT NULL,
        value TEXT NOT NULL
    );
    `
];

let database: SQLite.SQLiteDatabase | null = null;

export async function openDatabase(): Promise<SQLite.SQLiteDatabase> {
    if (database) {
        return database;
    }

    const db = await SQLite.openDatabaseAsync(DATABASE_NAME);

    // WAL keeps the UI's reads from blocking on the sync worker's writes.
    await db.execAsync('PRAGMA journal_mode = WAL;');
    await db.execAsync('PRAGMA foreign_keys = ON;');

    await migrate(db);

    database = db;
    return db;
}

async function migrate(db: SQLite.SQLiteDatabase): Promise<void> {
    const row = await db.getFirstAsync<{ user_version: number }>('PRAGMA user_version');
    const currentVersion = row?.user_version ?? 0;

    if (currentVersion >= SCHEMA_VERSION) {
        return;
    }

    for (let version = currentVersion; version < SCHEMA_VERSION; version++) {
        await db.execAsync(MIGRATIONS[version]);
    }

    // PRAGMA does not accept bound parameters, and SCHEMA_VERSION is a
    // module-level integer constant, so interpolation is safe here.
    await db.execAsync(`PRAGMA user_version = ${SCHEMA_VERSION}`);
}

/** Drops every local row. Used when signing out of a server. */
export async function resetDatabase(): Promise<void> {
    const db = await openDatabase();

    await db.withTransactionAsync(async () => {
        await db.execAsync(`
            DELETE FROM local_transactions;
            DELETE FROM photos;
            DELETE FROM categories;
            DELETE FROM accounts;
            DELETE FROM tags;
            DELETE FROM meta;
        `);
    });
}
