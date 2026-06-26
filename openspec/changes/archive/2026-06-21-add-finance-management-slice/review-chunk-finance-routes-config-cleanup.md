# Chunk Review — finance-routes-config-cleanup

## Round 1

- Trigger: follow-up cleanup set review for chunk 9.1
- Verdict: not safe to continue past chunk 9.1 yet
- Scope fit: yes, changes stay within generated app route artifacts plus finance fixture runtime config wiring
- What is correct:
  - finance fixture CLI wiring now resolves runtime settings from DI/config values instead of reading env vars directly in `finance_cmd.go`
  - targeted route generation does come from `v1routes` output; `go generate ./internal/api/http` rewrites the current finance/jobs handler/model artifacts
- Blocking issue:
  - generated validators are not build-clean for map fields, so the regenerated route set does not compile (`internal/api/http/v1routes/internal/finance_csv_import_confirm_request_validation.go`, `internal/api/http/v1routes/internal/finance_csv_import_preview_response_validation.go` both instantiate `EnsureNonDefault[map[string]string]`, which violates the `comparable` constraint)
- Verification:
  - `go generate ./internal/api/http` passed
  - `go test ./cmd/signal-foundry ./internal/api/http/... ./internal/config/...` failed on the generated validator build errors above
  - full `go generate ./...` was not a useful whole-module verifier here because an unrelated existing telemetry `mockgen` directive still fails when `mockgen` is unavailable in PATH
- Completion protocol: not satisfied because the changed app scope is not build/test clean
- Commit status: no commit yet is acceptable because the chunk is not review-clean
