const observedDispatchIDs = new Set<string>()

// This is intentionally memory-only: a reload/deep link has no invoking dispatch flow.
export function rememberObservedDispatch(jobId: string) {
  observedDispatchIDs.add(jobId)
}

export function isRememberedObservedDispatch(jobId: string) {
  return observedDispatchIDs.has(jobId)
}
