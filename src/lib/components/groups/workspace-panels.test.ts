import { render, screen, within } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';

import {
	groupDetailFixture,
	settlementListFixture
} from '../../../tests/fixtures/api-contract';

import BalancesPanel from './balances-panel.svelte';
import MembersPanel from './members-panel.svelte';

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
			expect.stringContaining(
				`from=${settlementListFixture.settlements[0].fromUserId}`
			)
		);
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
});
