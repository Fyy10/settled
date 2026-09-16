import { render, screen, within } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';

import {
	groupDetailFixture,
	settlementListFixture
} from '../../../tests/fixtures/api-contract';

import BalancesPanel from './balances-panel.svelte';
import MembersPanel from './members-panel.svelte';

vi.mock('$lib/config/public', () => ({
	API_BASE_URL: 'http://localhost:8080'
}));

describe('balances panel', () => {
	it('renders settlement rows in the exact order returned by the API', () => {
		const secondSettlement = {
			fromUserId: settlementListFixture.members[0].userId,
			toUserId: settlementListFixture.members[1].userId,
			amountCents: 525,
			currency: 'USD' as const
		};
		const { container } = render(BalancesPanel, {
			groupId: groupDetailFixture.group.id,
			status: 'ready',
			response: {
				settlements: [
					settlementListFixture.settlements[0],
					secondSettlement
				],
				members: settlementListFixture.members
			},
			onRetry: vi.fn()
		});

		const rows = Array.from(container.querySelectorAll('article'));
		expect(rows).toHaveLength(2);
		expect(rows[0]).toHaveTextContent('Bob');
		expect(rows[0]).toHaveTextContent('Alice');
		expect(rows[0]).toHaveTextContent('$20.00');
		const direction = rows[0].querySelector('p');
		expect(direction).toHaveTextContent('Bob should pay Alice');
		expect(direction).not.toHaveAttribute('aria-hidden');
		expect(rows[1]).toHaveTextContent('Alice');
		expect(rows[1]).toHaveTextContent('Bob');
		expect(rows[1]).toHaveTextContent('$5.25');

		const firstPaymentLink = within(rows[0]).getByRole('link', {
			name: 'Record payment'
		});
		expect(firstPaymentLink).toHaveAttribute(
			'href',
			`/groups/${groupDetailFixture.group.id}/repayments/new?from=${settlementListFixture.settlements[0].fromUserId}&to=${settlementListFixture.settlements[0].toUserId}&amountCents=${settlementListFixture.settlements[0].amountCents}`
		);
	});

	it('suppresses preserved settlement rows after a refresh failure', () => {
		render(BalancesPanel, {
			groupId: groupDetailFixture.group.id,
			status: 'error',
			response: settlementListFixture,
			onRetry: vi.fn()
		});

		expect(
			screen.getByText('Settled couldn’t load balances.')
		).toBeInTheDocument();
		expect(
			screen.queryByRole('link', { name: 'Record payment' })
		).not.toBeInTheDocument();
		expect(screen.queryByText('$20.00')).not.toBeInTheDocument();
	});
});

describe('members panel', () => {
	it('preserves API member order and renders Unicode-safe avatar fallbacks', () => {
		const members = [
			{
				...groupDetailFixture.members[1],
				displayName: '李 雷'
			},
			groupDetailFixture.members[0]
		];
		const { container } = render(MembersPanel, { members });

		const rows = Array.from(
			container.querySelectorAll<HTMLElement>('ul > li')
		);
		expect(rows).toHaveLength(2);
		expect(rows[0]).toHaveTextContent('李 雷');
		expect(rows[1]).toHaveTextContent('Alice');
		expect(within(rows[0]).getByText('李雷')).toBeInTheDocument();
		expect(screen.getByText('Owner')).toBeInTheDocument();
	});

	it('shows removal only to owners and never for the owner row', () => {
		const member = groupDetailFixture.members[1];
		const memberView = render(MembersPanel, {
			members: groupDetailFixture.members,
			groupId: groupDetailFixture.group.id,
			canManage: false
		});
		expect(
			screen.queryByRole('button', { name: `Remove member ${member.displayName}` })
		).not.toBeInTheDocument();
		memberView.unmount();

		render(MembersPanel, {
			members: groupDetailFixture.members,
			groupId: groupDetailFixture.group.id,
			canManage: true
		});
		expect(
			screen.getByRole('button', { name: `Remove member ${member.displayName}` })
		).toBeInTheDocument();
		expect(
			screen.queryByRole('button', {
				name: `Remove member ${groupDetailFixture.members[0].displayName}`
			})
		).not.toBeInTheDocument();
	});
});
