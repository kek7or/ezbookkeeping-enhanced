# ezbookkeeping mobile

Offline-first Android capture app for the ezbookkeeping server in the parent
directory. Enter transactions and photograph receipts in a shop with no signal,
then press one button at home to upload everything.

## Expo HAS CHANGED

Read the exact versioned docs at <https://docs.expo.dev/versions/v57.0.0/>
before writing any code. `expo-file-system` is now a class-based `File`/`Directory`/
`Paths` API, and the old `FileSystem.documentDirectory` helpers only exist under
`expo-file-system/legacy`.

## Architecture

SQLite is the source of truth. Screens read and write local rows only; they
never call the API. `src/sync/worker.ts` is the sole component that touches the
network. That split is what makes every failure mode collapse into "the row kept
its `pending` state, try again later".

```text
src/api/      typed client for the Go server, one method per endpoint
src/db/       schema + migrations, and a repo of typed queries
src/sync/     the sync worker — the only networked component
src/screens/  UI, reads and writes the db exclusively
```

`local_transactions` and `photos` are the outbox. They use real typed columns
rather than opaque payload blobs, so the UI can list queued items without
deserialising and a schema mistake surfaces as a SQL error rather than a request
the server rejects.

## Server contract, and the parts that bite

- **int64 ids are JSON strings** (`json:"id,string"` on the Go structs).
  Snowflake ids exceed `Number.MAX_SAFE_INTEGER`. Keep them `string`
  end-to-end; never pass one through `Number()`.
- **Amounts are integer minor units.** No float touches a monetary value. See
  `src/utils/money.ts`, which parses via string manipulation rather than
  `Math.round(value * 100)`.
- **`import.json` is all-or-nothing.** It validates the whole batch up front and
  rejects all of it if any single row is bad
  (`pkg/api/transactions.go` `TransactionImportHandler`). One stale category id
  would otherwise fail every queued transaction with one opaque error, so the
  worker falls back to sending them individually to isolate the offender.
- **`clientSessionId` is the idempotency key.** Resending a batch under the same
  id returns the original count instead of double-importing. The worker records
  the id and marks rows `syncing` *before* the request goes out, so an
  interrupted run can ask `import/process.json` whether the batch landed.
- **The server's duplicate checker is in-memory** (`pkg/duplicatechecker/`). It
  resets on server restart, so idempotency is best-effort, not a guarantee.
- **Recognition is queued, and the app never waits for it.** `receipt/jobs/submit.json`
  returns as soon as the image is stored; a server-side cron worker
  (`pkg/services/receipt_recognition_worker.go`) does the model round-trip later.
  The app submits, then reconciles against `receipt/jobs/list.json` on a
  subsequent sync. Nothing in the UI ever blocks on a model.
  The older synchronous `recognize_receipt_image.json` still exists for the web
  UI and shares the same pipeline via `services.ReceiptRecognitions`.
- **Resolving a receipt must be reported back.** The user resolves offline, so
  `closeResolvedJobs` tells the server on the next sync. Skipping that would
  leave the server's queue returning the same finished jobs forever.

## Known v1 limits

- Two decimal places assumed for every currency. Zero-decimal (JPY) and
  three-decimal (KWD) currencies need the server's per-currency decimal table.
- Expense and income only; transfers are not exposed in the form.
- No two-factor login.
- Cleartext HTTP is enabled (`expo-build-properties`) so a LAN server on plain
  `http://` works. Android 9+ blocks it otherwise.
