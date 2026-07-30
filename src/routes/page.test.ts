import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';

import RootPage from './+page.svelte';

describe('root page baseline', () => {
	it('provides one main landmark and one page heading', () => {
		render(RootPage);

		expect(screen.getAllByRole('main')).toHaveLength(1);
		expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1);
		expect(
			screen.getByRole('heading', { level: 1, name: 'Opening your ledger' })
		).toBeInTheDocument();
		expect(screen.getByRole('status', { name: 'Checking your session' })).toBeInTheDocument();
	});
});
