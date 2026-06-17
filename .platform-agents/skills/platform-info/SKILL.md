---
name: platform-info
description: Use when explaining how a venue or platform works, investigating vendor-side limits, or separating a Signal Foundry bug from exchange or API behavior.
---
# Platform info

Use this skill when:
- a user asks how a venue or platform works
- a run/result suggests a vendor limitation, retention boundary, or unsupported workflow
- you need to decide whether an observed issue is in Signal Foundry or in the upstream platform

## Workflow

1. Start with local product context before making claims about the platform.
2. Read the relevant local venue reference and implementation files for the scope you are investigating.
3. For facts that may change, verify with the platform's primary docs and, when safe, a live read-only probe.
4. Report the result as one of:
   - confirmed Signal Foundry bug
   - confirmed vendor/platform limitation
   - still uncertain, with the next bounded verification step

## Hyperliquid first-read set

When the scope is Hyperliquid, read these first:
- `../../../docs/hyperliquid.md`
- `../../../runtime/venueedge/hyperliquid_perps.go`

Then verify unstable claims against official Hyperliquid docs, especially for:
- market-data retention windows
- supported endpoints and intervals
- websocket versus REST behavior
- rate limits
- account model, API wallets, and signing rules

## Historical market-data checks

Before calling missing historical venue data a bug:
- state the exact UTC start and end dates
- state the exact venue, symbol, and timeframe
- check retention and pagination limits in the vendor docs
- verify whether the same range is available at a coarser timeframe
- prefer a live read-only probe when the endpoint is public and safe

## Reporting rules

- Be explicit about what was verified locally versus what was verified from vendor docs.
- Use absolute dates, not only relative phrases like "today" or "recently".
- If the limitation is vendor-side, include at least one practical workaround when possible.
- Do not describe impossible backfills or unsupported platform behavior as if they should work.

## Safety boundaries

- Do not invent vendor capabilities, retained history, or supported workflows.
- Do not claim a vendor outage without direct evidence.
- Do not rely on stale memory for platform limits that may have changed.
