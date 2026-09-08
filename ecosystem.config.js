// const path = require('path');

module.exports = {
  apps: [
    {
      name: 'api',
      namespace: 'backend',
      cwd: './apps/sumweave',
      script: 'go run ./cmd/sumweave start --env local --json-logs | pino-pretty',
      interpreter: 'none',
    },
    {
      name: 'worker',
      namespace: 'backend',
      cwd: './apps/sumweave',
      script: 'go run ./cmd/sumweave jobs worker --env local --json-logs | pino-pretty',
      interpreter: 'none',
    },
    {
      name: 'ui',
      script: 'npm run dev -- --host 127.0.0.1 --port 5173',
      interpreter: 'none',
      cwd: './apps/sumweave-ui',
      autorestart: false,
    },
  ],
}
