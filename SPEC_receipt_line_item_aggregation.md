# Spec: server-side aggregation of receipt line items

Status: proposal, ready to implement
Target: ezbookkeeping backend (Go) — upstream source, `github.com/mayswind/ezbookkeeping`
Author context: written from a debugging session against a local Windows build, Ollama +
`qwen3-vl:8b-instruct`, German (Lidl) receipts.

---

## 1. Problem

Receipt image recognition asks the LLM to do two very different jobs in one shot:

1. **Read** the receipt — OCR each line, its price, and pick a category. 
2. **Aggregate** — group lines by category and sum each group.

Job 1 works. Job 2 fails consistently on small local models, and it fails *silently* —
the output is well-formed JSON with plausible-looking numbers, so nothing downstream
detects it.

### 1.1 Evidence

Test receipt (Lidl Berlin, 28.07.2026, total **32,79 €**), 14 purchased lines:

| # | Line | Price |
|---|------|-------|
| 1 | Broccoli | 1,49 |
| 2 | Kartoffeln früh | 2,99 |
| 3 | Strauchtomaten | 1,69 |
| 4 | Bio Möhren | 1,89 |
| 5 | Kiwi | 2,29 |
| 6 | Heidelbeeren | 2,19 |
| 7 | Frischer O-Saft o.F. | 2,59 |
| 8 | Pfand 0,25 EM | 0,25 |
| 9 | Monster Mango Loco | 1,49 |
| 10 | Pfand 0,25 M | 0,25 |
| 11 | Milchcreme Cookies | 1,49 |
| 12 | Papiertragetasche | 0,20 |
| 13 | Akku Ni-MH-0532273 | 6,99 |
| 14 | Akku Ni-MH-0532273 | 6,99 |

Observed failures across four prompt iterations, all with valid JSON returned:

- **Amounts lifted from the VAT summary table.** Returned `14.03` (the `Brutto` cell of the
  `A 7%` row) and `3.92` (the `Summe MWST` cell) as transaction amounts. Total 17,95.
- **Sums invented.** Its own itemization listed `1.49 + 2.99 + 1.89 + 2.29`, and it emitted
  `14.32` as the group amount (correct: 8,66).
- **Cross-category contamination.** With an explicit `calculation` field forcing it to write
  the addition before the result, the `Food` group's calculation was
  `1.49 + 2.99 + 1.89 + 2.29 + 2.19 + 2.59 + 0.25 + 1.49 + 0.25 + 0.20 + 6.99 + 6.99` —
  every category's items mixed together, including batteries, yielding `Food = 32.79`
  (the whole receipt total). The same Akku lines were simultaneously counted in
  `Electronics = 13.98`. Groups summed to 63,94 against a 32,79 receipt.
- **Dropped lines.** In two runs, items vanished entirely (`Milchcreme Cookies`, both
  `Akku` lines, `Papiertragetasche`).

By contrast, in the same failing response the **line-item stage was structurally perfect**:
all 14 lines present, German names correct and verbatim, categories correct
(`Akku → Electronics`, `Papiertragetasche → Houseware`, `Pfand → Drink`, `Kiwi → Fruit &
Snack`).

### 1.2 Why the model cannot do this

- `format: "json"` is sent on every request, which grammar-constrains output. A reasoning
  model cannot emit prose or close a `</think>` block under that constraint, so **thinking is
  unavailable regardless of the `enable_thinking` setting**. (Observed directly: with
  `qwen3-vl:latest`, a thinking-capable model, the entire JSON answer was emitted into
  Ollama's `thinking` field and `content` came back empty, so `parse_import` failed with
  "not found transaction data". Switching to `qwen3-vl:8b-instruct` fixed that.)
- Without a reasoning channel, an 8B model must perform multi-operand decimal addition in a
  single forward pass while also tracking group membership. It doesn't; it pattern-matches
  numbers that "look like totals" — which is why the VAT table is such a strong attractor.
- Prompt engineering was exhausted: explicit bans on the MWST table, a worked example, a
  JSON scratchpad forcing itemization first, and a `calculation` field forcing the operands
  to be written before the result. Each fixed some instances and none fixed the class.

Aggregation is deterministic arithmetic over data the model already produced correctly.
It belongs in Go.

---

## 2. Goal

Move grouping and summing out of the prompt and into the backend.

- The LLM returns **line items** (name, price, category) — what it is reliably good at.
- The backend groups by category, sums with exact decimal arithmetic, and builds the
  transactions.
- The backend **validates** the itemization against the receipt total and surfaces a warning
  when they disagree, so silent OCR errors become visible instead of landing in the ledger.

### Non-goals

- Improving OCR accuracy. (See §8 — a separate, still-open problem.)
- Changing the import UI's data model or the transaction schema.
- Any change to text-based (non-image) transaction recognition.

---

## 3. Design

### 3.1 New LLM response contract

The receipt-image prompt asks for `line_items` instead of `transactions`:

```json
{
  "receipt_total": "32.79",
  "time": "2026-07-28 20:12:00",
  "account": "Checking account",
  "line_items": [
    { "name": "Broccoli",           "price": "1.49", "category": "Food" },
    { "name": "Kartoffeln früh",    "price": "2.99", "category": "Food" },
    { "name": "Pfand 0,25 EM",      "price": "0.25", "category": "Drink" },
    { "name": "Akku Ni-MH-0532273", "price": "6.99", "category": "Electronics" }
  ]
}
```

Field notes:

- `receipt_total` — read from the `Zu zahlen` / `Summe` / `Gesamt` line. Used only for
  validation, never as a transaction amount. Optional; validation is skipped if absent.
- `time`, `account` — receipt-level, applied to every generated transaction. Same
  semantics and format as today (`YYYY-MM-DD HH:mm:ss`).
- `price` — a decimal string. Must be the price printed on that line. May be negative for
  discount lines (see §5).
- `category` — must match one of the user's expense category names exactly.

### 3.2 Backward compatibility

Keep accepting the current `transactions` array. Resolution order when parsing a response:

1. If `line_items` is present and non-empty → aggregate it (new path).
2. Else if `transactions` is present → use as-is (current path, unchanged).
3. Else → existing "not found transaction data" error.

This keeps non-receipt images (single vouchers, bank screenshots, transfer confirmations)
working through the old path with no prompt changes, and lets the feature ship behind a
config flag without regressing anything.

### 3.3 Aggregation algorithm

```
group := map[categoryName][]lineItem   // preserve first-seen order for stable output

for each item in line_items:
    if item.price is unparseable        -> collect into `skipped`, continue
    if item.category is empty/unknown   -> assign fallback category (§5), record warning
    append to group[item.category]

for each category in first-seen order:
    amount      := exact sum of that group's prices
    description := join(item names, ", ")
    emit transaction{ type: expense, time, account, category, amount, description }
```

**Arithmetic must be exact.** Use the same decimal handling the rest of the codebase uses for
transaction amounts (integer minor units, e.g. cents, or the existing amount parser). Do not
use `float64`. Parse `"1.49"` → `149`, sum as integers, format back at the end.

Follow whatever the existing importer already does to turn an amount string into the internal
representation, so that rounding and currency behaviour stay consistent with manual entry and
CSV import.

### 3.4 Validation

After aggregation, if `receipt_total` was supplied:

```
sum(all line item prices)  ==  receipt_total   ?
```

- **Match** → proceed normally.
- **Mismatch** → still return the transactions (the user can fix them in the Check & Modify
  step), but attach a warning carrying: the computed sum, the stated receipt total, the
  difference, and the number of line items parsed.

This is the single highest-value part of the feature. In the test run above, the model's
line items summed to 30,06 against a stated 32,79 — a 2,73 discrepancy caused by
misread prices that would otherwise have been imported silently. The user needs to see that.

Implementation note: check how `parse_import.json` currently returns non-fatal information to
the frontend. If there is no warning channel, the minimum viable version is a `[WARN]` log
line plus a new optional field on the parse response; the frontend can render it later
(§8). Do not turn a mismatch into a hard error — a slightly misread receipt is still worth
importing after manual correction.

### 3.5 Configuration

Add to `[llm_image_recognition]` in `conf/ezbookkeeping.ini`:

```ini
# Whether the LLM should return individual receipt line items which the server then groups
# by category and sums itself, instead of asking the LLM to do the grouping and arithmetic.
# Strongly recommended for small local models. Default is true.
receipt_line_item_aggregation = true
```

When `false`, use the existing prompt and the existing `transactions` path — a clean escape
hatch if a user runs a large hosted model that aggregates fine.

### 3.6 Prompt template

`templates/prompt/batch_receipt_image_recognition.tmpl` needs a variant (or a conditional
block) for the line-item mode. The heavily-iterated German-receipt prompt in this repo
already contains the parts worth keeping — carry them over verbatim:

- German-language rules (no translation; category names stay English; `&amp;` → `&`)
- Date rules: `YYYY-MM-DD HH:mm:ss`, never ISO 8601, and **ignore the TSE
  `2026-07-28T18:12:39.000Z` signature timestamps** — use the human-readable local
  `28.07.2026 20:12` near the payment block
- The "what is NOT a line item" list — VAT summary table (`MWST% / MWST / Netto / Brutto`,
  rows `A`/`B`/`Summe`), totals, payment lines, Payback / Lidl Plus, TSE block, header/footer
- The note that trailing `A` / `B` on item lines are VAT markers, not prices
- Pfand handling: not included in the drink's price, printed as its own line, inherits the
  drink's category
- The non-food keyword list (`Akku`, `Papiertragetasche`, `Spülmittel`, …) that stops
  everything landing in `Food`

Everything about grouping, summing, `calculation`, and "one transaction per category" is
deleted — the model is now told only to copy lines out.

---

## 4. Where to change

The local deployment is a compiled binary, so these are anchors from runtime logs rather than
verified file paths. Locate by symbol name in the upstream Go source:

| Symbol seen in logs | Role |
|---|---|
| `transactions.TransactionParseImportFileHandler` | HTTP handler for `POST /api/v1/transactions/parse_import.json` |
| `data_table_transaction_data_importer.ParseImportedData` | Turns parsed rows into transaction data; where "transaction time is invalid" and "not found transaction data" originate |
| `common_http_large_language_model_provider.getTextualResponse` | Provider-agnostic response unwrapping; reads `message.content` |
| `ollama_large_language_model_adapter.buildJsonRequestBody` | Builds the Ollama request; this is what sets `"format":"json"` and `"think"` |

The JSON contract for the LLM response is defined near the image-recognition importer —
find the struct that currently unmarshals `{"transactions":[...]}` and add `LineItems` and
`ReceiptTotal` alongside it. Aggregation should sit between unmarshalling and the existing
"build transaction data" step, so that the old path and the new path converge on the same
downstream code.

---

## 5. Edge cases

| Case | Required behaviour |
|---|---|
| **Pfand (deposit)** | An ordinary line item. Not included in the drink's price on German receipts, printed separately, inherits the drink's category. No special handling in Go — it just sums. |
| **Deposit return** (`Leergut`, `Pfandrückgabe`) | Decided: kept, as a line of its own. Recognized by name before its price is trusted, because a refund read with the wrong sign is still a refund, and never charged against the item printed above it — the bottles were bought weeks ago. It is grouped apart from the purchases of its category and imported as a **negative expense** there, so that it cancels out the deposits that were charged to the same category when the bottles were bought. Negative amounts do reach transaction creation, which the schema allows (`SourceAmount` is bound `min=-999999999999999`) and which raises the account balance, since an expense is booked as `balance - amount`. |
| **Discount lines** (`Rabatt`, `Nachlass`, `Coupon`) | Negative price attached to the preceding item. Simplest correct handling: let the prompt emit them as their own negative line item in the same category as the item above, and let aggregation net them out. |
| **Unknown / hallucinated category** | Do not drop the item — that silently loses money. Map to a configurable fallback (`Other Expense`) and record a warning naming the item. |
| **Unparseable price** | Skip the item, record a warning naming it. It will also surface via the total mismatch. |
| **Empty `line_items`** | Fall through to the `transactions` path, then to the existing error. |
| **A category group summing to 0.00** | Emit nothing for that group. |
| **Multiple images in one batch** | Each image is its own receipt: aggregate per image, never across images. `receipt_total` validation is per image. |
| **Non-receipt image** (single voucher, transfer) | Model returns `transactions`, not `line_items`; old path handles it. Prompt must permit this. |
| **Duplicate identical lines** (`Akku` ×2) | Both are real purchases. Keep both, sum both. Do not deduplicate. |
| **An article bought before** | Implemented: filed under the category the user last put it in, overriding the model. See §5.1. |

---

### 5.1 Remembered categories

The model re-guesses the category of the same weekly shop every time, and guesses differently
from run to run — the same image produced 41.74 and 42.94 on consecutive runs. Categorizing is
the one part of reading a receipt where the answer does not have to be re-derived at all: the
user already gave it, the last time they bought the article.

Every line of an imported receipt is recorded as `article name → category id`, in
`receipt_line_item_category` (one row per user per article, `UQE_..._uid_normalized_name`).
Recording happens after the import succeeds, so what is kept is what the user accepted; a
receipt they cancelled teaches nothing. Every line is recorded rather than only the ones they
dragged — an article filed correctly by chance should not be left to chance next time.

On the next recognition the lines are looked up before they are grouped:

- The name is normalized first — lowercased, diacritics folded (`früh` → `fruh`), punctuation
  dropped, whitespace collapsed. Most re-encounters are an exact hit on that key and never
  reach the fuzzy pass at all.
- Otherwise the closest remembered name within `ReceiptLineItemNameSimilarityThreshold` (0.9,
  normalized Levenshtein) wins, which absorbs a genuine misread of about one character in ten.
- A fuzzy match must agree on **every digit**. `Pfand 0,25` and `Pfand 0,15` are 90% alike and
  are not the same thing; neither are two article numbers differing in the last place.
- The user's answer beats the model's whenever it exists. Where they have said nothing, the
  model's choice stands.

The lookup runs **after** parsing, so it acts on exactly the lines the user can see and drag: a
deposit already folded into its drink is never looked up on its own and cannot be filed
somewhere the drink is not. Refunds are looked up too, and stay in a group of their own.

The category is remembered by **id** and resolved to its current name at recognition time, so
renaming a category keeps everything filed under it. An entry whose category has since been
deleted or hidden resolves to nothing and is skipped — which is also how the memory forgets.

Lines placed from memory come back flagged (`remembered` on the line item response) and are
marked in the assignment board, so the mechanism is visible rather than magic. Dragging such a
line away and importing overwrites the entry: correcting a wrong memory is the same gesture as
teaching it in the first place.

#### Seeding it from the receipts already imported

The memory starts empty, but the answers are not lost — a receipt import writes the names of the
lines it summed into the comment of the transaction it created, joined by `", "`, so every receipt
imported so far already records which articles were filed under which category.

```
ezbookkeeping userdata receipt-line-item-category-learn -n <username> [--dry-run] [--since <time>]
```

`--since` defaults to **2026-08-06 21:43:07**, the mojibake repair (`3ef95920`). The aggregation
itself landed on 2026-07-31 (`ae0dbedd`), but for that first week every non-ASCII name reached the
comment already corrupted — `Kartoffeln frÃ¼h` normalizes to `kartoffeln frã¼h` and could never
match a correctly read receipt again. Names are filtered through `utils.ContainsMojibake` as well,
so a corrupted one is skipped rather than stored even if the cutoff is moved back.

Parsing the comment back into names:

- The separator is `", "`, never a bare comma — a German price or weight inside a name carries its
  comma with no space after it (`Pfand 0,25 EM`).
- A comment that hit the 255-character limit ends in `…` and was cut **on an article boundary**, so
  every name before the ellipsis is whole and only the articles beyond it were lost.
- A single name that was itself cut short has no boundary to fall back on and is dropped entirely.
- Transactions are read oldest-created first, so the most recent answer is the one that survives —
  the same rule as importing the same receipt twice.

Only expense transactions in an existing expense sub-category are read. Nothing marks a transaction
as receipt-derived, so a hand-written comment containing `", "` can still be misread as a list of
articles; `--dry-run` prints every pair it would record and changes nothing.

---

## 6. Testing

### 6.1 Unit tests — aggregation

Pure function, no LLM needed. Use the Lidl receipt above as the primary fixture.

Input: the 14 correct line items from §1.1. Expected output, 5 transactions:

| Category | Items | Amount |
|---|---|---|
| Food | Broccoli, Kartoffeln früh, Strauchtomaten, Bio Möhren, Milchcreme Cookies | **9.55** |
| Fruit & Snack | Kiwi, Heidelbeeren | **4.48** |
| Drink | Frischer O-Saft o.F., Pfand 0,25 EM, Monster Mango Loco, Pfand 0,25 M | **4.58** |
| Electronics | Akku Ni-MH-0532273 ×2 | **13.98** |
| Houseware | Papiertragetasche | **0.20** |

Sum = **32.79** = `receipt_total` → no warning.

Additional cases:
- Same fixture with `Strauchtomaten` price changed to `1.89` → sum 32,99 ≠ 32,79 → warning
  with difference `0.20`.
- Fixture with a line whose category is `Sonstiges` (not a user category) → lands in the
  fallback category, warning names the item.
- Fixture with a `-0.50` `Rabatt` line → its group's amount is reduced by 0,50.
- Empty `line_items` with a populated `transactions` array → old path, unchanged output.
- Decimal precision: `0.1 + 0.2` must produce exactly `0.30`.

### 6.2 Integration test

Record a real Ollama response for the Lidl image as a fixture and assert the full
`parse_import` path produces the 5 transactions. This also locks in that unknown fields in
the LLM response are ignored rather than fatal.

### 6.3 Manual verification

Run the real image through a local Ollama with `qwen3-vl:8b-instruct`. Expect correct
categories and grouping; expect the total-mismatch warning to fire if the photo is tilted
(see §8).

---

## 7. Acceptance criteria

1. A German supermarket receipt yields one transaction per category, with amounts that are
   exact sums of the line items in that category.
2. No transaction amount is ever traceable to the VAT summary table or the receipt total.
3. No purchased line is silently dropped: every line either lands in a transaction or is
   named in a warning.
4. A mismatch between the line-item sum and the printed receipt total produces a visible
   warning and does not block import.
5. `receipt_line_item_aggregation = false` reproduces today's behaviour exactly.
6. Non-receipt images continue to work through the `transactions` path.
7. Arithmetic uses exact decimal handling — no `float64` anywhere in the sum path.

---

## 8. Known remaining problem: OCR row-slip (out of scope)

Independent of aggregation, and worth knowing before you trust the output.

On a photo where the receipt is even slightly rotated, the model reads prices from the
neighbouring row. On the test receipt, prices were consistently taken one row off, e.g.
`Strauchtomaten` → `1.89` (which is Bio Möhren's price) and `Frischer O-Saft` → `0.25`
(which is the Pfand line's). Across the width of a receipt, a ~2° tilt accumulates to a full
line height, and the model follows the visual alignment.

The `receipt_total` validation in §3.4 is what makes this *detectable* — it converts a silent
wrong import into a visible warning. Actually fixing it needs one of:

- **Deskew before inference** — detect the dominant text angle and rotate the image square
  before sending it to the LLM. Highest-value follow-up; benefits every provider.
- A larger vision model (`qwen3-vl:32b` and up), which needs VRAM most self-hosters lack.
- User discipline: photograph square-on.

## 9. Optional follow-ups

- **Show line items in the Check & Modify step**, collapsed under each transaction, so the
  user can see what was aggregated and spot a misread price before importing.
- **Make `format: "json"` configurable per provider.** It is what makes thinking impossible
  (§1.2). With it off and a thinking model, the LLM could reason before answering — at the
  cost of needing tolerant JSON extraction from the response text.
- **Per-item import mode** — a config option to emit one transaction per line item instead of
  per category, for users who want item-level history. Trivial once line items are a
  first-class concept.

---

## 10. Build notes

The local install is a prebuilt `ezbookkeeping.exe`; implementing this requires building from
source. Upstream needs Go and Node (backend + frontend). Configuration lives in
`conf/ezbookkeeping.ini`, prompt templates in `templates/prompt/`. Prompt template changes
take effect on restart with no rebuild — useful for iterating on §3.6 separately from the Go
work.
