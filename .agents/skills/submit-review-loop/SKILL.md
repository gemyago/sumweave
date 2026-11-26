---
name: Submit Review Loop
description: Use this skill when user requests to submit a pull request, then wait for a review, address fixes and re-submit.
---

## Steps

The flow is as trivial as this, but there must be an explicit evidence that this is exactly what user wants. Either they explicitly mention this skill or they say follow submit and review loop/process e.t.c. If not sure - do not guess - ask.

So steps are:
- Follow available instructions to submit a pull request
- Do not submit in draft mode, reviews will happen only for non draft pull requests
- Use waiting mechanism available to you, such as sleep, scheduled task e.t.c. If scheduled task is available - prefer it over other methods
- Wait for around 5 minutes, if no signs of review even started - wait for 5 more minutes re-check then stop and notify the user
- If after initial waiting round review results are not yet complete, wait for 5 more (up to 15 minutes in total including initial waiting). Notify the user that review results are not available.
- Once review results are available, use appropriate workflow to analyse findings, fix what's relevant, push back on what's not. Then re-submit the pull request.
- No waiting is required after re-submission
