import { copy } from '$lib/copy/en';

let online = $state(typeof navigator === 'undefined' ? true : navigator.onLine);

export const networkState = {
	get online(): boolean {
		return online;
	},
	get mutationDisabledReason(): string | null {
		return online ? null : copy.pwa.mutationDisabled;
	}
};

/** Connectivity is only a hint; request failures still use ordinary API errors. */
export function startNetworkObserver(): () => void {
	if (typeof window === 'undefined') return () => {};
	const update = () => { online = navigator.onLine; };
	update();
	window.addEventListener('online', update);
	window.addEventListener('offline', update);
	return () => {
		window.removeEventListener('online', update);
		window.removeEventListener('offline', update);
	};
}
