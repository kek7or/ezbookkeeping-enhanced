# Spec: crypto asset tracking

Status: **implemented**
Target: this fork — Go backend (`pkg/…`) + Vue 3 / Vuetify frontend (`src/…`)
Data source: CoinStats Open API (free plan, one API key and one portfolio share token held by
the server operator)
Scope: **phase 1 — a standalone, read-only crypto page.** Integration with Net Assets /
Total Assets is deliberately out of scope; see §9.

---

## 1. Problem

Coins held on an exchange or in a wallet are part of net worth, but ezBookkeeping has no way
to hold them. The two workarounds both fail:

- **An account denominated in BTC.** `Account.Currency` is `VARCHAR(3)` with a
  `validCurrency` binding against `validators.AllCurrencyNames`
  ([pkg/models/account.go:81](pkg/models/account.go#L81),
  [pkg/models/account.go:104](pkg/models/account.go#L104)). `BTC` is not an ISO currency,
  `USDT` and `DOGE` do not even fit in three characters, and there is no rate for any of them
  in the exchange rate feed. The balance would display but never convert.
- **A fiat account the user retypes.** The value has to be re-entered by hand every time the
  market moves, which is the whole thing worth automating.

**The portfolio already exists in CoinStats.** Asking the user to re-enter quantities that a
service they already use is tracking for them would be a second ledger to keep in sync — and
the wrong one, because the exchange connections update themselves and a typed number does
not. This page mirrors the CoinStats portfolio and never asks for input.

### 1.1 The binding constraint

The CoinStats free plan is **20,000 credits per month at 2 requests per second**, and
`/portfolio/coins` costs **8 credits per request**. That is not scarce in absolute terms, but
the feature must not be able to burn it — no request per page load, no polling. Every design
decision below is subordinate to this.

The budget this spec spends:

| Traffic | Frequency | Credits / month |
|---|---|---|
| Automatic portfolio refresh | once per day | 240 |
| Manual "Update Now" button | capped at 4 requests/day total | ≤ 960 |
| **Total** | | **≤ ~1,000 of 20,000 (5%)** |

The cap is on *requests*, so the automatic refresh and the button share one allowance of four
per day; the worst case above already assumes all four are spent every day.

---

## 2. Goals

- Show the CoinStats portfolio — every coin, its quantity, its price, and what it is worth in
  the user's default currency — **with no data entry of any kind**.
- Refresh **at most once a day automatically**, plus a **manual update button**, with a hard
  per-day request ceiling the button cannot exceed.
- **Cache the snapshot in the database.** A restart, a second browser, or a failed upstream
  call must never cost a request or lose the last known figures.
- A crypto page that stands on its own: total value, per-coin value, 24h change, and each
  coin's share of the portfolio.

### Non-goals for phase 1

- **Any effect on Net Assets, Total Assets, the home page or the Statistics charts.** Crypto
  lives on its own page and changes no existing number anywhere. §9 covers what wiring it in
  would take.
- **Any editing.** No adding, removing or correcting coins or quantities; CoinStats is the
  source of truth and this page is a mirror of it. Writing back (`POST /portfolio/transaction`
  exists) is out of scope.
- Cost basis and P&L. `/portfolio/value` returns them for another 10 credits; see §11.
- Price history or charts. One snapshot, replaced on refresh.
- Multiple portfolios. One share token, one portfolio.
- Mobile UI. Desktop only.

---

## 3. Data source

**Endpoint:** `GET https://openapiv1.coinstats.app/portfolio/coins`
**Auth:** `X-API-KEY` header (the account's API key) **and** a `sharetoken` header — the token
from the "Share" button on the CoinStats portfolio page. A share link protected by a passcode
also needs a `passcode` header.
**Cost:** 8 credits per request.
**Paging:** `page` and `limit`; `meta.hasNextPage` says whether another page follows.

Response (fields the implementation reads, everything else ignored):

```json
{
  "meta": { "page": 1, "limit": 100, "itemCount": 2, "pageCount": 1, "hasNextPage": false },
  "result": [
    {
      "count": 44.4987,
      "coin": {
        "rank": 2,
        "identifier": "ethereum",
        "symbol": "ETH",
        "name": "Ethereum",
        "icon": "https://static.coinstats.app/coins/1650455629727.png",
        "priceChange24h": -5.74
      },
      "price": { "USD": 4343.56, "BTC": 0.03948, "ETH": 1 }
    }
  ]
}
```

`count` and the prices are JSON numbers. They are unmarshalled into `json.Number`, never
`float64`, so the digits the data source printed are the digits that get stored — decoding
`4343.56` into a float64 and formatting it again is how it becomes `4343.560000000001`.

`price` arrives keyed by currency. Only the configured currency's key is read; falling back to
whichever key happened to be present would quietly report a value in the wrong currency, so a
coin priced only in BTC when USD is configured is stored with **no** price rather than a wrong
one.

Sources: [CoinStats API docs](https://coinstats.app/api-docs/openapi/),
[Get Portfolio Coins](https://coinstats.app/api-docs/openapi/get-portfolio-coins),
[CoinStats API](https://coinstats.app/api/).

---

## 4. Design

### 4.1 Data model

Two tables, synced in `cmd/database.go` next to the existing
`SyncStructs(new(models.UserCustomExchangeRate))` call
([cmd/database.go:144](cmd/database.go#L144)). Neither carries a `uid`: there is one
configured portfolio for the instance, and its figures are the same fact for everyone.

**`crypto_portfolio_coin`** — the snapshot, one row per coin.

| Column | Type | Notes |
|---|---|---|
| `CoinId` | `VARCHAR(64) PK` | the CoinStats `coin.identifier` |
| `Symbol` / `Name` / `IconUrl` | `VARCHAR` | as last reported |
| `Currency` | `VARCHAR(3)` | the currency `Price` is quoted in |
| `Count` | `VARCHAR(40)` | quantity, decimal string |
| `Price` | `VARCHAR(40)` | decimal string, exactly as the API printed it |
| `PriceChange1d` | `VARCHAR(16)` | percent, display only |
| `MarketRank` | `int32` | not named `Rank`: that is reserved in MySQL 8 |
| `DisplayOrder` | `int32` | preserves the order CoinStats returned |
| `UpdatedUnixTime` | `int64` | when this snapshot was taken |

**Why `Count` and `Price` are strings.** A fixed `int64` scale cannot cover the range: at 8
decimal places `int64` tops out around 9.2 × 10¹⁰ tokens, which an ordinary meme coin position
exceeds, and at the 18 decimal places an ERC-20 uses it overflows before one whole token.
Every calculation on them goes through `math/big.Rat` from the standard library — no new
dependency, no overflow, no `float64`.

**`crypto_data_source_state`** — one row (`Id` = 1), the request accounting that makes §4.3
enforceable.

| Column | Type | Notes |
|---|---|---|
| `LastPriceRefreshTime` | `int64` | last **successful** refresh |
| `LastAttemptTime` | `int64` | last attempt, success or failure — the stampede guard |
| `LastErrorMessage` | `VARCHAR(255)` | surfaced in the UI so a dead token is visible |
| `RequestCountDate` | `int32` | `YYYYMMDD` in UTC |
| `RequestCountToday` | `int32` | reset when `RequestCountDate` rolls over |

Putting instance-global tables in `UserDataStore` is a slight abuse of the name, but
`DataStore` already ignores its sharding key (`Choose` returns `databases[0]`,
[pkg/datastore/datastore.go:26](pkg/datastore/datastore.go#L26)), so there is nowhere better
and nothing breaks.

### 4.2 What the page computes

Per coin:

- **value** = count × price, in minor units of the price currency
- **share** = value ÷ total value, as a percentage
- **24h change** = the coin's `priceChange24h`, straight from the snapshot

And for the portfolio:

- **total value**, the sum of coins that have a price
- **24h change**, the value-weighted mean of the per-coin changes — `Σ(value × change) ÷ Σ(value)`.
  This is the honest number: a 40% move on a coin that is 1% of the portfolio must not read as
  a 40% portfolio move.

The arithmetic, server-side:

```
value := new(big.Rat).Mul(count, price)   // both parsed from their decimal strings
valueInMinorUnits := round(value × 100)    // half-up, matching utils.ParseAmount's scale
```

Only the final result is rounded, so a value never drifts the way it would if the price were
held in a float64. Rounding half-up per coin means the rows add up to the displayed total,
which is what a reader checks.

The snapshot is taken in **one fixed currency for the whole instance** — `USD` by default,
configurable. The fiat leg is already solved: the frontend converts to the user's default
currency with `exchangeRatesStore.getExchangedAmount`
([src/stores/exchangeRates.ts:304](src/stores/exchangeRates.ts#L304)), the same call a
foreign-currency bank account goes through, so the answer stays consistent with the rest of
the app.

### 4.3 Refresh policy

This is the part the whole feature is judged on.

**Reading never fetches.** `GET /api/v1/crypto/portfolio.json` returns the cached snapshot. A
page load costs zero credits.

**Three settings, doing three different jobs.** They are easy to confuse, so:

| Setting | Question it answers | Default |
|---|---|---|
| `price_cache_lifetime` | How old may the snapshot get before the page refreshes it *on its own*? | 86400 (1 day) |
| `min_refresh_interval` | How long after *any* refresh is the button dead, so repeated clicking cannot spend credits? | 300 (5 min) |
| `max_requests_per_day` | Hard ceiling on outbound requests, across both paths | 4 |

**Two, and only two, ways a request happens:**

1. **Automatic.** On a read, if the snapshot has expired **or there is no snapshot at all**,
   and no attempt has been made within `min_refresh_interval`, refresh once and then serve.
   The "no snapshot at all" case is what makes the very first visit show something instead of
   an empty page until tomorrow.

2. **Manual.** `POST /api/v1/crypto/portfolio/refresh.json`, the **Update Now** button.
   Subject to the same interval and the same daily ceiling.

`LastAttemptTime` is written **before** the HTTP call, inside the same transaction that reads
it, and an in-process mutex covers the single-instance case. That is what stops two tabs, or a
reverse-proxy retry, from both spending a request.

**The daily ceiling** is checked before every outbound call and incremented whether the call
succeeds or fails, so a failing data source cannot be retried into the ground.

**Failure never destroys the snapshot.** A timeout, a 429, a revoked key, an expired share
token: the cached rows stay exactly as they are and keep being served with their real
`UpdatedUnixTime`. The error text goes to `LastErrorMessage` and comes back on the read
response, so the UI shows "the last update failed: … — the figures below are from the last
update that succeeded" instead of pretending everything is fine. Same principle as §3.4 of
[SPEC_receipt_line_item_aggregation.md](SPEC_receipt_line_item_aggregation.md): a degraded
result the user can see beats a silent wrong one.

**A successful refresh replaces the whole table.** Rows are deleted and re-inserted rather
than merged, so a coin that has been sold disappears instead of lingering forever at the
quantity it last had.

**No cron job.** [pkg/cron/cron_jobs.go](pkg/cron/cron_jobs.go) exists and adding a daily job
would be easy, but it would spend credits on an instance nobody is looking at. Lazy refresh on
read ties spending to use. Listed as an alternative in §10.

### 4.4 Configuration

New `[crypto]` section in `conf/ezbookkeeping.ini`, after `[exchange_rates]` and following its
conventions (`request_timeout`, `proxy`, `skip_tls_verify` mean what they mean there):

```ini
[crypto]
data_source = coinstats
coinstats_api_key =
coinstats_share_token =
coinstats_passcode =
coinstats_portfolio_id =
price_currency = USD
price_cache_lifetime = 86400
min_refresh_interval = 300
max_requests_per_day = 4
request_timeout = 10000
proxy = system
skip_tls_verify = false
```

**The secrets come from the environment, not the ini.** Every config item is overridable as
`EBK_<SECTION>_<ITEM>`
([pkg/settings/setting.go:1543](pkg/settings/setting.go#L1543)), so

```
EBK_CRYPTO_COINSTATS_API_KEY=<key>
EBK_CRYPTO_COINSTATS_SHARE_TOKEN=<token>
```

work, as does `EBKCFP_CRYPTO_…` pointing at a file containing the value.

The one piece that was missing is a `.env` reader: the binary read the process environment but
nothing populated it from a file, so a key written in `.env` had no effect.
`LoadConfiguration` now reads `.env` from the working directory before any setting is looked
up ([pkg/settings/environment_file.go](pkg/settings/environment_file.go)). A variable already
present in the real environment is never overwritten, so the file supplies defaults for a
local run and can never overrule what a container or a service manager was told to use.

There is **no feature flag.** Without a key and a share token the page loads, shows whatever
was last cached, and states that the data source is not configured; refresh fails with
`ErrCryptoDataSourceNotConfigured`. That is a clearer failure than a route that does not
exist, and it saves a config item that would only ever be set one way.

### 4.5 API surface

Two routes, registered in `cmd/webserver.go` next to the exchange rate routes
([cmd/webserver.go:488-491](cmd/webserver.go#L488-L491)).

| Method | Path | Purpose | Costs credits |
|---|---|---|---|
| `GET` | `crypto/portfolio.json` | the cached snapshot, valued | only if stale or empty (§4.3) |
| `POST` | `crypto/portfolio/refresh.json` | the Update Now button | yes |

Response:

```json
{
  "coins": [
    {
      "coinId": "ethereum",
      "symbol": "ETH",
      "name": "Ethereum",
      "iconUrl": "https://static.coinstats.app/coins/1650455629727.png",
      "count": "44.4987",
      "price": "4343.56",
      "priceChange1d": "-5.74",
      "value": 19325265
    }
  ],
  "valueCurrency": "USD",
  "totalValue": 19325265,
  "totalValueIncomplete": false,
  "totalPriceChange1d": "-5.74",
  "pricesUpdateTime": 1785269520,
  "nextRefreshAvailableTime": 1785269820,
  "requestsRemainingToday": 3,
  "lastErrorMessage": "",
  "dataSourceConfigured": true
}
```

`value` and `totalValue` are `int64` minor units of `valueCurrency`, the same shape as every
other amount in the API. `value` is `null`, and excluded from `totalValue`, when the coin has
no usable price; `totalValueIncomplete` is then `true` so the UI marks the total rather than
under-reporting it silently.

Error codes in `pkg/errs/crypto_asset.go`: `ErrCryptoDataSourceNotConfigured`,
`ErrCryptoPortfolioNotAccessible` (a 401/403 from the data source — wrong key or share token),
`ErrCryptoPriceRefreshTooFrequent`, `ErrCryptoPriceRefreshLimitExceeded`,
`ErrInvalidCryptoAmount`.

### 4.6 UI

**New page, `/crypto`**, registered in [src/router/desktop.ts](src/router/desktop.ts) between
`/account/list` and `/exchange_rates`, with a nav entry beside them. Desktop only. Read-only:
no add, no edit, no delete, no dialogs.

- **Total value** in the user's default currency, large; the portfolio's 24h change beside it,
  coloured; `totalValueIncomplete` rendered as the existing incomplete-amount suffix rather
  than a silently smaller number.
- **Portfolio Last Updated** and the **Update Now** button. The button is disabled until
  `nextRefreshAvailableTime` with the remaining reason in its tooltip, and disabled outright
  when `requestsRemainingToday` is 0. Both are a courtesy; §4.3 is what enforces them.
- `lastErrorMessage`, when non-empty, is a warning strip above the table. Stale figures are
  shown, not hidden.
- One row per coin: icon, name, symbol, quantity, unit price, 24h change, share of portfolio,
  value. Quantities are printed without the padding zeros the data source adds.

Strings go in `src/locales/en.json` only, following this fork's convention.

---

## 5. Where to change

Nothing in this list modifies an existing behaviour; every entry is a new file or an additive
registration.

| File | Change |
|---|---|
| `pkg/models/crypto_portfolio.go` | **new** — snapshot model, data source state, response view-objects |
| `pkg/cryptoprices/crypto_prices_data_provider.go` | **new** — provider interface, mirroring `pkg/exchangerates/` |
| `pkg/cryptoprices/coinstats_datasource.go` | **new** — request building, paging and response parsing |
| `pkg/cryptoprices/crypto_prices_data_provider_container.go` | **new** — `InitializeCryptoPricesDataSource(config)` |
| `pkg/services/crypto_prices.go` | **new** — the snapshot cache, the refresh policy of §4.3, the request accounting |
| `pkg/services/crypto_valuation.go` | **new** — the `big.Rat` valuation and the weighted change |
| `pkg/api/crypto_assets.go` | **new** — the two handlers of §4.5 |
| `pkg/errs/crypto_asset.go` | **new** — the error codes |
| `pkg/settings/environment_file.go` | **new** — the `.env` reader of §4.4 |
| `pkg/settings/setting.go` | `[crypto]` section parsing, `.env` load before any lookup |
| `cmd/database.go` | `SyncStructs` for the two new models |
| `cmd/webserver.go` | route registration |
| `cmd/initializer.go` | `InitializeCryptoPricesDataSource`, api key masked in the boot log |
| `conf/ezbookkeeping.ini` | §4.4 |
| `src/models/crypto_asset.ts` | **new** — response types, share and quantity formatting |
| `src/stores/cryptoAsset.ts` | **new** — pinia store, load + refresh |
| `src/views/base/CryptoAssetsPageBase.ts` | **new** — currency conversion and display formatting |
| `src/views/desktop/crypto/ListPage.vue` | **new** — §4.6 |
| `src/router/desktop.ts`, `src/views/desktop/MainLayout.vue` | the `/crypto` route and nav entry |
| `src/lib/services.ts`, `src/stores/index.ts` | the two API calls, store reset on logout |
| `src/locales/en.json` | new strings |

---

## 6. Edge cases

| Case | Required behaviour |
|---|---|
| **No snapshot yet (first visit)** | Refresh immediately rather than waiting for the cache to expire, still subject to the interval and the daily cap. |
| **Coin priced only in another currency** | Listed with its quantity, no value, `totalValueIncomplete: true`. Never valued from the wrong currency. |
| **Coin sold in CoinStats** | Gone on the next refresh — the snapshot is replaced, not merged. |
| **Portfolio larger than one page** | Paged through (100 per page, up to 10 pages) so a big portfolio is not truncated. Each page is a request against the same allowance. |
| **Snapshot is stale (refresh failing for days)** | Serve it with its real age and `lastErrorMessage`. The UI states the age; it does not hide the numbers. |
| **No api key or share token** | Page loads and serves the cache; refresh fails with `ErrCryptoDataSourceNotConfigured`. Logged once as `[WARN]` at boot, not a fatal startup error. |
| **Wrong or expired share token** | 401/403 → `ErrCryptoPortfolioNotAccessible`, cached snapshot untouched, reason shown in the warning strip. |
| **Daily ceiling hit** | No outbound request. `ErrCryptoPriceRefreshLimitExceeded`; the button says when it resets (UTC midnight). |
| **Two refreshes racing** | `LastAttemptTime` written before the HTTP call within the read transaction, plus an in-process mutex. The second caller gets the first caller's result. |
| **Empty portfolio** | Empty table, "No crypto assets". Not an error. |
| **Price currency missing from the exchange rate feed** | Values shown in the price currency, total marked incomplete — the same way a foreign-currency account already behaves. |
| **`price_currency` equals the user's default currency** | No conversion, no rate lookup. |

---

## 7. Testing

### 7.1 Unit — valuation (`pkg/services/crypto_valuation_test.go`)

Pure arithmetic over `big.Rat`, no network.

- `0.34210000 × 43250.75 = 14796.081575` → `1479608` minor units (half-up), and the half-up
  boundary either side of it.
- `1000000000 × 0.00002341` → `2341000` — the case a fixed `int64` scale would have overflowed.
- 18 fraction digits; a price in exponent notation; a holding worth less than a cent rounding
  to `0` without being an error.
- Portfolio 24h change is value-weighted: 1% of the portfolio moving 40% and 99% moving 0%
  gives `0.40`, not `20`.
- No `float64` in the value path.

### 7.2 Unit — refresh policy (`pkg/services/crypto_prices_test.go`)

The decision is a pure function so it can be driven without a database:

- 2-hour-old snapshot → not refreshed. 25-hour-old → refreshed.
- No snapshot at all → refreshed even though the cache has not expired.
- Attempt 60 seconds ago → too frequent, for the manual button, the expired cache and the
  missing snapshot alike.
- `RequestCountToday` at the maximum → limit exceeded, no request.
- A count from yesterday → resets.
- A fresh cache with the allowance exhausted → "not needed", not an error.
- First ever refresh from a zero state → allowed.

### 7.3 Unit — parsing (`pkg/cryptoprices/coinstats_datasource_test.go`)

A recorded response fixture with unread fields left in place. Asserts: unknown fields ignored;
`count` and `price` preserved exactly as printed; the price taken from the requested currency
and **not** from another one when that currency is absent; a coin without a count skipped;
`hasNextPage` respected; malformed JSON is a clean error. Plus request building — the api key,
share token and passcode headers, the path, and the paging parameters.

### 7.4 Unit — the `.env` reader (`pkg/settings/environment_file_test.go`)

Quoted values, `export` prefixes, values containing `=`, comments, malformed lines, a missing
file, and — the one that matters — a variable already set in the real environment winning over
the file.

### 7.5 Frontend (`src/models/__tests__/crypto_asset.test.ts`)

Shares sum to 100% when every coin has a price; a `null`-valued coin has no share rather than
a zero one; quantity formatting keeps every significant digit of a tiny amount while dropping
padding zeros.

### 7.6 Regression

`getNetAssets`, `getTotalAssets`, `getTotalLiabilities` and the statistics endpoints are
untouched in phase 1.

### 7.7 Manual

With a real free-plan key and share token: confirm the first load fetches once and the next
five load zero times, that the button is disabled for five minutes afterwards, that the fifth
refresh of the day is refused, and that a wrong share token produces the warning strip rather
than an empty page.

---

## 8. Acceptance criteria

1. The page shows the CoinStats portfolio with **no data entry anywhere in the feature**.
2. A coin's value equals count × price, converted to the user's default currency by the same
   path a foreign-currency account uses, with no `float64` in the value path.
3. Loading the page with a fresh snapshot makes **zero** upstream requests.
4. The snapshot refreshes automatically **at most once per 24 hours**.
5. A manual update button exists, is rate-limited to `min_refresh_interval`, and cannot exceed
   `max_requests_per_day` — enforced server-side, not by disabling the button.
6. The snapshot survives a server restart, because it is in the database.
7. A failed refresh never clears or zeroes the snapshot, and its cause is visible in the UI
   along with the true age of the figures.
8. A coin with no usable price shows a dash and marks the total incomplete; it never reads as
   zero.
9. The api key and share token never reach the browser, and both can be set from `.env`.
10. Net Assets, Total Assets and every statistics figure are numerically identical to what they
    are without this feature.

---

## 9. Phase 2 — wiring crypto into the overall stats

Deliberately not built. Recorded here because phase 1 is shaped to make it a small change.

The switch would be one boolean, `includeCryptoInTotalAmount`, defaulting to **false** — a
user application cloud setting exactly like `totalAmountExcludeAccountIds` already is
([pkg/models/user_app_cloud_setting.go:52](pkg/models/user_app_cloud_setting.go#L52),
[src/core/setting.ts:78](src/core/setting.ts#L78),
[src/stores/setting.ts:350-352](src/stores/setting.ts#L350-L352)).

When on, the crypto total joins the summation loops in
[src/stores/account.ts](src/stores/account.ts) — `getNetAssets` (line 474) and `getTotalAssets`
(line 508). Those loops already walk `{ balance, currency }` pairs and convert anything that is
not the default currency, setting `hasUnCalculatedAmount` and appending
`INCOMPLETE_AMOUNT_SUFFIX` when a rate is missing. A crypto total in USD is one more pair
appended to the list they iterate; the loops themselves do not change.
`getTotalLiabilities` is never touched — a holding is always an asset.

The **Statistics charts are a separate question and probably a "no"**: they are built entirely
from transactions, and a holding has none. Showing crypto there would mean synthesising
transactions that never happened, which would corrupt income and expense figures. The honest
version is a net-worth-over-time chart fed by snapshot history (§11), not an entry in the
existing charts.

---

## 10. Alternatives considered

- **Manual holdings entered in ezBookkeeping.** Built first, then removed: it is a second
  ledger to keep in sync with the one CoinStats already maintains, and the typed one goes stale
  the moment a trade happens. Reading the portfolio is strictly less work for the user and
  strictly more correct.
- **Crypto as a currency.** Widen `Account.Currency`, extend `validCurrency`, inject crypto
  rates into `LatestExchangeRateResponse`. Every account, transaction, import and statistics
  path would then handle crypto for free — genuinely attractive. Rejected because the currency
  code is `VARCHAR(3)` in a shipped schema, `validCurrency` is used across ~24 files of request
  bindings, and a 24-hour-old price masquerading as an exchange rate would silently revalue
  historical transactions.
- **Reading the portfolio in each user's currency.** One cached snapshot per distinct currency,
  and the crypto value would drift from every other converted amount on the page because it
  came through a different rate source.
- **`/portfolio/value` for the total.** 10 more credits per refresh for a number this page can
  compute exactly from the coins it already has. Worth it only alongside P&L (§11).
- **A daily cron job.** [pkg/cron/cron_jobs.go](pkg/cron/cron_jobs.go) makes it a ten-line
  change and it would keep the snapshot warm. Rejected as the default because it spends the
  quota on an idle instance. Worth adding later behind
  `[cron] enable_refresh_crypto_portfolio = false`.
- **Browser-side caching (localStorage), like exchange rates.** A second browser or a private
  window would each pay for their own refresh, and a restart would lose everything.
- **Calling CoinStats from the browser.** Would put the api key and share token in the client.
  Never.

---

## 11. Further follow-ups

- **Cost basis and P&L** from `GET /portfolio/value` (10 credits): total cost, unrealized,
  realized and all-time profit/loss. One more request per refresh, ~18 credits per refresh
  total, still under 2,500 a month at the current cap.
- **Snapshot history**, one row per day, enabling a portfolio-value-over-time chart. The daily
  refresh already produces exactly one data point per day; only the write is missing.
- **Transaction history** from `GET /portfolio/transactions`, which would make importing crypto
  buys as ezBookkeeping transactions possible.
- **Mobile page** (`src/views/mobile/crypto/`), which this cut deliberately omits.
- **A second data source** (CoinGecko), which is why §5 puts a provider interface in front of
  CoinStats rather than calling it directly.
