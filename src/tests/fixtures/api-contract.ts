import type {
	ApiErrorBody,
	CurrentUserResponse,
	ExpenseListResponse,
	GroupDetailResponse,
	RepaymentListResponse,
	SettlementListResponse
} from '$lib/api/types';

export const currentUserFixture = {
	user: {
		id: '0a4b8d9a-4f65-4a88-8e58-9467df994a61',
		email: 'alice@example.com',
		displayName: 'Alice',
		createdAt: '2026-06-30T18:00:00Z',
		updatedAt: '2026-06-30T18:00:00Z'
	}
} satisfies CurrentUserResponse;

export const groupDetailFixture = {
	group: {
		id: '9a41c3a3-169f-4219-998b-822d31352f90',
		name: 'Lake Trip',
		ownerUserId: currentUserFixture.user.id,
		memberCount: 2,
		currentUserRole: 'owner',
		createdAt: '2026-06-30T18:00:00Z',
		updatedAt: '2026-06-30T18:00:00Z'
	},
	members: [
		{
			userId: currentUserFixture.user.id,
			email: currentUserFixture.user.email,
			displayName: currentUserFixture.user.displayName,
			role: 'owner',
			joinedAt: '2026-06-30T18:00:00Z'
		},
		{
			userId: '6c251b8f-e8e1-48fa-a1f1-2f18543e7ef2',
			email: 'bob@example.com',
			displayName: 'Bob',
			role: 'member',
			joinedAt: '2026-06-30T18:05:00Z'
		}
	]
} satisfies GroupDetailResponse;

export const expenseListFixture = {
	expenses: [
		{
			id: 'd87d80c8-c6bc-42b1-9e8f-6b99d29fa8d6',
			groupId: groupDetailFixture.group.id,
			paidByUserId: currentUserFixture.user.id,
			description: 'Groceries',
			amountCents: 5400,
			currency: 'USD',
			expenseDate: '2026-06-30',
			createdByUserId: currentUserFixture.user.id,
			splits: [
				{ userId: currentUserFixture.user.id, amountCents: 2700 },
				{ userId: groupDetailFixture.members[1].userId, amountCents: 2700 }
			],
			createdAt: '2026-06-30T18:00:00Z',
			updatedAt: '2026-06-30T18:00:00Z'
		}
	],
	members: groupDetailFixture.members.map(({ userId, displayName }) => ({
		userId,
		displayName
	}))
} satisfies ExpenseListResponse;

export const repaymentListFixture = {
	repayments: [
		{
			id: '21feeb5e-ff5c-44ec-b0bc-8daae2d0522f',
			groupId: groupDetailFixture.group.id,
			fromUserId: groupDetailFixture.members[1].userId,
			toUserId: currentUserFixture.user.id,
			amountCents: 2000,
			currency: 'USD',
			note: 'Venmo',
			repaymentDate: '2026-06-30',
			createdByUserId: groupDetailFixture.members[1].userId,
			createdAt: '2026-06-30T18:00:00Z',
			updatedAt: '2026-06-30T18:00:00Z'
		},
		{
			id: '455cb458-55bd-47c8-91ce-4fad0fa204d0',
			groupId: groupDetailFixture.group.id,
			fromUserId: currentUserFixture.user.id,
			toUserId: groupDetailFixture.members[1].userId,
			amountCents: 500,
			currency: 'USD',
			note: null,
			repaymentDate: '2026-07-01',
			createdByUserId: currentUserFixture.user.id,
			createdAt: '2026-07-01T18:00:00Z',
			updatedAt: '2026-07-01T18:00:00Z'
		}
	],
	members: groupDetailFixture.members.map(({ userId, displayName }) => ({
		userId,
		displayName
	}))
} satisfies RepaymentListResponse;

export const settlementListFixture = {
	settlements: [
		{
			fromUserId: groupDetailFixture.members[1].userId,
			toUserId: currentUserFixture.user.id,
			amountCents: 2000,
			currency: 'USD'
		}
	],
	members: groupDetailFixture.members.map(({ userId, displayName }) => ({
		userId,
		displayName
	}))
} satisfies SettlementListResponse;

export const commonErrorFixtures = {
	badRequest: error('bad_request', 'The request is malformed.'),
	unauthorized: error('unauthorized', 'Authentication is required.'),
	forbidden: error('forbidden', 'This action is not available.'),
	notFound: error('not_found', 'The requested resource was not found.'),
	conflict: error('conflict', 'The request conflicts with the current state.'),
	csrfRequired: error('csrf_required', 'A CSRF token is required.'),
	csrfInvalid: error('csrf_invalid', 'The CSRF token is invalid.'),
	validationFailed: error('validation_failed', 'One or more fields are invalid.', {
		email: 'Email is required.'
	}),
	internalError: error('internal_error', 'An unexpected error occurred.')
} satisfies Record<string, ApiErrorBody>;

function error(
	code: string,
	message: string,
	fields?: Record<string, string>
): ApiErrorBody {
	return {
		error: {
			code,
			message,
			...(fields === undefined ? {} : { fields })
		}
	};
}
