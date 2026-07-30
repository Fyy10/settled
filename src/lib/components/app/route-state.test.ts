import { render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';

import RouteState from './route-state.svelte';

describe('RouteState', () => {
	it('renders a stable and accessible route placeholder', () => {
		const { container } = render(RouteState, {
			title: 'Your groups',
			description: 'Your private shared ledgers live here.',
			loadingLabel: 'Loading groups'
		});

		expect(screen.getByRole('heading', { level: 1, name: 'Your groups' })).toBeInTheDocument();
		expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1);
		expect(screen.getByText('Your private shared ledgers live here.')).toBeInTheDocument();
		expect(screen.getByRole('status', { name: 'Loading groups' })).toBeInTheDocument();

		const hiddenLedger = container.querySelector('[aria-hidden="true"]');
		expect(hiddenLedger).toBeInTheDocument();
		expect(within(hiddenLedger as HTMLElement).queryByRole('status')).not.toBeInTheDocument();
		expect(hiddenLedger?.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(9);
	});
});
