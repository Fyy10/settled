import { fireEvent } from '@testing-library/svelte';
import { vi } from 'vitest';
import { startNetworkObserver } from '$lib/state/network.svelte';

export async function withNetworkState(
	run: (setOnline: (online: boolean) => Promise<void>) => Promise<void>
): Promise<void> {
	const getter = vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true);
	const stop = startNetworkObserver();
	const setOnline = async (online: boolean) => {
		getter.mockReturnValue(online);
		await fireEvent(window, new Event(online ? 'online' : 'offline'));
	};
	try {
		await run(setOnline);
	} finally {
		await setOnline(true);
		stop();
		getter.mockRestore();
	}
}
