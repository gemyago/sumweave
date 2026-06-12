---
name: notion-user
description: Notion specific skill. Use when you need to work with Notion, like reading/fetching pages, tasks e.t.c. Also use this skill when you see notion url (e.g app.notion.com/...) and need to work with it.
---

# Notion User

This skill is about using Notion connector tools correctly.

Use it for:
- finding/reading Notion pages and databases
- reading schema before database writes
- updating properties and page content
- creating pages and handling comments
- changing database schema/views
- moving or duplicating pages

## Tools

Commonly available capabilities:
- `search`
- `fetch`
- `update_page`
- `create_pages`
- `get_comments`
- `create_comment`
- `update_data_source`
- `create_view`
- `update_view`
- `move_pages`
- `duplicate_page`
- `get_users`

Use the tool name available in your environment (some clients use a prefix like `notion.*`).

## Core rules

- Prefer full Notion URLs (`https://www.notion.so/...`) for targets.
- Always `fetch` before edits.
- For database writes, fetch both page and its data-source/collection object first.
- Use exact property names and exact option values from schema.
- Use exact `old_str` for `update_content`.
- Prefer `update_content` over `replace_content`.
- If `replace_content` risks deleting child pages/databases, confirm before proceeding.
- Re-fetch and verify after each write.
- Keep requests minimal: lower `page_size`/`max_highlight_length` when possible and only fetch properties/content you need.

### Environment-specific behavior (this runtime)

- Do not rely on a dedicated DB query API as the primary path; it can be non-operational in this environment. Use `fetch` + `search` + `fetch` (for verification).
- Use `fetch` on a page/database URL first. If DB-related, also fetch its `collection://...` (or data-source) object before writes.
- When creating pages in a database, some connector calls validate more reliably with the raw `data_source_id` UUID instead of the `collection://...` form returned by `fetch`. If a create call fails validation, retry with the bare UUID.
- In this connector, `query` text is often used for discovery and status filtering; keep status terms directly in `query` instead of expecting property-level `filters` as status selectors.
- Use a broad discover query with a single space: `query: " "`, this allows you to fetch all pages in the database.
- For `query_type: "internal"` calls, always include `filters: {}` (required by this environment), along with `query_type: "internal"`.
- Keep `page_size` within connector limits;
- Cap `page_size` at environment limits; start at 25 or less (this environment rejects >25).
- Some `create_pages` and `update_properties` calls are stricter than they look. If a create fails validation, try a minimal page body first, then update database properties in a second step. Some `update_properties` calls in this environment need `content_updates: []` and `new_str: ""` to pass validation.
- Notion page content can rewrite markdown links in surprising ways. For local repository filenames or paths, prefer plain text unless you explicitly need a clickable Notion page/database link.
- Avoid `view://...` URLs unless explicitly required by this environment; prefer full Notion URLs.
- Reduce query passes: use one broad discover pass first, then add follow-up queries only if results are incomplete or need disambiguation.
- Never rely on undocumented query APIs or tool paths as the only source of truth; fallback to `fetch` candidates when results look partial.

## Common workflows

### 1) User gives a URL/ID

```json
{"id": "https://www.notion.so/..."}
```

### 2) User describes target, no URL

```json
{
  "query": "beta success metrics",
  "query_type": "internal",
  "page_url": "https://www.notion.so/.../database",
  "page_size": 10,
  "max_highlight_length": 120,
  "filters": {}
}
```

Then `fetch` the chosen result URL.

### 3) Update database properties

```json
{
  "page_id": "page-uuid",
  "command": "update_properties",
  "properties": {
    "Status": "Ready For Work",
    "Priority": "High"
  },
  "content_updates": [],
  "new_str": ""
}
```

### 4) Update page content in place

```json
{
  "page_id": "page-uuid",
  "command": "update_content",
  "properties": {},
  "content_updates": [
    {
      "old_str": "## Description\nOld text here",
      "new_str": "## Description\nNew text here"
    }
  ]
}
```

### 5) Replace full page content (only when intentional)

```json
{
  "page_id": "page-uuid",
  "command": "replace_content",
  "properties": {},
  "content_updates": [],
  "new_str": "# New heading\nNew content"
}
```

### 6) Create page

Under page:

```json
{
  "parent": { "page_id": "parent-page-uuid" },
  "pages": [{
    "properties": { "title": "New page title" },
    "content": "## Section\nBody text"
  }]
}
```

Inside database:

```json
{
  "parent": { "data_source_id": "data-source-uuid" },
  "pages": [
    {
      "properties": {
        "Task name": "Define beta metrics",
        "Status": "Ready For Work",
        "Priority": "High"
      }
    }
  ]
}
```

If the create call is rejected, retry with only the title property first, then call `update_page` / `update_properties` to fill the rest.

### 7) Comment or reply

```json
{
  "page_id": "page-uuid",
  "rich_text": ["Need owner confirmation on the acceptance criteria." ]
}
```

```json
{
  "page_id": "page-uuid",
  "discussion_id": "discussion://page/block/discussion",
  "rich_text": ["Updated the acceptance criteria to reflect this."]
}
```

### 8) Move or duplicate

```json
{
  "page_or_database_ids": ["page-uuid"],
  "new_parent": { "page_id": "new-parent-uuid" }
}
```

```json
{ "page_id": "page-uuid" }
```

## Property formatting

- text/select/status/title: strings
- multi-select: array of strings
- people: array of user IDs
- checkbox: `__YES__` / `__NO__`
- numeric: numbers
- date keys: `date:Due date:start`, `date:Due date:end`, `date:Due date:is_datetime`

For DB listing without query APIs:
1. `fetch` database and its data source.
2. `search` with `page_url` set to the database URL and `query: " "`.
3. If needed, search again with status-like terms for discovery only.
4. `fetch` candidates and verify properties before acting.
5. Deduplicate by page URL/ID and mention partial results if coverage is uncertain.
