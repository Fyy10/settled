import { expect, it, vi } from 'vitest';
import { networkState, startNetworkObserver } from './network.svelte';

it('reads initial connectivity, tracks transitions, and removes listeners', () => {
	const getter = vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false);
	const stop = startNetworkObserver();
	expect(networkState.online).toBe(false);
	expect(networkState.mutationDisabledReason).toBeTruthy();
	getter.mockReturnValue(true);
	window.dispatchEvent(new Event('online'));
	expect(networkState.online).toBe(true);
	expect(networkState.mutationDisabledReason).toBeNull();
	stop();
	getter.mockReturnValue(false);
	window.dispatchEvent(new Event('offline'));
	expect(networkState.online).toBe(true);
	getter.mockRestore();
});
