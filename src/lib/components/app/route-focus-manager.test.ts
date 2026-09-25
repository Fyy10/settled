import { render } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

type NavigationCallback = (navigation: {
	from: { url: URL } | null;
	to: { url: URL } | null;
}) => void;

const navigationMock = vi.hoisted(() => ({
	callback: undefined as NavigationCallback | undefined
}));

vi.mock('$app/navigation', () => ({
	afterNavigate: (callback: NavigationCallback) => {
		navigationMock.callback = callback;
	}
}));

import RouteFocusManager from './route-focus-manager.svelte';

describe('RouteFocusManager', () => {
	beforeEach(() => {
		navigationMock.callback = undefined;
		vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
			callback(0);
			return 1;
		});
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('moves DOM focus on path changes without stealing it for query-only tab changes', () => {
		const { container } = render(RouteFocusManager);
		const main = document.createElement('main');
		const heading = document.createElement('h1');
		const currentControl = document.createElement('button');

		main.id = 'main-content';
		heading.tabIndex = -1;
		main.append(heading, currentControl);
		container.append(main);
		currentControl.focus();

		expect(navigationMock.callback).toBeTypeOf('function');

		navigationMock.callback?.({
			from: { url: new URL('https://settled.example/groups/group-id?view=balances') },
			to: { url: new URL('https://settled.example/groups/group-id?view=members') }
		});
		expect(document.activeElement).toBe(currentControl);

		navigationMock.callback?.({
			from: { url: new URL('https://settled.example/groups/group-id?view=members') },
			to: { url: new URL('https://settled.example/groups/group-id/expenses/new') }
		});
		expect(document.activeElement).toBe(heading);
	});
});
