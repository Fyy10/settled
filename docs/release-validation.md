# First-release validation

Validated on September 15–16, 2026, for implementation tasks 23 and 24.
Task 23 was committed separately as `a634f19` after its own review and acceptance.

## Automated gates

| Command | Result |
| --- | --- |
| `pnpm install --frozen-lockfile` | Passed |
| `pnpm check` | Passed, 0 errors and 0 warnings |
| `pnpm test` | Passed, 68 files and 542 tests |
| `PUBLIC_API_BASE_URL=https://api.settled.example pnpm build` | Passed, static adapter and generated service worker |
| `pnpm audit --audit-level high` | Passed, no moderate/high/critical advisories; one low advisory remains |
| `git diff --check` | Passed |
| Go source check with `gofmt -l` | No unformatted Go files |
| `go vet ./...` in `server/` | Passed |
| `go test -count=1 ./...` in `server/` | Passed |
| `go test -race -count=1 ./...` in `server/` | Passed |
| `PATH="/opt/homebrew/opt/libpq/bin:$PATH" ./scripts/test-integration.sh` in `server/` | Passed against disposable PostgreSQL |
| `docker build --build-arg PUBLIC_API_BASE_URL=https://api.settled.example -t settled-frontend:task24 .` | Passed |
| `docker build -t settled-api:task24 server` | Passed |

No frontend formatter is configured. Changes retain the repository's existing
style and were checked with `git diff --check`; no formatter run is claimed.
The final test teardown waits for Bits UI's deferred body-style restoration
before jsdom disposal. This fixed an intermittent post-suite exception; the
complete suite then passed without unhandled errors.

The PWA dependencies are `@vite-pwa/sveltekit` (static-adapter integration),
`vite-plugin-pwa` (manifest and worker generation), and `workbox-window`
(controlled browser registration). Node types cover the build configuration.
Compatible updates to existing SvelteKit, Svelte, Vite, Vitest, and affected
transitive dependencies removed the audit's higher-severity findings.
The remaining low advisory is `GHSA-pxg6-pf52-xh8x` in SvelteKit's `cookie@0.6.0`.
The deployed frontend contains static files only; session cookies are issued
by the Go API, not SvelteKit cookie serialization.

## Container and artifact checks

Images were built from source with the frozen lockfile, without `--pull`.
The final runtime users are `101:101` for Nginx and `65532:65532` for the API.
An isolated PostgreSQL 17 database received `server/sql/schema.sql` as a
separate step before the API was started. Both health endpoints returned 200.
The frontend and API ran in separate containers behind separate temporary
HTTPS origins; the frontend did not proxy API traffic.

Verified the manifest's name, short name, root ID/start URL/scope, standalone
display, and four PNG icon entries and purposes. ImageMagick confirmed the
192/512 dimensions; the maskable icons have an opaque background. Inspected
the generated worker's 83 precache entries: only prerendered application
shells, fallback, immutable bundles, icons, favicon, and manifest. There is no
server output, API response, `__data.json`, or dynamic `_app/env.js` bootstrap.
The client embeds the same API origin used by the HTML CSP.

Actual Nginx responses confirmed:

- Dynamic routes return the 200 HTML fallback; `/api`, `/api/me`, and missing
  static resources return 404.
- HTML, service worker, and manifest require revalidation; successful immutable
  bundles and hashed Workbox assets have one-year immutable caching.
- Manifest MIME is `application/manifest+json`.
- The CSP response header contains only `frame-ancestors 'none'`; the complete
  hash policy remains in HTML. Frame denial, nosniff, no-referrer, and restrictive
  permissions headers are present, including on errors.

## Real-browser workflows

Chrome DevTools Protocol drove a real isolated Chrome browser against the
production containers and real database. No E2E dependency was added.

| Workflow | Observed result |
| --- | --- |
| Registration/login | Alice's Unicode password and Bob's ASCII password worked. Session and CSRF cookies were HttpOnly, Secure, SameSite=None. Browser storage contained no credentials or drafts. |
| Expired session | Removing the session cookie caused a real 401 and the expected login redirect with encoded `next`, without a loop or retained private UI. |
| CSRF retry | A deliberately invalid token caused one 403, one token refresh and one retry; only one group was created. |
| Create/join | Alice created a group and Bob joined through the code input. The code was not placed in URL or browser storage. |
| Expense splits | A $10.01 equal split persisted 501/500 cents; exact and 60/40 percentage splits persisted 601/400, matching the previews. |
| Expense edit | An equal expense reopened with authoritative exact amounts. Saving updated Activity and Balances. |
| Repayment | Settlement shortcut prefilled direction and amount, with the outside-payment disclaimer. A single POST cleared the authoritative settlement. Task 23 additionally verified $5 → $4 editing changed the remaining balance from $0 to $1. |
| Deletion | Expense and repayment confirmation dialogs each sent one DELETE; Activity and settlement amounts refreshed correctly. |
| Member removal | An unused member could be removed and rejoin. Removal after ledger participation returned 409 with a focused explanation. |
| Owner controls | Bob lacked owner actions and was redirected from direct settings access. Alice renamed and dissolved the group. Both users subsequently saw empty group lists. |
| Direct routes | Group, expense new/edit, repayment new/edit, and settings all passed actual hard reload with HTTP 200, hydration, one h1, and no horizontal overflow. |
| PWA | Manifest parsed without errors; worker installed and activated. Chrome reported no installation-eligibility errors on trusted localhost. See offline/update checks below. |
| Accessibility | Keyboard focus, dialog trap/return, accessible field errors, live announcements, heading order, reduced motion, contrast, 320 CSS px, actual 200% browser zoom, and 44 px primary targets were checked. |

## Offline and update acceptance

- Offline mode retained rendered content and editable in-memory drafts, disabled
  writes with an explanation, and sent no queued request after reconnecting.
  Component tests cover every mutation surface, including auth and owner actions.
- Stopping both temporary HTTPS origins verified a genuine offline deep reload:
  the cached shell booted, showed the offline banner and connection/Retry UI,
  and did not recover private account or ledger data. Merely emulating offline
  on the page target was insufficient because the worker is a separate target.
- Enumerated real CacheStorage: all 83 entries were static shell/assets, with
  no API URL or authenticated JSON.
- Served two different production builds. The new worker remained waiting
  while a repayment draft was dirty, and while a real POST was deliberately
  held pending. Reload became available only after the request completed and
  the form was clean; clicking it activated the new worker and reloaded once.
- First installation claims the existing page without reloading it. Native
  worker events are observed as well as Workbox callbacks so a rapid update
  cannot bypass the dirty/blocking guard.

## Accessibility evidence and environment limits

Chrome's accessibility tree associated the invalid amount textbox with its
label, error description, and invalid state. Tab stayed inside dialogs;
Escape returned focus to the trigger. At 320×280, the dialog scrolled internally
and kept actions reachable. Reduced-motion emulation reduced animation duration.
Actual browser zoom was set to 200% through the isolated profile's settings:
device pixel ratio was 2, layout width halved, and the group page and dialog
remained usable without horizontal overflow. Pinch magnification was checked
separately.

Measured foreground/background, muted text, primary action, and destructive
text contrast in Chrome. The lowest tested ratio was 4.99:1 (dark destructive
text on card); light muted text was 5.09:1. Both exceed normal-text AA.

Safe-area CSS covers all four edges and visual-viewport behavior has unit tests.
The viewport/keyboard checks were emulated, not performed on physical notched
iOS/Android devices. Screen-reader semantics were inspected through Chrome's
accessibility tree, not a physical screen-reader session. Native OS installation
and launcher integration were not exercised by headless Chrome; installation
eligibility was checked on trusted localhost because the disposable self-signed
HTTPS origin is not accepted by Chrome's installability check. The HTTPS workflow
tests used the temporary certificate exception only in the isolated profile.
Real device safe areas, mobile keyboards, and OS launcher behavior remain
platform-specific release checks.

Independent reviews found no remaining high/medium or release-blocking code
findings. Temporary infrastructure is separate from the user's existing services;
only resources created for this acceptance run are removed.
