export type AccessTokenPermission = 'read-only' | 'read-write'
export type AccessTokenStatus = 'active' | 'expired' | 'revoked'

export interface AccessTokenMetadata {
  id: string
  name: string
  hint: string
  permission: AccessTokenPermission
  status: AccessTokenStatus
  expiresAt: string | null
  revokedAt: string | null
  createdAt: string
  updatedAt: string
}

export interface AccessTokenIssuedResponse {
  token: AccessTokenMetadata
  apiToken: string
}

export interface CreateAccessTokenRequest {
  name: string
  permission: AccessTokenPermission
  expiresAt?: string | null
}

export interface RotateAccessTokenRequest {
  expiresAt: string | null
}

export interface AccessTokensApi {
  list(): Promise<AccessTokenMetadata[]>
  create(request: CreateAccessTokenRequest): Promise<AccessTokenIssuedResponse>
  rotate(tokenId: string, request: RotateAccessTokenRequest): Promise<AccessTokenIssuedResponse>
  revoke(tokenId: string): Promise<void>
}

export function createAccessTokensApi(fetcher: typeof fetch): AccessTokensApi {
  async function send<T>(path: string, init?: RequestInit): Promise<T> {
    const response = await fetcher(`/api/v1/auth/access-tokens${path}`, init)
    if (!response.ok) throw new Error(`Access token request failed: ${response.status}`)
    return response.json() as Promise<T>
  }

  return {
    async list() {
      const response = await send<{ items: AccessTokenMetadata[] }>('', { method: 'GET' })
      return response.items
    },
    create(request) {
      return send<AccessTokenIssuedResponse>('', jsonRequest('POST', request))
    },
    rotate(tokenId, request) {
      return send<AccessTokenIssuedResponse>(`/${encodeURIComponent(tokenId)}/rotate`, jsonRequest('POST', request))
    },
    async revoke(tokenId) {
      const response = await fetcher(`/api/v1/auth/access-tokens/${encodeURIComponent(tokenId)}`, { method: 'DELETE' })
      if (!response.ok) throw new Error(`Access token request failed: ${response.status}`)
    },
  }
}

function jsonRequest(method: string, body: object): RequestInit {
  return { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }
}
