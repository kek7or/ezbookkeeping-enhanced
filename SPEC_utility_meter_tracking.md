# Spec: utility meter tracking

Status: **implemented**
Target: this fork — Go backend (`pkg/…`) + Vue 3 / Vuetify frontend (`src/…`)
Scope: **a standalone page for meters, their tariffs and their readings.** No transaction is ever
created, read or changed by any of it; see §2.

---

## 1. Problem

A German energy contract is not billed the way it is paid. The supplier collects a fixed monthly
**Abschlag** (prepayment) and settles the real cost once a year against the meter, which arrives as
a refund or a demand nine months after the money was spent. Everything needed to see that coming is
already on hand — the meter on the wall and the tariff on the bill — and none of it fits in the
ledger:

- **A transaction says what left the account.** That is the Abschlag: the same figure every month,
  which is exactly the number that is *not* what the energy cost. Recording usage as transactions
  would put money in the ledger that never moved.
- **The spreadsheet this replaces** is a column per month holding the meter start, the meter end,
  the difference, a formula, and a running gap against the Abschlag. It works, and it has to be
  re-derived every year, has no place to put a mid-year price change, and quietly hides the fact
  that its Brennwert was a guess.

What is wanted is small: enter a reading when you take one, and see what it cost, what you paid, and
what the year is heading for.

### 1.1 The arithmetic being replaced

Electricity, priced per kilowatt hour as the meter reads them:

```
cost = used kWh x Arbeitspreis + Grundpreis
```

Gas, where the meter counts volume but gas is sold by energy:

```
kWh  = m³ x Brennwert x Zustandszahl
cost = kWh x Arbeitspreis + Grundpreis
```

The **Brennwert** (calorific value, kWh/m³) and the **Zustandszahl** (state number, correcting the
volume measured at the meter's pressure and temperature to standard conditions) are both printed on
every German gas bill. H-Gas runs roughly 10–13.1 kWh/m³, so a Brennwert taken as a round 10 can be
a fifth of the bill out on its own. Both are per-meter, per-contract facts and belong in
configuration, not in a formula.

## 2. Goals

- **Enter a reading, get an estimate.** What was used between two readings, what it cost at the
  tariff in force, what was prepaid over the same days, and the difference.
- **Every number of the tariff configurable** — unit price, Grundpreis, Abschlag, Brennwert,
  Zustandszahl — with **a tariff history**, so a price change in the middle of a billing year prices
  the months on each side of it the way they were actually paid for.
- **The billing year is the supplier's**, not the calendar's, because that is the year the refund
  covers.
- **A projection**: what the year ends at if the rest of it runs like the part already read.

### Non-goals

- **Any transaction.** Nothing here writes, reads or references the ledger. The Abschlag is already
  in it as the standing order it is.
- **Any effect on Net Assets, the home page or the statistics charts.**
- **Being the bill.** The supplier rounds differently, may bill on its own estimated readings, and
  applies tax treatment this knows nothing about. This is an estimate that tells a refund from a
  demand months early, and nothing more.
- Tariff import, supplier APIs, price comparison.
- Mobile UI. Desktop only, as with the crypto page.

## 3. Data model

Three tables, all soft-deleted and scoped by `uid`, synced in
[cmd/database.go](cmd/database.go) and defined in
[pkg/models/utility_meter.go](pkg/models/utility_meter.go).

**`UtilityMeter`** — a name, a kind (electricity / gas / water / heating / other, which decides only
the icon and the wording), the unit the dial counts in (kWh / m³ / l), a currency, the meter number
off the dial, and `BillingYearStartMonth`.

**`UtilityTariff`** — what the meter costs from `StartDate` (YYYYMMDD) on, until the next tariff of
that meter begins. The earliest tariff also covers everything before it, so a year of back-entered
readings is priced rather than left blank.

**`UtilityReading`** — one number off one dial on one day, plus an `Estimated` flag for a figure
that was worked out rather than seen. **The reading is stored, never the difference**: a reading can
be checked against the dial a year later, a difference cannot.

### 3.1 Scales

Every number is an integer, because a tariff held as a float stops being the number the bill
printed:

| Number | Scale | Example |
|---|---|---|
| Meter reading | ×1,000 | `312.38 m³` → `312380` |
| Unit price | ×1,000,000 | `0.3280 /kWh` → `328000` |
| Brennwert, Zustandszahl | ×10,000 | `0.95` → `9500` |
| Grundpreis, Abschlag, every cost | minor units (×100) | `13.90` → `1390` |

The Grundpreis is stored **annually** in one form only; the form offers it per year or per month and
multiplies by twelve on the way in, since German bills quote it both ways.

## 4. The calculation

[pkg/services/utility_billing.go](pkg/services/utility_billing.go), pure functions over the rows,
tested in [pkg/services/utility_billing_test.go](pkg/services/utility_billing_test.go) against the
figures from the spreadsheet this replaces.

For each consecutive pair of readings:

1. **Cut at every billing year boundary inside the interval.** A meter read on the third of each
   month against a year that turns on the first would otherwise count two days of the old year into
   the new one. The reading at the cut is interpolated over the days on either side and the piece is
   flagged `partial`.
2. **Cut again at every tariff change**, and spread the use evenly over the days of each piece.
   Wrong in the small — a cold week costs more than a mild one — and the only division available
   from two readings with a price change between them.
3. Per piece: convert volume to energy (`units × Brennwert × Zustandszahl`, skipped entirely when no
   Brennwert is set), price it, and add `Grundpreis × days / 365` and `Abschlag × 12 × days / 365`.
4. `difference = prepaid − cost`, positive when in credit.

All of it in `int64` through `math/big`, rounded half away from zero, since two numbers each scaled
by up to a million overflow an int64 before the division brings them back down.

A reading below the one before it — a replaced meter, or one that rolled past its last digit — is
counted as **no use**, never as negative use, which would report a refund that is not owed.

### 4.1 The billing year

Periods are grouped into the supplier's year. Each year reports its totals over the days actually
read, plus:

- **`projectedCost`** — the cost so far carried across the whole year by days. The flattest
  projection there is: a gas year read only through the summer will project a refund the winter
  takes back, and the page says how many of the year's days it is based on.
- **`projectedPrepaid`** — twelve monthly collections at whatever the Abschlag stood at in each
  month, counted for the whole year including months not yet read, because that is how it is
  actually taken.
- **`projectedDifference`** — the refund to expect, or the demand.

### 4.2 Where it deliberately differs from the spreadsheet

The Grundpreis is charged **by the day** (`annual × days / 365`), not one whole month per row. A
June-to-July period at 11.90 a month therefore costs 11.74 rather than 11.90. This is the price of
letting a period be any number of days long; over a full year the two agree to within a rounding.

## 5. API

Ten routes under `/api/v1/utilities/`, registered in [cmd/webserver.go](cmd/webserver.go):

| Route | Purpose |
|---|---|
| `GET meters/list.json` | every meter, with its tariffs, its readings and every billing year worked out |
| `POST meters/add.json` | a meter, optionally with its first tariff in the same transaction |
| `POST meters/modify.json`, `meters/delete.json` | deleting takes the tariffs and readings with it |
| `POST tariffs/add.json`, `modify.json`, `delete.json` | |
| `POST readings/add.json`, `modify.json`, `delete.json` | |

The list is one response for the whole page: a meter is a handful of tariffs and a reading a month,
and the totals only mean anything once all of it is there.

**What is refused** ([pkg/errs/utility_meter.go](pkg/errs/utility_meter.go)): a reading below an
earlier one or above a later one (a dropped digit, caught while it is still one field to fix rather
than a month of impossible use); two readings of one meter on one day; two tariffs of one meter
starting on one day; a Zustandszahl with no Brennwert beside it, which would silently convert
nothing.

## 6. UI

`/utilities`, one card per meter ([src/views/desktop/utilities/](src/views/desktop/utilities/)):

- **Five figures for the selected billing year** — used, cost so far, paid so far, the balance, and
  what the year is projected to end at, coloured by sign.
- **A row per period** — the two readings, the days, the use (with the converted kWh underneath for
  a gas meter), the cost with its base fee called out, the prepayment, and the difference. Rows cut
  at a year boundary, sitting next to an estimated reading, or spanning a price change carry a note
  saying so.
- **Readings and tariffs** in expansion panels, each editable in place.
- The tariff form spells out the conversion as it is typed — *"one m³ off this meter counts as
  10.925 kWh"* — because a Brennwert typed into the wrong field is otherwise invisible until a
  yearly total is fifteen percent out. The reading form does the same for use since the previous
  reading.

## 7. Limits

500 tariffs and 5,000 readings per meter — far past any real meter, and only there so that one
meter cannot be made unboundedly expensive to read.
