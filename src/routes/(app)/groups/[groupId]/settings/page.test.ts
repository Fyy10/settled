import {
	fireEvent,
	render,
	screen,
	waitFor,
	within
} from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '$lib/api/errors';
import type { GroupSummary } from '$lib/api/types';
import {
	groupDetailFixture,
	groupListFixture
} from '../../../../../tests/fixtures/api-contract';

import SettingsPage from './+page.svelte';

const mocks = vi.hoisted(() => ({
	goto: vi.fn(),
	renameGroup: vi.fn(),
	getGroupJoinCode: vi.fn(),
	dissolveGroup: vi.fn(),
	removeGroupMember: vi.fn(),
	toastSuccess: vi.fn(),
	toastError: vi.fn(),
	groupDetail: {
		groupId: '',
		status: 'ready' as 'loading' | 'ready' | 'error' | 'hidden',
		group: null as GroupSummary | null,
		members: [] as (typeof groupDetailFixture)['members'][number][],
		updateGroup: vi.fn(),
		refreshDetail: vi.fn(),
		markHidden: vi.fn(),
		clear: vi.fn()
	},
	groupList: {
		refresh: vi.fn()
	}
}));

vi.mock('$app/navigation', () => ({ goto: mocks.goto }));
vi.mock('$lib/api/groups', () => ({
	renameGroup: mocks.renameGroup,
	getGroupJoinCode: mocks.getGroupJoinCode,
	dissolveGroup: mocks.dissolveGroup,
	removeGroupMember: mocks.removeGroupMember
}));
vi.mock('$lib/config/public', () => ({
	API_BASE_URL: 'http://localhost:8080'
}));
vi.mock('$lib/state/group-detail.svelte', () => ({
	useGroupDetailContext: () => mocks.groupDetail
}));
vi.mock('$lib/state/group-list.svelte', () => ({
	useGroupListContext: () => mocks.groupList
}));
vi.mock('svelte-sonner', () => ({
	toast: {
		success: mocks.toastSuccess,
		error: mocks.toastError
	}
}));

beforeEach(() => {
	vi.stubGlobal(
		'IntersectionObserver',
		class {
			observe(): void {}
			disconnect(): void {}
			unobserve(): void {}
			takeRecords(): IntersectionObserverEntry[] {
				return [];
			}
		}
	);
	mocks.goto.mockReset().mockResolvedValue(undefined);
	mocks.renameGroup.mockReset();
	mocks.getGroupJoinCode.mockReset().mockResolvedValue('ABCD1234');
	mocks.dissolveGroup.mockReset().mockResolvedValue(undefined);
	mocks.removeGroupMember.mockReset();
	mocks.toastSuccess.mockReset();
	mocks.toastError.mockReset();
	mocks.groupDetail.groupId = groupDetailFixture.group.id;
	mocks.groupDetail.status = 'ready';
	mocks.groupDetail.group = groupDetailFixture.group;
	mocks.groupDetail.members = [...groupDetailFixture.members];
	mocks.groupDetail.updateGroup.mockReset();
	mocks.groupDetail.refreshDetail.mockReset().mockResolvedValue(groupDetailFixture);
	mocks.groupDetail.markHidden.mockReset();
	mocks.groupDetail.clear.mockReset();
	mocks.groupList.refresh.mockReset().mockResolvedValue(groupListFixture.groups);
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('owner settings route', () => {
	it('never renders owner content or requests the code for a direct member visit', async () => {
		mocks.groupDetail.group = {
			...groupListFixture.groups[1],
			id: groupDetailFixture.group.id
		};

		render(SettingsPage);

		await waitFor(() =>
			expect(mocks.goto).toHaveBeenCalledWith(
				`/groups/${groupDetailFixture.group.id}`,
				{ replaceState: true }
			)
		);
		expect(
			screen.queryByRole('heading', { level: 1, name: 'Group settings' })
		).not.toBeInTheDocument();
		expect(mocks.getGroupJoinCode).not.toHaveBeenCalled();
	});

	it('re-reads detail and redirects when a rename proves the role changed', async () => {
		mocks.renameGroup.mockRejectedValue(
			new ApiError({
				status: 403,
				code: 'forbidden',
				message: 'Forbidden.',
				fields: {}
			})
		);
		mocks.groupDetail.refreshDetail.mockResolvedValue({
			...groupDetailFixture,
			group: {
				...groupDetailFixture.group,
				currentUserRole: 'member'
			}
		});
		render(SettingsPage);

		await fireEvent.click(
			screen.getByRole('button', { name: 'Save changes' })
		);

		await waitFor(() =>
			expect(mocks.goto).toHaveBeenCalledWith(
				`/groups/${groupDetailFixture.group.id}`,
				{ replaceState: true }
			)
		);
		expect(mocks.groupDetail.refreshDetail).toHaveBeenCalledOnce();
		expect(mocks.getGroupJoinCode).not.toHaveBeenCalled();
	});

	it('refreshes real detail and app-scoped list state after rename', async () => {
		const renamed = { ...groupDetailFixture.group, name: 'Beach Trip' };
		mocks.renameGroup.mockResolvedValue(renamed);
		render(SettingsPage);
		await fireEvent.input(screen.getByRole('textbox', { name: 'Group name' }), {
			target: { value: 'Beach Trip' }
		});

		await fireEvent.click(
			screen.getByRole('button', { name: 'Save changes' })
		);

		await waitFor(() => {
			expect(mocks.groupDetail.updateGroup).toHaveBeenCalledWith(renamed);
			expect(mocks.groupDetail.refreshDetail).toHaveBeenCalledOnce();
			expect(mocks.groupList.refresh).toHaveBeenCalledWith({ preserve: true });
			expect(mocks.toastSuccess).toHaveBeenCalledWith('Changes saved');
		});
	});

	it('treats dissolution as committed when list refresh fails and still replaces history', async () => {
		mocks.groupList.refresh.mockRejectedValue(new TypeError('Offline'));
		render(SettingsPage);
		await fireEvent.click(
			screen.getByRole('button', { name: 'Dissolve group' })
		);
		const dialog = await screen.findByRole('alertdialog');
		await fireEvent.input(
			within(dialog).getByLabelText('Type the current group name to confirm'),
			{ target: { value: groupDetailFixture.group.name } }
		);

		await fireEvent.click(
			within(dialog).getByRole('button', { name: 'Dissolve group' })
		);

		await waitFor(() => {
			expect(mocks.dissolveGroup).toHaveBeenCalledOnce();
			expect(mocks.groupList.refresh).toHaveBeenCalledWith({ preserve: true });
			expect(mocks.goto).toHaveBeenCalledWith('/groups', {
				replaceState: true
			});
			expect(mocks.groupDetail.markHidden).toHaveBeenCalledOnce();
			expect(mocks.groupDetail.clear).not.toHaveBeenCalled();
			expect(mocks.toastSuccess).toHaveBeenCalledWith('Group dissolved');
			expect(mocks.toastError).toHaveBeenCalledWith(
				'The group was dissolved, but the group list could not be refreshed before navigation.'
			);
		});
	});

	it('clears dissolved metadata before containing a navigation failure', async () => {
		mocks.goto.mockRejectedValue(new Error('Navigation failed'));
		render(SettingsPage);
		await fireEvent.click(
			screen.getByRole('button', { name: 'Dissolve group' })
		);
		const dialog = await screen.findByRole('alertdialog');
		await fireEvent.input(
			within(dialog).getByLabelText('Type the current group name to confirm'),
			{ target: { value: groupDetailFixture.group.name } }
		);

		await fireEvent.click(
			within(dialog).getByRole('button', { name: 'Dissolve group' })
		);

		await waitFor(() => {
			expect(mocks.groupDetail.markHidden).toHaveBeenCalledOnce();
			expect(mocks.goto).toHaveBeenCalledWith('/groups', {
				replaceState: true
			});
			expect(mocks.toastError).toHaveBeenCalledWith(
				'The group was dissolved, but Settled could not open your groups.'
			);
		});
		expect(mocks.dissolveGroup).toHaveBeenCalledOnce();
	});
});
