# Instruction to rebase onto some target

Use this when the current branch is a **continuation** of work from another branch or PR — a few commits on top of that work, not a full fork from `main`.

Do **not** use plain `git rebase main` here: it replays every commit since the merge-base with `main`, including work already merged (often with different SHAs or a squash merge), which commonly causes large, pointless conflicts.

Instead, replay only commits **after** the target tip onto current `main`:

```bash
git rebase --onto main <target-tip> <current-branch>
```

The target may be a branch name or a PR URL/number. If a PR is given, use `gh` to resolve the target tip (see below).

## The flow

Ensure the repo has no uncommitted changes. If there are, notify the user and wait for further instructions.

If the working tree is clean:

1. Fetch `main`: `git fetch origin main:main`
2. Resolve `<target-tip>` — the commit where the continuation branch started (tip of the base branch/PR at the time you branched off):
   - **Open PR**: `git fetch origin <headRefName>` then use `origin/<headRefName>`
   - **Merged PR**: use the PR head at merge time — `$(gh pr view <n> --json mergeCommit -q .mergeCommit.oid)^2` (second parent of the merge commit). Do not rely on a local copy of the feature branch; it may be stale or deleted on the remote.
   - **Branch only** (no PR): `git fetch origin <branch>` then use `origin/<branch>`, or the commit SHA if the remote branch was deleted
3. Sanity-check what will be replayed: `git log --oneline <target-tip>..<current-branch>` — should list only the continuation commits, not the base PR's history
4. Rebase:
   ```bash
   git rebase --onto main <target-tip> <current-branch>
   ```
5. Verify: `git log --oneline main..<current-branch>` and `git cherry -v main HEAD` — `+` marks commits to keep; `-` means a commit's patch is already on `main` (upstream tip was wrong)
