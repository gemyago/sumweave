import { expect, request, type APIRequestContext } from '@playwright/test';

const apiURL = 'http://127.0.0.1:4501';

export type Credentials = {
  username: string;
  password: string;
};

export function anonymousSession(): Promise<APIRequestContext> {
  return request.newContext({ baseURL: apiURL });
}

export async function signedInSession(credentials: Credentials): Promise<APIRequestContext> {
  const login = await anonymousSession();
  try {
    const response = await login.post('/api/v1/auth/login', { data: credentials });
    expect(response.status(), `API sign-in for ${credentials.username}`).toBe(200);
    const { accessToken }: { accessToken: string } = await response.json();
    expect(typeof accessToken, 'sign-in returned a bearer token').toBe('string');
    expect(Boolean(accessToken), 'bearer token is nonempty').toBe(true);

    return request.newContext({
      baseURL: apiURL,
      extraHTTPHeaders: { Authorization: `Bearer ${accessToken}` },
    });
  } finally {
    await login.dispose();
  }
}
