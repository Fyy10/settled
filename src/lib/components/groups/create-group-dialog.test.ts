import { withNetworkState } from '../../../tests/network';
import {
	fireEvent,
	render,
	screen,
	waitFor,
	within
} from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '$lib/api/errors';
import type { GroupSummary } from '$lib/api/types';
import { groupListFixture } from '../../../tests/fixtures/api-contract';

import CreateGroupDialog from './create-group-dialog.svelte';

const mocks = vi.hoisted(() => ({
	createGroup: vi.fn()
}));

vi.mock('$lib/api/groups', () => ({ createGroup: mocks.createGroup }));
vi.mock('$lib/config/public', () => ({ API_BASE_URL: 'http://localhost:8080' }));

beforeEach(() => {
	mocks.createGroup.mockReset();
});

describe('CreateGroupDialog', () => {
	it('retains editable entries and blocks direct submission while offline', async () => {
		await withNetworkState(async (setOnline) => {
			render(CreateGroupDialog, { onSuccess: vi.fn() });
			await openCreateDialog();
			const input = screen.getByLabelText('Group name');
			await fireEvent.input(input, { target: { value: 'Offline trip' } });
			await setOnline(false);
			const button = screen.getAllByRole('button', { name: 'Create group' }).at(-1)!;
			expect(button).toBeDisabled();
			expect(
				screen.getByText('Reconnect to continue. Your entries will stay here.')
			).toBeInTheDocument();
			await fireEvent.submit(input.closest('form')!);
			expect(mocks.createGroup).not.toHaveBeenCalled();
			expect(input).toHaveValue('Offline trip');
			expect(input).toBeEnabled();
			await setOnline(true);
			expect(input).toHaveValue('Offline trip');
			expect(mocks.createGroup).not.toHaveBeenCalled();
		});
	});

	it('focuses the labelled group-name input when opened', async () => {
		render(CreateGroupDialog, { onSuccess: vi.fn() });
		const trigger = screen.getByRole('button', { name: 'Create group' });

		await fireEvent.click(trigger);

		expect(
			await screen.findByRole('dialog', { name: 'Create group' })
		).toBeInTheDocument();
		const input = screen.getByLabelText('Group name');
		await waitFor(() => expect(input).toHaveFocus());
		expect(input).toHaveAttribute('autocomplete', 'off');
	});

	it('validates locally and focuses the first invalid field', async () => {
		render(CreateGroupDialog, { onSuccess: vi.fn() });
		await openCreateDialog();

		const dialog = screen.getByRole('dialog', { name: 'Create group' });
		await fireEvent.click(
			within(dialog).getByRole('button', { name: 'Create group' })
		);

		const input = screen.getByLabelText('Group name');
		expect(await screen.findByText('Enter a group name.')).toHaveAttribute(
			'id',
			'create-group-name-error'
		);
		await waitFor(() => expect(input).toHaveFocus());
		expect(input).toHaveAttribute('aria-invalid', 'true');
		expect(mocks.createGroup).not.toHaveBeenCalled();
	});

	it('maps API field errors without discarding the draft', async () => {
		mocks.createGroup.mockRejectedValue(
			new ApiError({
				status: 422,
				code: 'validation_failed',
				message: 'One or more fields are invalid.',
				fields: { name: 'That group name is not available.' }
			})
		);
		render(CreateGroupDialog, { onSuccess: vi.fn() });
		await openCreateDialog();
		const input = screen.getByLabelText('Group name');
		await fireEvent.input(input, { target: { value: 'Lake Trip' } });

		const dialog = screen.getByRole('dialog', { name: 'Create group' });
		await fireEvent.click(
			within(dialog).getByRole('button', { name: 'Create group' })
		);

		expect(
			await screen.findByText('That group name is not available.')
		).toBeInTheDocument();
		expect(input).toHaveValue('Lake Trip');
		await waitFor(() => expect(input).toHaveFocus());
	});

	it('prevents dismissal and repeated submission while pending', async () => {
		const response = deferred<GroupSummary>();
		const onSuccess = vi.fn();
		mocks.createGroup.mockReturnValue(response.promise);
		render(CreateGroupDialog, { onSuccess });
		await openCreateDialog();
		await fireEvent.input(screen.getByLabelText('Group name'), {
			target: { value: ' Lake Trip ' }
		});

		const dialog = screen.getByRole('dialog', { name: 'Create group' });
		const submit = within(dialog).getByRole('button', { name: 'Create group' });
		await fireEvent.click(submit);

		expect(
			screen.getByRole('button', { name: /creating group/i })
		).toBeDisabled();
		expect(screen.getByRole('button', { name: 'Cancel' })).toBeDisabled();
		expect(mocks.createGroup).toHaveBeenCalledWith({ name: 'Lake Trip' });
		await fireEvent.click(submit);
		expect(mocks.createGroup).toHaveBeenCalledOnce();

		await fireEvent.keyDown(document, { key: 'Escape' });
		expect(screen.getByRole('dialog', { name: 'Create group' })).toBeInTheDocument();

		response.resolve(groupListFixture.groups[0]);
		await waitFor(() =>
			expect(onSuccess).toHaveBeenCalledWith(groupListFixture.groups[0])
		);
		await waitFor(() =>
			expect(
				screen.queryByRole('dialog', { name: 'Create group' })
			).not.toBeInTheDocument()
		);
		await waitFor(() =>
			expect(screen.getByRole('button', { name: 'Create group' })).toHaveFocus()
		);
	});

	it('closes with Escape or Cancel and returns focus to its trigger', async () => {
		render(CreateGroupDialog, { onSuccess: vi.fn() });
		const trigger = screen.getByRole('button', { name: 'Create group' });
		await fireEvent.click(trigger);
		await screen.findByRole('dialog', { name: 'Create group' });

		await fireEvent.keyDown(document, { key: 'Escape' });
		await waitFor(() =>
			expect(
				screen.queryByRole('dialog', { name: 'Create group' })
			).not.toBeInTheDocument()
		);
		await waitFor(() => expect(trigger).toHaveFocus());

		await fireEvent.click(trigger);
		await screen.findByRole('dialog', { name: 'Create group' });
		await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
		await waitFor(() =>
			expect(
				screen.queryByRole('dialog', { name: 'Create group' })
			).not.toBeInTheDocument()
		);
		await waitFor(() => expect(trigger).toHaveFocus());
	});
});

async function openCreateDialog(): Promise<void> {
	await fireEvent.click(screen.getByRole('button', { name: 'Create group' }));
	await screen.findByRole('dialog', { name: 'Create group' });
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
