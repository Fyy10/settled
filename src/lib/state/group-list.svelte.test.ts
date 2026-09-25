import { describe, expect, it, vi } from 'vitest';

import type { GroupSummary } from '$lib/api/types';
import { groupListFixture } from '../../tests/fixtures/api-contract';
import {
	GroupListContext,
	type GroupListLoader
} from './group-list.svelte';

vi.mock('$lib/config/public', () => ({
	API_BASE_URL: 'http://localhost:8080'
}));

describe('app-scoped group list context', () => {
	it('retains a successful list and supports an explicit preserving refresh', async () => {
		const loader = vi
			.fn<GroupListLoader>()
			.mockResolvedValue(groupListFixture.groups);
		const context = new GroupListContext(loader);

		await expect(context.refresh()).resolves.toEqual(groupListFixture.groups);
		expect(context.status).toBe('ready');
		expect(context.groups).toEqual(groupListFixture.groups);

		await context.refresh({ preserve: true });
		expect(loader).toHaveBeenCalledTimes(2);
		expect(context.status).toBe('ready');
	});

	it('aborts a stale refresh and keeps only the latest response', async () => {
		const requests: Array<{
			signal: AbortSignal;
			resolve: (groups: GroupSummary[]) => void;
		}> = [];
		const loader: GroupListLoader = ({ signal }) =>
			new Promise((resolve) => requests.push({ signal, resolve }));
		const context = new GroupListContext(loader);

		const stale = context.refresh();
		const latest = context.refresh();
		expect(requests[0].signal.aborted).toBe(true);

		requests[0].resolve([groupListFixture.groups[1]]);
		requests[1].resolve(groupListFixture.groups);
		await expect(stale).resolves.toEqual([groupListFixture.groups[1]]);
		await expect(latest).resolves.toEqual(groupListFixture.groups);
		expect(context.groups).toEqual(groupListFixture.groups);
	});

	it('preserves ready data on refresh failure and clears app-lifetime state', async () => {
		const loader = vi
			.fn<GroupListLoader>()
			.mockResolvedValueOnce(groupListFixture.groups)
			.mockRejectedValueOnce(new TypeError('Offline'));
		const context = new GroupListContext(loader);
		await context.refresh();

		await expect(context.refresh({ preserve: true })).rejects.toThrow('Offline');
		expect(context.status).toBe('ready');
		expect(context.groups).toEqual(groupListFixture.groups);
		expect(context.error).toBeInstanceOf(TypeError);

		context.clear();
		expect(context.status).toBe('idle');
		expect(context.groups).toEqual([]);
	});
});
