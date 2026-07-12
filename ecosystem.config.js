// const path = require('path');

module.exports = {
  apps: [
    {
      name: 'signal-foundry-api',
      cwd: './apps/signal-foundry',
      script: 'go run ./cmd/signal-foundry start-all --env local --json-logs | pino-pretty',
      interpreter: "none"
    },
    {
      name: 'signal-foundry-ui',
      script: 'npm run dev -- --host 127.0.0.1 --port 5173',
      interpreter: 'none',
      cwd: './apps/signal-ui',
      autorestart: false,
    },
  ],
}
