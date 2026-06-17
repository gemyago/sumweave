---
name: strategy-research-loop
description: Run the bounded alpha loop from data discovery through saved-version evaluation.
---
# Strategy research loop

1. Start with `sf_data_list_candle_availability`, then use bounded candle/evidence reads for the exact scope.
2. Draft one candidate strategy, validate it, and only propose save/create after validation is clean.
3. Evaluate saved ready versions with bounded backtests.
4. Read report and evidence before drawing conclusions.

Safety boundaries:
- No live trading, order placement, wallet actions, or readiness claims.
- Do not bypass validation, saved-version requirements, or failed/missing data checks.
- If evidence is missing or truncated, say so and request the next bounded read.
