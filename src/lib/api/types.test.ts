import { describe, expect, expectTypeOf, it } from 'vitest';

import {
	commonErrorFixtures,
	currentUserFixture,
	expenseListFixture,
	groupDetailFixture,
	repaymentListFixture,
	settlementListFixture
} from '../../tests/fixtures/api-contract';
import type {
	CurrentUserResponse,
	ExpenseListResponse,
	GroupDetailResponse,
	RepaymentListResponse,
	SettlementListResponse
} from './types';
import {
	isApiErrorBody,
	isCurrentUserResponse,
	isExpenseListResponse,
	isGroupDetailResponse,
	isRepaymentListResponse,
	isSettlementListResponse
} from './validation';

describe('API contract fixtures', () => {
	it('keeps the documented response wrappers typed and valid', () => {
		expectTypeOf(currentUserFixture).toMatchTypeOf<CurrentUserResponse>();
		expectTypeOf(groupDetailFixture).toMatchTypeOf<GroupDetailResponse>();
		expectTypeOf(expenseListFixture).toMatchTypeOf<ExpenseListResponse>();
		expectTypeOf(repaymentListFixture).toMatchTypeOf<RepaymentListResponse>();
		expectTypeOf(settlementListFixture).toMatchTypeOf<SettlementListResponse>();

		expect(isCurrentUserResponse(currentUserFixture)).toBe(true);
		expect(isGroupDetailResponse(groupDetailFixture)).toBe(true);
		expect(isExpenseListResponse(expenseListFixture)).toBe(true);
		expect(isRepaymentListResponse(repaymentListFixture)).toBe(true);
		expect(isSettlementListResponse(settlementListFixture)).toBe(true);
	});

	it('uses camelCase wire keys, nullable notes, and literal USD currency', () => {
		for (const fixture of [
			currentUserFixture,
			groupDetailFixture,
			expenseListFixture,
			repaymentListFixture,
			settlementListFixture
		]) {
			expect(allObjectKeys(fixture).every((key) => !key.includes('_'))).toBe(true);
		}

		expect(repaymentListFixture.repayments.map((repayment) => repayment.note)).toContain(null);
		expect(expenseListFixture.expenses[0].currency).toBe('USD');
		expect(repaymentListFixture.repayments.every(({ currency }) => currency === 'USD')).toBe(
			true
		);
		expect(settlementListFixture.settlements[0].currency).toBe('USD');
	});

	it('covers every documented common and hidden-state error shape', () => {
		expect(Object.keys(commonErrorFixtures)).toEqual([
			'badRequest',
			'unauthorized',
			'forbidden',
			'notFound',
			'conflict',
			'csrfRequired',
			'csrfInvalid',
			'validationFailed',
			'internalError'
		]);
		expect(Object.values(commonErrorFixtures).every(isApiErrorBody)).toBe(true);
		expect(commonErrorFixtures.notFound.error.code).toBe('not_found');
		expect(commonErrorFixtures.validationFailed.error.fields).toEqual({
			email: 'Email is required.'
		});
	});

	it('rejects unsafe cents, zero persisted shares, invalid roles, currency, and nullability', () => {
		const unsafeExpense = structuredClone(expenseListFixture);
		unsafeExpense.expenses[0].amountCents = Number.MAX_SAFE_INTEGER + 1;
		expect(isExpenseListResponse(unsafeExpense)).toBe(false);

		const zeroShare = structuredClone(expenseListFixture);
		zeroShare.expenses[0].splits[0].amountCents = 0;
		expect(isExpenseListResponse(zeroShare)).toBe(false);

		const invalidRole: unknown = {
			...groupDetailFixture,
			group: { ...groupDetailFixture.group, currentUserRole: 'admin' }
		};
		expect(isGroupDetailResponse(invalidRole)).toBe(false);

		const invalidCurrency: unknown = {
			...settlementListFixture,
			settlements: [{ ...settlementListFixture.settlements[0], currency: 'EUR' }]
		};
		expect(isSettlementListResponse(invalidCurrency)).toBe(false);

		const missingNullableNote: unknown = {
			...repaymentListFixture,
			repayments: [
				{
					...repaymentListFixture.repayments[0],
					note: undefined
				}
			]
		};
		expect(isRepaymentListResponse(missingNullableNote)).toBe(false);
	});
});

function allObjectKeys(value: unknown): string[] {
	if (Array.isArray(value)) {
		return value.flatMap(allObjectKeys);
	}
	if (typeof value !== 'object' || value === null) {
		return [];
	}

	return Object.entries(value).flatMap(([key, child]) => [key, ...allObjectKeys(child)]);
}
