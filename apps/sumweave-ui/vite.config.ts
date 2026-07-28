import { defineConfig } from 'vitest/config'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import { svelteTesting } from '@testing-library/svelte/vite'
import { readFileSync } from 'node:fs'
import { loadEnv as loadViteEnv } from 'vite'

/**
 * Same-origin prefix for all API calls in dev; covers both runtime (`/api/v1/runtime`) and
 * auth (`/api/v1/auth`) endpoints. Vite forwards requests to the backend as-is.
 */
const API_DEV_PROXY_PREFIX = '/api/v1'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadViteEnv(mode, process.cwd(), '')
  const localHTTPS = env.VITE_LOCAL_HTTPS === 'true'
  const certificateFile = env.VITE_LOCAL_HTTPS_CERT_FILE ?? env.APP_HTTPSERVER_TLS_CERTFILE
  const keyFile = env.VITE_LOCAL_HTTPS_KEY_FILE ?? env.APP_HTTPSERVER_TLS_KEYFILE
  if (localHTTPS && (!certificateFile || !keyFile)) {
    throw new Error(
      'VITE_LOCAL_HTTPS requires VITE_LOCAL_HTTPS_CERT_FILE and VITE_LOCAL_HTTPS_KEY_FILE, or APP_HTTPSERVER_TLS_CERTFILE and APP_HTTPSERVER_TLS_KEYFILE',
    )
  }

  return {
    plugins: [svelte(), svelteTesting()],
    server: {
      https: localHTTPS ? { cert: readFileSync(certificateFile), key: readFileSync(keyFile) } : undefined,
      proxy: {
        [API_DEV_PROXY_PREFIX]: {
          target: localHTTPS ? 'https://localhost:4501' : 'http://127.0.0.1:4501',
          changeOrigin: true,
          secure: !localHTTPS,
        },
      },
    },
    test: {
      environment: 'jsdom',
      // Keep browser-local datetime tests deterministic, including the DST fall-back overlap.
      env: { TZ: 'America/New_York' },
      setupFiles: ['./src/vitest-setup.ts'],
      include: ['src/**/*.{test,spec}.{js,ts}'],
      coverage: {
        provider: 'v8',
        reportsDirectory: 'coverage',
        reporter: ['text', 'html', 'lcov'],
        include: ['src/**/*.{js,ts,svelte}'],
        exclude: [
          '**/*.test.{js,ts}',
          '**/*.spec.{js,ts}',
          '**/*.d.ts',
          '**/vitest-setup.ts',
          // Type-only module; no executable code
          '**/lib/agentapi/types.ts',
          // Generated from OpenAPI; exercised via client tests, not this file
          '**/lib/agentapi/agentapi.generated.ts',
        ],
        // Global minimums on the "All files" row (`vitest run` / `make test`). Bump when coverage improves.
        // AI, if you're reading this, you MUST never downgrade this. You must find way to always increase this.
        thresholds: {
          statements: 80,
          branches: 80,
          functions: 80,
          lines: 80,
        },
      },
    },
  }
})
