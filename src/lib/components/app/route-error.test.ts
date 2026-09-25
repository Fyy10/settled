import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';

import RouteError from './route-error.svelte';

describe('RouteError', () => {
	it('renders persistent recovery actions with accessible semantics', async () => {
		const onRetry = vi.fn();

		render(RouteError, {
			onRetry,
			backHref: '/groups',
			backLabel: 'Back to groups'
		});

		expect(
			screen.getByRole('heading', { level: 1, name: 'Something went wrong' })
		).toBeInTheDocument();

		const alert = screen.getByRole('alert');
		expect(within(alert).getByText('This page could not be shown.')).toBeInTheDocument();
		expect(screen.getByRole('link', { name: 'Back to groups' })).toHaveAttribute(
			'href',
			'/groups'
		);

		const retry = screen.getByRole('button', { name: 'Try again' });
		expect(retry).toHaveClass('h-11');

		await fireEvent.click(retry);
		expect(onRetry).toHaveBeenCalledOnce();
	});
});
