import { withNetworkState } from '../../../tests/network';
import {
	fireEvent,
	render,
	screen,
	waitFor,
	within
} from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError, networkError } from '$lib/api/errors';
import { groupDetailFixture } from '../../../tests/fixtures/api-contract';

import type { MemberNotFoundResolution } from './member-removal';
import RemoveMemberDialog from './remove-member-dialog.svelte';

const mocks = vi.hoisted(() => ({
	removeGroupMember: vi.fn(),
	toastSuccess: vi.fn(),
	toastError: vi.fn()
}));

vi.mock('$lib/api/groups', () => ({
	removeGroupMember: mocks.removeGroupMember
}));
vi.mock('$lib/config/public', () => ({
	API_BASE_URL: 'http://localhost:8080'
}));
vi.mock('svelte-sonner', () => ({
	toast: {
		success: mocks.toastSuccess,
		error: mocks.toastError
	}
}));

const member = groupDetailFixture.members[1];

beforeEach(() => {
	mocks.removeGroupMember.mockReset().mockResolvedValue(undefined);
	mocks.toastSuccess.mockReset();
	mocks.toastError.mockReset();
});

describe('RemoveMemberDialog', () => {
	it('blocks offline mutations and permits retry after reconnecting', async () => {
		await withNetworkState(async (setOnline) => {
			renderDialog();
			await openDialog();
			await setOnline(false);
			const button = screen.getAllByRole('button', { name: 'Remove member' }).at(-1)!;
			expect(button).toBeDisabled();
			expect(
				screen.getByText('Reconnect to continue. Your entries will stay here.')
			).toBeInTheDocument();
			await fireEvent.click(button);
			expect(mocks.removeGroupMember).not.toHaveBeenCalled();
			await setOnline(true);
			expect(button).toBeEnabled();
			expect(mocks.removeGroupMember).not.toHaveBeenCalled();
		});
	});

	it('keeps a 409 conflict open, retains the member action, and focuses the exact explanation', async () => {
		mocks.removeGroupMember.mockRejectedValue(
			new ApiError({
				status: 409,
				code: 'conflict',
				message: 'Conflict.',
				fields: {}
			})
		);
		renderDialog();
		await openDialog();
		const dialog = screen.getByRole('alertdialog');
		expect(dialog).toHaveClass('w-[calc(100%-2rem)]');

		await fireEvent.click(
			within(dialog).getByRole('button', { name: 'Remove member' })
		);

		const alert = await within(dialog).findByRole('alert');
		expect(alert).toHaveTextContent(
			'This member is used by a current expense or payment record. Update or delete those records before removing them.'
		);
		await waitFor(() => expect(alert).toHaveFocus());
		expect(screen.getByRole('alertdialog')).toBeInTheDocument();
		expect(
			within(dialog).getByRole('button', { name: 'Remove member' })
		).toBeEnabled();
	});

	it('keeps a network failure open and focused without treating it as committed', async () => {
		mocks.removeGroupMember.mockRejectedValue(
			networkError(new TypeError('Offline'))
		);
		renderDialog();
		await openDialog();
		const dialog = screen.getByRole('alertdialog');

		await fireEvent.click(
			within(dialog).getByRole('button', { name: 'Remove member' })
		);

		const alert = await within(dialog).findByRole('alert');
		expect(alert).toHaveTextContent('Settled can’t reach the server.');
		await waitFor(() => expect(alert).toHaveFocus());
		expect(screen.getByRole('alertdialog')).toBeInTheDocument();
	});

	it('locks a committed target even when follow-up revalidation rejects', async () => {
		const onCommitted = vi.fn().mockRejectedValue(new Error('Refresh failed'));
		const onCloseFocus = vi.fn();
		renderDialog({ onCommitted, onCloseFocus });
		await openDialog();

		await fireEvent.click(
			within(screen.getByRole('alertdialog')).getByRole('button', {
				name: 'Remove member'
			})
		);

		await waitFor(() =>
			expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
		);
		const trigger = screen.getByRole('button', { name: 'Remove member Bob' });
		expect(trigger).toBeDisabled();
		expect(mocks.removeGroupMember).toHaveBeenCalledOnce();
		expect(onCommitted).toHaveBeenCalledWith(member.userId);
		expect(onCloseFocus).toHaveBeenCalled();
		await waitFor(() =>
			expect(mocks.toastError).toHaveBeenCalledWith(
				'The member was removed, but some group information could not be refreshed.'
			)
		);

		await fireEvent.click(trigger);
		expect(mocks.removeGroupMember).toHaveBeenCalledOnce();
	});

	it.each([
		['removed', true, false],
		['handled', false, false],
		['unresolved', false, true]
	] as const)(
		'resolves an ambiguous 404 as %s without blindly repeating DELETE',
		async (resolution, commits, staysOpen) => {
			mocks.removeGroupMember.mockRejectedValue(
				new ApiError({
					status: 404,
					code: 'not_found',
					message: 'Not found.',
					fields: {}
				})
			);
			const onCommitted = vi.fn().mockResolvedValue(undefined);
			const onNotFound = vi.fn().mockResolvedValue(
				resolution as MemberNotFoundResolution
			);
			renderDialog({ onCommitted, onNotFound });
			await openDialog();

			await fireEvent.click(
				within(screen.getByRole('alertdialog')).getByRole('button', {
					name: 'Remove member'
				})
			);

			await waitFor(() =>
				expect(onNotFound).toHaveBeenCalledWith(member.userId)
			);
			expect(onCommitted).toHaveBeenCalledTimes(commits ? 1 : 0);
			if (staysOpen) {
				expect(screen.getByRole('alertdialog')).toBeInTheDocument();
				expect(
					await screen.findByText(
						'Settled could not remove this member. Try again.'
					)
				).toBeInTheDocument();
			} else {
				await waitFor(() =>
					expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
				);
			}
			expect(mocks.removeGroupMember).toHaveBeenCalledOnce();
		}
	);
});

function renderDialog({
	onCommitted = vi.fn().mockResolvedValue(undefined),
	onNotFound = vi.fn().mockResolvedValue('unresolved'),
	onProtectedError = vi.fn().mockResolvedValue(false),
	onCloseFocus = vi.fn()
}: {
	onCommitted?: (userId: string) => void | Promise<void>;
	onNotFound?: (
		userId: string
	) => MemberNotFoundResolution | Promise<MemberNotFoundResolution>;
	onProtectedError?: (error: ApiError) => boolean | Promise<boolean>;
	onCloseFocus?: () => void;
} = {}): void {
	render(RemoveMemberDialog, {
		groupId: groupDetailFixture.group.id,
		member,
		onCommitted,
		onNotFound,
		onProtectedError,
		onCloseFocus
	});
}

async function openDialog(): Promise<void> {
	await fireEvent.click(
		screen.getByRole('button', { name: 'Remove member Bob' })
	);
	await screen.findByRole('alertdialog');
}
