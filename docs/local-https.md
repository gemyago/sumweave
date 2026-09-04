# Local HTTPS

Use this optional workflow when the backend and Vite development server must both
run on HTTPS. It uses a locally trusted `mkcert` certificate; no certificate or
private key is stored in the repository.

## One-time certificate setup

From the repository root, install `mkcert` using your system package manager,
then run:

```bash
mkcert -install
mkdir -p .local-https
mkcert -cert-file .local-https/localhost.pem -key-file .local-https/localhost-key.pem localhost 127.0.0.1 ::1
```

`.local-https/` is ignored by Git. Keep its key private.

## Configure local TLS paths

Create or update the ignored root `.envrc.local` with paths that are valid from
both PM2 application working directories:

```dotenv
export APP_HTTPSERVER_TLS_CERTFILE=../../.local-https/localhost.pem
export APP_HTTPSERVER_TLS_KEYFILE=../../.local-https/localhost-key.pem
```

The backend automatically consumes these values. Vite uses them as its
certificate-path fallback, but Vite HTTPS remains separately opt-in below.

## Start with PM2

Prepare the required local PostgreSQL schemas from the repository root, then
create or start the PM2 applications:

```bash
make postgres-bootstrap
pm2 start ecosystem.config.js
```

Set Vite's explicit HTTPS enablement in `apps/sumweave-ui/.env.local`:

```dotenv
VITE_LOCAL_HTTPS=true
VITE_AGENT_API_BASE_URL=/api/v1/runtime/
```

`VITE_LOCAL_HTTPS` intentionally does not infer HTTPS from the backend TLS
paths. Add the optional `VITE_LOCAL_HTTPS_CERT_FILE` and
`VITE_LOCAL_HTTPS_KEY_FILE` values only when Vite needs paths different from
the shared `APP_` values.

Open `https://localhost:5173`. Vite serves HTTPS and proxies `/api/v1` to the
HTTPS backend, so auth, runtime, finance, and other same-origin API calls stay
on HTTPS without CORS configuration. Vite accepts the locally generated
certificate when proxying this development-only connection.

Use `pm2 status` and `pm2 logs` to inspect the processes. If the backend
ecosystem command or arguments change, recreate it with
`pm2 delete backend && pm2 start ecosystem.config.js`.

## Return to the standard local HTTP workflow

HTTPS is opt-in. To return both PM2 services to the normal HTTP endpoints,
remove the two `APP_HTTPSERVER_TLS_*` exports from the ignored root
`.envrc.local` and remove `VITE_LOCAL_HTTPS` plus its optional certificate-path
variables from the ignored `apps/sumweave-ui/.env.local`. Keep
`VITE_AGENT_API_BASE_URL=/api/v1/runtime/` so Vite continues to proxy the
same-origin API calls.

Then prepare the PostgreSQL schemas and recreate both PM2 applications from the
repository root so neither process retains an old TLS environment:

```bash
make postgres-bootstrap
pm2 delete backend
pm2 delete ui
pm2 start ecosystem.config.js
pm2 status
```

Verify the resulting HTTP services before opening the browser:

```bash
curl -i http://127.0.0.1:4501/health
curl -I http://127.0.0.1:5173/
```

The expected endpoints are `http://127.0.0.1:4501` for the API and
`http://127.0.0.1:5173` for Vite. Do not use this reset while an HTTPS-only
local test is in progress.

## Direct-start diagnostics only

Direct startup is only for isolating a local problem, not normal development.
With the same environment loaded, run `go run ./cmd/sumweave start-all
--env local` from `apps/sumweave` and `npm run dev -- --host localhost
--port 5173` from `apps/sumweave-ui` in separate terminals.
