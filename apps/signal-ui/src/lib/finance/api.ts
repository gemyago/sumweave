import type { AuthStore } from '../auth/auth-store.svelte'
import { createAuthFetch } from '../auth/auth-fetch'
import { ResponseTimestampError, parseRequiredResponseTimestamp, serializeRequestTimestamp } from '../timestamp'

type FetchLike = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>

export interface FinanceTenantSummary {
  id: string
  name: string
  displayCurrency: string
  archivedAt?: Date | null
  joinedAt: Date
  createdAt: Date
  updatedAt: Date
}

export interface FinanceTenantMember {
  tenantId: string
  userId: string
  username?: string
  joinedAt: Date
}

export interface FinanceTenantInvite {
  id: string
  tenantId: string
  code: string
  recipient: string
  createdByUserId: string
  acceptedByUserId?: string | null
  createdAt: Date
  acceptedAt?: Date | null
}

export interface FinanceAccount {
  id: string
  tenantId: string
  name: string
  currency: string
  kind: string
  bookedBalanceMinor: number
  pendingBalanceMinor: number
  provider?: string
  providerAccountId?: string
  hiddenAt?: Date | null
  createdAt: Date
  updatedAt: Date
}

export interface FinanceCategory {
  id: string
  tenantId: string
  name: string
  kind: string
  seededDefault: boolean
  hiddenAt?: Date | null
  createdAt: Date
  updatedAt: Date
}

export interface FinanceTag {
  id: string
  tenantId: string
  name: string
  hiddenAt?: Date | null
  createdAt: Date
  updatedAt: Date
}

export interface FinanceTransaction {
  id: string
  tenantId: string
  accountId: string
  source: string
  status: string
  kind: string
  amountMinor: number
  currency: string
  description: string
  effectiveAt: Date
  categoryId?: string | null
  tagIds: string[]
  transferGroupId?: string | null
  transferMatchedAt?: Date | null
  hiddenAt?: Date | null
  providerOriginal?: FinanceTransactionProviderOriginal
  createdAt: Date
  updatedAt: Date
}

export interface FinanceTransactionProviderOriginal {
  amountMinor: number
  currency: string
  description: string
  effectiveAt?: Date | null
}

export interface FinanceTransferCandidatePage {
  items: FinanceTransaction[]
}

export interface FinanceProviderEvidenceMetadata {
  id: string
  scope: string
  providerObjectId: string
  capturedAt: Date
}

export interface FinanceProviderEvidence extends FinanceProviderEvidenceMetadata {
  payload?: Record<string, unknown>
}

export interface FinanceBankConnectionSchedule {
  connectionId: string
  intervalSeconds: number
  nextRunAt?: Date | null
  lastScheduledAt?: Date | null
  lastStartedAt?: Date | null
  lastCompletedAt?: Date | null
  lastJobId?: string
  enabled: boolean
  createdAt: Date
  updatedAt: Date
}

export interface FinanceBankConnection {
  id: string
  tenantId: string
  provider: string
  displayName: string
  providerReference: string
  externalId: string
  state: string
  lastSyncJobId?: string
  lastSyncStartedAt?: Date | null
  lastSuccessfulSyncAt?: Date | null
  lastSyncError?: string
  createdAt: Date
  updatedAt: Date
  schedule?: FinanceBankConnectionSchedule
}

export interface FinanceConnectionSyncedAccount {
  financeAccountId: string
  name: string
  currency: string
  lastSuccessfulSyncAt?: Date | null
}

export interface FinanceDashboardPeriodWindow {
  startDate: Date
  endDate: Date
}

export interface FinanceDashboardPeriod {
  preset: string
  startDate: Date
  endDate: Date
  previous: FinanceDashboardPeriodWindow
  next: FinanceDashboardPeriodWindow
}

export interface FinanceDashboardMoneySummary {
  displayCurrency: string
  incomeMinor: number
  expenseMinor: number
  netMinor: number
  transactionCount: number
  complete: boolean
}

export interface FinanceDashboardCategoryBreakdown {
  categoryId: string
  categoryName: string
  kind: string
  incomeMinor: number
  expenseMinor: number
  transactionCount: number
}

export interface FinanceDashboardAccountBalance {
  accountId: string
  accountName: string
  currency: string
  nativeBookedMinor: number
  nativePendingMinor: number
  displayBookedMinor: number | null
  displayPendingMinor: number | null
  missingFx: boolean
}

export interface FinanceDashboardAlert {
  code: string
  severity: string
  count: number
}

export interface FinanceDashboardFXCoverage {
  provider: string
  baseCurrency: string
  quoteCurrency: string
  affectedTransactionCount: number
  affectedAccountCount: number
}

export interface FinanceDashboardCurrentFxRate {
  provider: string
  baseCurrency: string
  quoteCurrency: string
  effectiveAt: Date
  lastSuccessfulRefreshAt: Date
  stale: boolean
}

export interface FinanceDashboardCurrencyTotal {
  currency: string
  incomeMinor: number
  expenseMinor: number
  netMinor: number
}

export interface FinanceDashboard {
  period: FinanceDashboardPeriod
  settled: FinanceDashboardMoneySummary
  pending: FinanceDashboardMoneySummary
  categoryBreakdowns: FinanceDashboardCategoryBreakdown[]
  accountBalances: FinanceDashboardAccountBalance[]
  alerts: FinanceDashboardAlert[]
  fxCoverage: FinanceDashboardFXCoverage[]
  currentFxRates: FinanceDashboardCurrentFxRate[]
  nativeSettledTotals: FinanceDashboardCurrencyTotal[]
}

export interface FinanceFXProviderDiagnostic {
  name: string
  default: boolean
  ready: boolean
}

export interface FinanceFXDiagnostics {
  defaultProvider: string
  storedRatesCount: number
  providers: FinanceFXProviderDiagnostic[]
}

export interface FinanceCSVRejectedRow {
  rowNumber: number
  field?: string
  reason: string
}

export interface FinanceCSVImportAccountOption {
  name: string
  sourceRowCount: number
  selected: boolean
}

export interface FinanceCSVImportRowOutcome {
  rowNumber: number
  transactionId?: string
  status: string
  reason: string
  createdAt: Date
  updatedAt: Date
}

export interface FinanceCSVImportPreview {
  importId: string
  importableCount: number
  headers: string[]
  duplicateRows: FinanceCSVRejectedRow[]
  rejectedRows: FinanceCSVRejectedRow[]
  wouldCreateAccounts: string[]
  wouldCreateCategories: string[]
  wouldCreateTags: string[]
  accountOptions: FinanceCSVImportAccountOption[]
}

export interface FinanceCSVImportConfirmation {
  importId: string
  jobId: string
  jobType: string
}

export interface FinanceCSVImportAudit {
  importId: string
  tenantId: string
  status: string
  jobId: string
  confirmedByUserId: string
  importedCount: number
  rejectedRows: FinanceCSVRejectedRow[]
  rowOutcomes: FinanceCSVImportRowOutcome[]
  createdAt: Date
  confirmedAt?: Date | null
  completedAt?: Date | null
}

export interface FinanceJobRef {
  jobId: string
  jobType: string
  provider?: string
}

export interface FinanceConnectionRedirectStart {
  provider: string
  authorizationUrl: string
  state: string
}

export interface FinanceSyntheticLinkStateConfiguredAccount {
  key: string
  name: string
  currency: string
}

export interface FinanceSyntheticLinkState {
  provider: string
  state: string
  configuredAccounts: FinanceSyntheticLinkStateConfiguredAccount[]
  canFinish: boolean
}

export interface SignalFinanceApi {
  listTenants(): Promise<FinanceTenantSummary[]>
  createTenant(body: { name: string; displayCurrency: string; seedDefaults: boolean }): Promise<FinanceTenantSummary>
  updateTenant(params: { tenantId: string; name: string; displayCurrency: string }): Promise<void>
  archiveTenant(params: { tenantId: string }): Promise<void>
  getTenant(params: { tenantId: string }): Promise<FinanceTenantSummary>
  listTenantMembers(params: { tenantId: string }): Promise<FinanceTenantMember[]>
  listTenantInvites(params: { tenantId: string }): Promise<FinanceTenantInvite[]>
  createTenantInvite(params: { tenantId: string; recipient: string }): Promise<FinanceTenantInvite>
  acceptTenantInvite(params: { code: string }): Promise<FinanceTenantMember>
  listAccounts(params: { tenantId: string; includeHidden?: boolean }): Promise<FinanceAccount[]>
  createAccount(params: { tenantId: string; name: string; currency: string; kind: string }): Promise<FinanceAccount>
  getAccount(params: { tenantId: string; accountId: string }): Promise<FinanceAccount>
  renameAccount(params: { tenantId: string; accountId: string; name: string }): Promise<void>
  hideAccount(params: { tenantId: string; accountId: string }): Promise<void>
  restoreAccount(params: { tenantId: string; accountId: string }): Promise<void>
  listAccountProviderEvidence(params: { tenantId: string; accountId: string }): Promise<FinanceProviderEvidenceMetadata[]>
  getAccountProviderEvidence(params: { tenantId: string; accountId: string; evidenceId: string }): Promise<FinanceProviderEvidence>
  listCategories(params: { tenantId: string; includeHidden?: boolean }): Promise<FinanceCategory[]>
  createCategory(params: { tenantId: string; name: string; kind: string }): Promise<FinanceCategory>
  updateCategory(params: { tenantId: string; categoryId: string; name: string; kind: string }): Promise<void>
  listTags(params: { tenantId: string; includeHidden?: boolean }): Promise<FinanceTag[]>
  createTag(params: { tenantId: string; name: string }): Promise<FinanceTag>
  renameTag(params: { tenantId: string; tagId: string; name: string }): Promise<void>
  listTransactions(params: {
    tenantId: string
    limit: number
    accountId?: string
    source?: string
    status?: string
    includeHidden?: boolean
    offset?: number
  }): Promise<FinanceTransaction[]>
  getTransaction(params: { tenantId: string; transactionId: string }): Promise<FinanceTransaction>
  listTransactionProviderEvidence(params: { tenantId: string; transactionId: string }): Promise<FinanceProviderEvidenceMetadata[]>
  getTransactionProviderEvidence(params: { tenantId: string; transactionId: string; evidenceId: string }): Promise<FinanceProviderEvidence>
  createTransaction(params: {
    tenantId: string
    accountId: string
    source: string
    status: string
    kind: string
    amountMinor: number
    currency: string
    description: string
    effectiveAt: Date
    categoryId?: string
    tagIds: string[]
  }): Promise<FinanceTransaction>
  updateTransaction(params: {
    tenantId: string
    transactionId: string
    description: string
    amountMinor: number
    effectiveAt?: Date
    categoryId?: string | null
    tagIds: string[]
  }): Promise<FinanceTransaction>
  linkTransferPair(params: { tenantId: string; firstTransactionId: string; secondTransactionId: string }): Promise<void>
  unlinkTransferPair(params: { tenantId: string; firstTransactionId: string; secondTransactionId: string }): Promise<void>
  listTransferCandidates(params: {
    tenantId: string
    transactionId: string
    effectiveFrom: Date
    effectiveBefore: Date
    limit: number
    offset?: number
  }): Promise<FinanceTransferCandidatePage>
  getTransferPartner(params: { tenantId: string; transactionId: string }): Promise<FinanceTransaction>
  listConnections(params: { tenantId: string }): Promise<FinanceBankConnection[]>
  listConnectionSyncedAccounts(params: { tenantId: string; connectionId: string }): Promise<FinanceConnectionSyncedAccount[]>
  linkTokenConnection(params: { tenantId: string; provider: string; token: string }): Promise<FinanceBankConnection>
  startRedirectConnection(params: { tenantId: string; provider: string; callbackUrl: string }): Promise<FinanceConnectionRedirectStart>
  finishRedirectConnection(params: { tenantId: string; provider: string; code?: string; state: string }): Promise<FinanceBankConnection>
  getSyntheticLinkState(params: { tenantId: string; state: string }): Promise<FinanceSyntheticLinkState>
  saveSyntheticLinkState(params: {
    tenantId: string
    state: string
    configuredAccounts: Array<{
      key?: string
      name: string
      currency: string
    }>
  }): Promise<FinanceSyntheticLinkState>
  deleteConnection(params: { tenantId: string; connectionId: string }): Promise<void>
  triggerConnectionSync(params: {
    tenantId: string
    connectionId: string
    reason: string
    windowStart?: Date
    windowEnd?: Date
  }): Promise<FinanceJobRef>
  getDashboard(params: { tenantId: string; preset?: string; startDate?: Date; endDate?: Date }): Promise<FinanceDashboard>
  getFXDiagnostics(): Promise<FinanceFXDiagnostics>
  triggerFXSync(params: {
    provider?: string
  }): Promise<FinanceJobRef>
  previewCSVImport(params: {
    tenantId: string
    fileName: string
    csv: string
    selectedAccountNames?: string[]
  }): Promise<FinanceCSVImportPreview>
  confirmCSVImport(params: {
    tenantId: string
    importId: string
  }): Promise<FinanceCSVImportConfirmation>
  getCSVImportAudit(params: { tenantId: string; importId: string }): Promise<FinanceCSVImportAudit>
  listRecentCSVImportAudits(params: { tenantId: string }): Promise<FinanceCSVImportAudit[]>
}

export class FinanceApiError extends Error {
  readonly status: number

  constructor(params: { status: number; method: string; path: string; message: string }) {
    super(`Finance API ${params.method} ${params.path} failed: ${params.message}`)
    this.name = 'FinanceApiError'
    this.status = params.status
  }
}

export class FinanceResponseError extends Error {
  constructor(params: { field: string; issue: string }) {
    super(`Finance API response contract violation: ${params.field} ${params.issue}`)
    this.name = 'FinanceResponseError'
  }
}

export function createSignalFinanceApi(params: { baseUrl: string; fetch: FetchLike }): SignalFinanceApi {
  const request = async <T>(requestParams: {
      method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
    path: string
    query?: URLSearchParams
    body?: unknown
  }): Promise<T> => {
    const url = new URL(`${params.baseUrl}${requestParams.path}`, window.location.origin)
    if (requestParams.query) {
      url.search = requestParams.query.toString()
    }
    const response = await params.fetch(url.toString(), {
      method: requestParams.method,
      headers: {
        Accept: 'application/json',
        ...(requestParams.body ? { 'Content-Type': 'application/json' } : {}),
      },
      ...(requestParams.body ? { body: JSON.stringify(serializeJson(requestParams.body)) } : {}),
    })
    if (!response.ok) {
      throw new FinanceApiError({
        status: response.status,
        method: requestParams.method,
        path: requestParams.path,
        message: response.statusText || 'Request failed',
      })
    }
    const json = await response.json().catch(() => undefined)
    return json as T
  }

  return {
    async listTenants() {
      const json = await request<{ items?: RawTenantSummary[] }>({ method: 'GET', path: '/finance/tenants' })
      return requireItems<RawTenantSummary>(json, 'finance.tenants.items').map(mapTenant)
    },
    async createTenant(body) {
      return mapTenant(await request<RawTenantSummary>({ method: 'POST', path: '/finance/tenants', body }))
    },
    async updateTenant({ tenantId, name, displayCurrency }) {
      await request<void>({
        method: 'PATCH',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}`,
        body: { name, displayCurrency },
      })
    },
    async archiveTenant({ tenantId }) {
      await request<void>({
        method: 'POST',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/archive`,
      })
    },
    async getTenant({ tenantId }) {
      return mapTenant(await request<RawTenantSummary>({ method: 'GET', path: `/finance/tenants/${encodeURIComponent(tenantId)}` }))
    },
    async listTenantMembers({ tenantId }) {
      const json = await request<{ items?: RawTenantMember[] }>({ method: 'GET', path: `/finance/tenants/${encodeURIComponent(tenantId)}/members` })
      return requireItems<RawTenantMember>(json, 'finance.tenantMembers.items').map(mapTenantMember)
    },
    async listTenantInvites({ tenantId }) {
      const json = await request<{ items?: RawTenantInvite[] }>({ method: 'GET', path: `/finance/tenants/${encodeURIComponent(tenantId)}/invites` })
      return requireItems<RawTenantInvite>(json, 'finance.tenantInvites.items').map(mapTenantInvite)
    },
    async createTenantInvite({ tenantId, recipient }) {
      return mapTenantInvite(
        await request<RawTenantInvite>({
          method: 'POST',
          path: `/finance/tenants/${encodeURIComponent(tenantId)}/invites`,
          body: { recipient },
        }),
      )
    },
    async acceptTenantInvite({ code }) {
      return mapTenantMember(
        await request<RawTenantMember>({ method: 'POST', path: '/finance/invites/accept', body: { code } }),
      )
    },
    async listAccounts({ tenantId, includeHidden }) {
      const json = await request<{ items?: RawAccount[] }>({
        method: 'GET',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/accounts`,
        query: buildSearchParams({ includeHidden }),
      })
      return requireItems<RawAccount>(json, 'finance.accounts.items').map(mapAccount)
    },
    async createAccount({ tenantId, name, currency, kind }) {
      return mapAccount(
        await request<RawAccount>({
          method: 'POST',
          path: `/finance/tenants/${encodeURIComponent(tenantId)}/accounts`,
          body: { name, currency, kind },
        }),
      )
    },
    async getAccount({ tenantId, accountId }) {
      return mapAccount(await request<RawAccount>({
        method: 'GET',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/accounts/${encodeURIComponent(accountId)}`,
      }))
    },
    async renameAccount({ tenantId, accountId, name }) {
      await request<void>({
        method: 'PATCH',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/accounts/${encodeURIComponent(accountId)}`,
        body: { name },
      })
    },
    async hideAccount({ tenantId, accountId }) {
      await request<void>({
        method: 'POST',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/accounts/${encodeURIComponent(accountId)}/hide`,
      })
    },
    async restoreAccount({ tenantId, accountId }) {
      await request<void>({
        method: 'POST',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/accounts/${encodeURIComponent(accountId)}/unhide`,
      })
    },
    async listAccountProviderEvidence({ tenantId, accountId }) {
      const json = await request<{ items?: RawProviderEvidenceMetadata[] }>({
        method: 'GET',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/accounts/${encodeURIComponent(accountId)}/evidence`,
      })
      return requireItems<RawProviderEvidenceMetadata>(json, 'finance.accountEvidence.items').map(mapProviderEvidenceMetadata)
    },
    async getAccountProviderEvidence({ tenantId, accountId, evidenceId }) {
      return mapProviderEvidence(await request<RawProviderEvidence>({
        method: 'GET',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/accounts/${encodeURIComponent(accountId)}/evidence/${encodeURIComponent(evidenceId)}`,
      }))
    },
    async listCategories({ tenantId, includeHidden }) {
      const json = await request<{ items?: RawCategory[] }>({
        method: 'GET',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/categories`,
        query: buildSearchParams({ includeHidden }),
      })
      return requireItems<RawCategory>(json, 'finance.categories.items').map(mapCategory)
    },
    async createCategory({ tenantId, name, kind }) {
      return mapCategory(
        await request<RawCategory>({
          method: 'POST',
          path: `/finance/tenants/${encodeURIComponent(tenantId)}/categories`,
          body: { name, kind },
        }),
      )
    },
    async updateCategory({ tenantId, categoryId, name, kind }) {
      await request<void>({
        method: 'PATCH',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/categories/${encodeURIComponent(categoryId)}`,
        body: { name, kind },
      })
    },
    async listTags({ tenantId, includeHidden }) {
      const json = await request<{ items?: RawTag[] }>({
        method: 'GET',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/tags`,
        query: buildSearchParams({ includeHidden }),
      })
      return requireItems<RawTag>(json, 'finance.tags.items').map(mapTag)
    },
    async createTag({ tenantId, name }) {
      return mapTag(
        await request<RawTag>({
          method: 'POST',
          path: `/finance/tenants/${encodeURIComponent(tenantId)}/tags`,
          body: { name },
        }),
      )
    },
    async renameTag({ tenantId, tagId, name }) {
      await request<void>({
        method: 'PATCH',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/tags/${encodeURIComponent(tagId)}`,
        body: { name },
      })
    },
    async listTransactions({ tenantId, accountId, source, status, includeHidden, limit, offset }) {
      const json = await request<{ items?: RawTransaction[] }>({
        method: 'GET',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/transactions`,
        query: buildSearchParams({ accountId, source, status, includeHidden, limit, offset }),
      })
      return requireItems<RawTransaction>(json, 'finance.transactions.items').map(mapTransaction)
    },
    async getTransaction({ tenantId, transactionId }) {
      return mapTransaction(
        await request<RawTransaction>({
          method: 'GET',
          path: `/finance/tenants/${encodeURIComponent(tenantId)}/transactions/${encodeURIComponent(transactionId)}`,
        }),
      )
    },
    async listTransactionProviderEvidence({ tenantId, transactionId }) {
      const json = await request<{ items?: RawProviderEvidenceMetadata[] }>({
        method: 'GET',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/transactions/${encodeURIComponent(transactionId)}/evidence`,
      })
      return requireItems<RawProviderEvidenceMetadata>(json, 'finance.transactionEvidence.items').map(mapProviderEvidenceMetadata)
    },
    async getTransactionProviderEvidence({ tenantId, transactionId, evidenceId }) {
      return mapProviderEvidence(await request<RawProviderEvidence>({
        method: 'GET',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/transactions/${encodeURIComponent(transactionId)}/evidence/${encodeURIComponent(evidenceId)}`,
      }))
    },
      async createTransaction(params) {
        return mapTransaction(
          await request<RawTransaction>({
            method: 'POST',
            path: `/finance/tenants/${encodeURIComponent(params.tenantId)}/transactions`,
            body: {
             accountId: params.accountId,
              source: params.source,
              status: params.status,
              kind: params.kind,
              amountMinor: params.amountMinor,
              currency: params.currency,
            description: params.description,
            effectiveAt: serializeRequestTimestamp(params.effectiveAt),
            ...(params.categoryId === undefined ? {} : { categoryId: params.categoryId }),
            tagIds: params.tagIds,
          },
        }),
      )
    },
    async updateTransaction(params) {
      return mapTransaction(
        await request<RawTransaction>({
          method: 'PATCH',
          path: `/finance/tenants/${encodeURIComponent(params.tenantId)}/transactions/${encodeURIComponent(params.transactionId)}`,
          body: {
            description: params.description,
            amountMinor: params.amountMinor,
            ...(params.effectiveAt === undefined ? {} : { effectiveAt: serializeRequestTimestamp(params.effectiveAt) }),
            ...(params.categoryId === undefined
              ? {}
              : params.categoryId === null
                ? { clearCategory: true }
                 : { categoryId: params.categoryId }),
            tagIds: params.tagIds,
          },
        }),
      )
    },
    async linkTransferPair({ tenantId, firstTransactionId, secondTransactionId }) {
      await request<void>({
        method: 'POST',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/transactions/transfer-links`,
        body: { firstTransactionId, secondTransactionId },
      })
    },
    async unlinkTransferPair({ tenantId, firstTransactionId, secondTransactionId }) {
      await request<void>({
        method: 'DELETE',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/transactions/transfer-links`,
        body: { firstTransactionId, secondTransactionId },
      })
    },
    async listTransferCandidates({ tenantId, transactionId, effectiveFrom, effectiveBefore, limit, offset }) {
      const json = await request<{ items?: RawTransaction[] }>({
        method: 'GET',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/transactions/${encodeURIComponent(transactionId)}/transfer-candidates`,
        query: buildSearchParams({
          effectiveFrom: serializeRequestTimestamp(effectiveFrom),
          effectiveBefore: serializeRequestTimestamp(effectiveBefore),
          limit,
          offset,
        }),
      })
      return { items: requireItems<RawTransaction>(json, 'finance.transferCandidates.items').map(mapTransaction) }
    },
    async getTransferPartner({ tenantId, transactionId }) {
      return mapTransaction(await request<RawTransaction>({
        method: 'GET',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/transactions/${encodeURIComponent(transactionId)}/transfer-partner`,
      }))
    },
    async listConnections({ tenantId }) {
      const json = await request<{ items?: RawConnection[] }>({ method: 'GET', path: `/finance/tenants/${encodeURIComponent(tenantId)}/connections` })
      return requireItems<RawConnection>(json, 'finance.connections.items').map(mapConnection)
    },
    async listConnectionSyncedAccounts({ tenantId, connectionId }) {
      const json = await request<{ items?: RawConnectionSyncedAccount[] }>({
        method: 'GET',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/connections/${encodeURIComponent(connectionId)}/accounts`,
      })
      return requireItems<RawConnectionSyncedAccount>(json, 'finance.connectionSyncedAccounts.items').map(mapConnectionSyncedAccount)
    },
    async linkTokenConnection({ tenantId, provider, token }) {
      return mapConnection(
        await request<RawConnection>({
          method: 'POST',
          path: `/finance/tenants/${encodeURIComponent(tenantId)}/connections/link-token`,
          body: { provider, token },
        }),
      )
    },
    async startRedirectConnection({ tenantId, provider, callbackUrl }) {
      return mapConnectionRedirectStart(
        await request<RawConnectionRedirectStart>({
          method: 'POST',
          path: `/finance/tenants/${encodeURIComponent(tenantId)}/connections/link-redirect/start`,
          body: { provider, callbackUrl },
        }),
      )
    },
    async finishRedirectConnection({ tenantId, provider, code, state }) {
      return mapConnection(
        await request<RawConnection>({
          method: 'POST',
          path: `/finance/tenants/${encodeURIComponent(tenantId)}/connections/link-redirect/finish`,
          body: { provider, code, state },
        }),
      )
    },
    async getSyntheticLinkState({ tenantId, state }) {
      return mapSyntheticLinkState(
        await request<RawSyntheticLinkState>({
          method: 'GET',
          path: `/finance/tenants/${encodeURIComponent(tenantId)}/connections/synthetic-link-states/state/${encodeURIComponent(state)}`,
        }),
      )
    },
    async saveSyntheticLinkState({ tenantId, state, configuredAccounts }) {
      return mapSyntheticLinkState(
        await request<RawSyntheticLinkState>({
          method: 'PUT',
          path: `/finance/tenants/${encodeURIComponent(tenantId)}/connections/synthetic-link-states/state/${encodeURIComponent(state)}`,
          body: {
            configuredAccounts: configuredAccounts.map((account) => {
              if (account.key) {
                return account
              }
              return { name: account.name, currency: account.currency }
            }),
          },
        }),
      )
    },
    async deleteConnection({ tenantId, connectionId }) {
      await request<void>({
        method: 'DELETE',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/connections/${encodeURIComponent(connectionId)}`,
      })
    },
    async triggerConnectionSync(params) {
      const json = await request<RawJobRef>({
        method: 'POST',
        path: `/finance/tenants/${encodeURIComponent(params.tenantId)}/connections/${encodeURIComponent(params.connectionId)}/sync`,
        body: {
          reason: params.reason,
          ...(params.windowStart === undefined ? {} : { windowStart: serializeRequestTimestamp(params.windowStart) }),
          ...(params.windowEnd === undefined ? {} : { windowEnd: serializeRequestTimestamp(params.windowEnd) }),
        },
      })
      return mapJobRef(json)
    },
    async getDashboard({ tenantId, preset, startDate, endDate }) {
      return mapDashboard(
        await request<RawDashboard>({
          method: 'GET',
          path: `/finance/tenants/${encodeURIComponent(tenantId)}/dashboard`,
          query: buildSearchParams({
            preset,
            startDate: startDate === undefined ? undefined : serializeRequestTimestamp(startDate),
            endDate: endDate === undefined ? undefined : serializeRequestTimestamp(endDate),
          }),
        }),
      )
    },
    async getFXDiagnostics() {
      return mapFXDiagnostics(await request<RawFXDiagnostics>({ method: 'GET', path: '/finance/fx/diagnostics' }))
    },
    async triggerFXSync(params) {
      return mapJobRef(await request<RawJobRef>({
        method: 'POST',
        path: '/finance/fx/sync',
        body: params.provider?.trim() ? { provider: params.provider.trim() } : {},
      }))
    },
    async previewCSVImport(params) {
      return mapCSVPreview(
        await request<RawCSVImportPreview>({
          method: 'POST',
          path: `/finance/tenants/${encodeURIComponent(params.tenantId)}/imports/preview`,
          body: {
            fileName: params.fileName,
            csv: params.csv,
            ...(params.selectedAccountNames === undefined ? {} : { selectedAccountNames: params.selectedAccountNames }),
          },
        }),
      )
    },
    async confirmCSVImport(params) {
      return mapCSVImportConfirmation(
        await request<RawCSVImportConfirmation>({
          method: 'POST',
          path: `/finance/tenants/${encodeURIComponent(params.tenantId)}/imports/${encodeURIComponent(params.importId)}/confirm`,
        }),
      )
    },
    async getCSVImportAudit(params) {
      return mapCSVImportAudit(
        await request<RawCSVImportAudit>({
          method: 'GET',
          path: `/finance/tenants/${encodeURIComponent(params.tenantId)}/imports/${encodeURIComponent(params.importId)}`,
        }),
      )
    },
    async listRecentCSVImportAudits(params) {
      const response = await request<{ items: RawCSVImportAudit[] }>({
        method: 'GET',
        path: `/finance/tenants/${encodeURIComponent(params.tenantId)}/imports`,
      })
      return requireItems<RawCSVImportAudit>(response, 'finance.csvAudits').map(mapCSVImportAudit)
    },
  }
}

export function createSignalFinanceApiForAuth(params: { baseUrl: string; authStore: AuthStore }): SignalFinanceApi {
  return createSignalFinanceApi({ baseUrl: params.baseUrl, fetch: createAuthFetch(params.authStore) })
}

interface RawTenantSummary { id: string; name: string; displayCurrency: string; archivedAt?: string | null; joinedAt: string; createdAt: string; updatedAt: string }
interface RawTenantMember { tenantId: string; userId: string; username?: string; joinedAt: string }
interface RawTenantInvite { id: string; tenantId: string; code: string; recipient: string; createdByUserId: string; acceptedByUserId?: string | null; createdAt: string; acceptedAt?: string | null }
interface RawAccount { id: string; tenantId: string; name: string; currency: string; kind: string; bookedBalanceMinor: number; pendingBalanceMinor: number; provider?: string; providerAccountId?: string; hiddenAt?: string | null; createdAt: string; updatedAt: string }
interface RawCategory { id: string; tenantId: string; name: string; kind: string; seededDefault: boolean; hiddenAt?: string | null; createdAt: string; updatedAt: string }
interface RawTag { id: string; tenantId: string; name: string; hiddenAt?: string | null; createdAt: string; updatedAt: string }
interface RawTransactionProviderOriginal { amountMinor: number; currency: string; description: string; effectiveAt?: string | null }
interface RawProviderEvidenceMetadata { id: string; scope: string; providerObjectId: string; capturedAt: string }
interface RawProviderEvidence extends RawProviderEvidenceMetadata { payload?: Record<string, unknown> }
interface RawTransaction { id: string; tenantId: string; accountId: string; source: string; status: string; kind: string; amountMinor: number; currency: string; description: string; effectiveAt: string; categoryId?: string | null; tagIds: string[]; transferGroupId?: string | null; transferMatchedAt?: string | null; hiddenAt?: string | null; providerOriginal?: RawTransactionProviderOriginal; createdAt: string; updatedAt: string }
interface RawConnectionSchedule { connectionId: string; intervalSeconds: number; nextRunAt?: string | null; lastScheduledAt?: string | null; lastStartedAt?: string | null; lastCompletedAt?: string | null; lastJobId?: string; enabled: boolean; createdAt: string; updatedAt: string }
interface RawConnection { id: string; tenantId: string; provider: string; displayName: string; providerReference: string; externalId: string; state: string; lastSyncJobId?: string; lastSyncStartedAt?: string | null; lastSuccessfulSyncAt?: string | null; lastSyncError?: string; createdAt: string; updatedAt: string; schedule?: RawConnectionSchedule }
interface RawConnectionSyncedAccount { financeAccountId: string; name: string; currency: string; lastSuccessfulSyncAt?: string | null }
interface RawConnectionRedirectStart { provider: string; authorizationUrl: string; state: string }
interface RawSyntheticLinkStateConfiguredAccount { key: string; name: string; currency: string }
interface RawSyntheticLinkState {
  provider: string
  state: string
  configuredAccounts: RawSyntheticLinkStateConfiguredAccount[]
  canFinish: boolean
}
interface RawDashboardPeriodWindow { startDate: string; endDate: string }
interface RawDashboardPeriod { preset: string; startDate: string; endDate: string; previous: RawDashboardPeriodWindow; next: RawDashboardPeriodWindow }
interface RawMoneySummary { displayCurrency: string; incomeMinor: number; expenseMinor: number; netMinor: number; transactionCount: number; complete: boolean }
interface RawCategoryBreakdown { categoryId: string; categoryName: string; kind: string; incomeMinor: number; expenseMinor: number; transactionCount: number }
interface RawAccountBalance { accountId: string; accountName: string; currency: string; nativeBookedMinor: number; nativePendingMinor: number; displayBookedMinor: number | null; displayPendingMinor: number | null; missingFx: boolean }
interface RawDashboardAlert { code: string; severity: string; count: number }
interface RawDashboardFXCoverage { provider: string; baseCurrency: string; quoteCurrency: string; affectedTransactionCount: number; affectedAccountCount: number }
interface RawCurrentFxRate { provider: string; baseCurrency: string; quoteCurrency: string; effectiveAt: string; lastSuccessfulRefreshAt: string; stale: boolean }
interface RawCurrencyTotal { currency: string; incomeMinor: number; expenseMinor: number; netMinor: number }
interface RawDashboard { period: RawDashboardPeriod; settled: RawMoneySummary; pending: RawMoneySummary; categoryBreakdowns: RawCategoryBreakdown[]; accountBalances: RawAccountBalance[]; alerts: RawDashboardAlert[]; fxCoverage: RawDashboardFXCoverage[]; currentFxRates: RawCurrentFxRate[]; nativeSettledTotals: RawCurrencyTotal[] }
interface RawFXDiagnostics { defaultProvider: string; storedRatesCount: number; providers: RawFXProvider[] }
interface RawFXProvider { name: string; default: boolean; ready: boolean }
interface RawCSVRejectedRow { rowNumber: number; field?: string; reason: string }
interface RawCSVImportAccountOption { name: string; sourceRowCount: number; selected: boolean }
interface RawCSVImportPreview { importId: string; importableCount: number; headers: string[]; duplicateRows: RawCSVRejectedRow[]; rejectedRows: RawCSVRejectedRow[]; wouldCreateAccounts: string[]; wouldCreateCategories: string[]; wouldCreateTags: string[]; accountOptions: RawCSVImportAccountOption[] }
interface RawCSVImportConfirmation { importId: string; jobId: string; jobType: string }
interface RawCSVImportRowOutcome { rowNumber: number; transactionId?: string; status: string; reason: string; createdAt: string; updatedAt: string }
interface RawCSVImportAudit { importId: string; tenantId: string; status: string; jobId: string; confirmedByUserId: string; importedCount: number; rejectedRows: RawCSVRejectedRow[]; rowOutcomes: RawCSVImportRowOutcome[]; createdAt: string; confirmedAt?: string | null; completedAt?: string | null }
interface RawJobRef { jobId: string; jobType: string; provider?: string }

function mapTenant(item: RawTenantSummary): FinanceTenantSummary {
  requireFields(item, 'finance.tenant', ['id', 'name', 'displayCurrency', 'joinedAt', 'createdAt', 'updatedAt'])
  return { ...item, archivedAt: parseOptionalDate(item.archivedAt, 'finance.tenant.archivedAt'), joinedAt: parseRequiredDate(item.joinedAt, 'finance.tenant.joinedAt'), createdAt: parseRequiredDate(item.createdAt, 'finance.tenant.createdAt'), updatedAt: parseRequiredDate(item.updatedAt, 'finance.tenant.updatedAt') }
}
function mapTenantMember(item: RawTenantMember): FinanceTenantMember {
  requireFields(item, 'finance.tenantMember', ['tenantId', 'userId', 'joinedAt'])
  return { ...item, joinedAt: parseRequiredDate(item.joinedAt, 'finance.tenantMember.joinedAt') }
}
function mapTenantInvite(item: RawTenantInvite): FinanceTenantInvite {
  requireFields(item, 'finance.tenantInvite', ['id', 'tenantId', 'code', 'recipient', 'createdByUserId', 'createdAt'])
  return { ...item, createdAt: parseRequiredDate(item.createdAt, 'finance.tenantInvite.createdAt'), acceptedAt: parseOptionalDate(item.acceptedAt, 'finance.tenantInvite.acceptedAt') }
}
function mapAccount(item: RawAccount): FinanceAccount {
  requireFields(item, 'finance.account', ['id', 'tenantId', 'name', 'currency', 'kind', 'bookedBalanceMinor', 'pendingBalanceMinor', 'createdAt', 'updatedAt'])
  return { ...item, hiddenAt: parseOptionalDate(item.hiddenAt, 'finance.account.hiddenAt'), createdAt: parseRequiredDate(item.createdAt, 'finance.account.createdAt'), updatedAt: parseRequiredDate(item.updatedAt, 'finance.account.updatedAt') }
}
function mapCategory(item: RawCategory): FinanceCategory {
  requireFields(item, 'finance.category', ['id', 'tenantId', 'name', 'kind', 'seededDefault', 'createdAt', 'updatedAt'])
  return { ...item, hiddenAt: parseOptionalDate(item.hiddenAt, 'finance.category.hiddenAt'), createdAt: parseRequiredDate(item.createdAt, 'finance.category.createdAt'), updatedAt: parseRequiredDate(item.updatedAt, 'finance.category.updatedAt') }
}
function mapTag(item: RawTag): FinanceTag {
  requireFields(item, 'finance.tag', ['id', 'tenantId', 'name', 'createdAt', 'updatedAt'])
  return { ...item, hiddenAt: parseOptionalDate(item.hiddenAt, 'finance.tag.hiddenAt'), createdAt: parseRequiredDate(item.createdAt, 'finance.tag.createdAt'), updatedAt: parseRequiredDate(item.updatedAt, 'finance.tag.updatedAt') }
}
function mapTransaction(item: RawTransaction): FinanceTransaction {
  requireFields(item, 'finance.transaction', ['id', 'tenantId', 'accountId', 'source', 'status', 'kind', 'amountMinor', 'currency', 'description', 'effectiveAt', 'tagIds', 'createdAt', 'updatedAt'])
  requireArray(item.tagIds, 'finance.transaction.tagIds')
  const { providerOriginal, ...transaction } = item
  const effectiveAt = parseRequiredDate(item.effectiveAt, 'finance.transaction.effectiveAt')
  return {
    ...transaction,
    transferMatchedAt: parseOptionalDate(item.transferMatchedAt, 'finance.transaction.transferMatchedAt'),
    hiddenAt: parseOptionalDate(item.hiddenAt, 'finance.transaction.hiddenAt'),
    ...(providerOriginal === undefined ? {} : {
      providerOriginal: {
        ...requireFields(providerOriginal, 'finance.transaction.providerOriginal', ['amountMinor', 'currency', 'description']),
        effectiveAt: parseOptionalDate(providerOriginal.effectiveAt, 'finance.transaction.providerOriginal.effectiveAt'),
      },
    }),
    effectiveAt,
    createdAt: parseRequiredDate(item.createdAt, 'finance.transaction.createdAt'),
    updatedAt: parseRequiredDate(item.updatedAt, 'finance.transaction.updatedAt'),
  }
}
function mapProviderEvidenceMetadata(item: RawProviderEvidenceMetadata): FinanceProviderEvidenceMetadata {
  requireFields(item, 'finance.providerEvidence', ['id', 'scope', 'providerObjectId', 'capturedAt'])
  return { ...item, capturedAt: parseRequiredDate(item.capturedAt, 'finance.providerEvidence.capturedAt') }
}
function mapProviderEvidence(item: RawProviderEvidence): FinanceProviderEvidence {
  return { ...mapProviderEvidenceMetadata(item), ...(item.payload === undefined ? {} : { payload: item.payload }) }
}
function mapConnection(item: RawConnection): FinanceBankConnection {
  requireFields(item, 'finance.connection', ['id', 'tenantId', 'provider', 'displayName', 'providerReference', 'externalId', 'state', 'createdAt', 'updatedAt'])
  const { schedule, ...connection } = item
  return {
    ...connection,
    lastSyncStartedAt: parseOptionalDate(item.lastSyncStartedAt, 'finance.connection.lastSyncStartedAt'),
    lastSuccessfulSyncAt: parseOptionalDate(item.lastSuccessfulSyncAt, 'finance.connection.lastSuccessfulSyncAt'),
    createdAt: parseRequiredDate(item.createdAt, 'finance.connection.createdAt'),
    updatedAt: parseRequiredDate(item.updatedAt, 'finance.connection.updatedAt'),
    ...(schedule === undefined ? {} : {
      schedule: {
        ...requireFields(schedule, 'finance.connection.schedule', ['connectionId', 'intervalSeconds', 'enabled', 'createdAt', 'updatedAt']),
        nextRunAt: parseOptionalDate(schedule.nextRunAt, 'finance.connection.schedule.nextRunAt'),
        lastScheduledAt: parseOptionalDate(schedule.lastScheduledAt, 'finance.connection.schedule.lastScheduledAt'),
        lastStartedAt: parseOptionalDate(schedule.lastStartedAt, 'finance.connection.schedule.lastStartedAt'),
        lastCompletedAt: parseOptionalDate(schedule.lastCompletedAt, 'finance.connection.schedule.lastCompletedAt'),
        createdAt: parseRequiredDate(schedule.createdAt, 'finance.connection.schedule.createdAt'),
        updatedAt: parseRequiredDate(schedule.updatedAt, 'finance.connection.schedule.updatedAt'),
      },
    }),
  }
}
function mapConnectionSyncedAccount(item: RawConnectionSyncedAccount): FinanceConnectionSyncedAccount {
  requireFields(item, 'finance.connectionSyncedAccount', ['financeAccountId', 'name', 'currency'])
  return {
    ...item,
    lastSuccessfulSyncAt: parseOptionalDate(item.lastSuccessfulSyncAt, 'finance.connectionSyncedAccount.lastSuccessfulSyncAt'),
  }
}
function mapConnectionRedirectStart(item: RawConnectionRedirectStart): FinanceConnectionRedirectStart { return item }
function mapSyntheticLinkState(item: RawSyntheticLinkState): FinanceSyntheticLinkState {
  requireFields(item, 'finance.syntheticLinkState', ['provider', 'state', 'configuredAccounts', 'canFinish'])
  requireArray(item.configuredAccounts, 'finance.syntheticLinkState.configuredAccounts')
  return {
    ...item,
  }
}
function mapDashboard(item: RawDashboard): FinanceDashboard {
  requireFields(item, 'finance.dashboard', ['period', 'settled', 'pending', 'categoryBreakdowns', 'accountBalances', 'alerts', 'fxCoverage', 'currentFxRates', 'nativeSettledTotals'])
  requireFields(item.period, 'finance.dashboard.period', ['preset', 'startDate', 'endDate', 'previous', 'next'])
  requireFields(item.period.previous, 'finance.dashboard.period.previous', ['startDate', 'endDate'])
  requireFields(item.period.next, 'finance.dashboard.period.next', ['startDate', 'endDate'])
  requireArray(item.categoryBreakdowns, 'finance.dashboard.categoryBreakdowns')
  requireArray(item.accountBalances, 'finance.dashboard.accountBalances')
  requireArray(item.alerts, 'finance.dashboard.alerts')
  requireArray(item.fxCoverage, 'finance.dashboard.fxCoverage')
  requireArray(item.currentFxRates, 'finance.dashboard.currentFxRates')
  requireArray(item.nativeSettledTotals, 'finance.dashboard.nativeSettledTotals')
  requireFields(item.settled, 'finance.dashboard.settled', ['displayCurrency', 'incomeMinor', 'expenseMinor', 'netMinor', 'transactionCount', 'complete'])
  requireFields(item.pending, 'finance.dashboard.pending', ['displayCurrency', 'incomeMinor', 'expenseMinor', 'netMinor', 'transactionCount', 'complete'])
  item.categoryBreakdowns.forEach((breakdown, index) => requireFields(breakdown, `finance.dashboard.categoryBreakdowns[${index}]`, ['categoryId', 'categoryName', 'kind', 'incomeMinor', 'expenseMinor', 'transactionCount']))
  item.accountBalances.forEach((balance, index) => {
    requireFields(balance, `finance.dashboard.accountBalances[${index}]`, ['accountId', 'accountName', 'currency', 'nativeBookedMinor', 'nativePendingMinor', 'missingFx'])
    requirePresentFields(balance, `finance.dashboard.accountBalances[${index}]`, ['displayBookedMinor', 'displayPendingMinor'])
  })
  item.alerts.forEach((alert, index) => requireFields(alert, `finance.dashboard.alerts[${index}]`, ['code', 'severity', 'count']))
  item.fxCoverage.forEach((coverage, index) => {
    requireFields(coverage, `finance.dashboard.fxCoverage[${index}]`, ['provider', 'baseCurrency', 'quoteCurrency', 'affectedTransactionCount', 'affectedAccountCount'])
  })
  item.currentFxRates.forEach((rate, index) => requireFields(rate, `finance.dashboard.currentFxRates[${index}]`, ['provider', 'baseCurrency', 'quoteCurrency', 'effectiveAt', 'lastSuccessfulRefreshAt', 'stale']))
  item.nativeSettledTotals.forEach((total, index) => requireFields(total, `finance.dashboard.nativeSettledTotals[${index}]`, ['currency', 'incomeMinor', 'expenseMinor', 'netMinor']))
  return {
    period: {
      preset: item.period.preset,
      startDate: parseRequiredDate(item.period.startDate, 'finance.dashboard.period.startDate'),
      endDate: parseRequiredDate(item.period.endDate, 'finance.dashboard.period.endDate'),
      previous: {
        startDate: parseRequiredDate(item.period.previous.startDate, 'finance.dashboard.period.previous.startDate'),
        endDate: parseRequiredDate(item.period.previous.endDate, 'finance.dashboard.period.previous.endDate'),
      },
      next: {
        startDate: parseRequiredDate(item.period.next.startDate, 'finance.dashboard.period.next.startDate'),
        endDate: parseRequiredDate(item.period.next.endDate, 'finance.dashboard.period.next.endDate'),
      },
    },
    settled: item.settled,
    pending: item.pending,
    categoryBreakdowns: item.categoryBreakdowns,
    accountBalances: item.accountBalances.map((balance) => ({
      ...balance,
    })),
    alerts: item.alerts,
    fxCoverage: item.fxCoverage,
    currentFxRates: item.currentFxRates.map((rate, index) => ({
      ...rate,
      effectiveAt: parseRequiredDate(rate.effectiveAt, `finance.dashboard.currentFxRates[${index}].effectiveAt`),
      lastSuccessfulRefreshAt: parseRequiredDate(rate.lastSuccessfulRefreshAt, `finance.dashboard.currentFxRates[${index}].lastSuccessfulRefreshAt`),
    })),
    nativeSettledTotals: item.nativeSettledTotals,
  }
}
function mapFXDiagnostics(item: RawFXDiagnostics): FinanceFXDiagnostics {
  requireFields(item, 'finance.fxDiagnostics', ['defaultProvider', 'storedRatesCount', 'providers'])
  requireArray(item.providers, 'finance.fxDiagnostics.providers')
  return { defaultProvider: item.defaultProvider, storedRatesCount: item.storedRatesCount, providers: item.providers }
}
function mapCSVPreview(item: RawCSVImportPreview): FinanceCSVImportPreview {
  requireFields(item, 'finance.csvPreview', ['importId', 'importableCount', 'headers', 'duplicateRows', 'rejectedRows', 'wouldCreateAccounts', 'wouldCreateCategories', 'wouldCreateTags', 'accountOptions'])
  requireArray(item.headers, 'finance.csvPreview.headers')
  requireArray(item.duplicateRows, 'finance.csvPreview.duplicateRows')
  requireArray(item.rejectedRows, 'finance.csvPreview.rejectedRows')
  requireArray(item.wouldCreateAccounts, 'finance.csvPreview.wouldCreateAccounts')
  requireArray(item.wouldCreateCategories, 'finance.csvPreview.wouldCreateCategories')
  requireArray(item.wouldCreateTags, 'finance.csvPreview.wouldCreateTags')
  requireArray(item.accountOptions, 'finance.csvPreview.accountOptions')
  item.accountOptions.forEach((option, index) => requireFields(option, `finance.csvPreview.accountOptions[${index}]`, ['name', 'sourceRowCount', 'selected']))
  return {
    importId: item.importId,
    importableCount: item.importableCount,
    headers: item.headers,
    duplicateRows: item.duplicateRows,
    rejectedRows: item.rejectedRows,
    wouldCreateAccounts: item.wouldCreateAccounts,
    wouldCreateCategories: item.wouldCreateCategories,
    wouldCreateTags: item.wouldCreateTags,
    accountOptions: item.accountOptions,
  }
}
function mapCSVImportConfirmation(item: RawCSVImportConfirmation): FinanceCSVImportConfirmation { return item }
function mapCSVImportAudit(item: RawCSVImportAudit): FinanceCSVImportAudit {
  requireFields(item, 'finance.csvAudit', ['importId', 'tenantId', 'status', 'jobId', 'confirmedByUserId', 'importedCount', 'rejectedRows', 'rowOutcomes', 'createdAt'])
  requireArray(item.rejectedRows, 'finance.csvAudit.rejectedRows')
  requireArray(item.rowOutcomes, 'finance.csvAudit.rowOutcomes')
  return {
    ...item,
    createdAt: parseRequiredDate(item.createdAt, 'finance.csvAudit.createdAt'),
    confirmedAt: parseOptionalDate(item.confirmedAt, 'finance.csvAudit.confirmedAt'),
    completedAt: parseOptionalDate(item.completedAt, 'finance.csvAudit.completedAt'),
    rowOutcomes: item.rowOutcomes.map((outcome, index) => {
      requireFields(outcome, `finance.csvAudit.rowOutcomes[${index}]`, ['rowNumber', 'status', 'reason', 'createdAt', 'updatedAt'])
      return {
        ...outcome,
        createdAt: parseRequiredDate(outcome.createdAt, `finance.csvAudit.rowOutcomes[${index}].createdAt`),
        updatedAt: parseRequiredDate(outcome.updatedAt, `finance.csvAudit.rowOutcomes[${index}].updatedAt`),
      }
    }),
  }
}
function mapJobRef(item: RawJobRef): FinanceJobRef { return { jobId: item.jobId, jobType: item.jobType, ...(item.provider ? { provider: item.provider } : {}) } }

function parseRequiredDate(value: unknown, field: string): Date {
  try {
    return parseRequiredResponseTimestamp(value, { api: 'Finance', field })
  } catch (error) {
    if (error instanceof ResponseTimestampError) {
      throw new FinanceResponseError({ field, issue: error.message.split(`${field} `)[1] ?? 'is invalid' })
    }
    throw error
  }
}

function parseOptionalDate(value: unknown, field: string): Date | null | undefined {
  if (value === undefined || value === null) {
    return value
  }
  return parseRequiredDate(value, field)
}

function requireItems<T>(value: unknown, field: string): T[] {
  if (!value || typeof value !== 'object') {
    throw new FinanceResponseError({ field, issue: 'must be an object with an items array' })
  }
  return requireArray((value as { items?: T[] }).items, field)
}

function requireArray<T>(value: T[] | undefined, field: string): T[] {
  if (!Array.isArray(value)) {
    throw new FinanceResponseError({ field, issue: 'must be present as an array' })
  }
  return value
}

function requireFields<T extends object>(value: T, scope: string, fields: string[]): T {
  if (!value || typeof value !== 'object') {
    throw new FinanceResponseError({ field: scope, issue: 'must be an object' })
  }
  for (const field of fields) {
    if (!(field in value) || (value as Record<string, unknown>)[field] === null || (value as Record<string, unknown>)[field] === undefined) {
      throw new FinanceResponseError({ field: `${scope}.${field}`, issue: 'is required' })
    }
  }
  return value
}

function requirePresentFields<T extends object>(value: T, scope: string, fields: string[]): T {
  if (!value || typeof value !== 'object') {
    throw new FinanceResponseError({ field: scope, issue: 'must be an object' })
  }
  for (const field of fields) {
    if (!(field in value) || (value as Record<string, unknown>)[field] === undefined) {
      throw new FinanceResponseError({ field: `${scope}.${field}`, issue: 'is required' })
    }
  }
  return value
}

function serializeJson(value: unknown): unknown {
  if (value instanceof Date) {
    return serializeRequestTimestamp(value)
  }
  if (Array.isArray(value)) {
    return value.map(serializeJson)
  }
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value).map(([key, entry]) => [key, serializeJson(entry)]))
  }
  return value
}

function buildSearchParams(values: Record<string, string | number | boolean | undefined>): URLSearchParams {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(values)) {
    if (value === undefined || value === '') {
      continue
    }
    params.set(key, String(value))
  }
  return params
}
