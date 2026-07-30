import { request } from './client';
import { invalidResponseError } from './errors';
import type { ExpenseListResponse } from './types';
import { isExpenseListResponse, requireApiPayload } from './validation';

type ExpenseRequestOptions = {
	signal?: AbortSignal;
};

export async function listExpenses(
	groupId: string,
	options: ExpenseRequestOptions = {}
): Promise<ExpenseListResponse> {
	const payload = await request<unknown>(
		`/api/groups/${encodeURIComponent(groupId)}/expenses`,
		{ signal: options.signal }
	);

	const response = requireApiPayload(
		payload,
		isExpenseListResponse,
		'expense-list response'
	);
	if (response.expenses.some((expense) => expense.groupId !== groupId)) {
		throw invalidResponseError(
			200,
			undefined,
			'The server returned expenses for a different group.'
		);
	}

	return response;
}
