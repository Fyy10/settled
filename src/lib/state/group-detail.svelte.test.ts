import { describe, expect, it, vi } from 'vitest';

import { ApiError, networkError } from '$lib/api/errors';
import type { GroupDetailResponse } from '$lib/api/types';
import {
	groupDetailFixture,
	groupListFixture
} from '../../tests/fixtures/api-contract';
import {
	GroupDetailContext,
	type GroupDetailLoader
} from './group-detail.svelte';

vi.mock('$lib/config/public', () => ({
	API_BASE_URL: 'http://localhost:8080'
}));

describe('route-scoped group detail context', () => {
	it('loads a group once and exposes an explicit refresh', async () => {
		const loader = vi.fn<GroupDetailLoader>().mockResolvedValue(groupDetailFixture);
		const context = new GroupDetailContext(loader);

		await expect(context.load(groupDetailFixture.group.id)).resolves.toEqual(
			groupDetailFixture
		);
		await context.load(groupDetailFixture.group.id);
		expect(loader).toHaveBeenCalledOnce();
		expect(context.status).toBe('ready');
		expect(context.members).toEqual(groupDetailFixture.members);

		await context.refreshDetail();
		expect(loader).toHaveBeenCalledTimes(2);
	});

	it('aborts an obsolete route request and ignores its late data', async () => {
		const requests = new Map<
			string,
			{
				signal: AbortSignal;
				resolve: (detail: GroupDetailResponse) => void;
			}
		>();
		const loader: GroupDetailLoader = (groupId, { signal }) =>
			new Promise((resolve) => {
				requests.set(groupId, { signal, resolve });
			});
		const context = new GroupDetailContext(loader);

		const oldRequest = context.load(groupDetailFixture.group.id);
		const nextGroup = {
			group: groupListFixture.groups[1],
			members: [
				{
					...groupDetailFixture.members[1],
					role: 'owner' as const,
					userId: groupListFixture.groups[1].ownerUserId
				}
			]
		};
		const nextRequest = context.load(nextGroup.group.id);

		expect(requests.get(groupDetailFixture.group.id)?.signal.aborted).toBe(true);
		requests.get(groupDetailFixture.group.id)?.resolve(groupDetailFixture);
		requests.get(nextGroup.group.id)?.resolve(nextGroup);

		await expect(oldRequest).resolves.toEqual(groupDetailFixture);
		await expect(nextRequest).resolves.toEqual(nextGroup);
		expect(context.group).toEqual(nextGroup.group);
		expect(context.members).toEqual(nextGroup.members);
	});

	it('clears all metadata when the group becomes hidden', async () => {
		const loader = vi
			.fn<GroupDetailLoader>()
			.mockResolvedValueOnce(groupDetailFixture)
			.mockRejectedValueOnce(
				new ApiError({
					status: 404,
					code: 'not_found',
					message: 'Not found.',
					fields: {}
				})
			);
		const context = new GroupDetailContext(loader);
		await context.load(groupDetailFixture.group.id);

		await expect(context.refreshDetail()).rejects.toMatchObject({ status: 404 });
		expect(context.status).toBe('hidden');
		expect(context.group).toBeNull();
		expect(context.members).toEqual([]);
	});

	it('rejects late metadata for a different route group', async () => {
		const context = new GroupDetailContext(
			vi.fn<GroupDetailLoader>().mockResolvedValue(groupDetailFixture)
		);

		await expect(context.load('different-group')).rejects.toThrow(
			'Group detail does not match the active route.'
		);
		expect(context.status).toBe('error');
		expect(context.group).toBeNull();
		expect(context.members).toEqual([]);
	});

	it('keeps an ordinary initial failure retryable and supports Task 20 updates', async () => {
		const loader = vi
			.fn<GroupDetailLoader>()
			.mockRejectedValueOnce(networkError(new TypeError('Offline')))
			.mockResolvedValueOnce(groupDetailFixture);
		const context = new GroupDetailContext(loader);

		await expect(context.load(groupDetailFixture.group.id)).rejects.toMatchObject({
			code: 'network_error'
		});
		expect(context.status).toBe('error');
		expect(context.group).toBeNull();

		await context.refreshDetail();
		const renamed = { ...groupDetailFixture.group, name: 'Lake Weekend' };
		context.updateGroup(renamed);
		expect(context.group?.name).toBe('Lake Weekend');

		context.replaceDetail({
			group: renamed,
			members: groupDetailFixture.members.slice(0, 1)
		});
		expect(context.members).toHaveLength(1);

		context.clear();
		expect(context.status).toBe('loading');
		expect(context.group).toBeNull();
		expect(context.members).toEqual([]);
	});
});
