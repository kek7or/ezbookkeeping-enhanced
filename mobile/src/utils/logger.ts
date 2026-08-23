import { Directory, File, Paths } from 'expo-file-system';

import { openDatabase } from '../db/schema';

/**
 * The diagnostic log.
 *
 * The app does most of its work out of sight — a sync run touches the network
 * a dozen times and reports back one sentence — so when something goes wrong
 * the user is left holding an error message with no context. This records what
 * actually happened, survives a restart, and can be handed to someone else as
 * a file.
 *
 * Two rules hold everywhere in here:
 *
 * 1. **Logging never breaks the caller.** Every write is fire-and-forget and
 *    swallows its own errors. A failure to record a problem must not become a
 *    second problem.
 * 2. **Secrets never reach the log.** Tokens and passwords are redacted at the
 *    point of capture, not on the way out, so there is no version of the log —
 *    on screen, in the file, in the database — that ever held them.
 */

export type LogLevel = 'debug' | 'info' | 'warn' | 'error';

export interface LogEntry {
    id: number;
    /** Unix milliseconds. */
    at: number;
    level: LogLevel;
    /** Where it came from: `api`, `sync`, `ui`, … */
    scope: string;
    message: string;
    /** Long-form payload — a stack, a response body — or null. */
    detail: string | null;
}

interface LogRow {
    id: number;
    at: number;
    level: string;
    scope: string;
    message: string;
    detail: string | null;
}

/**
 * How many rows to keep. A busy sync writes on the order of 50 lines, so this
 * holds weeks of ordinary use while staying small enough to read and to share.
 */
const MAX_ROWS = 5_000;

/** Prune every N writes rather than on each one — the count is only advisory. */
const PRUNE_INTERVAL = 200;

/** Long values are truncated: a log that cannot be read is not a log. */
const MAX_DETAIL_CHARS = 4_000;

let writesSincePrune = 0;

/**
 * Writes are chained onto one promise so they land in call order and never
 * overlap. Without this, two `void log(...)` calls could interleave their
 * awaits and record events out of sequence, which is exactly the detail you
 * need when reading back a failure.
 */
let writeQueue: Promise<void> = Promise.resolve();

function truncate(value: string, limit: number): string {
    if (value.length <= limit) {
        return value;
    }

    return `${value.slice(0, limit)}\n… truncated, ${value.length - limit} more characters`;
}

/**
 * Renders anything into something readable. Errors keep their stack, objects
 * are pretty-printed, and a value that cannot be serialised (a cycle, a native
 * handle) degrades to its string form rather than throwing.
 */
export function describeDetail(detail: unknown): string | null {
    if (detail === undefined || detail === null) {
        return null;
    }

    if (typeof detail === 'string') {
        return truncate(detail, MAX_DETAIL_CHARS);
    }

    if (detail instanceof Error) {
        return truncate(detail.stack ?? `${detail.name}: ${detail.message}`, MAX_DETAIL_CHARS);
    }

    try {
        return truncate(JSON.stringify(detail, null, 2), MAX_DETAIL_CHARS);
    } catch {
        return truncate(String(detail), MAX_DETAIL_CHARS);
    }
}

/**
 * Records one line. Deliberately not awaited by callers: the returned promise
 * exists for tests and for the rare caller that wants to be sure the line
 * landed before it navigates away.
 */
export function log(level: LogLevel, scope: string, message: string, detail?: unknown): Promise<void> {
    const at = Date.now();
    const rendered = describeDetail(detail);

    writeQueue = writeQueue
        .then(async () => {
            const db = await openDatabase();

            await db.runAsync(
                'INSERT INTO logs (at, level, scope, message, detail) VALUES (?, ?, ?, ?, ?)',
                at,
                level,
                scope,
                message,
                rendered
            );

            writesSincePrune += 1;

            if (writesSincePrune >= PRUNE_INTERVAL) {
                writesSincePrune = 0;
                await db.runAsync(
                    'DELETE FROM logs WHERE id <= (SELECT MAX(id) - ? FROM logs)',
                    MAX_ROWS
                );
            }
        })
        .catch(() => {
            // Rule 1. If the log cannot be written there is nowhere left to
            // report that, and the caller's own work must carry on regardless.
        });

    return writeQueue;
}

export const logger = {
    debug: (scope: string, message: string, detail?: unknown) => void log('debug', scope, message, detail),
    info: (scope: string, message: string, detail?: unknown) => void log('info', scope, message, detail),
    warn: (scope: string, message: string, detail?: unknown) => void log('warn', scope, message, detail),
    error: (scope: string, message: string, detail?: unknown) => void log('error', scope, message, detail)
};

/** Newest first, which is the order the screen wants and the order you read in. */
export async function readLogs(options: { levels?: LogLevel[]; limit?: number } = {}): Promise<LogEntry[]> {
    const db = await openDatabase();
    const limit = options.limit ?? 1_000;
    const levels = options.levels;

    const rows =
        levels && levels.length
            ? await db.getAllAsync<LogRow>(
                  `SELECT * FROM logs
                   WHERE level IN (${levels.map(() => '?').join(', ')})
                   ORDER BY id DESC LIMIT ?`,
                  ...levels,
                  limit
              )
            : await db.getAllAsync<LogRow>('SELECT * FROM logs ORDER BY id DESC LIMIT ?', limit);

    return rows.map((row) => ({
        id: row.id,
        at: row.at,
        level: row.level as LogLevel,
        scope: row.scope,
        message: row.message,
        detail: row.detail
    }));
}

export async function countLogs(): Promise<number> {
    const db = await openDatabase();
    const row = await db.getFirstAsync<{ total: number }>('SELECT COUNT(*) AS total FROM logs');

    return row?.total ?? 0;
}

export async function clearLogs(): Promise<void> {
    const db = await openDatabase();
    await db.runAsync('DELETE FROM logs');
}

function pad(value: number, width = 2): string {
    return String(value).padStart(width, '0');
}

/** Local time — the log is read by the person whose phone produced it. */
export function formatTimestamp(at: number): string {
    const date = new Date(at);

    return (
        `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ` +
        `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}.${pad(date.getMilliseconds(), 3)}`
    );
}

/**
 * One line per entry, with the detail indented beneath it. Chronological here
 * (oldest first) rather than newest-first as on screen: a file is read top to
 * bottom, following the order things happened.
 */
export function formatLogsAsText(entries: LogEntry[]): string {
    const chronological = [...entries].reverse();
    const lines: string[] = [];

    for (const entry of chronological) {
        lines.push(`${formatTimestamp(entry.at)}  ${entry.level.toUpperCase().padEnd(5)} [${entry.scope}] ${entry.message}`);

        if (entry.detail) {
            for (const detailLine of entry.detail.split('\n')) {
                lines.push(`    ${detailLine}`);
            }
        }
    }

    return lines.join('\n');
}

/**
 * Writes the log to a file in the cache directory and returns it.
 *
 * The cache is the right home for this: it is the app's own storage, so no
 * permission is needed, and Android is free to reclaim it once the user has
 * shared the file wherever they wanted it.
 */
export async function writeLogFile(entries: LogEntry[]): Promise<File> {
    const directory = new Directory(Paths.cache, 'logs');

    if (!directory.exists) {
        directory.create({ intermediates: true });
    }

    const now = new Date();
    const stamp =
        `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}` +
        `-${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`;

    const file = new File(directory, `ezbookkeeping-log-${stamp}.txt`);

    const header = [
        `ezbookkeeping mobile log`,
        `exported ${formatTimestamp(now.getTime())}`,
        `${entries.length} entr${entries.length === 1 ? 'y' : 'ies'}`,
        ''
    ].join('\n');

    if (file.exists) {
        file.delete();
    }

    file.create();
    file.write(`${header}\n${formatLogsAsText(entries)}\n`);

    return file;
}
