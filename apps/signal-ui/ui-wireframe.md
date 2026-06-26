# Signal UI — wireframe (composition, states, behavior)

> **Editing this doc:** Prefer **bullet lists** (and **tables** for state matrices) over long unbroken prose—keep the same detail, but make it scannable and easy to diff.

**Stack**

- Vite + Svelte 5 SPA, `svelte-spa-router`.

**Shell (`Nav` + `<main>`)**

- **Nav — left:** Brand label **Signal Foundry** → `/chat`.
- **Nav — center:** **Chat** / **Data** / **Jobs** / **Finance** / **Providers** / **Strategies** / **Evaluations** / **Admin** share the same centered max-width column as `main-inner` on non-chat routes; the nav’s first grid column is content-sized so the brand does not overlap those links on narrow viewports.
- **Nav — right:** **Sign out** (text) **to the left of** the compact **Theme** segmented control; both **end-aligned** in the right margin; only when authenticated.
- **≤700px Nav:** brand + auth controls stay on the first row; route links wrap onto a dedicated second row instead of colliding with the brand or sign-out cluster.
- **`<main>`:** Centered content wrapper `main-inner` (~800–900px max width) with `Router` inside.
- **`/chat`:** `main-inner` is full width so the session rail can sit at the viewport’s left inset; shell is viewport-height with **no page scrollbar** — only the transcript strip scrolls.
- **Sign out:** `authStore.clearAuth()` (removes refresh token from `localStorage`, clears in-memory session) and `replace('/login')` so the user lands on the public login route without stacking authenticated history entries.
- **Unauthenticated routes:** No `Nav`; stored theme still applies to the global shell on load.

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

**Route guarding**

- `/chat`, `/data`, `/jobs*`, `/providers`, `/strategies*`, and `/evaluations*`: protected with `svelte-spa-router`’s `wrap` + `conditions`.
- If `authStore.isAuthenticated` is false: `conditionsFailed` stores the requested protected route and then `replace('/login')`.
- `/login` is public.
- Nav is hidden when unauthenticated.

---

## Routes

| Path | Behavior |
| :--- | :--- |
| `/` | `replace('/data')` on mount; brief status text. |
| `/login` | Login page; username + password form. On success, sets auth tokens and `push()`es the remembered protected destination or `/data`. On failure, shows inline error alert. |
| `/chat/:sessionId?` | Chat; optional id in URL after `sessionBound` (`replace`). One route entry so binding the id does not remount the page or abort the stream. |
| `/data` | Historical data browser. Protected (auth required). Browse-first availability loads on open, then exact candle reads stay editable. |
| `/jobs` | Durable historical ingestion job list. Protected. Summary-first stacked cards with filters, refresh, and open-detail actions. |
| `/jobs/:jobId` | Durable historical ingestion job detail. Protected. Separate route with request, timeline, worker, result, and safe error sections plus back links to Jobs and Data. |
| `/providers` | Provider configuration management page. Protected (auth required). |
| `/finance` | Finance dashboard. Protected. Tenant-aware KPI, alert, missing-FX, and account/category summary route. |
| `/finance/tenants` | Finance tenant selection/create/invite/join/member route. Protected. |
| `/finance/accounts` | Finance account list and create route. Protected. |
| `/finance/accounts/:accountId` | Finance account detail route. Protected. Separate detail route with recent transaction context. |
| `/finance/connections` | Finance connection list/link/sync route. Protected. Schedule and last/next sync visibility stay here, and operators can locally delete a link without removing imported ledger history. |
| `/finance/transactions` | Finance transactions browse/filter route. Protected. Card-first state cues plus explicit create/edit entry points. |
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
- Filter form remains available below availability, with fields for **venue**, **symbol**, **asset class**, **timeframe**, a shared UTC-aware range picker, and optional **ingestion run ID**.
- The shared UTC range picker shows separate UTC **date** and **time** controls, an inline calendar, visible resolved UTC ISO `start` / `end` values, and deterministic quick presets whenever the selected availability/timeframe exposes an anchor.
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
  - invalid or incomplete UTC range values before a valid ISO range is resolved
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
| Validation error | Required field missing, invalid UTC range, selected range exceeds 10,000 intervals, or selected range leaves the chosen availability window | Inline alert semantics above results; **Load candles** does not call APIs. |
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

- Finance routes stay visually distinct from trading/data routes and always render a finance sub-navigation strip first.
- Tenant-aware finance routes share one client-side active-tenant workspace choice via local storage.
- If the operator belongs to exactly one finance tenant, tenant-scoped finance routes auto-select it and continue without an extra step.
- If the operator belongs to multiple finance tenants and no active tenant is stored yet, tenant-scoped finance routes stop on the requested route and require one explicit tenant choice there before loading tenant data.
- After selection, the active tenant is reused across `#/finance`, `#/finance/accounts`, `#/finance/accounts/:accountId`, `#/finance/transactions`, `#/finance/transactions/new`, `#/finance/transactions/:transactionId`, `#/finance/categories`, `#/finance/connections`, `#/finance/imports`, and `#/finance/jobs/:jobId` until changed.
- Tenant-aware routes keep a visible selected-tenant control near the top of the page.
- Finance detail flows prefer separate routes over split panes; the first slice uses `/finance/accounts/:accountId` and `/finance/jobs/:jobId` for that purpose.
- Finance user-facing dates render in browser-local date or date-time format instead of raw ISO strings.

**Dashboard (`/finance`)**

- Header: **Finance** heading + short tenant-workspace copy.
- Top controls: tenant picker, previous/current/next period controls, and a custom date-range form.
- In `current_month` mode, the visible start/end date controls stay populated with the active month bounds on first load and after **Current month** is clicked.
- Previous period, next period, and custom-range actions keep the visible date inputs synchronized with the reporting window returned by the dashboard API.
- Body order:
  - KPI cards for settled net, pending net, and alert count
  - alerts + missing-FX stack with a deep link to `#/admin/finance/fx`
  - account balance summaries with a link to `#/finance/accounts`
  - category breakdown summaries with a link to `#/finance/transactions`

**Tenants (`/finance/tenants`)**

- Header: **Finance tenants** heading + copy that explains select/create/invite/join flow.
- Panels: selected-tenant picker, create-tenant form, accept-invite form, members list, invites list, create-invite form.

**Accounts (`/finance/accounts`, `/finance/accounts/:accountId`)**

- Accounts list: tenant picker, include-hidden toggle, create-account form, stacked account summary cards, and explicit **Open account detail** links.
- Account detail: one focused account summary panel plus a recent-transactions stack and backlinks to Accounts and Transactions.
- Direct entry to `#/finance/accounts/:accountId` preserves the requested route after tenant resolution instead of bouncing to another finance page.
- If multiple tenants are joined and no active tenant is stored yet, the detail route shows a tenant selector plus an explicit “select active tenant” message before loading account data.

**Transactions (`/finance/transactions`, `/finance/transactions/new`, `/finance/transactions/:transactionId`)**

- Transactions browse route: tenant/account/status/source/sort filters plus a clear **Create transaction** action.
- Browse results: stacked cards with explicit state badges for pending, hidden, transfer, refund, and reconciliation signals plus direct **Open transaction** links.
- Shared transaction editor: reused for both create and edit routes, with a single-column mobile-friendly layout, explicit save/cancel actions, and visible transaction state context.
- Edit route: loads one tenant-scoped transaction directly, keeps finance navigation context intact, and shows provider-original values when present so synced data stays distinguishable from operator edits.

**Categories / tags (`/finance/categories`)**

- Two stacked management panels: one for categories, one for tags.
- Each panel includes a create form and a simple stacked list of existing tenant-local items.

**Connections (`/finance/connections`)**

- Header copy calls out the only supported bank-link choices: monobank token linking and PKO bank login via Enable Banking.
- The route does not render a free-text provider field or ask operators to enter connector names such as `enable-banking`.
- Panels: tenant picker, monobank token form, PKO bank-login start panel, operator notes, then stacked connection cards.
- PKO start sends the browser route `{origin}/#/finance/connections` to the backend, the backend derives `/enable-banking/callback` for the provider redirect and looks up the stored browser handoff by returned `state`, and the browser return is handed back to `{origin}/?code=...&state=...#/finance/connections`; the page clears the consumed query string only after a successful finish, keeps the hash route active, and preserves failed return params for a retry on refresh/re-open.
- Each connection card shows provider/state plus a stable secondary identifier (provider reference, external id, or created timestamp), along with last sync outcome, schedule visibility, job deep links, and an in-card delete confirmation that repeats the selected row identifier before removing it immediately after a successful delete.

**Imports (`/finance/imports`)**

- Workflow stays step-by-step: preview form → preview/mapping panel → import audit panel.
- Preview panel shows resolved headers, editable mapping fields, would-create lists, and confirm action.
- Audit panel exposes import status and a deep link to `#/finance/jobs/:jobId`.

**Finance job detail (`/finance/jobs/:jobId`)**

- Direct entry preserves the requested finance job route after tenant resolution.
- If multiple tenants are joined and no active tenant is stored yet, the route shows a tenant selector and explicit active-tenant prompt before rendering job detail content.
- Once the active tenant is resolved, the page renders the shared job-detail content with finance-local back links and local date-time formatting.

---

## Admin (`/admin*`)

- Admin routes render a utilitarian admin sub-navigation strip.
- `#/admin` is a compact overview page with links to generic jobs, finance FX diagnostics, and provider diagnostics.
- `#/admin/jobs` and `#/admin/jobs/:jobId` reuse the generic jobs list/detail flow with admin copy.
- `#/admin/finance/fx` shows stored-rate counts, provider readiness, and a manual FX sync form that deep-links into admin job detail.
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
- Run form fields: selected strategy version, shared UTC-aware range picker, quantity, optional note.
- Evaluation range controls include deterministic Last 24h / 7d / 30d / 90d / 180d presets that resolve once to explicit UTC values and remain stable until changed again.
- Client validation prevents requests for missing selection, invalid UTC range values, `start >= end`, or non-positive quantity.
- Successful create keeps the operator on the history page, shows the returned synchronous status/result (`completed` or `failed`), refreshes history, and exposes an **Open evaluation detail** link.

**History summaries**

- Loads from `GET /api/v1/evaluations/backtests` on mount.
- Filters: strategy id and status at minimum; **Apply filters** reruns the list call.
- Each run summary surfaces run id, strategy id/version, artifact hash, instrument triple, timeframe, full tested range, status, decision, trade counts, blocked/rejected counts, and UTC created/updated lifecycle timestamps.
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
| Run validation error | Missing selection, bad UTC range, or invalid quantity | Inline alert list; create request not sent. |
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
