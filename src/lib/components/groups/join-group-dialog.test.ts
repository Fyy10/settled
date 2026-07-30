import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '$lib/api/errors';
import type { GroupSummary } from '$lib/api/types';
import { groupListFixture } from '../../../tests/fixtures/api-contract';

import JoinGroupDialog from './join-group-dialog.svelte';

const mocks = vi.hoisted(() => ({
	joinGroup: vi.fn()
}));

vi.mock('$lib/api/groups', () => ({ joinGroup: mocks.joinGroup }));
vi.mock('$lib/config/public', () => ({ API_BASE_URL: 'http://localhost:8080' }));

beforeEach(() => {
	mocks.joinGroup.mockReset();
});

describe('JoinGroupDialog', () => {
	it('focuses a paste-friendly normal input and normalizes its visible value', async () => {
		render(JoinGroupDialog, { onSuccess: vi.fn() });
		await openJoinDialog();

		const input = screen.getByLabelText('Group code');
		await waitFor(() => expect(input).toHaveFocus());
		expect(input).toHaveAttribute('type', 'text');
		expect(input).toHaveAttribute('autocomplete', 'off');
		expect(input).toHaveAttribute('autocapitalize', 'characters');
		expect(input).toHaveAttribute('spellcheck', 'false');

		await fireEvent.input(input, { target: { value: '  abcd1234  ' } });
		expect(input).toHaveValue('ABCD1234');
	});

	it('validates an empty code locally and focuses the field', async () => {
		render(JoinGroupDialog, { onSuccess: vi.fn() });
		await openJoinDialog();

		await fireEvent.click(screen.getByRole('button', { name: 'Join group' }));

		const input = screen.getByLabelText('Group code');
		expect(await screen.findByText('Enter a group code.')).toHaveAttribute(
			'id',
			'join-group-code-error'
		);
		await waitFor(() => expect(input).toHaveFocus());
		expect(mocks.joinGroup).not.toHaveBeenCalled();
	});

	it('uses privacy-preserving copy for a hidden or unknown code and preserves input', async () => {
		mocks.joinGroup.mockRejectedValue(
			new ApiError({
				status: 404,
				code: 'not_found',
				message: 'The requested resource was not found.',
				fields: {}
			})
		);
		render(JoinGroupDialog, { onSuccess: vi.fn() });
		await openJoinDialog();
		const input = screen.getByLabelText('Group code');
		await fireEvent.input(input, { target: { value: 'abcd1234' } });

		await fireEvent.click(screen.getByRole('button', { name: 'Join group' }));

		const alert = await screen.findByRole('alert');
		expect(alert).toHaveTextContent('That group code is not valid.');
		expect(alert).not.toHaveTextContent(/dissolved|removed|existed/i);
		expect(input).toHaveValue('ABCD1234');
		await waitFor(() => expect(alert).toHaveFocus());
	});

	it('maps API joinCode errors to the labelled field', async () => {
		mocks.joinGroup.mockRejectedValue(
			new ApiError({
				status: 422,
				code: 'validation_failed',
				message: 'One or more fields are invalid.',
				fields: { joinCode: 'Join code must use the current format.' }
			})
		);
		render(JoinGroupDialog, { onSuccess: vi.fn() });
		await openJoinDialog();
		const input = screen.getByLabelText('Group code');
		await fireEvent.input(input, { target: { value: 'abcd1234' } });

		await fireEvent.click(screen.getByRole('button', { name: 'Join group' }));

		expect(
			await screen.findByText('Join code must use the current format.')
		).toBeInTheDocument();
		expect(input).toHaveAttribute('aria-invalid', 'true');
		await waitFor(() => expect(input).toHaveFocus());
	});

	it('prevents repeated submission and accepts an already-active membership response', async () => {
		const response = deferred<GroupSummary>();
		const onSuccess = vi.fn();
		mocks.joinGroup.mockReturnValue(response.promise);
		render(JoinGroupDialog, { onSuccess });
		await openJoinDialog();
		await fireEvent.input(screen.getByLabelText('Group code'), {
			target: { value: ' abcd1234 ' }
		});

		const submit = screen.getByRole('button', { name: 'Join group' });
		await fireEvent.click(submit);

		expect(screen.getByRole('button', { name: /joining group/i })).toBeDisabled();
		expect(mocks.joinGroup).toHaveBeenCalledWith({ joinCode: 'ABCD1234' });
		await fireEvent.click(submit);
		expect(mocks.joinGroup).toHaveBeenCalledOnce();

		response.resolve(groupListFixture.groups[1]);
		await waitFor(() =>
			expect(onSuccess).toHaveBeenCalledWith(groupListFixture.groups[1])
		);
	});

	it('closes with Cancel and returns focus to its trigger', async () => {
		render(JoinGroupDialog, { onSuccess: vi.fn() });
		const trigger = screen.getByRole('button', { name: 'Join with code' });
		await fireEvent.click(trigger);
		await screen.findByRole('dialog', { name: 'Join a group' });

		await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));

		await waitFor(() =>
			expect(
				screen.queryByRole('dialog', { name: 'Join a group' })
			).not.toBeInTheDocument()
		);
		await waitFor(() => expect(trigger).toHaveFocus());
	});
});

async function openJoinDialog(): Promise<void> {
	await fireEvent.click(screen.getByRole('button', { name: 'Join with code' }));
	await screen.findByRole('dialog', { name: 'Join a group' });
}

function deferred<T>(): {
	promise: Promise<T>;
	resolve: (value: T) => void;
} {
	let resolve!: (value: T) => void;
	const promise = new Promise<T>((resolvePromise) => {
		resolve = resolvePromise;
	});

	return { promise, resolve };
}
