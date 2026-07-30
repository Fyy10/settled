import { cleanup, render, waitFor } from '@testing-library/svelte';
import type { BeforeNavigate } from '@sveltejs/kit';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { hasDirtyForms } from '$lib/state/dirty-forms.svelte';

import UnsavedChangesGuard from './unsaved-changes-guard.svelte';

type NavigationCallback = (navigation: BeforeNavigate) => void;

const mocks = vi.hoisted(() => ({
	callbacks: [] as NavigationCallback[]
}));

vi.mock('$app/navigation', () => ({
	beforeNavigate: (callback: NavigationCallback) => {
		mocks.callbacks.push(callback);
	}
}));

beforeEach(() => {
	mocks.callbacks.length = 0;
	vi.stubGlobal('confirm', vi.fn());
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
	vi.restoreAllMocks();
});

describe('UnsavedChangesGuard', () => {
	it('does nothing while the form is clean', () => {
		render(UnsavedChangesGuard, {
			active: false,
			message: 'Discard unsaved changes?'
		});
		const { navigation, cancel } = createNavigation('goto');

		registeredCallback()(navigation);

		expect(globalThis.confirm).not.toHaveBeenCalled();
		expect(cancel).not.toHaveBeenCalled();
	});

	it('cancels an internal navigation when discarding is declined', () => {
		const confirm = vi.mocked(globalThis.confirm);
		confirm.mockReturnValue(false);
		render(UnsavedChangesGuard, {
			active: true,
			message: 'Discard unsaved changes?'
		});
		const { navigation, cancel } = createNavigation('goto');

		registeredCallback()(navigation);

		expect(confirm).toHaveBeenCalledWith('Discard unsaved changes?');
		expect(cancel).toHaveBeenCalledOnce();
	});

	it('allows an internal navigation when discarding is confirmed', () => {
		const confirm = vi.mocked(globalThis.confirm);
		confirm.mockReturnValue(true);
		render(UnsavedChangesGuard, {
			active: true,
			message: 'Discard unsaved changes?'
		});
		const { navigation, cancel } = createNavigation('goto');

		registeredCallback()(navigation);

		expect(confirm).toHaveBeenCalledWith('Discard unsaved changes?');
		expect(cancel).not.toHaveBeenCalled();
	});

	it('unconditionally blocks navigation while a mutation is pending', () => {
		const confirm = vi.mocked(globalThis.confirm);
		confirm.mockReturnValue(true);
		render(UnsavedChangesGuard, {
			active: true,
			blocked: true,
			message: 'Discard unsaved changes?'
		});
		const { navigation, cancel } = createNavigation('goto');

		registeredCallback()(navigation);

		expect(cancel).toHaveBeenCalledOnce();
		expect(confirm).not.toHaveBeenCalled();
	});

	it('allows the authoritative session-expiry redirect while pending', () => {
		render(UnsavedChangesGuard, {
			active: true,
			blocked: true,
			message: 'Discard unsaved changes?'
		});
		const { navigation, cancel } = createNavigation('goto');
		if (navigation.to !== null) {
			navigation.to.url = new URL(
				'https://settled.example/login?reason=session-expired&next=%2Fgroups'
			);
		}

		registeredCallback()(navigation);

		expect(cancel).not.toHaveBeenCalled();
		expect(globalThis.confirm).not.toHaveBeenCalled();
	});

	it('allows the committed logout redirect without a post-mutation prompt', () => {
		render(UnsavedChangesGuard, {
			active: true,
			message: 'Discard unsaved changes?'
		});
		const { navigation, cancel } = createNavigation('goto');
		if (navigation.to !== null) {
			navigation.to.url = new URL('https://settled.example/login');
		}

		registeredCallback()(navigation);

		expect(cancel).not.toHaveBeenCalled();
		expect(globalThis.confirm).not.toHaveBeenCalled();
	});

	it('delegates a leave navigation to the browser beforeunload prompt', () => {
		render(UnsavedChangesGuard, {
			active: true,
			message: 'Discard unsaved changes?'
		});
		const { navigation, cancel } = createNavigation('leave');

		registeredCallback()(navigation);

		expect(cancel).toHaveBeenCalledOnce();
		expect(globalThis.confirm).not.toHaveBeenCalled();
	});

	it.each([
		['declined', false, true],
		['confirmed', true, false]
	] as const)(
		'uses the custom confirmation for an external link when discarding is %s',
		(_label, confirmed, shouldCancel) => {
			const confirm = vi.mocked(globalThis.confirm);
			confirm.mockReturnValue(confirmed);
			render(UnsavedChangesGuard, {
				active: true,
				message: 'Discard unsaved changes?'
			});
			const { navigation, cancel } = createNavigation('link', true);

			registeredCallback()(navigation);

			expect(confirm).toHaveBeenCalledWith('Discard unsaved changes?');
			expect(cancel).toHaveBeenCalledTimes(shouldCancel ? 1 : 0);
		}
	);

	it('uses the latest active state and message without remounting', async () => {
		const confirm = vi.mocked(globalThis.confirm);
		confirm.mockReturnValue(false);
		const result = render(UnsavedChangesGuard, {
			active: false,
			message: 'Old message'
		});
		const callback = registeredCallback();

		await result.rerender({
			active: true,
			message: 'Discard the latest changes?'
		});
		const firstNavigation = createNavigation('goto');
		callback(firstNavigation.navigation);
		expect(confirm).toHaveBeenLastCalledWith(
			'Discard the latest changes?'
		);
		expect(firstNavigation.cancel).toHaveBeenCalledOnce();

		await result.rerender({
			active: false,
			message: 'Discard the latest changes?'
		});
		const secondNavigation = createNavigation('goto');
		callback(secondNavigation.navigation);
		expect(confirm).toHaveBeenCalledOnce();
		expect(secondNavigation.cancel).not.toHaveBeenCalled();
	});

	it('aggregates simultaneous guards and unregisters them on teardown', async () => {
		const first = render(UnsavedChangesGuard, {
			active: true,
			message: 'Discard first form?'
		});
		const second = render(UnsavedChangesGuard, {
			active: true,
			message: 'Discard second form?'
		});

		expect(hasDirtyForms.current).toBe(true);

		await first.rerender({
			active: false,
			message: 'Discard first form?'
		});
		expect(hasDirtyForms.current).toBe(true);

		second.unmount();
		await waitFor(() => expect(hasDirtyForms.current).toBe(false));
	});

	it('never persists draft state', async () => {
		const setItem = vi.spyOn(Storage.prototype, 'setItem');
		const result = render(UnsavedChangesGuard, {
			active: true,
			message: 'Discard unsaved changes?'
		});

		await waitFor(() => expect(hasDirtyForms.current).toBe(true));
		await result.rerender({
			active: false,
			message: 'Discard unsaved changes?'
		});

		expect(setItem).not.toHaveBeenCalled();
	});
});

function registeredCallback(): NavigationCallback {
	expect(mocks.callbacks).toHaveLength(1);
	return mocks.callbacks[0];
}

function createNavigation(
	type: 'goto' | 'link' | 'leave',
	willUnload = type === 'leave'
): {
	navigation: BeforeNavigate;
	cancel: ReturnType<typeof vi.fn>;
} {
	const cancel = vi.fn();
	const from = {
		params: { groupId: 'group-id' },
		route: { id: '/groups/[groupId]' },
		url: new URL('https://settled.example/groups/group-id')
	};
	const to =
		type === 'leave'
			? null
			: {
					params: type === 'link' && willUnload ? null : { groupId: 'group-id' },
					route: {
						id: type === 'link' && willUnload ? null : '/groups/[groupId]'
					},
					url: new URL(
						type === 'link' && willUnload
							? 'https://example.net/'
							: 'https://settled.example/groups/group-id?view=activity'
					)
				};

	return {
		navigation: {
			cancel,
			complete: new Promise<void>(() => undefined),
			delta: undefined,
			event:
				type === 'link'
					? ({ currentTarget: null } as unknown as PointerEvent)
					: undefined,
			from,
			to,
			type,
			willUnload
		} as unknown as BeforeNavigate,
		cancel
	};
}
