import {
	fireEvent,
	render,
	screen,
	waitFor,
	within
} from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { networkError } from '$lib/api/errors';
import type { GroupSummary } from '$lib/api/types';
import { groupListFixture } from '../../../tests/fixtures/api-contract';

import GroupsPage from './+page.svelte';

const mocks = vi.hoisted(() => ({
	goto: vi.fn(),
	listGroups: vi.fn(),
	createGroup: vi.fn(),
	joinGroup: vi.fn(),
	toastSuccess: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: mocks.goto }));
vi.mock('$lib/api/groups', () => ({
	listGroups: mocks.listGroups,
	createGroup: mocks.createGroup,
	joinGroup: mocks.joinGroup
}));
vi.mock('$lib/config/public', () => ({ API_BASE_URL: 'http://localhost:8080' }));
vi.mock('svelte-sonner', () => ({
	toast: { success: mocks.toastSuccess }
}));

beforeEach(() => {
	mocks.goto.mockReset().mockResolvedValue(undefined);
	mocks.listGroups.mockReset();
	mocks.createGroup.mockReset();
	mocks.joinGroup.mockReset();
	mocks.toastSuccess.mockReset();
});

describe('groups page states', () => {
	it('renders three stable card skeletons while loading', () => {
		mocks.listGroups.mockReturnValue(new Promise(() => {}));
		const { container } = render(GroupsPage);

		expect(
			screen.getByRole('heading', { level: 1, name: 'Your groups' })
		).toBeInTheDocument();
		expect(
			screen.getByRole('status', { name: 'Loading your groups' })
		).toBeInTheDocument();
		expect(container.querySelectorAll('[data-slot="card"]')).toHaveLength(3);
	});

	it('renders the prescribed empty state with both actions', async () => {
		mocks.listGroups.mockResolvedValue([]);
		render(GroupsPage);

		expect(await screen.findByText('No groups yet')).toBeInTheDocument();
		expect(
			screen.getByText(
				'Create a group for a trip or household, or join one with a code.'
			)
		).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Create group' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Join with code' })).toBeInTheDocument();
	});

	it('shows an inline error and retries the initial request', async () => {
		mocks.listGroups
			.mockRejectedValueOnce(networkError(new TypeError('Failed to fetch')))
			.mockResolvedValueOnce(groupListFixture.groups);
		render(GroupsPage);

		const alert = await screen.findByRole('alert');
		expect(alert).toHaveTextContent('Settled couldn’t load your groups.');
		expect(alert).toHaveTextContent('Check your connection and try again.');

		await fireEvent.click(screen.getByRole('button', { name: 'Retry' }));

		expect(
			await screen.findByRole('link', { name: /Open Lake Trip/ })
		).toBeInTheDocument();
		expect(mocks.listGroups).toHaveBeenCalledTimes(2);
	});

	it('renders only API-provided group metadata and an owner-only badge', async () => {
		mocks.listGroups.mockResolvedValue(groupListFixture.groups);
		render(GroupsPage);

		const lakeTrip = await screen.findByRole('link', { name: /Open Lake Trip/ });
		const household = screen.getByRole('link', { name: /Open Household/ });
		expect(lakeTrip).toHaveAttribute(
			'href',
			`/groups/${groupListFixture.groups[0].id}`
		);
		expect(household).toHaveAttribute(
			'href',
			`/groups/${groupListFixture.groups[1].id}`
		);
		expect(within(lakeTrip).getByText('2 members')).toBeInTheDocument();
		expect(within(household).getByText('4 members')).toBeInTheDocument();
		expect(screen.getAllByText('Owner')).toHaveLength(1);
		expect(screen.getByText(/Updated Jun 30, 2026/)).toBeInTheDocument();
		expect(screen.getByText(/Updated Jul 2, 2026/)).toBeInTheDocument();
		expect(screen.queryByText(/\$\d/)).not.toBeInTheDocument();
		expect(screen.queryByText(/balance/i)).not.toBeInTheDocument();
	});
});

describe('groups page mutations', () => {
	it('refreshes, toasts, and navigates after creating a group', async () => {
		mocks.listGroups.mockResolvedValue(groupListFixture.groups);
		mocks.createGroup.mockResolvedValue(groupListFixture.groups[0]);
		render(GroupsPage);
		await screen.findByRole('link', { name: /Open Lake Trip/ });

		await fireEvent.click(screen.getByRole('button', { name: 'Create group' }));
		const dialog = await screen.findByRole('dialog', { name: 'Create group' });
		await fireEvent.input(within(dialog).getByLabelText('Group name'), {
			target: { value: ' Lake Trip ' }
		});
		await fireEvent.click(
			within(dialog).getByRole('button', { name: 'Create group' })
		);

		await expectGroupNavigation(groupListFixture.groups[0], 'Group created');
		expect(mocks.createGroup).toHaveBeenCalledWith({ name: 'Lake Trip' });
		expect(mocks.listGroups).toHaveBeenCalledTimes(2);
	});

	it('navigates normally when join returns an already-active group', async () => {
		mocks.listGroups.mockResolvedValue(groupListFixture.groups);
		mocks.joinGroup.mockResolvedValue(groupListFixture.groups[1]);
		render(GroupsPage);
		await screen.findByRole('link', { name: /Open Lake Trip/ });

		await fireEvent.click(screen.getByRole('button', { name: 'Join with code' }));
		const dialog = await screen.findByRole('dialog', { name: 'Join a group' });
		await fireEvent.input(within(dialog).getByLabelText('Group code'), {
			target: { value: ' abcd1234 ' }
		});
		await fireEvent.click(
			within(dialog).getByRole('button', { name: 'Join group' })
		);

		await expectGroupNavigation(groupListFixture.groups[1], 'Group joined');
		expect(mocks.joinGroup).toHaveBeenCalledWith({ joinCode: 'ABCD1234' });
		expect(mocks.listGroups).toHaveBeenCalledTimes(2);
	});
});

async function expectGroupNavigation(
	group: GroupSummary,
	toastMessage: string
): Promise<void> {
	await waitFor(() => {
		expect(mocks.toastSuccess).toHaveBeenCalledWith(toastMessage);
		expect(mocks.goto).toHaveBeenCalledWith(`/groups/${group.id}`);
	});
}
