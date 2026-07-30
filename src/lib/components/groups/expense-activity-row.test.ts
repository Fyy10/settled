import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';

import { expenseListFixture } from '../../../tests/fixtures/api-contract';

import ExpenseActivityRow from './expense-activity-row.svelte';

describe('ExpenseActivityRow', () => {
	it('uses semantic, encoded edit links without nested interactive content', () => {
		const expense = {
			...expenseListFixture.expenses[0],
			id: 'expense/id'
		};
		const { container } = render(ExpenseActivityRow, {
			groupId: 'group/id',
			expense,
			payerName: 'Alice'
		});

		const rowLink = screen.getByRole('link', {
			name: /Groceries Alice paid Split with 2 people \$54\.00/
		});
		expect(rowLink).toHaveAttribute(
			'href',
			'/groups/group%2Fid/expenses/expense%2Fid/edit'
		);
		expect(rowLink.querySelector('a, button')).toBeNull();
		expect(container.querySelector('article')).toContainElement(rowLink);
		expect(screen.getAllByRole('link')).toHaveLength(1);
	});
});
