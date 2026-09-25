import { request } from './client';
import { invalidResponseError } from './errors';
import type {
	Expense,
	ExpenseInput,
	ExpenseListResponse
} from './types';
import {
	persistedSplitsForExpenseInput,
	trimExpenseDescription
} from '$lib/utils/expense-draft';
import {
	isExpenseListResponse,
	isExpenseResponse,
	requireApiPayload
} from './validation';

type ExpenseRequestOptions = {
	signal?: AbortSignal;
};

export async function listExpenses(
	groupId: string,
	options: ExpenseRequestOptions = {}
): Promise<ExpenseListResponse> {
	const payload = await request<unknown>(
		expenseCollectionPath(groupId),
		{ signal: options.signal, expectedStatus: 200 }
	);

	const response = requireApiPayload(
		payload,
		isExpenseListResponse,
		'expense-list response'
	);
	if (!isCoherentExpenseList(response, groupId)) {
		throw invalidResponseError(
			200,
			undefined,
			'The server returned an incoherent expense list.'
		);
	}

	return response;
}

export async function getExpense(
	groupId: string,
	expenseId: string,
	options: ExpenseRequestOptions = {}
): Promise<Expense> {
	const payload = await request<unknown>(
		expenseItemPath(groupId, expenseId),
		{ signal: options.signal, expectedStatus: 200 }
	);

	return requireCoherentExpense(
		payload,
		200,
		'expense response',
		groupId,
		expenseId
	);
}

export async function createExpense(
	groupId: string,
	input: Readonly<ExpenseInput>,
	options: ExpenseRequestOptions = {}
): Promise<Expense> {
	const payload = await request<unknown>(expenseCollectionPath(groupId), {
		method: 'POST',
		body: input,
		signal: options.signal,
		expectedStatus: 201
	});

	return requireCoherentExpense(
		payload,
		201,
		'create-expense response',
		groupId,
		undefined,
		input
	);
}

export async function replaceExpense(
	groupId: string,
	expenseId: string,
	input: Readonly<ExpenseInput>,
	options: ExpenseRequestOptions = {}
): Promise<Expense> {
	const payload = await request<unknown>(expenseItemPath(groupId, expenseId), {
		method: 'PUT',
		body: input,
		signal: options.signal,
		expectedStatus: 200
	});

	return requireCoherentExpense(
		payload,
		200,
		'replace-expense response',
		groupId,
		expenseId,
		input
	);
}

export async function deleteExpense(
	groupId: string,
	expenseId: string,
	options: ExpenseRequestOptions = {}
): Promise<void> {
	const payload = await request<unknown>(expenseItemPath(groupId, expenseId), {
		method: 'DELETE',
		signal: options.signal,
		expectedStatus: 204
	});
	if (payload !== undefined) {
		throw invalidResponseError(
			204,
			undefined,
			'The server returned an invalid delete-expense response.'
		);
	}
}

function expenseCollectionPath(groupId: string): string {
	return `/api/groups/${encodeURIComponent(groupId)}/expenses`;
}

function expenseItemPath(groupId: string, expenseId: string): string {
	return `${expenseCollectionPath(groupId)}/${encodeURIComponent(expenseId)}`;
}

function requireCoherentExpense(
	payload: unknown,
	status: number,
	description: string,
	groupId: string,
	expenseId?: string,
	input?: Readonly<ExpenseInput>
): Expense {
	if (!isExpenseResponse(payload)) {
		throw invalidResponseError(
			status,
			undefined,
			`The server returned an invalid ${description}.`
		);
	}
	if (
		payload.expense.groupId !== groupId ||
		(expenseId !== undefined && payload.expense.id !== expenseId)
	) {
		throw invalidResponseError(
			status,
			undefined,
			'The server returned an expense for a different resource.'
		);
	}
	if (input !== undefined && !matchesExpenseInput(payload.expense, input)) {
		throw invalidResponseError(
			status,
			undefined,
			'The server returned an expense that does not match the submitted input.'
		);
	}

	return payload.expense;
}

function isCoherentExpenseList(
	response: ExpenseListResponse,
	groupId: string
): boolean {
	const expenseIds = new Set<string>();
	const memberIds = new Set<string>();

	for (const member of response.members) {
		const identity = member.userId.toLowerCase();
		if (memberIds.has(identity)) {
			return false;
		}
		memberIds.add(identity);
	}

	for (const expense of response.expenses) {
		const identity = expense.id.toLowerCase();
		if (expense.groupId !== groupId || expenseIds.has(identity)) {
			return false;
		}
		expenseIds.add(identity);

		if (
			!memberIds.has(expense.paidByUserId.toLowerCase()) ||
			expense.splits.some(
				(split) => !memberIds.has(split.userId.toLowerCase())
			)
		) {
			return false;
		}
	}

	return true;
}

function matchesExpenseInput(
	expense: Expense,
	input: Readonly<ExpenseInput>
): boolean {
	if (
		expense.paidByUserId !== input.paidByUserId ||
		expense.description !== trimExpenseDescription(input.description) ||
		expense.amountCents !== input.amountCents ||
		expense.expenseDate !== input.expenseDate
	) {
		return false;
	}

	const expectedSplits = persistedSplitsForExpenseInput(input);
	return (
		expectedSplits.length === expense.splits.length &&
		expectedSplits.every(
			(split, index) =>
				split.userId === expense.splits[index]?.userId &&
				split.amountCents === expense.splits[index]?.amountCents
		)
	);
}
