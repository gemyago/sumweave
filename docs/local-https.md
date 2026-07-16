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

## Start the backend

Run this from `apps/signal-foundry` after the usual migration command:

```bash
go run ./cmd/signal-foundry db-migrate --env local
APP_HTTPSERVER_TLS_CERTFILE=../../.local-https/localhost.pem APP_HTTPSERVER_TLS_KEYFILE=../../.local-https/localhost-key.pem go run ./cmd/signal-foundry start-all --env local
```

The backend now listens at `https://localhost:4501`. Both TLS variables are
required; leaving both unset retains the normal HTTP workflow.

## Start the UI

Create `apps/signal-ui/.env.local` with:

```dotenv
VITE_LOCAL_HTTPS=true
VITE_LOCAL_HTTPS_CERT_FILE=../../.local-https/localhost.pem
VITE_LOCAL_HTTPS_KEY_FILE=../../.local-https/localhost-key.pem
VITE_AGENT_API_BASE_URL=/api/v1/runtime/
```

Then run this from `apps/signal-ui`:

```bash
npm run dev -- --host localhost --port 5173
```

Open `https://localhost:5173`. Vite serves HTTPS and proxies `/api/v1` to the
HTTPS backend, so auth, runtime, finance, and other same-origin API calls stay
on HTTPS without CORS configuration. Vite accepts the locally generated
certificate when proxying this development-only connection.
