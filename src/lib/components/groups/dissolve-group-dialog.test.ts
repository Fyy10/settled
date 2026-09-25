import { withNetworkState } from '../../../tests/network';
import {
	fireEvent,
	render,
	screen,
	waitFor,
	within
} from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { networkError } from '$lib/api/errors';
import { groupDetailFixture } from '../../../tests/fixtures/api-contract';

import DissolveGroupDialog from './dissolve-group-dialog.svelte';

const mocks = vi.hoisted(() => ({
	dissolveGroup: vi.fn()
}));

vi.mock('$lib/api/groups', () => ({
	dissolveGroup: mocks.dissolveGroup
}));
vi.mock('$lib/config/public', () => ({
	API_BASE_URL: 'http://localhost:8080'
}));

beforeEach(() => {
	mocks.dissolveGroup.mockReset().mockResolvedValue(undefined);
});

describe('DissolveGroupDialog', () => {
	it('retains editable entries and blocks direct submission while offline', async () => {
		await withNetworkState(async (setOnline) => {
			render(DissolveGroupDialog, {
				group: groupDetailFixture.group,
				onDissolved: vi.fn(),
				onProtectedError: vi.fn()
			});
			await openDialog();
			const input = screen.getByLabelText('Type the current group name to confirm');
			await fireEvent.input(input, { target: { value: 'Lake Trip' } });
			await setOnline(false);
			const button = screen.getAllByRole('button', { name: 'Dissolve group' }).at(-1)!;
			expect(button).toBeDisabled();
			expect(
				screen.getByText('Reconnect to continue. Your entries will stay here.')
			).toBeInTheDocument();
			await fireEvent.submit(input.closest('form')!);
			expect(mocks.dissolveGroup).not.toHaveBeenCalled();
			expect(input).toHaveValue('Lake Trip');
			expect(input).toBeEnabled();
			await setOnline(true);
			expect(input).toHaveValue('Lake Trip');
			expect(mocks.dissolveGroup).not.toHaveBeenCalled();
		});
	});

	it('requires an exact case-sensitive name and locks the committed mutation', async () => {
		const continuation = deferred<void>();
		const onDissolved = vi.fn().mockReturnValue(continuation.promise);
		render(DissolveGroupDialog, {
			group: groupDetailFixture.group,
			onDissolved,
			onProtectedError: vi.fn()
		});
		await openDialog();
		const dialog = screen.getByRole('alertdialog');
		expect(dialog).toHaveClass('w-[calc(100%-2rem)]');
		const input = within(dialog).getByLabelText(
			'Type the current group name to confirm'
		);
		const submit = within(dialog).getByRole('button', {
			name: 'Dissolve group'
		});

		await waitFor(() => expect(input).toHaveFocus());
		expect(submit).toBeDisabled();
		await fireEvent.input(input, { target: { value: 'lake trip' } });
		expect(submit).toBeDisabled();
		await fireEvent.input(input, { target: { value: 'Lake Trip' } });
		expect(submit).toBeEnabled();

		await fireEvent.click(submit);

		await waitFor(() => expect(mocks.dissolveGroup).toHaveBeenCalledOnce());
		expect(mocks.dissolveGroup.mock.calls[0][0]).toBe(
			groupDetailFixture.group.id
		);
		expect(mocks.dissolveGroup.mock.calls[0][1].signal).toBeInstanceOf(
			AbortSignal
		);
		expect(onDissolved).toHaveBeenCalledOnce();
		expect(
			within(dialog).getByRole('button', { name: /Group dissolved/ })
		).toBeDisabled();
		expect(within(dialog).getByRole('button', { name: 'Cancel' })).toBeDisabled();

		await fireEvent.keyDown(document, { key: 'Escape' });
		expect(screen.getByRole('alertdialog')).toBeInTheDocument();
		await fireEvent.click(
			within(dialog).getByRole('button', { name: /Group dissolved/ })
		);
		expect(mocks.dissolveGroup).toHaveBeenCalledOnce();
		continuation.resolve();
	});

	it('keeps the dialog open and focuses a network error before commit', async () => {
		mocks.dissolveGroup.mockRejectedValue(
			networkError(new TypeError('Offline'))
		);
		render(DissolveGroupDialog, {
			group: groupDetailFixture.group,
			onDissolved: vi.fn(),
			onProtectedError: vi.fn()
		});
		await openDialog();
		const dialog = screen.getByRole('alertdialog');
		await fireEvent.input(
			within(dialog).getByLabelText('Type the current group name to confirm'),
			{ target: { value: groupDetailFixture.group.name } }
		);
		await fireEvent.click(
			within(dialog).getByRole('button', { name: 'Dissolve group' })
		);

		const alert = await within(dialog).findByRole('alert');
		expect(alert).toHaveTextContent('Settled can’t reach the server.');
		await waitFor(() => expect(alert).toHaveFocus());
		expect(screen.getByRole('alertdialog')).toBeInTheDocument();
		expect(
			within(dialog).getByRole('button', { name: 'Dissolve group' })
		).toBeEnabled();
	});

	it('offers navigation-only recovery without repeating a committed dissolve', async () => {
		const onDissolved = vi.fn().mockRejectedValue(new Error('Navigation failed'));
		render(DissolveGroupDialog, {
			group: groupDetailFixture.group,
			onDissolved,
			onProtectedError: vi.fn()
		});
		await openDialog();
		const dialog = screen.getByRole('alertdialog');
		await fireEvent.input(
			within(dialog).getByLabelText('Type the current group name to confirm'),
			{ target: { value: groupDetailFixture.group.name } }
		);
		await fireEvent.click(
			within(dialog).getByRole('button', { name: 'Dissolve group' })
		);

		expect(
			await within(dialog).findByText(
				'The group was dissolved, but Settled could not open your groups.'
			)
		).toBeInTheDocument();
		expect(
			within(dialog).getByRole('link', { name: 'Continue to groups' })
		).toHaveAttribute('href', '/groups');
		expect(mocks.dissolveGroup).toHaveBeenCalledOnce();
		expect(onDissolved).toHaveBeenCalledOnce();
	});
});

async function openDialog(): Promise<void> {
	await fireEvent.click(
		screen.getByRole('button', { name: 'Dissolve group' })
	);
	await screen.findByRole('alertdialog');
}

function deferred<T>(): {
	promise: Promise<T>;
	resolve: (value: T | PromiseLike<T>) => void;
} {
	let resolve!: (value: T | PromiseLike<T>) => void;
	const promise = new Promise<T>((complete) => {
		resolve = complete;
	});

	return { promise, resolve };
}
