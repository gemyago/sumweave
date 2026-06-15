const apiCommand =
  'set -eu; ' +
  'existing_pid=$(lsof -tiTCP:4501 -sTCP:LISTEN || true); ' +
  'if [ -n "$existing_pid" ]; then ' +
  'existing_cmd=$(ps -p "$existing_pid" -o command= || true); ' +
  'case "$existing_cmd" in ' +
  '*"signal-foundry start"*) ' +
  'printf "Stopping stale signal-foundry listener on :4501 (pid %s)\\n" "$existing_pid" >&2; ' +
  'kill "$existing_pid"; ' +
  'for _ in 1 2 3 4 5 6 7 8 9 10; do ' +
  'if ! lsof -tiTCP:4501 -sTCP:LISTEN >/dev/null 2>&1; then break; fi; ' +
  'sleep 1; ' +
  'done; ' +
  'if lsof -tiTCP:4501 -sTCP:LISTEN >/dev/null 2>&1; then ' +
  'printf "Port 4501 stayed busy after stopping stale signal-foundry listener.\\n" >&2; ' +
  'exit 1; ' +
  'fi ' +
  ';; ' +
  '*) ' +
  'printf "Port 4501 is already in use by: %s\\n" "$existing_cmd" >&2; ' +
  'exit 1; ' +
  ';; ' +
  'esac; ' +
  'fi; ' +
  'exec direnv exec . env APP_DATADIR=apps/signal-foundry/data APP_DATALAYER_DATABASE_DSN=apps/signal-foundry/data/data-layer.db go run ./apps/signal-foundry/cmd/signal-foundry start --json-logs'

module.exports = {
  apps: [
    {
      name: 'signal-foundry-api',
      script: 'bash',
      args: ['-lc', apiCommand],
      interpreter: 'none',
      cwd: __dirname,
      autorestart: false,
    },
    {
      name: 'signal-foundry-ui',
      script: 'direnv',
      args: 'exec . npm --prefix ./apps/signal-ui run dev -- --host 127.0.0.1 --port 5173',
      interpreter: 'none',
      cwd: __dirname,
      autorestart: false,
    },
  ],
}
