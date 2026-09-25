import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { groupDetailFixture } from '../../../tests/fixtures/api-contract';
import {
	switchExpenseSplitMode,
	type ExpenseDraft
} from '$lib/utils/expense-draft';

import SplitEditor from './split-editor.svelte';

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('SplitEditor', () => {
	it('renders equal cent remainders in visible member order', () => {
		renderEditor({
			...equalDraft(),
			amount: '10.01'
		});

		expect(screen.getByText('2 people · $5.00–$5.01 each')).toBeInTheDocument();
		const items = screen.getAllByRole('listitem');
		expect(items[0]).toHaveTextContent(/Alice\s+\$5\.01/);
		expect(items[1]).toHaveTextContent(/Bob\s+\$5\.00/);
	});

	it('shows exact and percentage remaining totals with persisted-cent previews', () => {
		const exact = switchExpenseSplitMode(equalDraft(), 'exact');
		exact.participants[0].exactAmount = '30';
		exact.participants[1].exactAmount = '23';
		const result = renderEditor(exact);
		expect(
			screen.getByText('$53.00 of $54.00 assigned · $1.00 remaining')
		).toBeInTheDocument();

		result.unmount();
		const percentage = switchExpenseSplitMode(equalDraft(), 'percentage');
		percentage.participants[0].percentage = '60';
		percentage.participants[1].percentage = '39.5';
		renderEditor(percentage);
		expect(
			screen.getByText('99.50% assigned · 0.50% remaining')
		).toBeInTheDocument();
	});

	it('renders an over-assigned percentage total without throwing', () => {
		const percentage = switchExpenseSplitMode(equalDraft(), 'percentage');
		percentage.participants[0].percentage = '60';
		percentage.participants[1].percentage = '60';

		renderEditor(percentage);

		expect(
			screen.getByText('120.00% assigned · −20.00% remaining')
		).toBeInTheDocument();
	});

	it('confirms before a mode switch discards a manual share', async () => {
		const draft = switchExpenseSplitMode(equalDraft(), 'exact');
		draft.participants[0].exactAmount = '30';
		const onChange = vi.fn();
		const confirm = vi.fn().mockReturnValue(false);
		vi.stubGlobal('confirm', confirm);
		renderEditor(draft, onChange);

		await fireEvent.click(screen.getByRole('radio', { name: 'Equal' }));
		expect(confirm).toHaveBeenCalledWith(
			'Changing the split method will discard your manual shares. Continue?'
		);
		expect(onChange).not.toHaveBeenCalled();

		confirm.mockReturnValue(true);
		await fireEvent.click(screen.getByRole('radio', { name: 'Equal' }));
		expect(onChange).toHaveBeenCalledWith(
			expect.objectContaining({ splitMode: 'equal' })
		);
	});

	it('confirms participant removal and keeps outgoing order aligned to the list', async () => {
		const draft = switchExpenseSplitMode(equalDraft(), 'exact');
		draft.participants[1].exactAmount = '30';
		const onChange = vi.fn();
		const confirm = vi.fn().mockReturnValue(false);
		vi.stubGlobal('confirm', confirm);
		renderEditor(draft, onChange);

		await fireEvent.click(screen.getByRole('checkbox', { name: 'Bob' }));
		expect(confirm).toHaveBeenCalledWith(
			'Removing this participant will discard their manual share. Continue?'
		);
		expect(onChange).not.toHaveBeenCalled();

		confirm.mockReturnValue(true);
		await fireEvent.click(screen.getByRole('checkbox', { name: 'Bob' }));
		expect(
			onChange.mock.calls.at(-1)?.[0].participants.map(
				(participant: { userId: string }) => participant.userId
			)
		).toEqual([groupDetailFixture.members[0].userId]);
	});

	it('programmatically associates a participant-local error with its share', () => {
		const draft = switchExpenseSplitMode(equalDraft(), 'exact');
		renderEditor(draft, vi.fn(), {
			error: 'Enter a valid share.',
			errorParticipantUserId: groupDetailFixture.members[1].userId
		});

		const bobShare = screen.getByLabelText('Share for Bob');
		const aliceShare = screen.getByLabelText('Share for Alice');
		expect(bobShare).toHaveAttribute('aria-describedby', 'expense-split-error');
		expect(bobShare).toHaveAttribute('aria-invalid', 'true');
		expect(aliceShare).not.toHaveAttribute('aria-invalid');
		expect(screen.getByText('Enter a valid share.')).toHaveAttribute(
			'id',
			'expense-split-error'
		);
	});
});

function renderEditor(
	draft: ExpenseDraft,
	onChange = vi.fn(),
	options: { error?: string; errorParticipantUserId?: string } = {}
) {
	return render(SplitEditor, {
		draft,
		members: groupDetailFixture.members,
		onChange,
		...options
	});
}

function equalDraft(): ExpenseDraft {
	return {
		description: 'Shared dinner',
		amount: '54.00',
		paidByUserId: groupDetailFixture.members[0].userId,
		expenseDate: '2026-06-30',
		splitMode: 'equal',
		participants: groupDetailFixture.members.map((member) => ({
			userId: member.userId,
			exactAmount: '',
			seededExactAmount: '',
			percentage: '',
			seededPercentage: ''
		}))
	};
}
