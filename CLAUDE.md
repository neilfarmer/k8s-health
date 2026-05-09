# CLAUDE.md

Project-specific guidance for AI agents (Claude Code and similar) working in
this repository. Read this before making changes.

## Hard rules

1. **Keep CI green.** Every PR must end with a green pipeline. Don't declare a
   task done while checks are red.
2. **Don't assume — ask.** When the request leaves a meaningful detail
   unspecified (library choice, API shape, scope, security posture), use the
   AskUserQuestion tool before acting.
3. **Follow existing patterns.** Search the repo for similar code, naming, or
   conventions before introducing new ones. Don't create parallel ways to do
   the same thing.

Everything below is elaboration of these three rules.

## Pipeline discipline

After creating or pushing to a PR:

1. Subscribe to PR activity (`mcp__github__subscribe_pr_activity`).
2. Watch for `<github-webhook-activity>` events. For each failure:
   - Read the failing job's logs (use `WebFetch` on the job URL).
   - Decide: tractable mechanical fix, or human-input-required?
3. Push fixes for tractable failures. Stop when:
   - All checks are green, **or**
   - The next step requires a human decision.

**Tractable without asking:** dependency bump, formatting, lint rule fix,
missing import, action version typo, broken assertion in a test you just
wrote, version-mismatch in workflow vs `go.mod`, missing file from a stage.

**Stop and ask:** lowering a gate threshold, broadening a security
allowlist, changing exit-code policy, bypassing hooks (`--no-verify`),
disabling a check, anything touching release/secrets/RBAC, anything that
changes user-visible behavior of the binary or the public schema.

Don't loop on the same failure. If two pushes haven't fixed it, re-read the
logs, look for a different root cause, or ask.

## Asking vs deciding

Use `AskUserQuestion` when any of the following is true:

- Multiple reasonable libraries / approaches exist and you'd be picking one.
- The fix would deviate from a pattern already used in the repo.
- The change affects security posture, public API, on-disk format, or
  release behavior.
- The action is hard to reverse (force-push, delete branch, drop data,
  publish artifact, send external message).
- You'd be inferring intent from one ambiguous sentence.

Decide directly when:

- The fix is mechanical and the right answer is unambiguous from the
  failure.
- The user has previously approved the same kind of change in this session.
- The behavior is documented in `CLAUDE.md`, an ADR, or `docs/`.

When asking, give the user a short menu (2–4 options) with one labelled
**(Recommended)** so they can move quickly.

## Follow existing patterns

Before introducing a new pattern, check (in order):

1. **Files next to your change** — naming, structure, test layout.
2. **`docs/adr/`** — the decisions that shaped current code. If your change
   contradicts an ADR, write a new ADR (or amend the old one) before the
   code change, not after.
3. **`Makefile`** — canonical commands. Don't reinvent what `make build` /
   `make test` / `make cover-check` already do.
4. **`docs/architecture.md`** and **`docs/cli-reference.md`** — the source of
   truth for package layout and CLI surface.
5. **Recent `git log`** — match the commit-message style you see there.

If no pattern exists yet, propose one in a comment or ADR rather than
silently establishing one in code.

## Go code conventions

- New code lives under `internal/`. Promotion to `pkg/` is an explicit
  decision per ADR-0002 — ask first.
- One check per file in `internal/checks/<id>.go`, registering itself via
  `init()` (ADR-0004). Match the pattern of existing checks.
- Tests use `t.Parallel()` and table-driven subtests by default. Look at
  `internal/result/result_test.go` for the pattern.
- Integration tests use the `//go:build integration` tag and live under
  `test/integration/`. They run via `make integration` and the
  `integration` GitHub Actions workflow.
- Default to **no comments**. Add one only when the *why* is non-obvious.
- Don't add error handling for impossible cases. Trust internal callers.
- Don't add features, refactors, or abstractions beyond what the task
  requires.

## Commits and PRs

- One concern per commit. Imperative subject, ≤72 chars.
- Body explains the *why*, not the *what*. Read the most recent few commits
  with `git log --oneline -10` and match their style.
- Never `git push --force` to `main`. For feature branches, prefer creating
  a new commit over amending or force-pushing.
- Never pass `--no-verify` to `git commit` or `git push`. If a hook fails,
  fix the underlying issue.
- Open the PR immediately after the first commit so CI starts running.
- Develop on the branch the user specified — don't push elsewhere without
  permission.

## Gates that must not be silently lowered

These exist on purpose. Touching them requires explicit user approval:

| Gate | Where it lives | Don't |
|------|----------------|-------|
| Unit-test coverage **≥ 80%** | `Makefile` (`COVER_MIN`) and `.github/workflows/ci.yml` | Lower the threshold, drop `-coverpkg=./...`, exclude packages |
| Trivy: HIGH/CRITICAL fixable CVEs fail | `.github/workflows/security.yml` (table step `exit-code: "1"`) | Drop `exit-code`, change severity, drop `ignore-unfixed` |
| `gosec`, `govulncheck`, CodeQL, gitleaks | `.github/workflows/security.yml` | Disable, mark `continue-on-error`, or move out of the required path |
| `golangci-lint` clean | `.github/workflows/ci.yml` | Add per-file ignores without rationale, disable enabled linters |

To allowlist a Trivy finding, add an entry to `.trivyignore.yaml` with `id`,
`statement` (rationale), and `expired_at` — never weaken the gate itself.

## Definition of done

A task is done when **all** of the following are true:

- Local `make ci` is clean (vet, lint, cover-check at 80%).
- All commits are pushed to the agreed branch.
- The PR (if any) has all required checks green, or you have surfaced a
  blocker that needs the user's input.
- Your final user-facing message states what changed in 1–2 sentences and
  flags any open questions.

If you finish work but the pipeline is still running, say so explicitly —
don't claim done before checks complete.

## Anti-patterns

- Adding a parallel lint config / build script / test runner when one
  already exists.
- Creating decision/planning docs (`PLAN.md`, `NOTES.md`) without being
  asked. Use the conversation; promote to an ADR or `docs/` only on
  request.
- "Cleaning up" surrounding code while fixing an unrelated bug.
- Lowering a threshold or relaxing a check just to turn CI green.
- Re-running the same failed command in a loop. Diagnose first.
- Long narration of intent (`Now I'll read the file...`). Output the
  result, not the play-by-play.
- Touching `dist/`, `vendor/`, generated files, or `.github/` actions
  without first checking whether they're owned by another tool.

## When you're stuck

If two attempts haven't fixed an issue, or you're about to do something
irreversible, **stop and ask**. A 30-second clarifying question is cheaper
than a force-push at the wrong moment.
