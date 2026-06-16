<script lang="ts">
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { formatCompactIdentifier } from '../lib/compact-identifier'
  import {
    createSignalStrategyWorkspaceApiForAuth,
    type EvaluationDetail,
    type EvaluationEvidence,
    type EvaluationReport,
  } from '../lib/strategy-workspace/api'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'

  const workspaceApi = $derived.by(() =>
    createSignalStrategyWorkspaceApiForAuth({ baseUrl: appBaseUrl, authStore }),
  )

  let { params = {} } = $props<{ params?: { runId?: string } }>()

  let loading = $state(true)
  let error = $state<string | null>(null)
  let detail = $state<EvaluationDetail | null>(null)
  let report = $state<EvaluationReport | null>(null)
  let evidence = $state<EvaluationEvidence | null>(null)
  let detailRequestToken = 0

  $effect(() => {
    void loadDetail(params.runId)
  })

  async function loadDetail(runId: string | undefined) {
    const requestToken = ++detailRequestToken
    detail = null
    report = null
    evidence = null

    if (!runId) {
      error = 'Evaluation run id is required.'
      loading = false
      return
    }

    loading = true
    error = null
    try {
      const [detailValue, reportValue, evidenceValue] = await Promise.all([
        workspaceApi.getEvaluationBacktest({ runId }),
        workspaceApi.getEvaluationBacktestReport({ runId }),
        workspaceApi.getEvaluationBacktestEvidence({ runId }),
      ])
      if (requestToken !== detailRequestToken) {
        return
      }
      detail = detailValue
      report = reportValue
      evidence = evidenceValue
    } catch (loadError) {
      if (requestToken !== detailRequestToken) {
        return
      }
      error = loadError instanceof Error ? loadError.message : 'Failed to load evaluation detail'
      detail = null
      report = null
      evidence = null
    } finally {
      if (requestToken === detailRequestToken) {
        loading = false
      }
    }
  }

  function formatDate(value: Date | undefined): string {
    return value ? value.toISOString().replace('T', ' ').slice(0, 16) + 'Z' : '—'
  }
</script>

<section class="page" aria-labelledby="evaluation-detail-heading">
  <header class="page-header">
    <div>
      <h1 id="evaluation-detail-heading">Evaluation detail</h1>
      <p class="muted">Compact report summary and table-first evidence for a persisted deterministic run.</p>
    </div>
  </header>

  {#if error}
    <p class="error" role="alert">{error}</p>
  {:else if loading}
    <p class="muted" role="status">Loading evaluation detail…</p>
  {:else if detail && report && evidence}
    <section class="panel">
      <h2>Summary</h2>
      <dl class="summary-grid">
        <div>
          <dt>Run ID</dt>
          <dd><code title={detail.runId} aria-label={detail.runId}>{formatCompactIdentifier(detail.runId)}</code></dd>
        </div>
        <div><dt>Status</dt><dd>{detail.status}</dd></div>
        <div><dt>Decision</dt><dd>{detail.decision ?? '—'}</dd></div>
        <div><dt>Strategy</dt><dd>{detail.strategyId}/{detail.strategyVersion}</dd></div>
        <div>
          <dt>Artifact hash</dt>
          <dd>
            <code title={detail.strategyArtifactHash} aria-label={detail.strategyArtifactHash}>
              {formatCompactIdentifier(detail.strategyArtifactHash)}
            </code>
          </dd>
        </div>
        <div>
          <dt>Policy hash</dt>
          <dd>
            <code title={report.policyReference.policyHash} aria-label={report.policyReference.policyHash}>
              {formatCompactIdentifier(report.policyReference.policyHash)}
            </code>
          </dd>
        </div>
        <div>
          <dt>Dataset</dt>
          <dd>
            {#if report.datasetReference?.datasetId}
              <code title={report.datasetReference.datasetId} aria-label={report.datasetReference.datasetId}>
                {formatCompactIdentifier(report.datasetReference.datasetId)}
              </code>
            {:else}
              —
            {/if}
          </dd>
        </div>
        <div>
          <dt>Replay checksum</dt>
          <dd>
            {#if report.datasetReference?.replayChecksum}
              <code
                title={report.datasetReference.replayChecksum}
                aria-label={report.datasetReference.replayChecksum}
              >
                {formatCompactIdentifier(report.datasetReference.replayChecksum)}
              </code>
            {:else}
              —
            {/if}
          </dd>
        </div>
        <div><dt>Trade count</dt><dd>{report.metrics?.tradeCount ?? '—'}</dd></div>
        <div><dt>Max drawdown</dt><dd>{report.metrics?.maxDrawdown ?? '—'}</dd></div>
        <div><dt>Blocked</dt><dd>{report.metrics?.blockedGovernorDecisionCount ?? '—'}</dd></div>
        <div><dt>Rejected</dt><dd>{report.metrics?.rejectedGovernorDecisionCount ?? '—'}</dd></div>
      </dl>
      {#if detail.failureReason || detail.failureDetails}
        <p class="error" role="alert">{detail.failureReason}: {detail.failureDetails}</p>
      {/if}
      <p class="muted">Range: {formatDate(detail.testedRangeStart)} → {formatDate(detail.testedRangeEnd)}</p>
      <p class="muted">Request note: {report.aiReadyMetadata.note || '—'}</p>
    </section>

    <section class="panel">
      <h2>Evidence counts</h2>
      <dl class="summary-grid">
        <div><dt>Traces</dt><dd>{evidence.aiReadyMetadata.evidenceCounts.traces}</dd></div>
        <div><dt>Order intents</dt><dd>{evidence.aiReadyMetadata.evidenceCounts.orderIntents}</dd></div>
        <div><dt>Governor decisions</dt><dd>{evidence.aiReadyMetadata.evidenceCounts.governorDecisions}</dd></div>
        <div><dt>Execution records</dt><dd>{evidence.aiReadyMetadata.evidenceCounts.executionRecords}</dd></div>
      </dl>
    </section>

    <section class="panel">
      <h2>Traces</h2>
      {#if evidence.traces.length === 0}
        <p class="muted">No traces were persisted for this run.</p>
      {:else}
        <div class="table-wrap">
          <table>
            <thead><tr><th>Trace ID</th><th>Decision time</th><th>Result</th><th>Reason codes</th></tr></thead>
            <tbody>
              {#each evidence.traces as item (item.traceId)}
                <tr>
                  <td><code>{item.traceId}</code></td>
                  <td>{formatDate(item.decisionTime)}</td>
                  <td>{item.result}</td>
                  <td>{item.reasonCodes.join(', ') || '—'}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>

    <section class="panel">
      <h2>Order intents</h2>
      {#if evidence.orderIntents.length === 0}
        <p class="muted">No order intents were persisted for this run.</p>
      {:else}
        <div class="table-wrap">
          <table>
            <thead><tr><th>Intent ID</th><th>Status</th><th>Action</th><th>Quantity</th></tr></thead>
            <tbody>
              {#each evidence.orderIntents as item (item.intentId)}
                <tr>
                  <td><code>{item.intentId}</code></td>
                  <td>{item.status}</td>
                  <td>{item.actionKind}</td>
                  <td>{item.requestedQuantity}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>

    <section class="panel">
      <h2>Governor decisions</h2>
      {#if evidence.governorDecisions.length === 0}
        <p class="muted">No governor decisions were persisted for this run.</p>
      {:else}
        <div class="table-wrap">
          <table>
            <thead><tr><th>Decision ID</th><th>Status</th><th>Reason</th><th>Reference</th></tr></thead>
            <tbody>
              {#each evidence.governorDecisions as item (item.decisionId)}
                <tr>
                  <td><code>{item.decisionId}</code></td>
                  <td>{item.status}</td>
                  <td>{item.reason}</td>
                  <td>{item.reference}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>

    <section class="panel">
      <h2>Execution records</h2>
      {#if evidence.executionRecords.length === 0}
        <p class="muted">No execution records were persisted for this run.</p>
      {:else}
        <div class="table-wrap">
          <table>
            <thead><tr><th>Command ID</th><th>Order ID</th><th>Fill ID</th><th>Status</th></tr></thead>
            <tbody>
              {#each evidence.executionRecords as item (item.commandId)}
                <tr>
                  <td><code>{item.commandId}</code></td>
                  <td><code>{item.orderId}</code></td>
                  <td><code>{item.fillId}</code></td>
                  <td>{item.status}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>
  {/if}
</section>

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: var(--space-24);
  }

  .page-header {
    display: flex;
    justify-content: space-between;
    gap: var(--space-16);
  }

  .panel {
    border: 1px solid var(--border);
    border-radius: var(--radius-default);
    padding: var(--space-16);
  }

  .summary-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--space-12) var(--space-16);
    margin: 0 0 var(--space-16);
  }

  .summary-grid dt,
  .muted {
    color: var(--text-muted);
  }

  .summary-grid dd {
    margin: 0;
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .summary-grid code {
    display: inline-block;
    max-width: 100%;
    vertical-align: top;
  }

  .error {
    color: var(--color-danger-red);
  }

  .table-wrap {
    overflow-x: auto;
  }

  table {
    width: 100%;
    border-collapse: collapse;
  }

  th,
  td {
    padding: var(--space-12) var(--space-8);
    border-top: 1px solid var(--border);
    text-align: left;
    vertical-align: top;
  }

  @media (max-width: 900px) {
    .page-header {
      flex-direction: column;
    }

    .summary-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
