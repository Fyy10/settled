# Mini Task 17: Login, Registration, Logout, and Route Guards

## Goal

Deliver the complete frontend authentication loop and a reliable expired-session experience.

## Prerequisites

Mini Task 16.

## Implementation Scope

- Implement `/` bootstrap, `/login`, `/register`, auth/app layout guards, and app-header logout.
- Use the field semantics, autocomplete attributes, password visibility, pending/error states, and English copy specified by `05_frontend.md`.
- Submit login through UTF-8 Basic Auth and registration through JSON; on success, keep returned user and rotated CSRF in memory and replace history with a safe next path or `/groups`.
- On session-expiration `401`, clear in-memory auth and navigate to login; on network failure, retain the authenticated UI and do not misclassify the user as logged out.
- Retain the current UI when logout fails; replace history with `/login` only after success.
- Make field and form errors focusable and correctly associated for assistive technology.

## Exclusions

- Do not implement OAuth, password reset, email verification, 2FA, a marketing page, or persistent session tokens.

## Validation

- Run `pnpm check`, `pnpm test`, and `pnpm build`.
- Component-test invalid credentials, duplicate email, pending states, field-error mapping, session expiration, logout network failure, and focus behavior.
- Manually test ASCII and non-ASCII passwords, refresh, direct app-route visits, narrow layouts, and keyboard-only use.

## Completion Criteria

- New and returning users reach `/groups` through HttpOnly cookies, and the frontend never parses a JWT.
