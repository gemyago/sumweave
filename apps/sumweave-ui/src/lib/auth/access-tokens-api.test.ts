import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest'
import { faker } from '@faker-js/faker'
import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'
import { createAccessTokensApi, type AccessTokenMetadata } from './access-tokens-api'

const server = setupServer()

function tokenFixture(): AccessTokenMetadata {
  return {
    id: faker.string.uuid(), name: faker.word.noun(), hint: `swat_${faker.string.alphanumeric(8)}...`,
    permission: 'read-only', status: 'active', expiresAt: null, revokedAt: null,
    createdAt: faker.date.recent().toISOString(), updatedAt: faker.date.recent().toISOString(),
  }
}

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }))
afterEach(() => server.resetHandlers())
afterAll(() => server.close())

describe('access tokens API', () => {
  it('uses exact lifecycle paths and metadata-only list responses', async () => {
    const token = tokenFixture()
    const api = createAccessTokensApi(fetch)
    let createBody: unknown
    server.use(
      http.get('/api/v1/auth/access-tokens', () => HttpResponse.json({ items: [token] })),
      http.post('/api/v1/auth/access-tokens', async ({ request }) => {
        createBody = await request.json()
        return HttpResponse.json({ token, apiToken: faker.string.alphanumeric(48) }, { status: 201 })
      }),
      http.post(`/api/v1/auth/access-tokens/${token.id}/rotate`, () => HttpResponse.json({ token, apiToken: faker.string.alphanumeric(48) })),
      http.delete(`/api/v1/auth/access-tokens/${token.id}`, () => new HttpResponse(null, { status: 204 })),
    )

    expect(await api.list()).toEqual([token])
    await api.create({ name: token.name, permission: token.permission, expiresAt: null })
    expect(createBody).toEqual({ name: token.name, permission: token.permission, expiresAt: null })
    await api.rotate(token.id, { expiresAt: null })
    await expect(api.revoke(token.id)).resolves.toBeUndefined()
  })

  it('surfaces a recoverable status failure', async () => {
    server.use(http.get('/api/v1/auth/access-tokens', () => HttpResponse.json({}, { status: 500 })))
    await expect(createAccessTokensApi(fetch).list()).rejects.toThrow('Access token request failed: 500')
  })
})
