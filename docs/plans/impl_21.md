# Mini Task 21: Money, Date, and Expense Draft Model

## Goal

Deliver a thoroughly tested, floating-point-free frontend expense model before building the form.

## Prerequisites

Mini Tasks 10 and 15.

## Implementation Scope

- Implement decimal USD string parsing/formatting, strict local dates, and percentage-to-basis-point conversion.
- Implement equal, exact, and percentage draft previews using the same participant order and remainder rules as the backend.
- Implement participant selection, mode-switch seeding, dirty detection, and zero-share/invalid-total messages.
- Initialize persisted expenses in exact mode; do not infer the original mode.
- Convert valid drafts to the three discriminated request DTOs in `03_API.md`.
- Allow a payer who is not a participant; never add the payer implicitly.

## Exclusions

- Do not call the API, render full routes, or treat previews as authoritative settlement.
- Do not use floating-point `number` arithmetic to parse money.

## Validation

- Run `pnpm check` and `pnpm test`.
- Unit-test decimal input, invalid/range boundaries, equal-cent remainder, basis-point remainder, zero shares, mode changes, participant removal, exact edit initialization, dirty state, and DTO shapes.
- Compare frontend preview cents with backend split fixtures.

## Completion Criteria

- Every submittable draft preview sums exactly to the expense total and preserves active participant order in the API request.
