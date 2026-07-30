# Settled SvelteKit Frontend Implementation Design

This document defines the implementation-level design for the first-release
Settled frontend. It translates the product, architecture, entity, API, and
backend decisions in `00_project.md` through `04_backend.md` into concrete
SvelteKit routes, UI flows, state boundaries, API-client behavior, component
composition, PWA behavior, accessibility requirements, and validation strategy.

`03_API.md` remains the authority for the public HTTP contract. `04_backend.md`
remains the authority for backend security and implementation behavior. When
this document describes a frontend convenience such as hiding an owner action,
the backend remains the authorization source of truth.

## Resolved Decisions

The following decisions are fixed by the preceding design documents:

- The frontend is a static, client-rendered SvelteKit application; SSR is
  disabled.
- The browser calls a separately deployed Go API with
  `credentials: "include"`.
- Authentication uses an HttpOnly session cookie. JavaScript never stores or
  decodes the session JWT.
- Unsafe requests use the session-bound CSRF token from the API.
- Authenticated API data remains the source of truth. Offline mutations are not
  supported.
- No broad client state library, form framework, schema-validation library, or
  data-fetching library is introduced.
- The UI follows the shadcn-svelte aliases and semantic tokens configured in
  `components.json`.
- The first release is USD-only and uses pairwise settlement results returned by
  the backend.
- The first-release UI language is English. User-visible product copy, field
  labels, validation summaries, empty states, accessibility labels, manifest
  text, and document titles are English.
- Use `en-US` as the initial presentation locale for USD amounts and dates. API
  dates and request values retain their contract-defined formats regardless of
  display locale.
- User-visible copy remains centralized so terminology stays consistent and a
  future localization effort does not require changing workflows or component
  structure. Localization infrastructure is not added in the first release.

## Design Goals

- Let a member capture an ordinary shared expense accurately in under one
  minute on a phone.
- Make the answer to “who should pay whom?” the most legible information in a
  group.
- Keep accounting actions calm and explicit: no celebratory finance graphics,
  gamification, or payment-completion claims.
- Prevent invalid splits before submission while treating the backend response
  as authoritative.
- Keep navigation shallow and predictable across phone and desktop widths.
- Make loading, empty, offline, expired-session, and partial-failure states
  actionable rather than vague.
- Keep implementation direct: typed API modules, route-local data, focused
  shared state, and composable shadcn-svelte primitives.
- Preserve privacy by never persisting session material, CSRF tokens, group
  data, or draft accounting records in web storage.

## Explicit Non-Goals

- Marketing pages, public group discovery, social feeds, or public profiles.
- Server-side rendering or server-only SvelteKit actions.
- Client-authoritative authorization, balance calculation, or settlement
  calculation.
- Bearer tokens, localStorage sessions, or frontend JWT parsing.
- Offline expense, repayment, or group mutation queues.
- Optimistic accounting mutations that display unconfirmed financial data as
  saved.
- Multiple currencies, currency conversion, payment initiation, or payment
  completion tracking.
- Pagination, list virtualization, charts, or global minimum-transfer
  visualization in the first release.
- OAuth, password reset, email verification, two-factor authentication, owner
  transfer, join-code rotation, archives, or audit history.
- A global state framework, API SDK generator, form framework, or schema
  validation framework.

## Product Experience Model

Settled is a private shared ledger, not a banking product. Its core object is a
small trusted group, and its primary loop is:

```text
open group
  -> understand current balances
  -> add expense or record repayment
  -> see refreshed balances
```

The UI uses these user-facing concepts consistently:

| Domain concept | UI term | Meaning shown to users |
| --- | --- | --- |
| Group | Group | A private shared bill space |
| Expense | Expense | Something one member paid for selected people |
| Expense split | Split | How the expense amount is assigned |
| Repayment | Payment record | A manual record of money paid outside Settled |
| Settlement transfer | Balance | A current suggestion that one member should pay another |
| Join code | Group code | A private code used to join a group |
| Dissolve group | Dissolve group | Hide and stop all activity in the group |

The repayment screen must state that it records a payment made elsewhere.
Buttons and success messages use the same verbs:

- `Add expense` -> `Expense added`
- `Save changes` -> `Changes saved`
- `Record payment` -> `Payment recorded`
- `Delete expense` -> `Expense deleted`
- `Dissolve group` -> `Group dissolved`

The interface must never say that Settled “sent,” “completed,” or “verified” a
payment.

## Visual Direction

### Subject, Audience, And Screen Job

- **Subject:** a living shared ledger for friends.
- **Audience:** small trusted groups settling trips, meals, and household costs,
  often while together and using one hand on a phone.
- **Primary screen job:** make current obligations obvious, then make the next
  accounting action quick and unambiguous.

### Direction: The Quiet Ledger

The visual language takes its structure from a hand-kept ledger without
imitating paper, receipts, or bookkeeping software. Rows align, money uses
tabular numerals, dates act as real grouping labels, and pairwise obligations
are shown as direct relationships between people. Surfaces remain quiet so
amounts, direction, and action state carry the emphasis.

The distinctive element is the **settlement line**:

```text
[BO] Bob  -------------------->  Alice [AL]     $20.00
          should pay                         Record payment
```

On desktop it is a restrained horizontal relationship row. On narrow screens it
becomes a two-line directional sentence:

```text
Bob should pay Alice
$20.00                         Record payment
```

This is deliberately not a generic KPI card or chart. It encodes the actual
pairwise model and remains valid if the backend later changes how it produces
the same transfer response shape.

The single aesthetic risk is using the directional settlement line as the
product's signature motif. Everything else stays disciplined: no gradients,
decorative illustrations, glass effects, or multiple competing accent colors.

### Color And Tokens

Implementation uses semantic tokens from `src/routes/layout.css`, never raw
Tailwind color utilities inside components. The initial slate token set is
functional but generic; the implementation should tune the existing variables
in place to the following calm, ink-and-mint direction:

| Named role | Reference hex | Semantic use |
| --- | --- | --- |
| Ledger ink | `#17211D` | `--foreground`, primary action in light mode |
| Paper white | `#FAFCFA` | `--background` |
| Soft ledger | `#F0F4F1` | `--muted`, `--secondary`, quiet row surfaces |
| Rule line | `#D9E2DC` | `--border`, `--input` |
| Settled mint | `#2F6F58` | `--primary`, focus and positive action |
| Correction red | `#B9473F` | `--destructive` only |

Convert these reference colors to OKLCH when updating the existing variables.
Preserve accessible contrast in both light and dark themes. Do not create
direction-specific red/green debt colors: direction is communicated with names,
arrows, labels, and order rather than color alone.

Dark-mode tokens may remain present, but a user-facing theme switch is not
required for the first release. If the operating-system preference is honored,
it must happen without adding a theme dependency solely for this purpose.

### Typography

Use a local system stack to avoid an external font service and to keep the PWA
fast and private:

```css
font-family:
  Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont,
  "Segoe UI", "Noto Sans", "Noto Sans CJK SC", sans-serif;
```

Inter is used only if supplied locally or already present on the device; the
application must not fetch it from a third-party CDN. The fallback stack retains
broad Unicode coverage for member names and user-entered descriptions even
though application copy is English.

Roles:

- Page titles: 28/34 desktop, 24/30 mobile, weight 650–700.
- Section titles: 18/26, weight 600.
- Body and controls: 14/21 or 16/24 where touch input benefits from the larger
  size.
- Labels and metadata: 12/18 or 13/18, weight 500.
- Amounts, dates, codes, and percentages: tabular numerals with
  `font-variant-numeric: tabular-nums`.

Display names may wrap to two lines where needed. Emails, join codes, and
single-line navigation labels truncate with an accessible full value available
through surrounding context or a tooltip where appropriate.

### Spacing, Radius, And Elevation

- Base spacing rhythm: 4 px, with primary gaps of 8, 12, 16, 24, and 32 px.
- Interactive controls: at least 40 px high; primary mobile actions are 44 px or
  taller.
- Content width: `max-w-6xl` for the app shell, `max-w-2xl` for forms, and
  `max-w-md` for authentication.
- Keep the configured 10 px base radius; do not use fully rounded cards.
- Use borders and surface contrast before shadows. Reserve a subtle shadow for
  popovers, dialogs, and the sticky mobile action area.
- Use `Separator` for semantic section separation, not decorative border-only
  markup.

### Motion

- Use one 120–180 ms opacity/translate transition when a dialog, menu, or
  validation message appears.
- Do not animate monetary values, reorder activity with motion, or use ambient
  animation.
- Loading uses `Skeleton` or `Spinner`; it does not bounce layout.
- Honor `prefers-reduced-motion: reduce` and disable nonessential transitions.

## Information Architecture

### Route Map

Use SvelteKit route groups to separate public authentication screens from the
authenticated application without changing URLs:

```text
src/routes/
  +layout.svelte
  +layout.ts
  +page.ts
  +error.svelte
  (auth)/
    +layout.svelte
    login/
      +page.svelte
    register/
      +page.svelte
  (app)/
    +layout.svelte
    +layout.ts
    groups/
      +page.svelte
      [groupId]/
        +layout.svelte
        +layout.ts
        +page.svelte
        expenses/
          new/
            +page.svelte
          [expenseId]/
            edit/
              +page.svelte
        repayments/
          new/
            +page.svelte
          [repaymentId]/
            edit/
              +page.svelte
        settings/
          +page.svelte
```

Route behavior:

- `/` checks the current session and replaces history with `/groups` or
  `/login`.
- `/login` and `/register` redirect authenticated users to `/groups`.
- Every `(app)` route verifies the current session through `GET /api/me`.
- `/groups` is the first authenticated destination.
- `/groups/[groupId]` is the group workspace with `Balances`, `Activity`, and
  `Members` sections. The default view is `Balances`.
- Create and edit accounting workflows use full routes, not dialogs. This makes
  multi-step split input stable on small screens and gives refresh/back-button
  behavior a clear URL.
- Group create and join are compact dialogs on `/groups`.
- Group settings is a separate owner-only route. Non-owners do not see the
  navigation item, and a direct visit returns to the group after the API proves
  the role is insufficient.

The selected group section is encoded in the URL query parameter:

```text
/groups/{groupId}?view=balances
/groups/{groupId}?view=activity
/groups/{groupId}?view=members
```

Allowed values are `balances`, `activity`, and `members`. Missing or invalid
values normalize to `balances`. This preserves a single group data boundary
without creating fake resource routes.

### Navigation Model

Desktop:

```text
+--------------------------------------------------------------+
| Settled                                      Alice ▾          |
+--------------------------------------------------------------+
| Groups / Lake Trip                                            |
| Lake Trip                               + Add expense          |
| 3 members                             Record payment           |
| [Balances] [Activity] [Members]                     [•••]      |
+--------------------------------------------------------------+
| active view                                                  |
+--------------------------------------------------------------+
```

Mobile:

```text
+-------------------------------+
| ‹ Groups       Lake Trip   ••• |
| [Balances] [Activity] [Members]|
+-------------------------------+
| active view                   |
|                               |
+-------------------------------+
| + Add expense   Record payment|
+-------------------------------+
```

- The global shell uses a compact top bar, not a persistent sidebar. The first
  release has one primary collection and does not justify sidebar complexity.
- The group tabs remain visible near the top; on very narrow screens the tab
  list scrolls horizontally without clipping focus rings.
- Mobile accounting actions sit in a safe-area-aware sticky action bar. Desktop
  actions sit beside the group title.
- Back navigation from a form returns to the originating group view. Successful
  accounting mutations replace the form route with the group activity or
  balances view so browser Back does not reopen a completed submission.

## SvelteKit Runtime And Build Design

### Client-Only Static Application

The root `+layout.ts` exports:

```ts
export const ssr = false;
export const prerender = true;
```

Use `@sveltejs/adapter-static` with an SPA fallback such as `index.html`. Static
hosting must rewrite unknown application paths to that fallback so direct visits
to `/groups/{groupId}/expenses/new` work.

`adapter-auto` cannot be the final production adapter because the architecture
requires predictable static output. Replacing it with `adapter-static` is a
stack-consistent build change, not a change to the application architecture.

### Runtime Configuration

Define one public frontend variable:

```text
PUBLIC_API_BASE_URL
```

Rules:

- It is the absolute API origin with no trailing slash, for example
  `https://api.settled.example`.
- Local default is `http://localhost:8080`.
- Production builds require an HTTPS value.
- API paths are appended as `/api/...`.
- The value is public and contains no secret.
- The frontend origin must independently appear in the backend
  `ALLOWED_ORIGINS`.

Because SvelteKit public environment variables are embedded into a static build,
the first release produces an environment-specific frontend artifact. A runtime
`config.json` indirection is not introduced unless one identical artifact must
later serve multiple API origins.

### Root Layout Responsibilities

`src/routes/+layout.svelte` owns only application-global browser UI:

- import `layout.css`;
- favicon and PWA metadata;
- root document title template;
- one shadcn-svelte `Toaster`;
- a polite global route-loading announcement if navigation exceeds 300 ms;
- network online/offline observation;
- rendering the current route.

It does not fetch group data or contain route-specific navigation.

### Page Titles

Use meaningful titles:

| Route | Title |
| --- | --- |
| `/login` | `Log in · Settled` |
| `/register` | `Create account · Settled` |
| `/groups` | `Groups · Settled` |
| Group route | `{group name} · Settled` |
| New expense | `Add expense · {group name} · Settled` |
| Edit expense | `Edit expense · {group name} · Settled` |
| New repayment | `Record payment · {group name} · Settled` |
| Settings | `Settings · {group name} · Settled` |

## Source Organization

Use the following controlled frontend layout:

```text
src/
  lib/
    api/
      client.ts
      auth.ts
      groups.ts
      expenses.ts
      repayments.ts
      settlements.ts
      types.ts
    components/
      app/
        app-header.svelte
        group-header.svelte
        member-avatar.svelte
        money-amount.svelte
        route-state.svelte
      expenses/
        expense-form.svelte
        expense-row.svelte
        split-editor.svelte
      groups/
        create-group-dialog.svelte
        join-group-dialog.svelte
        member-list.svelte
      repayments/
        repayment-form.svelte
        repayment-row.svelte
      settlements/
        settlement-row.svelte
      ui/
        ...
    state/
      auth.svelte.ts
      csrf.ts
      network.svelte.ts
    utils/
      basic-auth.ts
      dates.ts
      money.ts
      names.ts
    copy/
      en.ts
  routes/
    ...
```

Boundaries:

- `api/` knows HTTP paths and wire DTOs but no Svelte components.
- `state/` contains only state shared across unrelated routes: current user,
  CSRF token, and online status.
- Route data stays in route load results or page/component state. There is no
  global groups/expenses/repayments cache.
- Domain components receive typed props and emit intent callbacks. They do not
  navigate or construct API URLs unless the component is explicitly a route
  form.
- `utils/` contains pure, independently testable conversion and display helpers.
- `copy/en.ts` centralizes user-visible English strings and vocabulary. It is a
  small typed object, not an internationalization framework.

Do not add generic repositories, services, hooks, stores, or barrel files until
there are multiple real consumers. Keep files named for the workflow they own.

## API Types

`src/lib/api/types.ts` mirrors `03_API.md` exactly:

```ts
export type User = {
  id: string;
  email: string;
  displayName: string;
  createdAt: string;
  updatedAt: string;
};

export type GroupRole = "owner" | "member";

export type GroupSummary = {
  id: string;
  name: string;
  ownerUserId: string;
  memberCount: number;
  currentUserRole: GroupRole;
  createdAt: string;
  updatedAt: string;
};

export type GroupMember = {
  userId: string;
  email: string;
  displayName: string;
  role: GroupRole;
  joinedAt: string;
};

export type MemberSummary = {
  userId: string;
  displayName: string;
};

export type ExpenseSplit = {
  userId: string;
  amountCents: number;
};

export type Expense = {
  id: string;
  groupId: string;
  paidByUserId: string;
  description: string;
  amountCents: number;
  currency: "USD";
  expenseDate: string;
  createdByUserId: string;
  splits: ExpenseSplit[];
  createdAt: string;
  updatedAt: string;
};

export type Repayment = {
  id: string;
  groupId: string;
  fromUserId: string;
  toUserId: string;
  amountCents: number;
  currency: "USD";
  note: string | null;
  repaymentDate: string;
  createdByUserId: string;
  createdAt: string;
  updatedAt: string;
};

export type Settlement = {
  fromUserId: string;
  toUserId: string;
  amountCents: number;
  currency: "USD";
};

export type ApiErrorBody = {
  error: {
    code: string;
    message: string;
    fields?: Record<string, string>;
  };
};
```

JavaScript numbers can exactly represent all integer cents up to
`Number.MAX_SAFE_INTEGER`, while the API's Go/PostgreSQL types allow larger
`int64` values. The frontend must reject locally parsed amounts beyond
`Number.MAX_SAFE_INTEGER` and treat any larger response as an invalid server
payload rather than silently losing precision. The backend remains responsible
for the full `int64` boundary.

Request input types are explicit discriminated unions:

```ts
type EqualExpenseInput = ExpenseBaseInput & {
  splitMode: "equal";
  participantUserIds: string[];
};

type ExactExpenseInput = ExpenseBaseInput & {
  splitMode: "exact";
  splits: ExpenseSplit[];
};

type PercentageExpenseInput = ExpenseBaseInput & {
  splitMode: "percentage";
  percentageSplits: Array<{
    userId: string;
    percentageBasisPoints: number;
  }>;
};

export type ExpenseInput =
  | EqualExpenseInput
  | ExactExpenseInput
  | PercentageExpenseInput;
```

Never send `currency`, `createdByUserId`, timestamps, or persisted split output
fields in mutation requests unless `03_API.md` requires them.

## HTTP Client Design

### Shared Request Function

`api/client.ts` exposes a narrow typed request helper:

```ts
request<T>(
  path: string,
  options?: {
    method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
    body?: unknown;
    signal?: AbortSignal;
    csrf?: boolean;
    retryCsrf?: boolean;
  }
): Promise<T>
```

Every request:

- builds an absolute URL from `PUBLIC_API_BASE_URL`;
- sends `Accept: application/json`;
- sets `credentials: "include"`;
- sets `Content-Type: application/json` only when a JSON body exists;
- attaches `X-CSRF-Token` only for unsafe requests;
- passes an `AbortSignal` from the route or component where available;
- accepts an empty body for `204`;
- parses JSON only when the response content type is JSON;
- converts non-2xx responses into a typed `ApiError`;
- treats malformed success JSON or an unexpected response shape as an
  application error.

Do not log request bodies, `Authorization`, CSRF tokens, group codes, cookies, or
passwords. User-presentable errors use the API's stable summary and field map;
diagnostic browser logging is limited to non-sensitive status, code, route name,
and request ID when returned by the server.

### API Error Class

`ApiError` contains:

```ts
type ApiErrorDetails = {
  status: number;
  code: string;
  message: string;
  fields: Record<string, string>;
  requestId?: string;
};
```

Handling rules:

| Status/code | Frontend behavior |
| --- | --- |
| `400 bad_request` | Form-level alert; do not guess a field |
| `401 unauthorized` | Clear in-memory auth/CSRF state and redirect to login |
| `403 csrf_required` / `csrf_invalid` | Refresh CSRF and retry once |
| Other `403` | Explain that the action is no longer available; refresh relevant data |
| `404` on group route | Show private not-found state with link to groups |
| `404` after an open edit form | Explain record/group is unavailable; return to group |
| `409` | Show workflow-specific conflict copy |
| `422 validation_failed` | Map known fields; show unknown fields in form alert |
| `500` or invalid response | Keep draft in memory and offer Retry |
| Network failure | Show offline/unreachable state; never imply the mutation saved |

The client retries only CSRF errors and only once. It does not automatically
retry timeouts, `500`, create requests, or network failures because the backend
does not expose mutation idempotency keys.

### CSRF State

The CSRF token lives only in module memory. It is never placed in localStorage,
sessionStorage, IndexedDB, a URL, or a JavaScript-readable cookie.

`getCsrfToken({ force?: boolean })`:

- returns the cached token when present and not forced;
- coalesces simultaneous refresh calls into one in-flight promise;
- calls `GET /api/auth/csrf` when absent or forced;
- caches the returned token in memory;
- clears the in-flight promise after success or failure.

Unsafe request flow:

```text
need unsafe request
  -> obtain current CSRF token
  -> send request with X-CSRF-Token
  -> csrf_required / csrf_invalid?
       yes -> force refresh -> retry original request exactly once
       no  -> return result or surface error
```

Registration and login responses rotate CSRF and return the new token; store it
immediately. Logout clears the authenticated token because the backend rotates
to an anonymous binding without returning its token.

### Login Basic Authentication

The login endpoint has no JSON body and uses HTTP Basic Auth. Email and password
must be encoded as UTF-8 bytes before Base64 encoding. Do not call `btoa()` on
the raw JavaScript string, because passwords may contain valid non-ASCII UTF-8.

`utils/basic-auth.ts`:

1. reject an email containing `:` before constructing credentials;
2. concatenate `${email}:${password}` without trimming the password;
3. encode with `TextEncoder`;
4. convert bytes to a binary string in bounded chunks;
5. Base64-encode that binary string;
6. return `Basic ${encoded}`.

The temporary encoded value is function-local and is not stored or logged.

### Authentication State And Guards

`auth.svelte.ts` holds:

```ts
type AuthState =
  | { status: "unknown"; user: null }
  | { status: "authenticated"; user: User }
  | { status: "anonymous"; user: null };
```

It exposes `ensureSession()`, `setAuthenticated(user)`, and `clearSession()`.
`ensureSession()` coalesces concurrent `GET /api/me` calls. The state is only a
rendering cache; `GET /api/me` remains authoritative.

Guard behavior:

- Root and auth routes call `ensureSession()`.
- App routes render a stable shell skeleton while status is `unknown`.
- Authenticated state allows app content.
- Anonymous state redirects with `replaceState` to `/login`.
- A `401` from any later request clears state and redirects to
  `/login?reason=session-expired&next=<safe-local-path>`.
- Login validates `next` as a local path beginning with one `/` and not `//`.
  Invalid values fall back to `/groups`.
- Logout first disables the menu action, calls the API, then clears state and
  replaces history with `/login`. If logout fails due to network, the current
  authenticated UI remains and explains that sign-out could not be completed.

Do not infer authentication from the presence of a cookie because it is
HttpOnly and may be expired.

## Data Loading And Mutation Policy

### Route Data

- App layout loads only the current user.
- Group list loads only `GET /api/groups`.
- Group layout loads `GET /api/groups/{groupId}` once and shares the group and
  members with child routes.
- Group overview loads expenses, repayments, and settlements in parallel.
- Edit routes load the target expense or repayment in parallel with parent group
  data.
- A route aborts obsolete requests during navigation using `AbortController`.
- Lists remain unpaginated because the API is unpaginated.

Group overview treats its three workflow panels independently. If settlements
load but activity fails, balances remain visible and activity shows a local
retry state. A single failed endpoint does not replace the whole group screen
unless the error establishes that the group is hidden or the session expired.

### Revalidation

Use small explicit refresh functions rather than a generic query cache:

- Group create/join/dissolve refreshes the group list.
- Expense create/edit/delete refreshes expenses and settlements.
- Repayment create/edit/delete refreshes repayments and settlements.
- Group rename refreshes group detail and group list.
- Member removal refreshes group detail, expenses, repayments, and settlements.

Accounting mutations wait for the server response before showing success.
Controls are disabled while the request is in flight. On success, navigate or
refresh and show one concise toast. On failure, retain entered form values in
memory.

No accounting list uses optimistic inserts, edits, or deletes. Correctness and
clear confirmation matter more than hiding ordinary API latency.

### Derived View Models

Client derivation is allowed only for presentation:

- Map `userId` to display name/avatar initials.
- Merge already-loaded expenses and repayments into a chronological activity
  view.
- Format cents as USD.
- Calculate live draft split previews before submission.
- Determine whether the current user appears as sender or recipient for copy.

The frontend must not produce an authoritative settlement result from activity.
The balances panel renders only `GET /settlements`.

## Money, Percentage, And Date Handling

### Money Input

The user edits a decimal USD string such as `54`, `54.5`, or `54.00`. Never
parse money with floating-point multiplication.

`parseUsdToCents(value)`:

- trims ordinary surrounding whitespace;
- accepts digits with an optional decimal point and one or two fractional
  digits;
- rejects signs, commas, exponent notation, currency symbols, and more than two
  decimals;
- constructs cents from the whole and padded fractional digit strings;
- rejects zero, overflow, and values beyond `Number.MAX_SAFE_INTEGER`;
- returns a field error rather than coercing.

Use `inputmode="decimal"` and `autocomplete="off"`. Display a fixed `$` addon
with shadcn-svelte `InputGroup`; do not put the symbol inside the editable
value.

Format API cents with one shared `Intl.NumberFormat`:

```ts
new Intl.NumberFormat("en-US", {
  style: "currency",
  currency: "USD"
});
```

Amounts use tabular numerals. Do not manually concatenate `$` for display.

### Split Preview

The frontend mirrors backend calculations only to give immediate feedback:

- Equal: integer division and remainder by selected participant order.
- Exact: every share is positive and the sum must equal the expense amount.
- Percentage: positive basis points totaling `10000`; floor each share and
  distribute remaining cents in participant order.

The preview and request order must use the visible member order: owner first,
then case-insensitive display name, then user ID. The backend response is final.
After save, render persisted exact splits rather than retaining the draft mode,
because the API does not persist split mode or percentage inputs.

Equal split rejects an expense whose cents are fewer than selected participants,
because it would create a zero-cent persisted split. Percentage mode likewise
surfaces zero-cent calculated shares before submission.

Percentage fields accept at most two decimal places and convert them to integer
basis points without floating-point math. The total indicator displays both
`100%` target and current value.

### Dates

- API dates remain local calendar strings in `YYYY-MM-DD`; never parse them with
  `new Date("YYYY-MM-DD")` for display because timezone conversion can shift the
  day.
- Default new expense and repayment date to the user's current local calendar
  date.
- Validate strict calendar shape and use a native date input unless the
  installed shadcn-svelte date picker proves materially more accessible.
- RFC 3339 timestamps are parsed only for metadata and ordering ties.
- Activity groups by calendar date from `expenseDate` or `repaymentDate`, then
  uses `createdAt` and ID for stable ordering consistent with API contracts.

## Screen Designs

### Root Bootstrap

`/` shows the application mark and a compact spinner while `GET /api/me`
resolves. It immediately replaces history with:

- `/groups` when authenticated;
- `/login` when unauthenticated.

If the API is unreachable, show:

- `Settled can’t reach the server.`
- `Check your connection and try again.`
- `Retry`

Do not redirect a network failure to login; lack of connectivity does not prove
the session is invalid.

### Login

Layout:

```text
Settled
Shared bills, made clear.

[ Email                              ]
[ Password                       👁  ]
[ Log in                              ]

New to Settled? Create account
```

Composition:

- `Card` with full header/content/footer composition.
- `Field.FieldGroup` and one `Field.Field` per input.
- `Input` for email and `InputGroup` for password visibility.
- `Alert` for invalid credentials or service failure.
- `Button` with `Spinner` while submitting.

Behavior:

- Fetch anonymous CSRF lazily on first submit; optionally prefetch after the
  page becomes idle.
- Email input uses `type="email"`, `autocomplete="username"`,
  `autocapitalize="none"`, and `spellcheck="false"`.
- Password uses `autocomplete="current-password"`.
- Invalid credentials produce one form-level message without revealing whether
  the email exists.
- `reason=session-expired` displays a quiet alert: `Your session expired. Log in
  again to continue.`
- Enter submits the form. Repeated submit is disabled.
- Successful login stores returned user and CSRF token, then replaces history
  with the safe `next` route or `/groups`.

### Registration

Fields:

- Display name, max 120 Unicode code points.
- Email.
- Password, 8–128 UTF-8 bytes.

Password guidance states only the actual rule: `Use 8–128 characters.` It does
not imply uppercase, symbol, or rotation requirements that the backend does not
have. Because the backend measures bytes, the frontend may provide a helpful
UTF-8 byte count near the boundary but must accept the backend field error as
final.

Behavior:

- Client validation catches blank values and obvious length/shape errors.
- API `409` maps to the email field: `An account already uses this email.`
- Successful registration establishes the session, stores returned user and
  rotated CSRF token, and replaces history with `/groups`.
- Link to login remains visible below the form.

### Group List

Header:

- `Your groups`
- primary `Create group`
- secondary `Join with code`

Group rows/cards show:

- group name;
- member count;
- `Owner` badge only when relevant;
- last-updated date as secondary metadata;
- chevron/link target with a minimum 44 px row hit area.

Use a one-column list on phone and a restrained two-column card grid only when
the viewport has enough width. Do not show fake balance totals because the group
list API does not return them.

Empty state uses shadcn-svelte `Empty`:

- `No groups yet`
- `Create a group for a trip or household, or join one with a code.`
- actions for create and join.

Loading uses three stable card skeletons. An error uses `Alert` plus `Retry`.

#### Create Group Dialog

- One group-name field, max 160 code points.
- `Cancel` and `Create group`.
- Dialog has an accessible title and description.
- On success close, refresh group list, toast `Group created`, and navigate to
  the group.

#### Join Group Dialog

- One group-code field.
- Trim and uppercase for the visible value; keep pasted input supported.
- Use a normal `Input`, not an OTP widget: the code is a single copyable value,
  and paste/edit behavior matters more than per-character decoration.
- `autocomplete="off"`, `autocapitalize="characters"`, and
  `spellcheck="false"`.
- `404` produces `That group code is not valid.` without exposing whether a
  dissolved group once existed.
- Success closes the dialog and navigates to the returned group, including when
  membership was already active.

### Group Workspace

The group header displays:

- back link to groups;
- group name;
- member count;
- owner badge for the current user;
- `Add expense` primary action;
- `Record payment` secondary action;
- overflow menu with `Group settings` for owners.

The initial load requests group detail, expenses, repayments, and settlements.
The view tabs are:

1. `Balances`
2. `Activity`
3. `Members`

Tabs use `Tabs.Root`, `Tabs.List`, `Tabs.Trigger`, and `Tabs.Content`. Updating a
tab replaces only the `view` query parameter and preserves scroll where useful.

#### Balances View

Each `SettlementRow` displays:

- sender avatar and display name;
- explicit `should pay`;
- recipient avatar and display name;
- formatted amount;
- `Record payment`.

Clicking `Record payment` navigates to:

```text
/groups/{groupId}/repayments/new
  ?from={fromUserId}
  &to={toUserId}
  &amountCents={amountCents}
```

The form validates all query values against active group members before using
them. Query parameters are hints, never trusted request data.

Empty state:

- `All settled`
- `There are no current balances in this group.`

This means only that the backend returned zero pairwise transfers. It does not
claim that off-app money movement was verified.

#### Activity View

Merge expenses and repayments already returned by their list endpoints:

```ts
type ActivityItem =
  | { kind: "expense"; occurredOn: string; createdAt: string; value: Expense }
  | { kind: "repayment"; occurredOn: string; createdAt: string; value: Repayment };
```

Sort by occurrence date descending, creation timestamp descending, then ID
descending. Group rows by occurrence date.

Expense row:

- description;
- payer sentence: `Alice paid`;
- formatted total;
- participant summary such as `Split with 3 people`;
- edit overflow action.

Repayment row:

- directional sentence: `Bob paid Alice`;
- formatted amount;
- optional note;
- `Recorded outside Settled`;
- edit overflow action.

The whole semantic row links to edit/detail except nested menu actions. Use
buttons and anchors with correct semantics; do not make a clickable `div`.

Empty state offers `Add expense` as the primary action and `Record payment` as
secondary.

#### Members View

Members are ordered owner first, then case-insensitive display name, then user
ID. Each row shows:

- avatar fallback initials;
- display name;
- email;
- owner badge where applicable;
- join date as secondary metadata.

Only owners see the remove action, and never on their own owner row. Removing a
member uses `AlertDialog` with the member's name and the effect on access.

If the API returns `409`, keep the member and show:

`This member is used by a current expense or payment record. Update or delete
those records before removing them.`

No UI attempts to identify the records because the API does not return them in
the conflict response.

### Expense Create And Edit

Use the shared `ExpenseForm` on both routes.

Form order:

1. Description
2. Amount
3. Paid by
4. Date
5. Split method
6. Participants and shares
7. Review summary

Components:

- `Field.FieldGroup` and `Field.Field`.
- `Input` / `InputGroup`.
- `Select` for payer.
- native date input.
- `ToggleGroup` for `Equal`, `Exact amounts`, and `Percentages`.
- checkbox list for participants.
- `Alert` for form-wide issues.
- sticky mobile footer with `Cancel` and `Add expense` / `Save changes`.

Defaults for create:

- payer: current authenticated user when they are an active member;
- date: current local calendar date;
- split mode: equal;
- participants: all active group members in stable display order.

Edit initialization:

- Load the persisted exact splits.
- Default split mode to `Exact amounts`, because the original mode and
  percentages are not persisted.
- Preserve payer, description, amount, date, and exact participant shares.
- Clearly label that changing split mode recalculates the draft.

Split-mode behavior:

- Switching from equal to exact seeds exact inputs from the current equal
  preview.
- Switching to percentage seeds equal percentages across current participants,
  distributing basis-point remainder by participant order.
- Switching back to equal discards manual share edits only after a lightweight
  confirmation when those edits differ from the seeded values.
- Unchecking a participant removes their draft share after confirmation if they
  had a manually edited exact or percentage value.
- At least one participant is required.
- The payer need not be a participant; the UI must not silently add them.

Live review states:

- Equal: `3 people · $18.00 each` plus any one-cent remainder detail where
  amounts differ.
- Exact: `Assigned $53.00 of $54.00 · $1.00 remaining`.
- Percentage: `99.50% assigned · 0.50% remaining`.
- Valid state lists each participant's persisted-cent preview.

Submission sends the discriminated API shape for the active mode. Disable save
until the client draft is structurally valid, but always map backend `422`
fields because membership or concurrent state may have changed.

Delete is available only on edit and uses `AlertDialog`. On success, navigate
with replacement to group activity, refresh balances, and show
`Expense deleted`.

Unsaved changes:

- Internal links and Cancel ask for confirmation when the draft differs from its
  initial value.
- Browser refresh/close uses `beforeunload` only while dirty.
- Drafts are not written to browser storage.

### Repayment Create And Edit

Use a shared `RepaymentForm`.

Fields:

1. `Paid by` (`fromUserId`)
2. `Paid to` (`toUserId`)
3. Amount
4. Date
5. Optional note

Defaults:

- From: current user.
- To: unselected.
- Date: current local calendar date.
- Query-string settlement hint may prefill from, to, and amount only after
  verifying both users remain active and distinct.

The form always displays:

`Settled records a payment made outside the app. It does not send money.`

Prevent choosing the same member in both selects by disabling the current
opposite selection, while retaining backend validation. Any active member may
record on behalf of another; the UI does not restrict sender to current user.

Successful create or edit refreshes repayments and settlements, then replaces
history with the balances view so the effect is immediately visible.

Delete uses `AlertDialog`. Success navigates to group activity and shows
`Payment record deleted`.

### Owner Group Settings

Only render the route content after group detail confirms
`currentUserRole === "owner"`.

Sections:

#### Group Name

- Current name input.
- `Save changes`.
- Inline validation and success toast.

#### Group Code

- Fetch `GET /join-code` only when this section becomes visible.
- Render the code in a read-only, tabular, selectable field.
- `Copy code` uses the Clipboard API.
- Clipboard failure leaves the code selectable and shows a small error.
- Explain: `Anyone with this code can join the group. Share it only with people
  you trust.`
- Do not offer rotation because the API does not support it.

#### Danger Zone

- `Dissolve group` destructive button.
- `AlertDialog` names the group and states that it disappears from ordinary
  views and stops new activity.
- Require typing the current group name before enabling the final destructive
  action.
- On success, clear group-local state, refresh group list, replace history with
  `/groups`, and show `Group dissolved`.

Do not call this action “Delete” because the backend soft-dissolves the group and
the product contract uses dissolution.

### Route And Global Error States

`+error.svelte` handles unexpected client route failures with:

- a plain title;
- concise explanation;
- `Try again`;
- `Back to groups` when authenticated or `Log in` when anonymous.

Known API states stay within their route:

- Hidden group: `This group isn’t available.` No existence detail.
- Hidden expense/repayment: `This record isn’t available.`
- Offline: retain loaded read data in memory, mark it as potentially stale, and
  disable mutation submission.
- API unavailable before any data: dedicated retry state.
- Partial group failure: local alert in the failed panel.

## Component System

### Installed And Required Components

The repository currently contains only `Button`. Add shadcn-svelte components
through the project's `pnpm` runner as implementation reaches each workflow.
Do not re-create their behavior with styled raw elements.

Planned components:

| Need | shadcn-svelte component |
| --- | --- |
| Forms | `field`, `input`, `input-group`, `select`, `checkbox`, `toggle-group` |
| Structure | `card`, `tabs`, `separator`, `scroll-area` |
| Identity/status | `avatar`, `badge` |
| Actions | existing `button`, `dropdown-menu` |
| Compact workflows | `dialog` |
| Destructive confirmation | `alert-dialog` |
| Feedback | `alert`, `empty`, `skeleton`, `spinner`, `sonner` |
| Context help | `tooltip` only for icon-only controls |

Rules:

- Forms use `Field.FieldGroup` and `Field.Field`.
- `Select.Item` and menu items remain inside their `Group`.
- Dialogs and alert dialogs always include a title.
- Cards use header/content/footer composition.
- Loading buttons compose `Spinner` and `Button disabled`.
- Empty states use `Empty`; callouts use `Alert`; loading placeholders use
  `Skeleton`; statuses use `Badge`; separators use `Separator`.
- Avatars always include `Avatar.Fallback`. User images are not in scope, so the
  first release can render fallback initials only.
- Lucide icons come from the installed `@lucide/svelte` package. Icons inside
  components use `data-icon` and no manual size class.
- Use semantic token utilities. Component `class` is for layout, not visual
  overrides.
- Use `gap-*`, never `space-x-*` or `space-y-*`.
- Use `cn()` for conditional class composition.

Before adding or using a component, read its current official shadcn-svelte
component documentation and inspect the generated source. CLI-generated source
must be reviewed for imports matching the aliases in `components.json`.

### App-Level Components

Keep app-level components workflow-specific:

- `MemberAvatar`: fallback initials and accessible name.
- `MoneyAmount`: formatted currency with tabular numerals.
- `RouteState`: loading, empty, error, and retry composition.
- `SettlementRow`: directional balance relation and repayment CTA.
- `ExpenseRow` / `RepaymentRow`: semantic activity links.
- `SplitEditor`: split-mode-specific draft editing; contains no network call.

Do not add a generic `DataTable`, `Modal`, `FormField`, or `EntityCard`
abstraction. The current workflows are different enough that such wrappers
would hide useful semantics.

## Responsive Design

Target widths are behavior thresholds, not device labels:

- `< 640 px`: single column, compact top bar, sticky bottom actions, full-width
  forms and dialogs with safe horizontal margins.
- `640–1023 px`: wider single column; settlement rows may become horizontal.
- `>= 1024 px`: centered content; group overview may place balances beside a
  short activity preview, while tabs still preserve the same content order.

Requirements:

- No control or text overlaps at 320 CSS px width.
- Respect `env(safe-area-inset-bottom)` for the sticky mobile action bar.
- Sticky actions never cover the final form field; reserve equivalent bottom
  padding.
- Member names and descriptions wrap; amounts remain unbroken and aligned.
- Dialogs fit within the visual viewport and scroll internally when the
  on-screen keyboard is open.
- Avoid hover-only information or actions.
- Use a minimum 44 × 44 px hit target for primary touch actions and row menus.
- Check 200% browser zoom without loss of content or operation.

## Accessibility

Required baseline:

- One `h1` per page and sequential section headings.
- Native links for navigation and buttons for actions.
- Every field has a persistent visible label.
- Field errors are associated using shadcn-svelte field semantics,
  `data-invalid`, `aria-invalid`, and descriptions.
- On failed submit, move focus to the first invalid field; if there is only a
  form-level error, focus the alert.
- Dialog focus is trapped and returns to its trigger on close.
- Destructive confirmation never depends on color alone.
- Tabs, menus, dialogs, and selects remain keyboard-operable through component
  primitives.
- Route changes focus the page heading unless navigation only switches a group
  tab.
- Success toasts use a polite live region. Blocking errors are rendered inline
  and are not available only as transient toasts.
- Amount direction is written in words (`Bob should pay Alice`) in addition to
  any arrow icon.
- Avatar initials are decorative when an adjacent name exists; otherwise the
  avatar receives an accessible label.
- Copy buttons announce `Code copied` without changing their width.
- `prefers-reduced-motion` is respected.
- Light and dark semantic token combinations meet WCAG AA contrast for normal
  text and visible focus.
- English interface copy remains understandable at 200% text zoom, while
  user-entered Unicode names and descriptions wrap without clipping.

## PWA And Offline Behavior

### Installability

Add the smallest PWA build integration compatible with Vite/SvelteKit, expected
to be `vite-plugin-pwa`, plus `@sveltejs/adapter-static`.

Manifest:

- name: `Settled`
- short name: `Settled`
- display: `standalone`
- start URL: `/`
- scope: `/`
- background color: current `--background` equivalent
- theme color: current `--primary` equivalent
- icons: maskable and regular 192 px and 512 px PNG assets

The existing SVG favicon may remain for browsers, but manifest icons require
appropriate PNG assets.

### Cache Policy

Precache only:

- versioned JavaScript and CSS bundles;
- application HTML shell/fallback;
- fonts and app icons shipped with the frontend.

Do not cache through the service worker:

- any `/api` response;
- `GET /api/me`;
- CSRF responses;
- group, expense, repayment, or settlement JSON;
- credentialed cross-origin requests.

Use network-only behavior for API calls. Previously rendered data may remain in
live page memory, but is discarded on reload and is clearly marked when the
browser becomes offline.

### Offline UX

- Global offline banner: `You’re offline. Saved information may be out of date.`
- Read-only interaction with already-rendered data remains possible.
- Mutation buttons are disabled with an explanation while `navigator.onLine` is
  false.
- If a request still fails despite `navigator.onLine === true`, show the normal
  network error and Retry; online status is only a hint.
- Never queue a mutation or claim it will sync later.
- Unsaved form data remains in component memory so a user can copy it, retry
  after reconnection, or leave with the normal dirty-form confirmation.

### Updates

Do not silently reload during a form. When a new service worker is ready:

- show a non-blocking `Update available` toast;
- offer `Reload` only when there is no dirty form;
- defer activation when a dirty form is active;
- allow the next clean navigation/reload to apply the update.

## Security And Privacy

- Session and CSRF cookies are backend-managed and HttpOnly.
- CSRF tokens exist only in memory.
- Every API fetch uses explicit credentials.
- Unsafe requests include the CSRF header and rely on the browser `Origin`.
- Do not use `mode: "no-cors"` or wildcard frontend proxy behavior.
- Do not persist passwords, Basic Auth values, group codes, member data, drafts,
  or API responses in browser storage.
- Do not put group codes in URLs, analytics, logs, or toast content.
- Do not introduce third-party analytics, error reporting, fonts, avatars, or
  hosted services without explicit approval.
- Clipboard access occurs only after an owner action and contains only the
  visible join code.
- All external links, if later added, use safe opener behavior.
- Static hosting should set an application-appropriate Content Security Policy,
  `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, and a
  restrictive permissions policy. The CSP must explicitly allow the configured
  API origin in `connect-src`.
- UI role checks are usability hints. Every protected action still handles
  `403`, `404`, and concurrent membership changes from the API.

## Validation Design

### Validation Layers

1. **Input affordance:** input type, mode, limits, and option disabling prevent
   obvious mistakes.
2. **Client validation:** pure helpers catch empty, malformed, inconsistent, or
   impossible drafts before a request.
3. **Backend validation:** authoritative API response covers Unicode, byte
   limits, current membership, hidden state, overflow, and transaction races.
4. **Response validation:** required discriminants and primitives are checked
   before data enters UI state.

Do not duplicate the backend's complete email parser or security validation in
the browser. Client checks improve interaction; they do not replace the API.

### Field Mapping

API field errors use request JSON keys. Forms keep an explicit map:

```text
displayName -> display name field
email -> email field
password -> password field
name -> group name field
joinCode -> group code field
description -> description field
amountCents -> amount field
paidByUserId -> payer field
expenseDate -> date field
participantUserIds / splits / percentageSplits -> split editor
fromUserId -> sender field
toUserId -> recipient field
repaymentDate -> date field
note -> note field
```

Unknown field keys are rendered in the form-level alert so backend changes do
not make an error invisible.

## Testing Strategy

The repository currently has type checking but no frontend test runner. Add the
smallest conventional test setup when implementing behavior-heavy helpers:

- Vitest for pure TypeScript and component tests.
- Testing Library for user-level Svelte component interaction.
- `jsdom` only for tests that need browser DOM behavior.

This is test tooling, not application runtime architecture. Do not add a browser
end-to-end framework until the first integrated frontend/backend workflow exists
and its value justifies the added runtime and maintenance cost.

### Unit Tests

Cover:

- decimal USD parsing without floating-point errors;
- currency formatting;
- percentage-to-basis-point conversion;
- equal, exact, and percentage draft previews including remainders and zero
  shares;
- strict local calendar dates;
- UTF-8 Basic Auth encoding with non-ASCII passwords;
- safe `next` route validation;
- API error parsing and `204` handling;
- CSRF request coalescing, rotation, and exactly-one retry;
- member lookup and avatar initials;
- activity merge and stable ordering.

### Component Tests

Cover:

- login invalid credentials and disabled pending state;
- registration field error mapping;
- create/join group dialogs and focus return;
- split-mode switching and unsaved-change confirmation;
- exact/percentage totals and first-invalid-field focus;
- repayment sender/recipient distinction;
- settlement prefill validation;
- member-removal conflict;
- destructive dialog confirmation;
- partial group panel errors;
- offline mutation disabling without loss of current draft.

### API Contract Fixtures

Maintain small typed JSON fixtures copied from `03_API.md` for:

- current user;
- group detail and members;
- expense list;
- repayment list;
- settlements;
- every common error shape.

Contract fixture tests verify camelCase fields, nullable repayment notes, USD
currency literals, and hidden-state errors. They do not replace backend handler
tests.

### Manual Workflow Checks

Before first release, verify:

1. Register and log in with ASCII and non-ASCII passwords.
2. Expire a session and confirm safe login redirection.
3. Force one CSRF failure and verify a single successful retry.
4. Create and join a group by pasted code.
5. Add equal, exact, and percentage expenses with cent remainders.
6. Edit a persisted expense and confirm it opens as exact amounts.
7. Record a settlement-suggested payment and confirm balances refresh.
8. Delete an expense and repayment and confirm balance changes.
9. Remove an unused member and surface the in-use member conflict.
10. Rename and dissolve a group as owner; confirm member controls stay hidden.
11. Navigate directly to every dynamic route on static hosting.
12. Install the PWA, reload offline, and confirm mutations are unavailable and
    API data is not service-worker cached.
13. Check keyboard-only use, visible focus, screen-reader field errors, reduced
    motion, 320 px width, 200% zoom, and safe-area bottom padding.

### Required Commands

After frontend code changes:

```sh
pnpm check
pnpm test
```

Run:

```sh
pnpm build
```

when changes affect routes, adapter configuration, production output, service
worker behavior, manifest, or PWA assets.

The current repository does not yet define `pnpm test`; add it together with the
first behavior tests rather than silently skipping tests. Format all changed
Svelte, TypeScript, CSS, JSON, and Markdown files with the project formatter
once one is configured. Until then, preserve the existing style and report the
formatter gap explicitly.

## Implementation Sequence

Each phase should leave `pnpm check` passing. Route/build/PWA phases also run
`pnpm build`.

### Phase 1: Static App Foundation

1. Replace `adapter-auto` with `adapter-static` and configure the SPA fallback.
2. Disable SSR and enable static prerendering at the root.
3. Add public API-base configuration and validation.
4. Establish root layout, route groups, app/auth shells, page titles, and route
   error UI.
5. Tune semantic theme tokens and global typography.
6. Add only the shadcn-svelte components needed by the current phase.

### Phase 2: HTTP And Authentication

1. Implement API types, `ApiError`, credentialed request helper, and response
   checks.
2. Implement in-memory CSRF lifecycle and one-retry behavior.
3. Implement UTF-8 Basic Auth.
4. Implement auth state, route guards, login, registration, logout, and session
   expiry.
5. Add unit and component tests for security-sensitive client behavior.

### Phase 3: Groups

1. Implement group list loading, states, create dialog, and join dialog.
2. Implement group layout/detail and member display.
3. Implement owner settings, join-code copy, rename, member removal, and
   dissolution.
4. Verify hidden-state and permission-change handling.

### Phase 4: Expenses

1. Implement money/date helpers and the pure split draft model.
2. Implement expense create form and all three split modes.
3. Implement activity expense rows and expense editing.
4. Implement delete and revalidation of expenses and settlements.
5. Add split boundary, remainder, accessibility, and dirty-form tests.

### Phase 5: Repayments And Settlements

1. Implement settlement rows and balance empty state.
2. Implement repayment create/edit/delete.
3. Implement validated settlement-to-repayment prefill.
4. Merge expense and repayment activity.
5. Verify every mutation refreshes authoritative settlement data.

### Phase 6: PWA And Release Polish

1. Add manifest, icons, static-asset service worker, and update UX.
2. Confirm that API requests and responses are never service-worker cached.
3. Add offline banner and mutation disabling.
4. Complete responsive, zoom, keyboard, screen-reader, and reduced-motion
   checks.
5. Run `pnpm check`, frontend tests, and a clean production build.

## First-Release Acceptance Criteria

- A new user can register, remain authenticated by cookie, and reach groups
  without any frontend-stored session token.
- A returning user can log in with a Unicode password and a CSRF-protected Basic
  Auth request.
- Expired sessions return users safely to login without misclassifying network
  failures as logout.
- A user can create or join a group and reach it from a clear, useful empty
  state.
- An ordinary equal-split expense can be entered on a 320 px-wide screen in
  under one minute.
- Exact and percentage split drafts show totals and cent remainders before save.
- Editing an expense faithfully represents persisted exact splits even though
  original split mode is not stored.
- Current pairwise balances clearly state who should pay whom and by how much.
- Recording a payment from a settlement suggestion refreshes the authoritative
  balances and never claims to move money.
- Owner-only actions are hidden for members, remain backend-enforced, and handle
  role changes without leaking hidden resources.
- Loading, empty, partial failure, offline, validation, conflict, and destructive
  states each provide a clear next action.
- The app installs as a PWA, direct dynamic routes work on static hosting, and no
  authenticated API data or mutation is cached for offline replay.
- Keyboard, focus, reduced-motion, screen-reader, contrast, mobile safe-area,
  and 200% zoom checks pass.
