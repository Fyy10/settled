import { describe, expect, it } from 'vitest';

import {
	expenseListFixture,
	groupDetailFixture,
	repaymentListFixture
} from '../../tests/fixtures/api-contract';
import {
	buildMemberLookup,
	formatActivityDate,
	formatMemberJoinedAt,
	groupActivity,
	groupViewUrl,
	memberDisplayName,
	memberInitials,
	mergeActivity,
	normalizeGroupView
} from './group-workspace';

describe('group view URLs', () => {
	it('accepts only the three documented views', () => {
		expect(normalizeGroupView('balances')).toBe('balances');
		expect(normalizeGroupView('activity')).toBe('activity');
		expect(normalizeGroupView('members')).toBe('members');
		expect(normalizeGroupView(null)).toBe('balances');
		expect(normalizeGroupView('settings')).toBe('balances');
		expect(normalizeGroupView('Members')).toBe('balances');
		expect(normalizeGroupView(['members', 'activity'])).toBe('balances');
		expect(normalizeGroupView(['activity'])).toBe('activity');
	});

	it('replaces only view while preserving the rest of the URL', () => {
		const url = new URL(
			'https://settled.test/groups/group-id?source=notification&view=members#latest'
		);

		expect(groupViewUrl(url, 'activity')).toBe(
			'/groups/group-id?source=notification&view=activity#latest'
		);
		expect(url.searchParams.get('view')).toBe('members');
	});
});

describe('member presentation', () => {
	it('builds a stable first-source lookup with an explicit fallback', () => {
		const lookup = buildMemberLookup(
			groupDetailFixture.members,
			[
				{
					userId: groupDetailFixture.members[0].userId,
					displayName: 'Stale Alice'
				},
				{ userId: 'new-user', displayName: 'Noor' }
			]
		);

		expect(
			memberDisplayName(
				lookup,
				groupDetailFixture.members[0].userId,
				'Unknown member'
			)
		).toBe('Alice');
		expect(memberDisplayName(lookup, 'new-user', 'Unknown member')).toBe('Noor');
		expect(memberDisplayName(lookup, 'missing', 'Unknown member')).toBe(
			'Unknown member'
		);
	});

	it('creates compact Unicode-safe fallback initials', () => {
		expect(memberInitials('Alice Zhang')).toBe('AZ');
		expect(memberInitials('  李 雷  ')).toBe('李雷');
		expect(memberInitials('Élodie')).toBe('ÉL');
		expect(memberInitials('👩‍👩‍👧‍👦 Family')).toBe('👩‍👩‍👧‍👦F');
		expect(memberInitials('   ')).toBe('?');
	});
});

describe('activity presentation', () => {
	it('merges by occurrence date, creation instant, and ID descending', () => {
		const tiedExpense = {
			...expenseListFixture.expenses[0],
			id: 'ffffffff-ffff-ffff-ffff-ffffffffffff'
		};
		const items = mergeActivity(
			[expenseListFixture.expenses[0], tiedExpense],
			repaymentListFixture.repayments
		);

		expect(items.map(({ kind, id }) => `${kind}:${id}`)).toEqual([
			`repayment:${repaymentListFixture.repayments[1].id}`,
			`expense:${tiedExpense.id}`,
			`expense:${expenseListFixture.expenses[0].id}`,
			`repayment:${repaymentListFixture.repayments[0].id}`
		]);
	});

	it('groups the sorted stream by API local calendar date', () => {
		const groups = groupActivity(
			mergeActivity(
				expenseListFixture.expenses,
				repaymentListFixture.repayments
			)
		);

		expect(groups.map(({ occurredOn, items }) => [occurredOn, items.length])).toEqual([
			['2026-07-01', 1],
			['2026-06-30', 2]
		]);
		expect(formatActivityDate('2026-06-30')).toBe('June 30, 2026');
		expect(formatActivityDate('not-a-date')).toBe('not-a-date');
		expect(formatMemberJoinedAt('2026-06-30T23:30:00Z')).toMatch(
			/Jun \d{1,2}, 2026/
		);
	});
});
