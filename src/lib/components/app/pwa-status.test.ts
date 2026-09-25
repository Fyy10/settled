import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { registerDirtyForm } from '$lib/state/dirty-forms.svelte';
import PwaStatus from './pwa-status.svelte';

const mocks = vi.hoisted(() => ({ update: vi.fn(), options: undefined as undefined | {
	onNeedRefresh(): void;
	onNeedReload(): void;
	onRegisteredSW(url: string, registration: ServiceWorkerRegistration | undefined): void;
} }));
vi.mock('virtual:pwa-register', () => ({ registerSW: (options: typeof mocks.options) => { mocks.options = options; return mocks.update; } }));

beforeEach(() => {
	mocks.options = undefined;
	mocks.update.mockReset().mockResolvedValue(undefined);
	Object.defineProperty(navigator, 'onLine', { configurable: true, value: true });
});
afterEach(() => { cleanup(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });

function nativeLifecycle(initialController: object | null = {}) {
	const reload = vi.fn();
	const realWindow = window;
	vi.stubGlobal('window', new Proxy(realWindow, {
		get(target, property) {
			if (property === 'location') return { reload };
			const value = Reflect.get(target, property, target);
			return typeof value === 'function' ? value.bind(target) : value;
		}
	}));
	const serviceWorkers = Object.assign(new EventTarget(), { controller: initialController });
	vi.stubGlobal('navigator', new Proxy(navigator, {
		get(target, property) {
			return property === 'serviceWorker' ? serviceWorkers : Reflect.get(target, property, target);
		}
	}));
	const worker = new EventTarget();
	const registration = Object.assign(new EventTarget(), {
		installing: worker as EventTarget | null,
		waiting: null as EventTarget | null
	});
	return {
		reload, serviceWorkers, worker, registration,
		registered() {
			mocks.options!.onRegisteredSW('/sw.js', registration as unknown as ServiceWorkerRegistration);
		},
		waiting() {
			registration.waiting = worker;
			registration.installing = null;
			worker.dispatchEvent(new Event('statechange'));
		},
		activated() {
			registration.waiting = null;
			serviceWorkers.controller = {};
			serviceWorkers.dispatchEvent(new Event('controllerchange'));
		}
	};
}

describe('PWA status', () => {
	it('detects and reloads a native waiting update without Workbox callbacks', async () => {
		const lifecycle = nativeLifecycle();
		lifecycle.registration.installing = null;
		render(PwaStatus);
		await waitFor(() => expect(mocks.options).toBeDefined());
		lifecycle.registered();
		lifecycle.registration.installing = lifecycle.worker;
		lifecycle.registration.dispatchEvent(new Event('updatefound'));
		lifecycle.waiting();
		await fireEvent.click(await screen.findByRole('button', { name: 'Reload' }));
		expect(mocks.update).toHaveBeenCalledOnce();
		expect(lifecycle.reload).not.toHaveBeenCalled();
		lifecycle.activated();
		expect(lifecycle.reload).toHaveBeenCalledOnce();
		mocks.options!.onNeedReload();
		expect(lifecycle.reload).toHaveBeenCalledOnce();
	});

	it('does not reload when the first worker claims the page', async () => {
		const lifecycle = nativeLifecycle(null);
		render(PwaStatus);
		await waitFor(() => expect(mocks.options).toBeDefined());
		lifecycle.registered();
		lifecycle.activated();
		expect(lifecycle.reload).not.toHaveBeenCalled();
		expect(screen.queryByText('Update available')).not.toBeInTheDocument();
	});

	it.each(['dirty', 'blocking'])('defers native external activation while %s, then reloads directly', async (kind) => {
		const lifecycle = nativeLifecycle();
		const form = registerDirtyForm(kind === 'dirty', kind === 'blocking');
		try {
			render(PwaStatus);
			await waitFor(() => expect(mocks.options).toBeDefined());
			lifecycle.activated();
			expect(await screen.findByText('Update available')).toBeInTheDocument();
			expect(lifecycle.reload).not.toHaveBeenCalled();
			expect(screen.queryByRole('button', { name: 'Reload' })).not.toBeInTheDocument();
			form.update(false, false);
			await fireEvent.click(await screen.findByRole('button', { name: 'Reload' }));
			expect(lifecycle.reload).toHaveBeenCalledOnce();
			expect(mocks.update).not.toHaveBeenCalled();
		} finally { form.unregister(); }
	});

	it('finds an already waiting worker and removes native listeners on disposal', async () => {
		const lifecycle = nativeLifecycle();
		const removeController = vi.spyOn(lifecycle.serviceWorkers, 'removeEventListener');
		const removeRegistration = vi.spyOn(lifecycle.registration, 'removeEventListener');
		const removeWorker = vi.spyOn(lifecycle.worker, 'removeEventListener');
		lifecycle.registration.waiting = lifecycle.worker;
		const { unmount } = render(PwaStatus);
		await waitFor(() => expect(mocks.options).toBeDefined());
		lifecycle.registered();
		expect(await screen.findByText('Update available')).toBeInTheDocument();
		unmount();
		expect(removeController).toHaveBeenCalledWith('controllerchange', expect.any(Function));
		expect(removeRegistration).toHaveBeenCalledWith('updatefound', expect.any(Function));
		expect(removeWorker).toHaveBeenCalledWith('statechange', expect.any(Function));
		lifecycle.activated();
		mocks.options!.onNeedReload();
		expect(lifecycle.reload).not.toHaveBeenCalled();
	});

	it('announces offline state and clears it on reconnect', async () => {
		render(PwaStatus);
		Object.defineProperty(navigator, 'onLine', { configurable: true, value: false });
		window.dispatchEvent(new Event('offline'));
		expect(await screen.findByText('You’re offline. Saved information may be out of date.')).toBeInTheDocument();
		Object.defineProperty(navigator, 'onLine', { configurable: true, value: true });
		window.dispatchEvent(new Event('online'));
		await waitFor(() => expect(screen.queryByText('You’re offline. Saved information may be out of date.')).not.toBeInTheDocument());
	});

	it.each(['dirty', 'blocking'])('keeps a waiting update until the %s form is clean', async (kind) => {
		const form = registerDirtyForm(kind === 'dirty', kind === 'blocking');
		try {
			render(PwaStatus);
			await waitFor(() => expect(mocks.options).toBeDefined());
			mocks.options!.onNeedRefresh();
			expect(await screen.findByText('Update available')).toBeInTheDocument();
			expect(screen.queryByRole('button', { name: 'Reload' })).not.toBeInTheDocument();
			expect(mocks.update).not.toHaveBeenCalled();
			form.update(false, false);
			await fireEvent.click(await screen.findByRole('button', { name: 'Reload' }));
			expect(mocks.update).toHaveBeenCalledOnce();
		} finally { form.unregister(); }
	});

	it('rechecks both registries synchronously when a rendered Reload is clicked', async () => {
		render(PwaStatus);
		await waitFor(() => expect(mocks.options).toBeDefined());
		mocks.options!.onNeedRefresh();
		const button = await screen.findByRole('button', { name: 'Reload' });
		const form = registerDirtyForm(false, true);
		try { button.click(); expect(mocks.update).not.toHaveBeenCalled(); }
		finally { form.unregister(); }
	});

	it('keeps a draft intact when another tab activates an update', async () => {
		const form = registerDirtyForm(true);
		try {
			render(PwaStatus);
			await waitFor(() => expect(mocks.options).toBeDefined());
			mocks.options!.onNeedReload();
			expect(await screen.findByText('Update available')).toBeInTheDocument();
			expect(screen.queryByRole('button', { name: 'Reload' })).not.toBeInTheDocument();
			expect(mocks.update).not.toHaveBeenCalled();
		} finally { form.unregister(); }
	});
});
