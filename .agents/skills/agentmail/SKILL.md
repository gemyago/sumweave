---
name: agentmail
description: Send and receive email via AgentMail CLI. Use when testing email flows, reading inbox messages, sending mail, or when the user mentions agentmail, .agentmail, or AGENTMAIL_API_KEY.
---

# AgentMail

Use the `agentmail` CLI (npm package `agentmail-cli`) to send and receive email through AgentMail inboxes configured in this repo.

## Before you start

1. **API key** — `AGENTMAIL_API_KEY` must be set (typically in `.envrc.local`; see `.envrc.local.template`). Without it, every CLI call fails.
2. **`.agentmail` file** — repo-root file mapping roles to inbox addresses.

If `.agentmail` is missing, **stop and ask the user to create it** before sending or receiving mail.

Format: one `<role>:<email-address>` per line. Example:

```text
primary-inbox:<email-address>
secondary-inbox:<email-address>
```

Read `.agentmail` at the start of every email task. Resolve the email address for each role you need; use that address as `--inbox-id` for all CLI commands (not the role name).

`.agentmail` is gitignored (local-only).

## CLI basics

- Binary: `agentmail` (from `node_modules/.bin/agentmail`, or `npx agentmail`).
- `--inbox-id` is always the full mailbox email from `.agentmail`, not the role.
- Output is JSON by default. Use `--format json`, `--transform '<gjson path>'`, and `--raw-output` to extract fields in scripts.
- `message_id` values contain angle brackets — always quote them in the shell.

```bash
agentmail inboxes:messages list --help
agentmail inboxes:messages send --help
```

## Send email

**Direct send** (most common):

```bash
agentmail inboxes:messages send \
  --inbox-id <sender-email> \
  --to <recipient-email> \
  --subject "Subject line" \
  --text "Plain text body"
```

`<sender-email>` and `<recipient-email>` come from `.agentmail` (or an external address for `--to`).

Optional flags: `--html`, `--cc`, `--bcc`, `--reply-to`, `--attachment`, `--headers`.

Returns `message_id` and `thread_id`.

**Draft then send** (for scheduled send or review before sending):

```bash
agentmail inboxes:drafts create \
  --inbox-id <sender-email> \
  --to <recipient-email> \
  --subject "Subject" \
  --text "Body"

agentmail inboxes:drafts send \
  --inbox-id <sender-email> \
  --draft-id "<draft-id-from-create>"
```

**Reply** to an existing message:

```bash
agentmail inboxes:messages reply \
  --inbox-id <inbox-email> \
  --message-id "<message-id>" \
  --text "Reply body"
```

Add `--reply-all` to include all original recipients.

## Receive email

Emails are stored in AgentMail cloud inboxes (API), not on the local filesystem.

**List recent messages** (metadata + preview):

```bash
agentmail inboxes:messages list \
  --inbox-id <inbox-email> \
  --limit 10
```

Each message includes: `message_id`, `thread_id`, `from`, `to`, `subject`, `preview`, `labels`, `timestamp`.

**Read full body**:

```bash
agentmail inboxes:messages get \
  --inbox-id <inbox-email> \
  --message-id "<message-id>"
```

Response includes `text`, `html`, `extracted_text`, and `extracted_html`.

**Raw MIME** (when needed):

```bash
agentmail inboxes:messages get-raw \
  --inbox-id <inbox-email> \
  --message-id "<message-id>"
```

**Attachments**:

```bash
agentmail inboxes:messages get-attachment \
  --inbox-id <inbox-email> \
  --message-id "<message-id>" \
  --attachment-id "<attachment-id>"
```

**Threads** (conversation view):

```bash
agentmail inboxes:threads list --inbox-id <inbox-email> --limit 10
```

### List filters

- `--label` — filter by label (e.g. `unread`)
- `--after` / `--before` — time bounds
- `--include-spam`, `--include-trash`, `--include-blocked` — include filtered mail
- `--page-token` — pagination from a previous list response

### Waiting for inbound mail

Delivery is not instant. After sending (or triggering an external email), poll `list` with a short sleep (1–3 s) and retry a few times before reporting failure.

Extract a field for scripting:

```bash
agentmail inboxes:messages list \
  --inbox-id <inbox-email> \
  --limit 1 \
  --transform 'messages.0.subject' \
  --raw-output
```

## Where to find emails

- **Configured mailbox addresses** — repo-root `.agentmail` (`<role>:<email-address>` per line)
- **API credentials** — `AGENTMAIL_API_KEY` in `.envrc.local`
- **Message storage** — AgentMail API (cloud inbox), accessed via CLI
- **Message list** (subjects, previews) — `inboxes:messages list`
- **Full body** (text/html) — `inboxes:messages get`
- **Conversations** — `inboxes:threads list`

There is no local maildir or on-disk mailbox. Always use the CLI against the inbox email resolved from `.agentmail`.

## Common workflows

### E2E: send from one role, verify in another

1. Read `.agentmail` and resolve sender/receiver emails by role (e.g. `primary-inbox`, `secondary-inbox`).
2. Send from the sender role with a unique subject (e.g. include a timestamp).
3. Poll `list` on the receiver inbox until the subject appears (sleep 1–3 s between attempts).
4. Copy `message_id` from the matching list entry, then `get` on the receiver inbox to read the full body.

Sent `--text` may not match `get` output byte-for-byte; AgentMail appends a `Sent via AgentMail` footer to `text` / `extracted_text`.

### Read latest unread mail

```bash
agentmail inboxes:messages list \
  --inbox-id <inbox-email> \
  --label unread \
  --limit 5
```

### Find a message by subject

List messages, then `get` the matching `message_id`. Use `--transform 'messages.#.subject'` to print subjects only.

## Troubleshooting

- **Auth errors** — confirm `AGENTMAIL_API_KEY` is set.
- **Unknown inbox** — verify the email in `.agentmail` matches the `--inbox-id` exactly.
- **Empty list after send** — wait and retry; check the correct receiving inbox.
- **Shell errors on message_id** — quote IDs that contain angle brackets.
