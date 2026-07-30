import { fireEvent, render, screen } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { groupDetailFixture, groupListFixture } from '../../../tests/fixtures/api-contract';

import GroupWorkspaceHeader from './group-workspace-header.svelte';

const mocks = vi.hoisted(() => ({
	goto: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: mocks.goto }));

beforeEach(() => {
	mocks.goto.mockReset().mockResolvedValue(undefined);
});

describe('group workspace header', () => {
	it('keeps a long Unicode name intact and exposes owner settings', async () => {
		const group = {
			...groupDetailFixture.group,
			name: `家庭旅行 ${'Long name '.repeat(12)}`.trim()
		};
		render(GroupWorkspaceHeader, { group });

		expect(screen.getByRole('heading', { level: 1, name: group.name })).toBeInTheDocument();
		expect(screen.getByText('2 members')).toBeInTheDocument();
		expect(screen.getByText('Owner')).toBeInTheDocument();
		expect(screen.getAllByRole('link', { name: 'Add expense' })).toHaveLength(2);

		await fireEvent.click(
			screen.getAllByRole('button', { name: 'Open group menu' })[0]
		);
		await fireEvent.click(
			await screen.findByRole('menuitem', { name: 'Group settings' })
		);

		expect(mocks.goto).toHaveBeenCalledWith(
			`/groups/${groupDetailFixture.group.id}/settings`
		);
	});

	it('does not render owner-only settings for a member', () => {
		render(GroupWorkspaceHeader, { group: groupListFixture.groups[1] });

		expect(
			screen.queryByRole('button', { name: 'Open group menu' })
		).not.toBeInTheDocument();
		expect(screen.queryByText('Owner')).not.toBeInTheDocument();
	});
});
