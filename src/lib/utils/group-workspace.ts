import type {
	Expense,
	GroupMember,
	MemberSummary,
	Repayment
} from '$lib/api/types';

import { isStrictLocalDate } from './dates';

export const groupViews = ['balances', 'activity', 'members'] as const;

export type GroupView = (typeof groupViews)[number];

export type ActivityItem =
	| {
			kind: 'expense';
			occurredOn: string;
			createdAt: string;
			id: string;
			value: Expense;
	  }
	| {
			kind: 'repayment';
			occurredOn: string;
			createdAt: string;
			id: string;
			value: Repayment;
	  };

export type ActivityGroup = {
	occurredOn: string;
	items: ActivityItem[];
};

type MemberIdentity = Pick<GroupMember, 'userId' | 'displayName'> | MemberSummary;

const localDateFormatter = new Intl.DateTimeFormat('en-US', {
	year: 'numeric',
	month: 'long',
	day: 'numeric',
	timeZone: 'UTC'
});

const joinedDateFormatter = new Intl.DateTimeFormat('en-US', {
	year: 'numeric',
	month: 'short',
	day: 'numeric'
});

const graphemeSegmenter =
	typeof Intl.Segmenter === 'function'
		? new Intl.Segmenter('en-US', { granularity: 'grapheme' })
		: null;

export function normalizeGroupView(
	value: string | null | undefined | readonly string[]
): GroupView {
	const candidate =
		typeof value === 'string'
			? value
			: value !== null && value !== undefined && value.length === 1
				? value[0]
				: null;

	return groupViews.includes(candidate as GroupView)
		? (candidate as GroupView)
		: 'balances';
}

export function groupViewUrl(url: URL, view: GroupView): string {
	const next = new URL(url);
	next.searchParams.set('view', view);

	return `${next.pathname}${next.search}${next.hash}`;
}

export function buildMemberLookup(
	...memberSets: ReadonlyArray<readonly MemberIdentity[]>
): ReadonlyMap<string, string> {
	const members = new Map<string, string>();

	for (const memberSet of memberSets) {
		for (const member of memberSet) {
			if (!members.has(member.userId)) {
				members.set(member.userId, member.displayName);
			}
		}
	}

	return members;
}

export function memberDisplayName(
	members: ReadonlyMap<string, string>,
	userId: string,
	fallback: string
): string {
	return members.get(userId) ?? fallback;
}

export function memberInitials(displayName: string): string {
	const words = displayName.trim().split(/\s+/u).filter(Boolean);
	if (words.length === 0) {
		return '?';
	}

	const initials =
		words.length > 1
			? `${firstGrapheme(words[0])}${firstGrapheme(words.at(-1) ?? '')}`
			: graphemes(words[0]).slice(0, 2).join('');

	return initials.toLocaleUpperCase('en-US') || '?';
}

export function mergeActivity(
	expenses: readonly Expense[],
	repayments: readonly Repayment[]
): ActivityItem[] {
	const items: ActivityItem[] = [
		...expenses.map(
			(value): ActivityItem => ({
				kind: 'expense',
				occurredOn: value.expenseDate,
				createdAt: value.createdAt,
				id: value.id,
				value
			})
		),
		...repayments.map(
			(value): ActivityItem => ({
				kind: 'repayment',
				occurredOn: value.repaymentDate,
				createdAt: value.createdAt,
				id: value.id,
				value
			})
		)
	];

	return items.sort(compareActivity);
}

export function groupActivity(items: readonly ActivityItem[]): ActivityGroup[] {
	const groups: ActivityGroup[] = [];

	for (const item of items) {
		const current = groups.at(-1);
		if (current?.occurredOn === item.occurredOn) {
			current.items.push(item);
		} else {
			groups.push({ occurredOn: item.occurredOn, items: [item] });
		}
	}

	return groups;
}

export function formatActivityDate(value: string): string {
	if (!isStrictLocalDate(value)) {
		return value;
	}

	const [year, month, day] = value.split('-').map(Number);
	const date = new Date(0);
	date.setUTCHours(12, 0, 0, 0);
	date.setUTCFullYear(year, month - 1, day);

	return localDateFormatter.format(date);
}

export function formatMemberJoinedAt(value: string): string {
	const date = new Date(value);
	return Number.isNaN(date.getTime()) ? value : joinedDateFormatter.format(date);
}

function compareActivity(left: ActivityItem, right: ActivityItem): number {
	const occurrenceOrder = compareStringsDescending(left.occurredOn, right.occurredOn);
	if (occurrenceOrder !== 0) {
		return occurrenceOrder;
	}

	const creationOrder =
		new Date(right.createdAt).getTime() - new Date(left.createdAt).getTime();
	if (creationOrder !== 0) {
		return creationOrder;
	}

	return compareStringsDescending(left.id, right.id);
}

function compareStringsDescending(left: string, right: string): number {
	if (left === right) {
		return 0;
	}

	return left < right ? 1 : -1;
}

function firstGrapheme(value: string): string {
	return graphemes(value)[0] ?? '';
}

function graphemes(value: string): string[] {
	if (graphemeSegmenter === null) {
		return [...value];
	}

	return Array.from(graphemeSegmenter.segment(value), ({ segment }) => segment);
}
