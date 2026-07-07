import type { AuthStore } from '../auth/auth-store.svelte'
import { createAuthFetch } from '../auth/auth-fetch'

type FetchLike = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>

export interface FinanceTenantSummary {
  id: string
  name: string
  displayCurrency: string
  joinedAt: Date
  createdAt: Date
  updatedAt: Date
}

export interface FinanceTenantMember {
  tenantId: string
  userId: string
  joinedAt: Date
}

export interface FinanceTenantInvite {
  id: string
  tenantId: string
  code: string
  recipient: string
  createdByUserId: string
  acceptedByUserId: string | null
  createdAt: Date
  acceptedAt: Date | null
}

export interface FinanceAccount {
  id: string
  tenantId: string
  name: string
  currency: string
  kind: string
  provider: string
  providerAccountId: string
  hiddenAt: Date | null
  createdAt: Date
  updatedAt: Date
}

export interface FinanceCategory {
  id: string
  tenantId: string
  name: string
  kind: string
  seededDefault: boolean
  hiddenAt: Date | null
  createdAt: Date
  updatedAt: Date
}

export interface FinanceTag {
  id: string
  tenantId: string
  name: string
  hiddenAt: Date | null
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
  categoryId: string | null
  transferGroupId: string | null
  transferMatchedAt: Date | null
  hiddenAt: Date | null
  providerOriginal: FinanceTransactionProviderOriginal | null
  createdAt: Date
  updatedAt: Date
}

export interface FinanceTransactionProviderOriginal {
  amountMinor: number
  currency: string
  description: string
  effectiveAt: Date | null
}

export interface FinanceBankConnectionSchedule {
  connectionId: string
  intervalSeconds: number
  nextRunAt: Date | null
  lastScheduledAt: Date | null
  lastStartedAt: Date | null
  lastCompletedAt: Date | null
  lastJobId: string
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
  lastSyncJobId: string
  lastSyncStartedAt: Date | null
  lastSuccessfulSyncAt: Date | null
  lastSyncError: string
  createdAt: Date
  updatedAt: Date
  schedule: FinanceBankConnectionSchedule | null
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

export interface FinanceDashboardMissingFx {
  source: string
  transactionId: string
  accountId: string
  baseCurrency: string
  quoteCurrency: string
  rateDate: Date
  provider: string
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
  missingFx: FinanceDashboardMissingFx[]
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
  reason: string
}

export interface FinanceCSVImportPreview {
  importId: string
  importType: string
  headers: string[]
  mapping: Record<string, string>
  duplicateRows: FinanceCSVRejectedRow[]
  rejectedRows: FinanceCSVRejectedRow[]
  wouldCreateAccounts: string[]
  wouldCreateCategories: string[]
  wouldCreateTags: string[]
}

export interface FinanceCSVImportConfirmation {
  importId: string
  jobId: string
  jobType: string
}

export interface FinanceCSVImportAudit {
  importId: string
  tenantId: string
  importType: string
  status: string
  jobId: string
  confirmedByUserId: string
  importedCount: number
  createdAt: Date
  confirmedAt: Date | null
  completedAt: Date | null
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
  createTenant(body: { name: string; displayCurrency: string }): Promise<FinanceTenantSummary>
  updateTenant(params: { tenantId: string; name: string; displayCurrency: string }): Promise<void>
  getTenant(params: { tenantId: string }): Promise<FinanceTenantSummary>
  listTenantMembers(params: { tenantId: string }): Promise<FinanceTenantMember[]>
  listTenantInvites(params: { tenantId: string }): Promise<FinanceTenantInvite[]>
  createTenantInvite(params: { tenantId: string; recipient: string }): Promise<FinanceTenantInvite>
  acceptTenantInvite(params: { code: string }): Promise<FinanceTenantMember>
  listAccounts(params: { tenantId: string; includeHidden?: boolean }): Promise<FinanceAccount[]>
  createAccount(params: { tenantId: string; name: string; currency: string; kind: string }): Promise<FinanceAccount>
  listCategories(params: { tenantId: string; includeHidden?: boolean }): Promise<FinanceCategory[]>
  createCategory(params: { tenantId: string; name: string; kind: string }): Promise<FinanceCategory>
  listTags(params: { tenantId: string; includeHidden?: boolean }): Promise<FinanceTag[]>
  createTag(params: { tenantId: string; name: string }): Promise<FinanceTag>
  listTransactions(params: {
    tenantId: string
    accountId?: string
    source?: string
    status?: string
    includeHidden?: boolean
  }): Promise<FinanceTransaction[]>
  getTransaction(params: { tenantId: string; transactionId: string }): Promise<FinanceTransaction>
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
    transferGroupId?: string
  }): Promise<FinanceTransaction>
  updateTransaction(params: {
    tenantId: string
    transactionId: string
    description: string
    amountMinor: number
    effectiveAt: Date
    categoryId?: string | null
  }): Promise<FinanceTransaction>
  listConnections(params: { tenantId: string }): Promise<FinanceBankConnection[]>
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
    windowStart?: Date | null
    windowEnd?: Date | null
  }): Promise<FinanceJobRef>
  getDashboard(params: { tenantId: string; preset?: string; startDate?: string; endDate?: string }): Promise<FinanceDashboard>
  getFXDiagnostics(): Promise<FinanceFXDiagnostics>
  triggerFXSync(params: {
    provider?: string
    baseCurrencies: string[]
    quoteCurrency: string
    startDate: Date
    endDate: Date
  }): Promise<FinanceJobRef>
  previewCSVImport(params: {
    tenantId: string
    importType: string
    fileName: string
    csv: string
  }): Promise<FinanceCSVImportPreview>
  confirmCSVImport(params: {
    tenantId: string
    importId: string
    mapping: Record<string, string>
  }): Promise<FinanceCSVImportConfirmation>
  getCSVImportAudit(params: { tenantId: string; importId: string }): Promise<FinanceCSVImportAudit>
}

export class FinanceApiError extends Error {
  readonly status: number

  constructor(params: { status: number; method: string; path: string; message: string }) {
    super(`Finance API ${params.method} ${params.path} failed: ${params.message}`)
    this.name = 'FinanceApiError'
    this.status = params.status
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
    const json = await response.json().catch(() => undefined)
    if (!response.ok) {
      throw new FinanceApiError({
        status: response.status,
        method: requestParams.method,
        path: requestParams.path,
        message: extractErrorMessage(response, json),
      })
    }
    return json as T
  }

  return {
    async listTenants() {
      const json = await request<{ items?: RawTenantSummary[] }>({ method: 'GET', path: '/finance/tenants' })
      return (json.items ?? []).map(mapTenant)
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
    async getTenant({ tenantId }) {
      return mapTenant(await request<RawTenantSummary>({ method: 'GET', path: `/finance/tenants/${encodeURIComponent(tenantId)}` }))
    },
    async listTenantMembers({ tenantId }) {
      const json = await request<{ items?: RawTenantMember[] }>({ method: 'GET', path: `/finance/tenants/${encodeURIComponent(tenantId)}/members` })
      return (json.items ?? []).map(mapTenantMember)
    },
    async listTenantInvites({ tenantId }) {
      const json = await request<{ items?: RawTenantInvite[] }>({ method: 'GET', path: `/finance/tenants/${encodeURIComponent(tenantId)}/invites` })
      return (json.items ?? []).map(mapTenantInvite)
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
      return (json.items ?? []).map(mapAccount)
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
    async listCategories({ tenantId, includeHidden }) {
      const json = await request<{ items?: RawCategory[] }>({
        method: 'GET',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/categories`,
        query: buildSearchParams({ includeHidden }),
      })
      return (json.items ?? []).map(mapCategory)
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
    async listTags({ tenantId, includeHidden }) {
      const json = await request<{ items?: RawTag[] }>({
        method: 'GET',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/tags`,
        query: buildSearchParams({ includeHidden }),
      })
      return (json.items ?? []).map(mapTag)
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
    async listTransactions({ tenantId, accountId, source, status, includeHidden }) {
      const json = await request<{ items?: RawTransaction[] }>({
        method: 'GET',
        path: `/finance/tenants/${encodeURIComponent(tenantId)}/transactions`,
        query: buildSearchParams({ accountId, source, status, includeHidden }),
      })
      return (json.items ?? []).map(mapTransaction)
    },
    async getTransaction({ tenantId, transactionId }) {
      return mapTransaction(
        await request<RawTransaction>({
          method: 'GET',
          path: `/finance/tenants/${encodeURIComponent(tenantId)}/transactions/${encodeURIComponent(transactionId)}`,
        }),
      )
    },
    async createTransaction(params) {
      return mapTransaction(
        await request<RawTransaction>({
          method: 'POST',
          path: `/finance/tenants/${encodeURIComponent(params.tenantId)}/transactions`,
          body: params,
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
            effectiveAt: params.effectiveAt,
            categoryId: params.categoryId ?? null,
          },
        }),
      )
    },
    async listConnections({ tenantId }) {
      const json = await request<{ items?: RawConnection[] }>({ method: 'GET', path: `/finance/tenants/${encodeURIComponent(tenantId)}/connections` })
      return (json.items ?? []).map(mapConnection)
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
          path: `/finance/tenants/${encodeURIComponent(tenantId)}/connections/synthetic-link-states/${encodeURIComponent(state)}`,
        }),
      )
    },
    async saveSyntheticLinkState({ tenantId, state, configuredAccounts }) {
      return mapSyntheticLinkState(
        await request<RawSyntheticLinkState>({
          method: 'PUT',
          path: `/finance/tenants/${encodeURIComponent(tenantId)}/connections/synthetic-link-states/${encodeURIComponent(state)}`,
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
        body: { reason: params.reason, windowStart: params.windowStart, windowEnd: params.windowEnd },
      })
      return mapJobRef(json)
    },
    async getDashboard({ tenantId, preset, startDate, endDate }) {
      return mapDashboard(
        await request<RawDashboard>({
          method: 'GET',
          path: `/finance/tenants/${encodeURIComponent(tenantId)}/dashboard`,
          query: buildSearchParams({ preset, startDate, endDate }),
        }),
      )
    },
    async getFXDiagnostics() {
      return mapFXDiagnostics(await request<RawFXDiagnostics>({ method: 'GET', path: '/finance/fx/diagnostics' }))
    },
    async triggerFXSync(params) {
      return mapJobRef(await request<RawJobRef>({ method: 'POST', path: '/finance/fx/sync', body: params }))
    },
    async previewCSVImport(params) {
      return mapCSVPreview(
        await request<RawCSVImportPreview>({
          method: 'POST',
          path: `/finance/tenants/${encodeURIComponent(params.tenantId)}/imports/preview`,
          body: params,
        }),
      )
    },
    async confirmCSVImport(params) {
      return mapCSVImportConfirmation(
        await request<RawCSVImportConfirmation>({
          method: 'POST',
          path: `/finance/tenants/${encodeURIComponent(params.tenantId)}/imports/${encodeURIComponent(params.importId)}/confirm`,
          body: { mapping: params.mapping },
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
  }
}

export function createSignalFinanceApiForAuth(params: { baseUrl: string; authStore: AuthStore }): SignalFinanceApi {
  return createSignalFinanceApi({ baseUrl: params.baseUrl, fetch: createAuthFetch(params.authStore) })
}

interface RawTenantSummary { id: string; name: string; displayCurrency: string; joinedAt: string; createdAt: string; updatedAt: string }
interface RawTenantMember { tenantId: string; userId: string; joinedAt: string }
interface RawTenantInvite { id: string; tenantId: string; code: string; recipient: string; createdByUserId: string; acceptedByUserId?: string | null; createdAt: string; acceptedAt?: string | null }
interface RawAccount { id: string; tenantId: string; name: string; currency: string; kind: string; provider?: string; providerAccountId?: string; hiddenAt?: string | null; createdAt: string; updatedAt: string }
interface RawCategory { id: string; tenantId: string; name: string; kind: string; seededDefault: boolean; hiddenAt?: string | null; createdAt: string; updatedAt: string }
interface RawTag { id: string; tenantId: string; name: string; hiddenAt?: string | null; createdAt: string; updatedAt: string }
interface RawTransactionProviderOriginal { amountMinor: number; currency: string; description: string; effectiveAt?: string | null }
interface RawTransaction { id: string; tenantId: string; accountId: string; source: string; status: string; kind: string; amountMinor: number; currency: string; description: string; effectiveAt: string; categoryId?: string | null; transferGroupId?: string | null; transferMatchedAt?: string | null; hiddenAt?: string | null; providerOriginal?: RawTransactionProviderOriginal | null; createdAt: string; updatedAt: string }
interface RawConnectionSchedule { connectionId: string; intervalSeconds: number; nextRunAt?: string | null; lastScheduledAt?: string | null; lastStartedAt?: string | null; lastCompletedAt?: string | null; lastJobId?: string; enabled: boolean; createdAt: string; updatedAt: string }
interface RawConnection { id: string; tenantId: string; provider: string; displayName: string; providerReference: string; externalId: string; state: string; lastSyncJobId?: string; lastSyncStartedAt?: string | null; lastSuccessfulSyncAt?: string | null; lastSyncError?: string; createdAt: string; updatedAt: string; schedule?: RawConnectionSchedule | null }
interface RawConnectionRedirectStart { provider: string; authorizationUrl: string; state: string }
interface RawSyntheticLinkStateConfiguredAccount { key: string; name: string; currency: string }
interface RawSyntheticLinkState {
  provider: string
  state: string
  configuredAccounts?: RawSyntheticLinkStateConfiguredAccount[]
  canFinish: boolean
}
interface RawDashboardPeriodWindow { startDate: string; endDate: string }
interface RawDashboardPeriod { preset: string; startDate: string; endDate: string; previous: RawDashboardPeriodWindow; next: RawDashboardPeriodWindow }
interface RawMoneySummary { displayCurrency: string; incomeMinor: number; expenseMinor: number; netMinor: number; transactionCount: number; complete: boolean }
interface RawCategoryBreakdown { categoryId: string; categoryName: string; kind: string; incomeMinor: number; expenseMinor: number; transactionCount: number }
interface RawAccountBalance { accountId: string; accountName: string; currency: string; nativeBookedMinor: number; nativePendingMinor: number; displayBookedMinor?: number | null; displayPendingMinor?: number | null; missingFx: boolean }
interface RawDashboardAlert { code: string; severity: string; count: number }
interface RawMissingFX { source: string; transactionId?: string; accountId?: string; baseCurrency: string; quoteCurrency: string; rateDate: string; provider: string }
interface RawCurrencyTotal { currency: string; incomeMinor: number; expenseMinor: number; netMinor: number }
interface RawDashboard { period: RawDashboardPeriod; settled: RawMoneySummary; pending: RawMoneySummary; categoryBreakdowns?: RawCategoryBreakdown[]; accountBalances?: RawAccountBalance[]; alerts?: RawDashboardAlert[]; missingFx?: RawMissingFX[]; nativeSettledTotals?: RawCurrencyTotal[] }
interface RawFXDiagnostics { defaultProvider: string; storedRatesCount: number; providers?: RawFXProvider[] }
interface RawFXProvider { name: string; default: boolean; ready: boolean }
interface RawCSVRejectedRow { rowNumber: number; reason: string }
interface RawCSVImportPreview { importId: string; importType: string; headers?: string[]; mapping?: Record<string, string>; duplicateRows?: RawCSVRejectedRow[]; rejectedRows?: RawCSVRejectedRow[]; wouldCreateAccounts?: string[]; wouldCreateCategories?: string[]; wouldCreateTags?: string[] }
interface RawCSVImportConfirmation { importId: string; jobId: string; jobType: string }
interface RawCSVImportAudit { importId: string; tenantId: string; importType: string; status: string; jobId: string; confirmedByUserId: string; importedCount: number; createdAt: string; confirmedAt?: string | null; completedAt?: string | null }
interface RawJobRef { jobId: string; jobType: string; provider?: string }

function mapTenant(item: RawTenantSummary): FinanceTenantSummary {
  return { ...item, joinedAt: new Date(item.joinedAt), createdAt: new Date(item.createdAt), updatedAt: new Date(item.updatedAt) }
}
function mapTenantMember(item: RawTenantMember): FinanceTenantMember { return { ...item, joinedAt: new Date(item.joinedAt) } }
function mapTenantInvite(item: RawTenantInvite): FinanceTenantInvite { return { ...item, acceptedByUserId: item.acceptedByUserId ?? null, createdAt: new Date(item.createdAt), acceptedAt: parseOptionalDate(item.acceptedAt) } }
function mapAccount(item: RawAccount): FinanceAccount { return { ...item, provider: item.provider ?? '', providerAccountId: item.providerAccountId ?? '', hiddenAt: parseOptionalDate(item.hiddenAt), createdAt: new Date(item.createdAt), updatedAt: new Date(item.updatedAt) } }
function mapCategory(item: RawCategory): FinanceCategory { return { ...item, hiddenAt: parseOptionalDate(item.hiddenAt), createdAt: new Date(item.createdAt), updatedAt: new Date(item.updatedAt) } }
function mapTag(item: RawTag): FinanceTag { return { ...item, hiddenAt: parseOptionalDate(item.hiddenAt), createdAt: new Date(item.createdAt), updatedAt: new Date(item.updatedAt) } }
function mapTransaction(item: RawTransaction): FinanceTransaction {
  return {
    ...item,
    categoryId: item.categoryId ?? null,
    transferGroupId: item.transferGroupId ?? null,
    transferMatchedAt: parseOptionalDate(item.transferMatchedAt),
    hiddenAt: parseOptionalDate(item.hiddenAt),
    providerOriginal: item.providerOriginal
      ? {
          ...item.providerOriginal,
          effectiveAt: parseOptionalDate(item.providerOriginal.effectiveAt),
        }
      : null,
    effectiveAt: new Date(item.effectiveAt),
    createdAt: new Date(item.createdAt),
    updatedAt: new Date(item.updatedAt),
  }
}
function mapConnection(item: RawConnection): FinanceBankConnection {
  return {
    ...item,
    lastSyncJobId: item.lastSyncJobId ?? '',
    lastSyncStartedAt: parseOptionalDate(item.lastSyncStartedAt),
    lastSuccessfulSyncAt: parseOptionalDate(item.lastSuccessfulSyncAt),
    lastSyncError: item.lastSyncError ?? '',
    createdAt: new Date(item.createdAt),
    updatedAt: new Date(item.updatedAt),
    schedule: item.schedule ? { ...item.schedule, lastJobId: item.schedule.lastJobId ?? '', nextRunAt: parseOptionalDate(item.schedule.nextRunAt), lastScheduledAt: parseOptionalDate(item.schedule.lastScheduledAt), lastStartedAt: parseOptionalDate(item.schedule.lastStartedAt), lastCompletedAt: parseOptionalDate(item.schedule.lastCompletedAt), createdAt: new Date(item.schedule.createdAt), updatedAt: new Date(item.schedule.updatedAt) } : null,
  }
}
function mapConnectionRedirectStart(item: RawConnectionRedirectStart): FinanceConnectionRedirectStart { return item }
function mapSyntheticLinkState(item: RawSyntheticLinkState): FinanceSyntheticLinkState {
  return {
    ...item,
    configuredAccounts: item.configuredAccounts ?? [],
  }
}
function mapDashboard(item: RawDashboard): FinanceDashboard {
  return {
    period: {
      preset: item.period.preset,
      startDate: new Date(item.period.startDate),
      endDate: new Date(item.period.endDate),
      previous: { startDate: new Date(item.period.previous.startDate), endDate: new Date(item.period.previous.endDate) },
      next: { startDate: new Date(item.period.next.startDate), endDate: new Date(item.period.next.endDate) },
    },
    settled: item.settled,
    pending: item.pending,
    categoryBreakdowns: item.categoryBreakdowns ?? [],
    accountBalances: (item.accountBalances ?? []).map((balance) => ({
      ...balance,
      displayBookedMinor: balance.displayBookedMinor ?? null,
      displayPendingMinor: balance.displayPendingMinor ?? null,
    })),
    alerts: item.alerts ?? [],
    missingFx: (item.missingFx ?? []).map((missing) => ({ ...missing, transactionId: missing.transactionId ?? '', accountId: missing.accountId ?? '', rateDate: new Date(missing.rateDate) })),
    nativeSettledTotals: item.nativeSettledTotals ?? [],
  }
}
function mapFXDiagnostics(item: RawFXDiagnostics): FinanceFXDiagnostics { return { defaultProvider: item.defaultProvider, storedRatesCount: item.storedRatesCount, providers: item.providers ?? [] } }
function mapCSVPreview(item: RawCSVImportPreview): FinanceCSVImportPreview {
  return {
    importId: item.importId,
    importType: item.importType,
    headers: item.headers ?? [],
    mapping: item.mapping ?? {},
    duplicateRows: item.duplicateRows ?? [],
    rejectedRows: item.rejectedRows ?? [],
    wouldCreateAccounts: item.wouldCreateAccounts ?? [],
    wouldCreateCategories: item.wouldCreateCategories ?? [],
    wouldCreateTags: item.wouldCreateTags ?? [],
  }
}
function mapCSVImportConfirmation(item: RawCSVImportConfirmation): FinanceCSVImportConfirmation { return item }
function mapCSVImportAudit(item: RawCSVImportAudit): FinanceCSVImportAudit { return { ...item, createdAt: new Date(item.createdAt), confirmedAt: parseOptionalDate(item.confirmedAt), completedAt: parseOptionalDate(item.completedAt) } }
function mapJobRef(item: RawJobRef): FinanceJobRef { return { jobId: item.jobId, jobType: item.jobType, ...(item.provider ? { provider: item.provider } : {}) } }

function parseOptionalDate(value?: string | null): Date | null {
  return value ? new Date(value) : null
}

function extractErrorMessage(response: Response, json: unknown): string {
  if (json && typeof json === 'object' && 'message' in json && typeof json.message === 'string') {
    return json.message
  }
  return response.statusText || 'Request failed'
}

function serializeJson(value: unknown): unknown {
  if (value instanceof Date) {
    return value.toISOString()
  }
  if (Array.isArray(value)) {
    return value.map(serializeJson)
  }
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value).map(([key, entry]) => [key, serializeJson(entry)]))
  }
  return value
}

function buildSearchParams(values: Record<string, string | boolean | undefined>): URLSearchParams {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(values)) {
    if (value === undefined || value === '') {
      continue
    }
    params.set(key, String(value))
  }
  return params
}
