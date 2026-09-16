import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';

import { repaymentListFixture } from '../../../tests/fixtures/api-contract';

import RepaymentActivityRow from './repayment-activity-row.svelte';

describe('RepaymentActivityRow', () => {
	it('uses one whole-row semantic edit link with complete off-app details', () => {
		const repayment = {
			...repaymentListFixture.repayments[0],
			id: 'repayment/id'
		};
		const { container } = render(RepaymentActivityRow, {
			groupId: 'group/id',
			repayment,
			fromName: 'Bob',
			toName: 'Alice'
		});

		const link = screen.getByRole('link', {
			name: /Bob paid Alice.*Venmo.*Recorded outside Settled.*\$20\.00/
		});
		expect(link).toHaveAttribute(
			'href',
			'/groups/group%2Fid/repayments/repayment%2Fid/edit'
		);
		expect(container.querySelectorAll('a')).toHaveLength(1);
		expect(link.closest('article')).not.toBeNull();
	});

	it('omits the optional note without weakening direction or provenance', () => {
		render(RepaymentActivityRow, {
			groupId: 'group-id',
			repayment: repaymentListFixture.repayments[1],
			fromName: 'Alice',
			toName: 'Bob'
		});

		expect(
			screen.getByRole('link', {
				name: /Alice paid Bob.*Recorded outside Settled.*\$5\.00/
			})
		).toBeInTheDocument();
		expect(screen.queryByText('Venmo')).not.toBeInTheDocument();
	});
});
