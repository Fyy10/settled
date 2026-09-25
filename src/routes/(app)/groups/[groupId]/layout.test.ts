import { createRawSnippet } from 'svelte';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError, networkError } from '$lib/api/errors';
import { groupDetailFixture } from '../../../../tests/fixtures/api-contract';

import GroupLayout from './+layout.svelte';

const mocks = vi.hoisted(() => ({
	getGroupDetail: vi.fn()
}));

vi.mock('$lib/api/groups', () => ({
	getGroupDetail: mocks.getGroupDetail
}));
vi.mock('$lib/config/public', () => ({
	API_BASE_URL: 'http://localhost:8080'
}));

const children = createRawSnippet(() => ({
	render: () => '<p>Group child route</p>'
}));

beforeEach(() => {
	mocks.getGroupDetail.mockReset();
});

describe('group route boundary', () => {
	it('shows the hidden-group copy and never retains group metadata after a 404', async () => {
		mocks.getGroupDetail.mockRejectedValue(
			new ApiError({
				status: 404,
				code: 'not_found',
				message: 'Not found.',
				fields: {}
			})
		);

		render(GroupLayout, {
			data: { groupId: groupDetailFixture.group.id },
			children
		});

		expect(
			await screen.findByRole('heading', {
				level: 1,
				name: 'This group isn’t available.'
			})
		).toBeInTheDocument();
		expect(screen.queryByText(groupDetailFixture.group.name)).not.toBeInTheDocument();
		expect(document.title).toBe('Group · Settled');
	});

	it('keeps an ordinary detail failure retryable at the route boundary', async () => {
		mocks.getGroupDetail
			.mockRejectedValueOnce(networkError(new TypeError('Failed to fetch')))
			.mockResolvedValueOnce(groupDetailFixture);

		render(GroupLayout, {
			data: { groupId: groupDetailFixture.group.id },
			children
		});

		expect(
			await screen.findByRole('heading', {
				level: 1,
				name: 'Settled couldn’t load this group.'
			})
		).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Retry' }));

		await waitFor(() => expect(mocks.getGroupDetail).toHaveBeenCalledTimes(2));
		await waitFor(() => {
			expect(
				screen.queryByText('Settled couldn’t load this group.')
			).not.toBeInTheDocument();
		});
		expect(document.title).toBe('Lake Trip · Settled');
	});
});
