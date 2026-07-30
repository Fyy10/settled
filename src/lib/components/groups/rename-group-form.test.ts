import {
	fireEvent,
	render,
	screen,
	waitFor
} from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '$lib/api/errors';
import { groupDetailFixture } from '../../../tests/fixtures/api-contract';

import RenameGroupForm from './rename-group-form.svelte';

const mocks = vi.hoisted(() => ({
	renameGroup: vi.fn(),
	toastSuccess: vi.fn()
}));

vi.mock('$lib/api/groups', () => ({
	renameGroup: mocks.renameGroup
}));
vi.mock('$lib/config/public', () => ({
	API_BASE_URL: 'http://localhost:8080'
}));
vi.mock('svelte-sonner', () => ({
	toast: { success: mocks.toastSuccess }
}));

beforeEach(() => {
	mocks.renameGroup.mockReset();
	mocks.toastSuccess.mockReset();
});

describe('RenameGroupForm', () => {
	it('saves a trimmed Unicode name and reports only a failed revalidation afterward', async () => {
		const renamed = { ...groupDetailFixture.group, name: '湖の旅' };
		mocks.renameGroup.mockResolvedValue(renamed);
		const onSaved = vi.fn().mockResolvedValue(false);
		render(RenameGroupForm, {
			group: groupDetailFixture.group,
			onSaved,
			onProtectedError: vi.fn()
		});
		const input = screen.getByRole('textbox', { name: 'Group name' });
		expect(input).toHaveValue(groupDetailFixture.group.name);
		await fireEvent.input(input, { target: { value: '  湖の旅  ' } });

		await fireEvent.click(
			screen.getByRole('button', { name: 'Save changes' })
		);

		await waitFor(() =>
			expect(mocks.renameGroup).toHaveBeenCalledWith(
				groupDetailFixture.group.id,
				{ name: '湖の旅' },
				{ signal: expect.any(AbortSignal) }
			)
		);
		expect(onSaved).toHaveBeenCalledWith(renamed);
		expect(mocks.toastSuccess).toHaveBeenCalledWith('Changes saved');
		expect(
			await screen.findByText(
				'Changes were saved, but some group information could not be refreshed.'
			)
		).toBeInTheDocument();
		expect(input).toHaveValue('湖の旅');
	});

	it('does not treat a repeated CSRF 403 as proof of a role change', async () => {
		mocks.renameGroup.mockRejectedValue(
			new ApiError({
				status: 403,
				code: 'csrf_invalid',
				message: 'Invalid CSRF token.',
				fields: {}
			})
		);
		const onProtectedError = vi.fn();
		render(RenameGroupForm, {
			group: groupDetailFixture.group,
			onSaved: vi.fn(),
			onProtectedError
		});

		await fireEvent.click(
			screen.getByRole('button', { name: 'Save changes' })
		);

		expect(
			await screen.findByText(
				'Settled could not save the group name. Check it and try again.'
			)
		).toBeInTheDocument();
		expect(onProtectedError).not.toHaveBeenCalled();
	});
});
