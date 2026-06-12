## Task 2 Summary

- What was implemented: replaced the live smoke scaffold with a real read-only Hyperliquid public market-data smoke that ingests the `hyperliquid-perps` BTC instrument, a recent fully closed 1m candle window, and a short recent trade window through the existing Hyperliquid adapter, `venueedge.IngestionFlow`, an ephemeral SQLite `data.DatabaseStore`, and canonical `data.ReadService` readback and replay checks.
- Uncertainties or deviations from the plan: used a fresh public `recentTrades` probe plus a small retry loop to size the short trade window because Hyperliquid's public recent-trades window can move between probe and ingest calls; this keeps the smoke read-only while reducing false failures from venue volatility.
