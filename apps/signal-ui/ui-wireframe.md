# Signal UI — wireframe (composition, states, behavior)

> **Editing this doc:** Prefer **bullet lists** (and **tables** for state matrices) over long unbroken prose—keep the same detail, but make it scannable and easy to diff.

**Stack**

- Vite + Svelte 5 SPA, `svelte-spa-router`.

**Authenticated shells (`Nav`, Finance shell, and `<main>`)**

- **Generic authenticated shell:** Non-finance authenticated routes keep the existing top `Nav` plus the centered `main-inner` content column.
- **Nav — left:** Brand label **Signal Foundry** → `/chat`.
- **Nav — center:** **Chat** / **Data** / **Jobs** / **Finance** / **Providers** / **Strategies** / **Evaluations** / **Admin** share the same centered max-width column as `main-inner` on non-chat routes; the nav’s first grid column is content-sized so the brand does not overlap those links on narrow viewports.
- **Nav — right:** **Sign out** (text) **to the left of** the compact **Theme** segmented control; both **end-aligned** in the right margin; only when the generic authenticated shell is active.
- **≤700px Nav:** brand + auth controls stay on the first row; route links wrap onto a dedicated second row instead of colliding with the brand or sign-out cluster.
- **Canonical Finance shell:** `#/finance*` replaces the generic `Nav` with one shared Bootstrap Finance shell: desktop left rail for supported Finance destinations, finance utility header in the content column, shell-owned sign out and theme controls, and at most one compact shell-owned tenant switcher when multiple tenants exist.
- **Finance shell utility header:** On wider viewports it presents a compact semantic Bootstrap breadcrumb. Section pages use the section as the active item; detail routes link `Finance` and their section parent, then use the current detail as a non-link active item. Route content does not repeat a standalone parent backlink.
- **Finance shell responsive shape:** At narrow widths, the Finance shell keeps the full Bootstrap navigation visible by stacking the aside as a full-width section above the utility header and route content. Its compact horizontal-wrapping link rows preserve usable targets, while the breadcrumb and duplicate rail labels hide; tenant, theme, and sign-out controls remain shell-owned and wrap below it with no menu-toggle state.
- **`<main>`:** Centered content wrapper `main-inner` (~800–900px max width) with `Router` inside; Finance/admin/strategy/evaluation routes can widen it.
- **`/chat`:** `main-inner` is full width so the session rail can sit at the viewport’s left inset; shell is viewport-height with **no page scrollbar** — only the transcript strip scrolls.
- **Sign out:** `authStore.clearAuth()` (removes refresh token from `localStorage`, clears in-memory session) and `replace('/login')` so the user lands on the public login route without stacking authenticated history entries.
- **Unauthenticated routes:** No authenticated shell chrome; stored theme still applies to the global shell on load, and canonical public entry is `#/login`.

**Theme**

- Compact **Theme** segmented control in `Nav` (Auto / Light / Dark; Lucide icons only; `aria-label` / `title`).
- Default **Auto** (follows `prefers-color-scheme`); choice stored in `localStorage` (`signal-ui-theme`).

**Chat URLs (behavior)**

- `/chat` without a `sessionId` starts with an empty transcript.
- `/chat/{sessionId}` loads history from the server on open/reload via `readSession`.

**Scope:** This doc is **not** about visual styling (see `DESIGN.md`).

**Auth bootstrap**

- On mount, `App` calls `authStore.tryRestoreSession()` (reads refresh token from `localStorage`, attempts refresh).
- While restoring: `authStore.restoring` is `true` and a loading indicator is shown instead of the shell.
- After restore completes: Router renders and route guards apply.

**Response timestamp safety**

- Product-instant response fields must be valid RFC3339 instants. The current client parser additionally limits them to Go's signed Unix-nanosecond range.
- Missing, `null`, empty, malformed, year-one, or out-of-range values fail through the route's existing bounded API error state unless that field explicitly supports omitted or `null`. Finance ranges and FX values are ordinary timestamp instants.

**Route guarding**

- `/chat`, `/data`, `/jobs*`, `/finance*`, `/admin*`, `/providers`, `/strategies*`, and `/evaluations*`: protected with `svelte-spa-router`’s `wrap` + `conditions`.
- If `authStore.isAuthenticated` is false: `conditionsFailed` stores the requested protected route and then `replace('/login')`.
- `/login` is public.
- Nav is hidden when unauthenticated.
- `#/finance*` uses one shared Bootstrap Finance shell instead of the generic `Nav`.
- **Document titles:** Route pages own their title by rendering `DocumentTitle`, which supplies the `<svelte:head>` entry. Use current location first, then parent section when useful, then `Signal Foundry` (for example, `Accounts · Finance · Signal Foundry`). Loaded account detail names replace the generic detail label; fallback stays generic while loading or unavailable.

---

## Routes

| Path | Behavior |
| :--- | :--- |
| `/` | `replace('/finance')` on mount; brief status text. |
| `/login` | Canonical Bootstrap login page; username + password form inside a compact Bootstrap card. On success, sets auth tokens and `push()`es the remembered protected destination or `/finance`. On failure, shows inline error alert. |
| `/chat/:sessionId?` | Chat; optional id in URL after `sessionBound` (`replace`). One route entry so binding the id does not remount the page or abort the stream. |
| `/data` | Historical data browser. Protected (auth required). Browse-first availability loads on open, then exact candle reads stay editable. |
| `/jobs` | Durable historical ingestion job list. Protected. Summary-first stacked cards with filters, refresh, and open-detail actions. |
| `/jobs/:jobId` | Durable historical ingestion job detail. Protected. Separate route with request, timeline, worker, result, and safe error sections plus back links to Jobs and Data. |
| `/providers` | Provider configuration management page. Protected (auth required). |
| `/finance` | Canonical Bootstrap finance dashboard. Protected. Uses the shared Finance shell, keeps current-FX-valuation balance-first summaries in the first viewport, exposes previous/current/next/custom reporting-period controls, caps account/category/recent-transaction sections, and keeps missing/stale FX plus sync/import follow-up visible. |
 | `/finance/tenants` | Finance tenant selection with demand-driven create/update/invite/join/member actions. Protected. |
| `/finance/accounts` | Finance account browse workspace. Protected. |
| `/finance/accounts/new` | Dedicated Finance account create screen. Protected. |
| `/finance/accounts/:accountId` | Finance account detail route. Protected. Separate detail route with recent transaction context. |
| `/finance/connections` | Finance connection list/link/sync route. Protected. Schedule and last/next sync visibility stay here, and operators can locally delete a link without removing imported ledger history. |
| `/finance/connections/synthetic` | Finance synthetic setup route. Protected. Loads pending synthetic setup by returned `state`, allows save/reload/add/remove account rows, finishes the local link, and returns to `/finance/connections`. |
| `/finance/transactions` | Finance transactions browse/filter route. Protected. Table-first ledger browsing with explicit create/edit entry points. |
| `/finance/transactions/new` | Dedicated protected transaction create editor route with a single-record mobile-friendly layout. |
| `/finance/transactions/:transactionId` | Dedicated protected transaction edit route that reuses the transaction editor and loads tenant-scoped detail directly. |
| `/finance/categories` | Finance categories and tags management route. Protected. |
| `/finance/imports` | Finance CSV preview/confirm/import-audit route. Protected. |
| `/finance/jobs/:jobId` | Finance-context durable job detail route. Protected. |
| `/strategies` | Strategy workspace list + local draft editor. Protected. New drafts start here. |
| `/strategies/:strategyId/:version` | Strategy version detail. Protected. Saved version stays immutable; duplicate creates a local draft. |
| `/evaluations` | Evaluation run form + history table. Protected. |
| `/evaluations/run/:strategyId/:version` | Same evaluation history page with the run form preselected to a ready strategy version. Protected. |
| `/evaluations/:runId` | Evaluation detail page with summary/report and table-first evidence. Protected. |
| `/admin` | Admin diagnostics overview. Protected. |
| `/admin/jobs` | Generic admin job list. Protected. Utilitarian filters and stacked summaries. |
| `/admin/jobs/:jobId` | Generic admin job detail route. Protected. |
| `/admin/finance/fx` | Admin FX diagnostics and manual sync route. Protected. |
| `/admin/finance/providers` | Admin provider readiness and recent finance-job diagnostics route. Protected. |

- `/strategies*` and `/evaluations*` may use a wider centered canvas than the default reading column so dense forms and audit tables stay readable.

---

## Chat (`/chat/:sessionId?`)

**Layout**

- **Shell:** Two columns below `Nav`: **session sidebar** (~200px) + **main column**; fills viewport height; **no document scroll**.
- **Sidebar position:** Inset from the **left edge of the viewport** (same horizontal inset as global main padding).
- **`SessionList` (rail):**
  - **New chat** — full width of the rail (Lucide `SquarePen` + label; secondary styling; resets state and `push('/chat')`).
  - **Session rows** — scrollable list (title + relative time from `updatedAt`) that grows to use the rail’s height; each row links to `/chat/{sessionId}`; **active** session highlighted from route `sessionId`.
- **≤768px:** Sidebar is a **fixed** panel (narrower than the main column, flush **left** to the viewport), **starts collapsed** off-screen; **Sessions** in the main header toggles open/closed (`sidebar-collapsed` on the shell).
- **Desktop:** Rail’s **right border** spans the **full height** of the chat shell.
- **Main column:** Header (“Chat” page title), then **chat column** in order:
  - **Transcript strip** — flex fills space between header and composer; **overflow-y** only on this strip.
  - **Turn activity** → optional **model/profile banners** (loading, error, empty, or strategy-assistant guidance) → **composer** (textarea, then **execution profile** `<select>` + **model** `<select>` on the left and **Send** on the right) **pinned to the bottom** of the viewport.
- **Message chrome:** User turns — bordered, raised bubble, **end**-aligned. Assistant turns — plain body text on default surface (no card), including streaming lines.
- **Composer:** Shown **by default** on `/chat` even when the URL has no `sessionId` (implicit new chat). **Enter** submits the message; **Shift+Enter** inserts a newline in the textarea.

**Session list data**

- On mount and after each send stream reaches `done`, the page calls `GET /sessions?limit=50` (`listSessions`) to refresh the sidebar.
- Failures are ignored (best-effort).

| State | When | UI |
| :--- | :--- | :--- |
| New chat (default) | No URL `sessionId` | Sidebar + header + empty transcript + composer (default focus). **New chat** still clears draft and `push('/chat')` when starting from a session or re-clicked. |
| Composer with session | URL has `sessionId` | Sidebar + messages + composer. **New chat** in sidebar resets to `/chat`. |
| Reconnecting | `sessionId` in URL on mount; `readSession` in progress | Streaming UI shown (busy); input disabled until session loads. |
| Streaming | `streamState.busy` | Extra assistant activity group: status line ("Thinking…", running-tool progress, or waiting-for-next-step copy) plus any live text/tool calls until `done`. |
| Error | `runError` | Alert in the turn-activity strip (scroll region), directly above composer. |
| Send off | Empty input or `sendDisabled` | Send disabled; textarea disabled while sending. |
| No selectable models | `listModels` succeeded with zero models, or not yet successful | Send disabled; short copy with in-app link to `/providers`. |
| Model options load error | `listModels` failed | Error alert; Send disabled until a successful load yields at least one selectable model. |
| Model options loading | `listModels` in flight | Send disabled; “Loading models…” shown above the composer. |

**Session / API**

- No session → `startAgentRun`; else → `continueAgentRun`.
- On `sessionBound`: `replace('/chat/{id}')` — the UI does **not** call `readSession` for that transition; the POST response stream is the only consumer until `done`.
- On `done`: append assistant text to `messages` and clear in-flight turn state.
- Errors from HTTP or stream → `runError`.
- New send or unmount aborts via `AbortController` (hydration vs agent-run scopes are separate).
- All API calls include `Authorization: Bearer <accessToken>` from `authStore`; `userId` is not sent in the body or query (server derives identity via `CallerIdentity`).

**Execution profile + model pickers**

- On mount: `listModels()` (`GET /models`).
- Also on mount: `listAgentProfiles()` (`GET /agent-profiles`) as a best-effort execution-profile load.
- Until `listModels()` succeeds with at least one selectable model: Send disabled.
- If `listModels()` fails: error shown; Send stays disabled.
- If `listModels()` succeeds with an empty list: short message + link to **Providers** (`/providers`) — no implied default model.
- When execution profiles load successfully: a non-blocking **Execution profile** `<select>` appears with **Direct model** first and saved profiles after it.
- If `strategy-assistant` exists and the user has no saved selection, it is preselected for the alpha flow.
- When `strategy-assistant` is selected: show a short bounded-workflow note (`data discovery → validate/save → evaluate → evidence critique`; no live trading or readiness claims).
- When models exist: **Model** `<select>` stays on the same row as **Send**; the request still carries the selected model even when a regular profile is active.
- Selected value: `localStorage` (`selectedModel`); restored on load if still in the list; otherwise first model selected and persisted.
- Execution profile selection is stored in `localStorage` (`selectedProfile`) when non-empty.
- Sends: fully-qualified name (`provider/model-name`) as `model` on `AgentRunRequest` while the list is valid, plus `profileName` when an execution profile is selected.

**Reconnection (URL has `sessionId` on mount)**

- `readSession` (GET SSE).
- Stream order: `sessionBound` (confirms session) → `sessionStatus` (`active` or `idle`) → zero or more `agent` events (history) → optionally live `agent` if active → `done`.
- **`idle`:** Historical `agent` events folded into the transcript (`user` and `model` from SSE); model chunks use the same incremental aggregation as live runs; turns flush on `turnComplete`, role transitions (user after model), trailing model text on `done`; composer ready when idle.
- **`active`:** Streaming UI while live events arrive; on `done`, streamed turn appended to `messages`.
- Subsequent sends: `continueAgentRun` as usual.

**Tool call blocks**

- Agent turns may include tool calls alongside or instead of text.
- Mixed assistant text and tool calls preserve the original stream order during both live runs and replayed history refreshes.
- Consecutive assistant rows that contain only tool calls stack with tighter spacing than normal message turns so tool bursts read as one cluster.
- Each: `ToolCallBlock` — collapsible `<details>` / `<summary>`.
- **Collapsed (default):** "Tool call:" + function name (e.g. `workspacefs_write_file`).
- **Expanded:** Formatted JSON for "Arguments" (`args`) and, if present, "Response" (`response`).
- When the payload exposes strategy/version or evaluation run identifiers, show quick links to the matching strategy or evaluation detail route.
- Placement: Below assistant text, or standalone if the turn has no text.
- Same in streaming turn-activity during a run and in committed transcript after `done`.
- Distinct muted-border style vs plain text messages.

**A11y**

- List: `aria-live="polite"`.
- Errors: `role="alert"`.
- Labeled input.

---

## Historical data (`/data`)

**Layout**

- Header: **Historical data** heading + short explanatory copy.
- Availability panel first, listing available normalized candle entries as **venue + symbol + asset class** cards/buttons.
- Each availability entry shows:
  - nested timeframe summaries (**timeframe**, **count**, **start/end range**)
  - that entry’s deterministic **default slice** (**timeframe**, **start**, **end**)
- Filter form remains available below availability, with fields for **venue**, **symbol**, **asset class**, **timeframe**, a shared client-local range picker, and optional **ingestion run ID**.
- The shared range picker stores native JavaScript `Date` values and shows separate client-local **date** and **time** DOM controls, visible resolved instant `start` / `end` values, and deterministic quick presets whenever the selected availability/timeframe exposes an anchor. ISO/RFC3339 remains transport or deep-link serialization only.
- Results below the form in this order once a candle scope exists:
  - explicit **Start historical backfill** panel using the current form scope; this is the only mutating path on the page and shows the created job link/status when successful
  - summary cards (**N normalized candles**, raw payload metadata loaded/not loaded, selected candle status, selected availability entry when present)
  - full-width normalized candle panel (chart + reliable selection table + visible selected-candle banner)
  - linked raw evidence panel for the selected candle
  - raw payload metadata panel (explicit secondary load + table + row detail action)
- Raw payload detail opens in a centered large dialog panel with dialog semantics, close button, copy actions, and an explicit note when only a bounded preview is available.

**Behavior**

- Route is protected; unauthenticated users are redirected to `/login` using the same guard behavior as other protected routes.
- Initial render calls `GET /api/v1/data/candle-availability` for the first page.
- If the availability endpoint returns `404 Not Found`, the page shows a non-alert note that points to a likely stale/older backend mismatch and keeps the manual exact candle form fully usable.
- When `/data` opens with `venue`, `symbol`, `assetClass`, `timeframe`, `start`, and `end` query params in the hash route, the page applies that exact scope and loads those candles instead of the browse-first default selection.
- If that exact-scope route load is present and availability returns `404 Not Found`, the page still loads the routed candle scope while showing the compatibility note.
- When the first availability page includes `defaultSelection`, the page immediately calls `GET /api/v1/data/candles` with that exact **venue**, **symbol**, **assetClass**, **timeframe**, **start**, and **end**.
- If availability is empty, the page shows an empty state and MUST NOT guess candle filters or call the candle endpoint.
- Selecting an availability entry uses that item’s per-entry `defaultSlice`, updates the filter form to match, and loads normalized candles for that exact slice.
- Manual **Load candles** validates required filters client-side before calling `GET /api/v1/data/candles`.
- **Start historical backfill** is explicit and separate from browse/load/select paths:
  - validates the same required current form scope before calling `POST /api/v1/jobs/historical-data-backfills`
  - includes optional idempotency key and page size controls (`0` delegates to the backend default page size)
  - shows the created job id/status, a route link to `#/jobs/:jobId`, and a visible **Reload availability** action on success
  - does **not** run during availability auto-load, manual candle load, candle selection, or raw payload browsing
- For the current Hyperliquid v0 contract, the backfill panel maps the Data-page `crypto` browse asset-class label to the backend job request's expected `future` asset-class value and calls this out in panel copy.
- Client-side validation covers:
  - missing required fields
  - invalid or incomplete range values before a valid instant range is resolved
  - `start >= end`
  - the documented 10,000-interval cap using the selected timeframe (matching the server rule and message)
  - selected availability window bounds when the current timeframe has persisted availability
- Each valid candle load starts a new candle-scope request, clears prior candle-linked evidence and broad raw metadata results immediately, and only applies responses from the latest submitted scope.
- All data calls use the authenticated app fetch wrapper (Bearer + refresh flow), not anonymous fetches.
- Candle chart renders persisted normalized candle OHLC values only; it does not synthesize missing intervals.
- The first returned candle row is selected automatically when available, and the page calls `GET /api/v1/data/candle-raw-payloads` with that candle’s `provenanceSource` and `provenanceIdentity`.
- Selecting a different candle row reloads linked evidence through the same provenance-based endpoint and updates visible selected state beyond the row highlight (selected button/banner/summary).
- Broad `GET /api/v1/data/raw-payloads` browsing is explicit and secondary:
  - it does **not** run on initial browse-first availability/default-candle load
  - it runs only when the user clicks **Load raw payload metadata** for the current candle scope
- Selecting a raw payload metadata row calls `GET /api/v1/data/raw-payloads/{id}` and opens the detail dialog with bounded preview metadata, copy actions, and explicit guidance when the backend only exposes a preview/body ref instead of the full payload.

| State | When | UI |
| :--- | :--- | :--- |
| Availability loading | First render | Availability panel shows loading status while the first page loads. |
| Availability compatibility fallback | Availability endpoint returns `404 Not Found` | Non-alert note explains this is usually a stale/older backend mismatch and manual exact candle reads remain available. |
| Availability empty | Availability response has no items | Availability empty message; form stays editable; no guessed candle scope. |
| Availability default scope loading | First availability page includes `defaultSelection` | Availability list renders; matching entry selected; normalized candle status shown while the exact default slice loads. |
| Validation error | Required field missing, invalid range, selected range exceeds 10,000 intervals, or selected range leaves the chosen availability window | Inline alert semantics above results; **Load candles** does not call APIs. |
| Backfill validation / create error | Explicit **Start historical backfill** used with invalid scope/page size (negative values only; `0` is valid) or the job API fails | Inline alert in the backfill panel; no job is created implicitly. |
| Backfill created | Job create succeeds | Success copy shows created job id/status plus a link to `#/jobs/:jobId`. |
| Candle empty | Candle response has no items | "No normalized candles matched these filters." |
| Raw payload idle | Candle scope exists, but explicit raw browsing not requested yet | Prompt explains that broad raw payload metadata is optional and secondary. |
| Raw payload loading | User clicked **Load raw payload metadata** | Explicit raw metadata status shown; button disabled while in flight. |
| Raw payload empty | Explicit raw payload response has no items | "No raw payload metadata matched these filters." |
| API error | Any data API request fails | Error copy uses alert semantics; unrelated successful data stays visible. |
| Linked evidence idle | No candle selected yet | Prompt tells the user to select a normalized candle row. |
| Linked evidence loading | Candle selected, linked evidence request in flight | Loading status in linked evidence panel. |
| Linked evidence empty | Candle selected, no linked payloads | Empty evidence message (not an error). |
| Detail dialog | Raw payload row selected | Large dialog-like panel with metadata, hashes, body ref, instrument/timeframe/range hints, body byte count, truncation flag, bounded preview body, copy actions, and explicit preview/full-body guidance. |

**A11y**

- Validation and API failures use `role="alert"`.
- Raw payload detail panel uses dialog semantics and a visible close button.
- Tables keep explicit headers for candle rows, raw payload metadata, and linked evidence.

---

## Jobs (`/jobs`, `/jobs/:jobId`)

**List (`/jobs`)**

- Header: **Jobs** heading + refresh action.
- Filters row: **status**, **job type**, **source**, then **Apply filters**.
- Body: stacked summary cards, not a dense table.
- Each card shows:
  - job id
  - status + job type
  - requested scope (**venue / symbol / asset class / timeframe**)
  - requester source/user
  - created time + attempt count
  - compact result or safe error summary when present
  - **Open job detail** route link
- When the API returns `nextCursor`, show a visible **Load more** button and append the next page of cards.

| State | When | UI |
| :--- | :--- | :--- |
| Loading | Initial open or refresh/apply in flight | `Loading jobs…` status. |
| Empty | API returns no items | `No durable jobs matched the current filters.` |
| Error | List request fails | Alert with safe API message. |
| Success | Items returned | Stacked job summary cards with open-detail links. |

**Detail (`/jobs/:jobId`)**

- Header: **Job detail** heading + backlinks to **Jobs** and **Data**.

---

## Finance (`/finance*`)

**Shared shape**

- Finance routes stay visually distinct from trading/data routes and always render inside one shared Bootstrap Finance shell.
- The Bootstrap Finance shell supersedes `restructure-finance-ui-shell` as the styling direction; only its behavior lessons such as route-preserving tenant selection and avoiding dead links carry forward.
- Finance routes use a wider shell canvas than the default reading column so the rail, dashboard grids, ledger tables, and inspectors can breathe.
- Returning to non-finance routes restores the generic authenticated nav and the existing non-finance styling stack.
- Tenant-aware finance routes share one client-side active-tenant workspace choice via local storage.
- If the operator belongs to exactly one finance tenant, tenant-scoped finance routes auto-select it and continue without an extra step.
- If the operator belongs to multiple finance tenants and no active tenant is stored yet, tenant-scoped finance routes stop on the requested route and require one explicit tenant choice there before loading tenant data.
- After selection, the active tenant is reused across `#/finance`, `#/finance/accounts`, `#/finance/accounts/new`, `#/finance/accounts/:accountId`, `#/finance/transactions`, `#/finance/transactions/new`, `#/finance/transactions/:transactionId`, `#/finance/categories`, `#/finance/connections`, `#/finance/imports`, and `#/finance/jobs/:jobId` until changed.
- Tenant-aware routes keep one visible selected-tenant control in shell chrome near the top of the page.
- Multi-tenant tenant selection is shell-owned and compact; dashboard/content routes do not repeat tenant picker panels or tenant-workspace explainer blocks.
- Single-tenant tenant-scoped finance routes do not show a tenant selector in normal shell chrome.
- Finance detail flows prefer separate routes over split panes; the first slice uses `/finance/accounts/:accountId` and `/finance/jobs/:jobId` for that purpose.
- Finance timestamps render in browser-local date or date-time format instead of raw ISO strings. Native date controls create local JavaScript `Date` values and serialize them at the API boundary; dashboard custom date-only controls resolve selected starts to local start-of-day and ends to local end-of-day.
- At narrow widths, the finance rail remains fully visible but stacks above the utility header and route body as a full-width Bootstrap aside; there is no separate menu-toggle state.
- At narrow mobile widths, the utility row keeps only compact route/tenant/auth controls and hides non-essential explainer copy.

**Dashboard (`/finance`)**

- Header: **Finance dashboard** heading + short workspace-oriented copy.
- This is the default authenticated landing when there is no remembered protected route.
- Top area: compact dashboard header plus direct links into accounts and transactions.
- Controls area: date-only reporting-period summary, direct **Previous month**, **Current month**, and **Next month** preset controls, and a compact custom date-range disclosure with an **Apply** action. Tenant control is not repeated here. Month controls call only their matching preset and never derive a range from a response window.
- In `current_month` mode, the visible start/end date controls stay populated with the active month bounds on first load and after **Current month** is clicked.
- Direct month controls and the custom-range action keep the visible date inputs synchronized with the reporting window returned by the dashboard API.
- Body order:
  - first row: booked-balance story plus compact income/expense/pending delta summaries beside the primary cash-flow visual
  - second row: capped top-category section and capped account snapshot section with links into the dedicated browse/detail routes
  - third row: full-width capped recent-transactions list, followed by visually secondary compact needs-attention cards for pending activity, missing FX, sync failures, and import follow-up
- When dashboard data is available, the first useful viewport should read in this order: compact header and period context, primary booked-balance story, compact income/expense/pending summaries, one primary visual summary, then capped activity and attention states.
- When settled income/expense reporting is incomplete, an unmistakable warning appears directly with those totals. It names missing distinct FX pairs, their affected value count, and links to FX diagnostics; the later needs-attention area remains secondary.
- Display-currency balances, settled/pending income and expense, categories, and prior/custom periods are **current FX valuations**. They use the latest successful rate and may change after a refresh; ledger membership and native values do not change.
- A compact collapsed **FX coverage** disclosure appears only when current, stale, or missing rates need context. It lists each provider/pair with market effective and last-successful-refresh times, stale markers, and each missing pair’s affected transaction/account-value counts. A stale rate (the backend threshold) produces a prominent warning and refresh link. Missing coverage names distinct pairs and affected values, never occurrence counts as “rates” or “gaps”; display totals stay partial/unavailable rather than substituting native minor units.
- Account and flow native totals remain corroboration only: they are shown separately by currency and never summed into a display-currency total.
- Current reporting and account snapshots exclude hidden accounts. Recent history retains a hidden account's name and **Hidden account** badge through a separate include-hidden lookup; that history context does not restore it to reporting.
- Responsive behavior preserves the same balance-first order on narrow screens; shell chrome, tenant chrome, and route actions should not push the money summary below the first viewport.
- On narrow mobile widths, the dashboard condenses the introductory padding and period divider, hides the repeated overview/FX-valuation explainers, and shortens route-action labels; the reporting controls and route actions remain available before the balance-first summary.

**Tenants (`/finance/tenants`)**

- Header: **Finance tenants** heading + copy that explains select/create/invite/join flow.
 - Layout: Bootstrap header and compact active-tenant list followed by selected-tenant members and structured invitation sections.
- Active tenants use responsive list rows consistent with the Finance transaction list: name, display currency, joined time, active state, and compact Select/Archive actions. Archive requires an in-row confirmation, then refreshes the active list; archived tenants disappear from the active workspace list.
- The active-tenant rows keep selection on-page and load member/invite details for the chosen tenant.
 - Explicit **Create tenant**, **Join by code**, **Edit selected**, and **Invite member** actions reveal exactly one focused inline panel at a time. Cancelling closes only that panel; opening another action replaces it.
 - The edit panel is prefilled with the selected tenant's current name and display currency and updates it in place without leaving `#/finance/tenants`.
- Both create and edit flows use the same bounded display-currency `<select>` backed by the product-supported tenant currency-code list instead of free-text entry.
- Create defaults to a checked, explicitly labeled **Add starter categories and tags** checkbox and always submits its `seedDefaults` boolean; operators can clear it for an empty catalog.
- Successful tenant edits refresh the visible selected-tenant state in both the route and shared Finance shell chrome.
 - Create, join, edit, and invite failures stay on the tenants route, preserve the active panel and its entered values, and show a recoverable inline error.
 - Members show **Username** as the primary label. When available, the UUID is a secondary technical identifier; when no username can be resolved, the UUID remains the fallback identity with an explicit unavailable label.
 - Invites are grouped into **Pending** and **Accepted** sections with recipient, status, and relevant created/accepted time. Pending codes are hidden initially and can be revealed then copied; accepted invitations never expose their codes.

**Accounts (`/finance/accounts`, `/finance/accounts/new`, `/finance/accounts/:accountId`)**

- Accounts browse: include-hidden toggle, compact responsive account table with name, type, currency, booked/pending native balances, hidden state, and explicit **Open details** links. It has no persistent create form; **Create account** opens `#/finance/accounts/new`.
- Account create: focused Bootstrap form for name, supported currency, and kind with save/cancel actions. It only offers creation for the active tenant.
- Account detail: loads the account directly through get-account, including hidden historical accounts. The displayed account name is the page title at the summary's left; compact kind, currency, provider, and hidden-state labels align opposite it. Its document title is `{account name} · Accounts · Signal Foundry`; a compact icon-only **Edit account name** action sits beside the title; opening it replaces that name area with a labeled name input and Save/Cancel controls. Hide confirmation and restore remain separate summary actions. Hiding is never described as deletion: copy says it removes the account from current dashboard reporting and new transactions while preserving history and provider sync. A Hidden historical source badge stays visible. The full-width summary is followed by a full-width recent-transactions card. Recent activity uses the shared responsive transaction list and fixed 10-row offset pages with reusable button-only pager controls; it resets to page one when the account or active tenant changes. During a pager request, the list and pager region remain mounted; on success the shared pager preserves its viewport top by compensating the exact post-commit delta, then focuses that pager region with `preventScroll: true`. Its absolute `scrollY` may change when a final short page shrinks content above it; failures do neither and preserve the current page. The summary includes a compact native collapsed **Current provider evidence** icon disclosure with an accessible label and mobile-sized touch target; its collapsed container is only as wide as the control, then expands to the summary width when opened. Opening it loads current metadata only; each distinct current provider object has a row and an explicit **Reveal current sanitized details** action. Copy identifies the latest sanitized observation and distinguishes it from raw provider payloads without a history affordance.
- Direct entry to `#/finance/accounts/:accountId` preserves the requested route after tenant resolution instead of bouncing to another finance page.
- If multiple tenants are joined and no active tenant is stored yet, the detail route shows a tenant selector plus an explicit “select active tenant” message before loading account data.

**Transactions (`/finance/transactions`, `/finance/transactions/new`, `/finance/transactions/:transactionId`)**

- Transactions browse route: Bootstrap filter card, tenant/account/status/source/sort filters, route-level action links, and visible summary chips.
- Transaction browse loads an include-hidden account lookup solely for account names and history filters. Hidden account filters and rows are explicitly labeled **Hidden**; hidden accounts are not selectable in the new-transaction editor.
- All transaction-list surfaces (browse ledger, dashboard recent activity, and account-detail recent activity) use one responsive stacked transaction-list component. Each row leads with amount, effective date, account, description, and any exceptional state labels; routine `booked` and `regular` labels are omitted. Matched internal transfers surface as a single transfer badge. Category and tag values follow in compact unboxed fields. The same shape stays readable at narrow widths without a horizontal table scan.
- Each shared row supports independent inline description, category, and tag edits. A labeled icon-only pencil enters one field edit at a time; icon-only save/cancel controls confirm or discard it. Description editing autofocuses, submits on Enter (except during IME composition), and cancels on Escape. Category and tag use equal Bootstrap grid cells (`col-12 col-md-6`): they stack on narrow screens and split evenly on desktop so entering either edit mode does not move the other field horizontally. Active category and tag editors keep their field content beside save/cancel actions in one non-wrapping row at desktop and narrow widths; only the flexible tag-choice area may wrap. On narrow screens, the icon-only edit, save, cancel, and full-detail actions have 44px minimum touch targets while desktop stays compact. Saves stay on the current route, preserve all non-edited transaction values, show pending/inline error states, and replace the visible row from the API response. Category can be cleared and tags are selected only from the tenant catalog. A compact icon-only **Open full transaction details** link uses an edit/detail icon for advanced changes.
- Category and tag catalog loads are independent and field-local: a failed catalog withholds that field's inline editor/save path, shows a recoverable alert with a retry action, and never permits a destructive save from an unavailable catalog. Description editing remains available.
- The shared component resolves assigned category and tag IDs from tenant-local catalogs, displaying `Unknown category` or `Unknown tag` rather than opaque IDs when a catalog item is absent.
- Browse results load fixed 20-row pages with a reusable button-only pager; changing tenant or filters returns to the first page. During a pager request, the ledger and pager region remain mounted; on success the shared pager preserves its viewport top by compensating the exact post-commit delta, then focuses that pager region with `preventScroll: true`. Its absolute `scrollY` may change when a final short page shrinks content above it; failures do neither and preserve the current page.
- The browse route does not render a selected-transaction inspector; row editing opens the dedicated detail route.
- Shared transaction editor: reused for both create and edit routes, with a single-column mobile-friendly form, explicit save/cancel actions, visible transaction state context, and provider-original values when present. It separately loads the tenant tag catalog and provides a labeled native checkbox group immediately after Category; it assigns existing tags only, preselects persisted IDs, and never creates tags inline. New records default to `expense` and the user's local calendar day; edit records retain their persisted effective value. The Kind selector includes persisted `regular`. **Amount** is a major-unit decimal field that exactly converts up to two fractional digits to API `amountMinor`; malformed and over-precision values are rejected before save. Currency is a selector backed by the product-supported currency-code list, not free text. On mobile the fields are ordered account, category, tags, kind, amount, currency, description, effective date, then status and source; transfer groups are not free-text editable. Unmatched detail records expose **Link transfer**, lazily load all-account candidates in a native editable local-date range defaulted to `[local start -2 days, local start +3 days)`, page candidates in fixed 20-row offsets, keep ineligible candidates visible and disabled with the backend-rule reason, and require a single eligible selection plus an explicit confirmation summary before linking. Applying a date range resets candidate paging; recoverable link conflicts retain the workflow with a refresh action. Matched detail records lazily load and summarize the exact partner and require an explicit confirmation before unlinking, then reload. Existing transaction detail routes include a compact native collapsed **Current provider evidence** disclosure with metadata-on-expansion and explicit current sanitized-detail reveal; it clearly says each item is the latest current observation for one provider object, not raw provider payload data. Create sends selected `tagIds`; every update replaces assignments with selected `tagIds`, including `[]` to clear. The browser's local `datetime-local` control uses native JavaScript millisecond precision, rejects skipped spring-forward wall times, and uses the browser's first fall-back occurrence when ambiguous.
- Editor refinement: the shell breadcrumb provides create/edit route context, so the editor has no standalone header-only card. **Details** is first; it has no transaction-context card, implementation prose, or duplicate tag summary. Save error/success feedback sits directly below its action row, clears upon field edits, and successful save focuses its status with `preventScroll: true`. Edit-only **Original synced values**, **Internal transfer**, and provider evidence follow Details in that order. The matched-partner summary includes **Open linked transaction**. Transfer candidate paging keeps the candidate list and pager region mounted during loads; on success the shared pager preserves its viewport top by compensating the exact post-commit delta, then focuses that pager region with `preventScroll: true`. Its absolute `scrollY` may change when a final short page shrinks content above it; failures do neither and preserve the current page. On narrow viewports, each candidate's compact description retains kind, account, effective date, amount, and eligibility state beside the selection control.
- Edit route: loads one tenant-scoped transaction directly, keeps finance navigation context intact, and shows provider-original values when present so synced data stays distinguishable from operator edits.
- Transfer and evidence refinement: candidate lookup returns other-account rows, so normal picker rows do not show a red eligibility-error affordance; client-side validation still disables a stale invalid candidate before confirmation. The collapsed **Current provider evidence** disclosure presents the latest sanitized observation per distinct provider object, never a history timeline.

**Categories / tags (`/finance/categories`)**

- Two independent stacked Bootstrap management sections: categories first, then tags.
- Each section keeps its compact **Add category/tag** action in the heading; its local create form is revealed only on demand and can be cancelled without affecting the other section.
- Each catalog row has an accessible labeled icon-only pencil **Edit** action with local save/cancel controls. Category editing reveals name plus an income/expense type select and submits `{name, kind}`; tag editing exposes only its name input and remains name-only. Category and tag editors remain independent and preserve drafts on failures. Category rows retain kind and **Starter default** context.
- Category and tag management is edit/create only; this route adds no hide or delete controls.

**Connections (`/finance/connections`)**

- Header copy calls out the only supported bank-link choices: monobank token linking, PKO bank login via Enable Banking, and synthetic local setup.
- The route does not render a free-text provider field or ask operators to enter connector names such as `enable-banking`.
- Panels: tenant picker, monobank token form, PKO bank-login start panel, synthetic setup start panel, operator notes, then Bootstrap connection cards with in-card sync/delete actions.
- PKO start sends the browser route `{origin}/#/finance/connections` to the backend, the backend derives `/enable-banking/callback` for the provider redirect and looks up the stored browser handoff by returned `state`, and the browser return is handed back to `{origin}/?code=...&state=...#/finance/connections`; the page clears the consumed query string only after a successful finish, keeps the hash route active, and preserves failed return params for a retry on refresh/re-open.
- Synthetic start also sends `{origin}/#/finance/connections` to the backend, but the returned authorization URL is the fixed local route `#/finance/connections/synthetic?state=...`; the browser stays in-app, pushes that hash route immediately, and the setup screen uses the returned `state` for load/save/finish calls.
- Each connection card shows provider/state plus a stable secondary identifier (provider reference, external id, or created timestamp), along with last sync outcome, schedule visibility, job deep links, and an in-card delete confirmation that repeats the selected row identifier before removing it immediately after a successful delete. Each **Sync now** action owns its compact live job-status widget: it polls queued/running lifecycle state, stops terminally, retains a Finance job link, and never overwrites another connection’s status. A native **Synced accounts** disclosure lazily loads safe account name, currency, last-successful-sync, and account links once per card; each card owns loading/error/retry state, and **Sync now** invalidates that card’s cache.

**Synthetic setup (`/finance/connections/synthetic`)**

- Header: **Synthetic setup** heading, back link to Connections, finance sub-navigation, tenant picker, and visible pending setup `state`.
- The route reads `state` from `#/finance/connections/synthetic?state=...`; opening the route without that query does not guess or create a new state and instead points the operator back to Connections to start the flow.
- Main form shows one or more Bootstrap configured-account rows with labeled **name** and **currency** inputs plus explicit **Add account**, per-row **Remove configured account N**, **Reload pending setup**, **Save configuration**, and **Finish link** actions.
- Saving sends the current configured rows to the synthetic link-state API at `/synthetic-link-states/state/{state}` and re-renders from the API response so stable synthetic account keys keep duplicate rows distinct after save/reload.
- Finish validates at least one complete configured row, saves the latest form values, calls synthetic redirect finish with the same `state`, then returns to `#/finance/connections` where the resulting synthetic connection is visible in the normal connection list.
- Save or finish validation/API failures stay on the synthetic setup route and use inline alert/status messaging instead of dropping the pending state.

**Imports (`/finance/imports`)**

- Transaction workflow stays step-by-step: Step 1 fixed CSV source → always-present Step 2 active workspace → secondary durable-import history.
- The page is transaction-only. Account-only CSV imports remain on their separately routed/API flow and are not selectable here.
- Input supports CSV file selection plus paste/edit. It shows a copyable and downloadable sample with the seven required headers: `Date,Account,Category,Tags,Expense amount,Income amount,Currency`, plus optional `Description`. Supported headers are matched by name in any order; missing or blank descriptions become `n/a`; unsupported extra columns are ignored wherever they occur.
- Transaction CSVs support up to 250,000 data rows (header excluded) and 64 MiB; oversized selected files are rejected before reading.
- Contract help states strict `dd.MM.yy` dates (`00`–`99` means 2000–2099), USD/EUR/PLN/UAH support, quoted multi-tags, and quoted localized amounts such as `"8 300,00"`.
- The active workspace is directly below Step 1 and is always present for pending preview, preview result, confirmation, audit, terminal/error, and idle states; Recent imports remains below it.
- Initial preview sends only `{fileName,csv}`. It returns preview-specific textual account options with name, source-row count, and selected state; these are not account entities. Step 2 renders them as native accessible checkboxes, all checked initially. Changing a checkbox debounces a replacement preview request with `{fileName,csv,selectedAccountNames}`; reparsing/re-uploading is intentional for this occasional workflow.
- Checked account names are the only rows included in diagnostics, duplicate checks, creation summaries, and confirmation. An explicit empty selection remains empty, reports `Transactions to import: 0`, and disables confirmation. Blank or otherwise unassignable account diagnostics remain visible.
- While account selection is dirty or its replacement preview is pending, confirmation is disabled and Step 2 shows `Updating preview…`. Only the latest account-selection response may replace the workspace, and background replacement previews do not scroll or move focus. Clicking **Preview transactions** immediately disables the duplicate action, exposes a polite pending status, and scrolls/focuses the active workspace before rendering headers, rejected and duplicate rows as row/field/reason, and would-create account/category/tag summaries.
- A rejected preview request shows the stable generic message `We could not validate this CSV preview. Check the file and try again.`; it does not display an HTTP error body.
- Preview separately names the matched required headers and any ignored source headers; unsupported columns are never presented as resolved.
- Preview visibly reports the API-provided `Transactions to import: N` count after rejected and duplicate rows are excluded, including `0` when no row can be confirmed.
- Confirmation sends no body. When preview excludes rejected or duplicate rows, copy explicitly says confirmation queues only remaining valid rows.
- Repeated confirmation is idempotent and recovers the existing import/job; terminal audits remove the confirmation action.
- Audit refreshes while status is running/non-terminal, also offers manual refresh, and renders final rejected rows plus durable row outcomes. Terminal audit loads synchronize the matching Recent imports summary; background polling does not move focus.
- A tenant-scoped recent-import list reopens durable confirmed audits after navigation or refresh; previews without a durable job do not appear in this history. Opening an audit immediately marks that item as loading, then scrolls/focuses the active workspace; the loaded item remains visibly selected.

**Finance job detail (`/finance/jobs/:jobId`)**

- Direct entry preserves the requested finance job route after tenant resolution.
- If multiple tenants are joined and no active tenant is stored yet, the route shows a tenant selector and explicit active-tenant prompt before rendering job detail content.
- Once the active tenant is resolved, the page renders Bootstrap summary, input, timeline, result, and safe-error cards with finance-local back links and local date-time formatting.

---

## Admin (`/admin*`)

- Admin routes render a utilitarian admin sub-navigation strip.
- `#/admin` is a compact overview page with links to generic jobs, finance FX diagnostics, and provider diagnostics.
- `#/admin/jobs` and `#/admin/jobs/:jobId` reuse the generic jobs list/detail flow with admin copy.
- `#/admin/finance/fx` shows current-rate counts, provider readiness, and a required-rate refresh form with an optional provider selector only. The refresh job dynamically discovers active-tenant account and transaction currency pairs at execution time; it has no base/quote or historical date-range fields. A compact live job-status widget polls queued/running refreshes and keeps an admin job-detail link in place.
- `#/admin/finance/providers` shows sanitized provider readiness plus recent finance-job summaries only; no secrets or raw payloads are shown.
- Add **Open data scope** route link to `#/data` with `venue`, `symbol`, `assetClass`, `timeframe`, `start`, and `end` query params for the job input scope.
- Sections, in order:
  - summary (**status**, **job type**, **requester**, **worker**, **attempt count**)
  - input (**ingestionRunId**, venue/symbol/asset class/timeframe, page size, start/end)
  - timeline/worker timestamps (**created**, **updated**, **started**, **completed**, **last attempt**)
  - optional result (**persisted/expected/missing/duplicate counts**, raw payload count, missing-preview cap, missing interval preview list)
  - optional safe error (**summary**, **code**, **details**)

| State | When | UI |
| :--- | :--- | :--- |
| Missing id | Route param absent | Inline alert: `Job id is required.` |
| Loading | Detail fetch in flight | `Loading job detail…` |
| Error | Detail fetch fails | Alert with safe API message. |
| Queued/running | Non-terminal job | Summary/input/timeline visible; result/error absent until present; detail auto-refreshes until terminal. |
| Succeeded | Terminal success | Result section visible, including missing interval preview when provided. |
| Failed | Terminal failure | Error section visible with safe summary/code/details. |

---

## Providers (`/providers`)

**Layout**

- Header ("Providers" heading + "Add Provider" button) → provider list/table → add/edit form (inline, on demand) → delete confirmation dialog.

**Provider list (table)**

- On mount: `listProviders()` loads data.
- Columns: display name (falls back to `name`; technical `name` below display name when set), `type`, `baseUrl`, `apiKeyPreview`, model count (`N models`), Edit and Delete.

| State | When | UI |
| :--- | :--- | :--- |
| Loading | API call in progress | Loading spinner/indicator; list hidden or dimmed. |
| Empty | No providers configured | "No providers configured yet. Add your first provider to get started." message. |
| List | Providers loaded | Table with one row per provider; Edit/Delete buttons per row. |
| Add form | "Add Provider" button clicked | Inline form with fields: `name` (text, required, create-only), `type` (select, create-only, only `openai-compatible`), `displayName` (text, optional), `baseUrl` (url, required), `apiKey` (password, required on create), `models` (dynamic list, see Models in forms below). Save + Cancel buttons. |
| Edit form | "Edit" button clicked on a row | Same form pre-populated; `name` and `type` are read-only. `apiKey` is optional (omit to keep current). `models` pre-populated with existing models. |
| Delete confirm | "Delete" button clicked | Confirmation dialog/prompt before deletion. On confirm, calls `deleteProvider`, refreshes list. |
| Error | API call fails | Alert shown with error message. |

**Models in forms**

- Create and edit: "Models" section with a list of entries.
- Each entry: `name` (required, placeholder `e.g. gpt-4.1`), `displayName` (optional, placeholder `e.g. GPT 4.1`), "✕ Remove model".
- "Add Model" appends a new empty entry.
- Blank `name` entries ignored on submit.
- On update, the models list **replaces** the provider’s existing models.

**API**

- Create → `createProvider` (includes `models` array).
- Edit → `updateProvider` (includes `models` array, replaces existing).
- Delete → `deleteProvider`.
- After each mutating operation: `listProviders()` to refresh the list.
- API key never shown in full — only `apiKeyPreview` (`...XXXX`).

---

## Strategies (`/strategies`, `/strategies/:strategyId/:version`)

**Layout**

- Header: **Strategies** heading + short v0 scope copy + **New draft** button.
- Strategy list panel first.
- **Strategy editor** sits below the list on the workspace route.
- Route-selected immutable detail opens on its own screen instead of sharing the draft workspace.

**Strategy list**

- On mount: `listStrategies()`.
- Columns: display name, `strategyId/version`, status, source label, artifact hash, instrument triple, timeframe, created timestamp.
- Dense strategy list rows may show compact artifact hashes in-table; full values remain available in detail/hover affordances.
- The list MUST NOT show or depend on latest evaluation summary data in v0.
- Demo rows show explicit example-only copy; evaluation still depends on matching local historical data.

**Editor / draft behavior**

- Supported fields only: fixed kind `moving-average-crossover`, venue, symbol, asset class, active flag, timeframe, fast window, slow window, display name, notes, strategy id, version.
- **New draft** resets local editor state and routes to `/strategies`.
- Draft state is local-only until **Validate** or **Save version**.
- Validation calls `POST /api/v1/strategies/validate` and shows either:
  - field errors (`path`, `message`) with alert semantics, or
  - canonical preview (`schemaVersion`, canonical JSON, artifact hash, existing artifact flag, canonical parameter summary).
- Saving calls `POST /api/v1/strategies/versions` and creates a new immutable `ready` record; successful save routes to the saved detail URL.

**Saved version detail**

- Route-selected saved version loads via `GET /api/v1/strategies/:strategyId/versions/:version`.
- Detail shows immutable metadata: status, source label, artifact hash, timeframe, and full constrained definition payload.
- Copy must state that saved versions are immutable.
- **Duplicate to draft** calls `POST /api/v1/strategies/:strategyId/versions/:version/duplicate`, hydrates the editor with a draft candidate, and returns the user to `/strategies`.
- **Run evaluation** link is visible only when the saved version status is `ready`; archived versions stay visible but non-runnable.

| State | When | UI |
| :--- | :--- | :--- |
| Strategy list loading | Initial `listStrategies()` | Loading status in the list panel. |
| Strategy list empty | No rows returned | Empty workspace copy. |
| Validation success | Backend validation returns `valid=true` | Canonical preview panel with hash + canonical JSON. |
| Validation error | Backend validation returns `valid=false` | Inline field error list with alert semantics. |
| Saved detail loading | Route includes `strategyId/version` | Detail panel loading state. |
| Immutable saved detail | Saved version loaded | Dedicated detail screen with immutable note + duplicate action + artifact hash visible. |
| Runnable saved version | Selected saved version status is `ready` | **Run evaluation** link visible. |
| Non-runnable saved version | Selected saved version status is `archived` | No **Run evaluation** action. |

**A11y**

- Validation and load/save failures use `role="alert"`.
- List/detail tables keep explicit headers.
- Editor inputs remain labeled; fixed kind is rendered as a disabled text field, not unlabeled copy.

---

## Evaluations (`/evaluations`, `/evaluations/run/:strategyId/:version`, `/evaluations/:runId`)

**History + run page**

- Header: **Evaluations** heading + deterministic backtest copy.
- Run form first, then history filters + history table.
- History results render as stacked run summaries rather than a wide audit table.
- Run form loads ready strategy versions from `listStrategies()` and offers a single select of `displayName — strategyId/version`.
- `/evaluations/run/:strategyId/:version` preselects the run form when that ready version is available, and later route-param changes update the selected ready version instead of leaving stale form state behind.
- Run form fields: selected strategy version, shared client-local range picker, quantity, optional note.
- Evaluation range controls include deterministic Last 24h / 7d / 30d / 90d / 180d presets that resolve once to explicit instants and remain stable until changed again.
- Client validation prevents requests for missing selection, invalid range values, `start >= end`, or non-positive quantity.
- Successful create keeps the operator on the history page, shows the returned synchronous status/result (`completed` or `failed`), refreshes history, and exposes an **Open evaluation detail** link.

**History summaries**

- Loads from `GET /api/v1/evaluations/backtests` on mount.
- Filters: strategy id and status at minimum; **Apply filters** reruns the list call.
- Each run summary surfaces run id, strategy id/version, artifact hash, instrument triple, timeframe, full tested range, status, decision, trade counts, blocked/rejected counts, and client-local created/updated lifecycle timestamps.
- Dense history rows may compact run ids and hashes for readability while preserving the full value in navigation/detail affordances.

**Detail page**

- `GET /api/v1/evaluations/backtests/:runId`, `/report`, and `/evidence` all load on open.
- Summary shows run id, status, decision, strategy id/version, artifact hash, dataset id/checksum when present, policy reference/hash, compact metrics, tested range, and request note.
- Summary-level technical identifiers may be compacted for readability; the full identifier remains available via native hover text.
- Evidence remains table-first in v0: traces, order intents, governor decisions, and execution records are primary. Empty evidence tables show non-error empty copy.
- No chart overlays, live trading controls, or AI runtime actions are introduced here.

| State | When | UI |
| :--- | :--- | :--- |
| Ready versions loading | `listStrategies()` in flight on run page | Strategy select disabled with loading state. |
| Run validation error | Missing selection, bad range, or invalid quantity | Inline alert list; create request not sent. |
| Run in flight | `createEvaluationBacktest()` in flight | **Start evaluation** button shows loading state. |
| Run created | Create call returns a run detail | Inline result panel with run id, status, and **Open evaluation detail** link. |
| History empty | No rows matched current filters | Non-error empty state. |
| Detail loading | Detail/report/evidence requests in flight | Loading status in detail route. |
| Detail failure | Any detail/report/evidence request fails | Alert copy; summary/evidence hidden. |
| Evidence empty | Any evidence table has zero rows | Non-error empty table copy per section. |

**A11y**

- Run validation and API failures use `role="alert"`.
- History and evidence tables keep explicit headers.
- Result and loading copy use status semantics (`role="status"` where appropriate).
