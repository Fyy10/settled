import { isStrictLocalDate } from '$lib/utils/dates';
import { isValidPersistedRepaymentNote } from '$lib/utils/repayment-draft';

import { invalidResponseError } from './errors';
import type {
	ApiErrorBody,
	AuthResponse,
	CsrfResponse,
	CurrentUserResponse,
	Expense,
	ExpenseListResponse,
	ExpenseResponse,
	ExpenseSplit,
	GroupDetailResponse,
	GroupListResponse,
	GroupMember,
	GroupResponse,
	GroupSummary,
	HealthResponse,
	JoinCodeResponse,
	MemberSummary,
	Repayment,
	RepaymentListResponse,
	RepaymentResponse,
	Settlement,
	SettlementListResponse,
	User
} from './types';

type JsonObject = Record<string, unknown>;
type PayloadGuard<T> = (value: unknown) => value is T;
const rfc3339Pattern =
	/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/;

export function isJsonObject(value: unknown): value is JsonObject {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function isApiErrorBody(value: unknown): value is ApiErrorBody {
	if (!isJsonObject(value) || !isJsonObject(value.error)) {
		return false;
	}

	const fields = value.error.fields;

	return (
		typeof value.error.code === 'string' &&
		typeof value.error.message === 'string' &&
		(fields === undefined || isStringRecord(fields))
	);
}

export function isHealthResponse(value: unknown): value is HealthResponse {
	return (
		isJsonObject(value) && (value.status === 'ok' || value.status === 'unavailable')
	);
}

export function isCsrfResponse(value: unknown): value is CsrfResponse {
	return isJsonObject(value) && isNonEmptyString(value.csrfToken);
}

export function isAuthResponse(value: unknown): value is AuthResponse {
	return (
		isJsonObject(value) && isNonEmptyString(value.csrfToken) && isUser(value.user)
	);
}

export function isCurrentUserResponse(value: unknown): value is CurrentUserResponse {
	return isJsonObject(value) && isUser(value.user);
}

export function isGroupListResponse(value: unknown): value is GroupListResponse {
	return isJsonObject(value) && isArrayOf(value.groups, isGroupSummary);
}

export function isGroupResponse(value: unknown): value is GroupResponse {
	return isJsonObject(value) && isGroupSummary(value.group);
}

export function isGroupDetailResponse(value: unknown): value is GroupDetailResponse {
	return (
		isJsonObject(value) &&
		isGroupSummary(value.group) &&
		isArrayOf(value.members, isGroupMember)
	);
}

export function isJoinCodeResponse(value: unknown): value is JoinCodeResponse {
	return isJsonObject(value) && isNonEmptyString(value.joinCode);
}

export function isExpenseListResponse(value: unknown): value is ExpenseListResponse {
	return (
		isJsonObject(value) &&
		isArrayOf(value.expenses, isExpense) &&
		isArrayOf(value.members, isMemberSummary)
	);
}

export function isExpenseResponse(value: unknown): value is ExpenseResponse {
	return isJsonObject(value) && isExpense(value.expense);
}

export function isRepaymentListResponse(value: unknown): value is RepaymentListResponse {
	return (
		isJsonObject(value) &&
		isArrayOf(value.repayments, isRepayment) &&
		isArrayOf(value.members, isMemberSummary)
	);
}

export function isRepaymentResponse(value: unknown): value is RepaymentResponse {
	return isJsonObject(value) && isRepayment(value.repayment);
}

export function isSettlementListResponse(value: unknown): value is SettlementListResponse {
	return (
		isJsonObject(value) &&
		isArrayOf(value.settlements, isSettlement) &&
		isArrayOf(value.members, isMemberSummary)
	);
}

export function requireApiPayload<T>(
	value: unknown,
	guard: PayloadGuard<T>,
	description: string
): T {
	if (!guard(value)) {
		throw invalidResponseError(200, undefined, `The server returned an invalid ${description}.`);
	}

	return value;
}

function isUser(value: unknown): value is User {
	return (
		isJsonObject(value) &&
		isNonEmptyString(value.id) &&
		typeof value.email === 'string' &&
		typeof value.displayName === 'string' &&
		typeof value.createdAt === 'string' &&
		typeof value.updatedAt === 'string'
	);
}

function isGroupSummary(value: unknown): value is GroupSummary {
	return (
		isJsonObject(value) &&
		isNonEmptyString(value.id) &&
		typeof value.name === 'string' &&
		isNonEmptyString(value.ownerUserId) &&
		isNonNegativeSafeInteger(value.memberCount) &&
		(value.currentUserRole === 'owner' || value.currentUserRole === 'member') &&
		isTimestamp(value.createdAt) &&
		isTimestamp(value.updatedAt)
	);
}

function isGroupMember(value: unknown): value is GroupMember {
	return (
		isJsonObject(value) &&
		isNonEmptyString(value.userId) &&
		typeof value.email === 'string' &&
		typeof value.displayName === 'string' &&
		(value.role === 'owner' || value.role === 'member') &&
		isTimestamp(value.joinedAt)
	);
}

function isMemberSummary(value: unknown): value is MemberSummary {
	return (
		isJsonObject(value) &&
		isNonEmptyString(value.userId) &&
		typeof value.displayName === 'string'
	);
}

function isExpenseSplit(value: unknown): value is ExpenseSplit {
	return (
		isJsonObject(value) &&
		isNonEmptyString(value.userId) &&
		isPositiveSafeInteger(value.amountCents)
	);
}

function isExpense(value: unknown): value is Expense {
	return (
		isJsonObject(value) &&
		isNonEmptyString(value.id) &&
		isNonEmptyString(value.groupId) &&
		isNonEmptyString(value.paidByUserId) &&
		typeof value.description === 'string' &&
		isPositiveSafeInteger(value.amountCents) &&
		value.currency === 'USD' &&
		typeof value.expenseDate === 'string' &&
		isStrictLocalDate(value.expenseDate) &&
		isNonEmptyString(value.createdByUserId) &&
		isArrayOf(value.splits, isExpenseSplit) &&
		hasCoherentExpenseSplits(value.splits, value.amountCents) &&
		isTimestamp(value.createdAt) &&
		isTimestamp(value.updatedAt)
	);
}

function hasCoherentExpenseSplits(
	splits: readonly ExpenseSplit[],
	amountCents: number
): boolean {
	if (splits.length === 0) {
		return false;
	}

	const userIds = new Set<string>();
	let total = 0n;
	for (const split of splits) {
		const userId = split.userId.toLocaleLowerCase('en-US');
		if (userIds.has(userId)) {
			return false;
		}

		userIds.add(userId);
		total += BigInt(split.amountCents);
	}

	return total === BigInt(amountCents);
}

function isRepayment(value: unknown): value is Repayment {
	return (
		isJsonObject(value) &&
		isNonEmptyString(value.id) &&
		isNonEmptyString(value.groupId) &&
		isNonEmptyString(value.fromUserId) &&
		isNonEmptyString(value.toUserId) &&
		value.fromUserId.toLocaleLowerCase('en-US') !==
			value.toUserId.toLocaleLowerCase('en-US') &&
		isPositiveSafeInteger(value.amountCents) &&
		value.currency === 'USD' &&
		(value.note === null || typeof value.note === 'string') &&
		isValidPersistedRepaymentNote(value.note) &&
		typeof value.repaymentDate === 'string' &&
		isStrictLocalDate(value.repaymentDate) &&
		isNonEmptyString(value.createdByUserId) &&
		isTimestamp(value.createdAt) &&
		isTimestamp(value.updatedAt)
	);
}

function isSettlement(value: unknown): value is Settlement {
	return (
		isJsonObject(value) &&
		isNonEmptyString(value.fromUserId) &&
		isNonEmptyString(value.toUserId) &&
		value.fromUserId.toLocaleLowerCase('en-US') !==
			value.toUserId.toLocaleLowerCase('en-US') &&
		isPositiveSafeInteger(value.amountCents) &&
		value.currency === 'USD'
	);
}

function isArrayOf<T>(value: unknown, guard: PayloadGuard<T>): value is T[] {
	return Array.isArray(value) && value.every(guard);
}

function isStringRecord(value: unknown): value is Record<string, string> {
	return (
		isJsonObject(value) && Object.values(value).every((item) => typeof item === 'string')
	);
}

function isNonEmptyString(value: unknown): value is string {
	return typeof value === 'string' && value.length > 0;
}

function isNonNegativeSafeInteger(value: unknown): value is number {
	return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0;
}

function isPositiveSafeInteger(value: unknown): value is number {
	return typeof value === 'number' && Number.isSafeInteger(value) && value > 0;
}

function isTimestamp(value: unknown): value is string {
	return (
		typeof value === 'string' &&
		rfc3339Pattern.test(value) &&
		Number.isFinite(Date.parse(value))
	);
}
