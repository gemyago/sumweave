// const path = require('path');

module.exports = {
  apps: [
    {
      name: 'sumweave-api',
      cwd: './apps/sumweave',
      script: 'go run ./cmd/sumweave start-all --env local --json-logs | pino-pretty',
      interpreter: "none"
    },
    {
      name: 'sumweave-ui',
      script: 'npm run dev -- --host 127.0.0.1 --port 5173',
      interpreter: 'none',
      cwd: './apps/sumweave-ui',
      autorestart: false,
    },
  ],
}
