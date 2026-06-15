# Planning Review

## Round 1

- Scope: `add-historical-data-browser-api-ui`
- Triggering input: OpenSpec planning output
- Findings:
  1. Oversized candle-range behavior is unresolved; the plan needs an explicit server-side max range before implementation.
  2. Candle evidence lookup is ambiguous when provenance is omitted; the plan must define required provenance behavior or matching rules.
- Verdict: replanning required

## Round 2 Replanning Notes

- Defined the server candle browse cap as 10,000 requested intervals, with exact timeframe durations and `400 Bad Request` behavior for oversized requests.
- Required `provenanceSource` and `provenanceIdentity` for candle-linked raw payload lookup; omission now deterministically returns `400 Bad Request` rather than ambiguous matching.

## Round 3

- Scope: `add-historical-data-browser-api-ui`
- Triggering input: updated OpenSpec planning output
- Findings: none
- Verdict: clean/ready
- Chunking: `runtime-data-browser-reads` -> `backend-data-api` -> `ui-data-browser`
