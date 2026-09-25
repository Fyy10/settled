import { describe, expect, it, vi } from 'vitest';

import { finishCommittedMutation } from './committed-mutation';

describe('finishCommittedMutation', () => {
	it('attempts navigation even when authoritative refresh fails', async () => {
		const refresh = vi.fn().mockRejectedValue(new Error('offline'));
		const navigate = vi.fn().mockResolvedValue(undefined);

		await expect(
			finishCommittedMutation({ refresh, navigate })
		).resolves.toEqual({
			refresh: 'failed',
			navigation: 'succeeded'
		});
		expect(navigate).toHaveBeenCalledOnce();
	});

	it('does not navigate until a failed refresh operation has fully settled', async () => {
		const refresh = deferred<void>();
		const navigate = vi.fn().mockResolvedValue(undefined);
		const result = finishCommittedMutation({
			refresh: () => refresh.promise,
			navigate
		});

		expect(navigate).not.toHaveBeenCalled();
		refresh.reject(new Error('refreshes settled'));
		await expect(result).resolves.toEqual({
			refresh: 'failed',
			navigation: 'succeeded'
		});
		expect(navigate).toHaveBeenCalledOnce();
	});

	it('reports navigation failure without making the committed mutation retryable', async () => {
		await expect(
			finishCommittedMutation({
				refresh: vi.fn().mockResolvedValue(undefined),
				navigate: vi.fn().mockRejectedValue(new Error('navigation failed'))
			})
		).resolves.toEqual({
			refresh: 'succeeded',
			navigation: 'failed'
		});
	});
});

function deferred<T>(): {
	promise: Promise<T>;
	reject: (reason: unknown) => void;
} {
	let reject!: (reason: unknown) => void;
	const promise = new Promise<T>((_resolve, fail) => {
		reject = fail;
	});
	return { promise, reject };
}
