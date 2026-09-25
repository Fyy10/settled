import { act, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';

import RouteLoadingAnnouncer from './route-loading-announcer.svelte';

describe('RouteLoadingAnnouncer', () => {
	afterEach(() => {
		vi.useRealTimers();
	});

	it('holds a delayed announcement for at least one second after it appears', async () => {
		vi.useFakeTimers();

		const { rerender } = render(RouteLoadingAnnouncer, {
			active: true,
			message: 'Loading the next page'
		});

		expect(screen.queryByText('Loading the next page')).not.toBeInTheDocument();

		await act(() => vi.advanceTimersByTime(299));
		expect(screen.queryByText('Loading the next page')).not.toBeInTheDocument();

		await act(() => vi.advanceTimersByTime(1));
		expect(screen.getByText('Loading the next page')).toHaveAttribute('aria-live', 'polite');

		await rerender({ active: false, message: 'Loading the next page' });
		await act(() => vi.advanceTimersByTime(999));
		expect(screen.getByText('Loading the next page')).toBeInTheDocument();

		await act(() => vi.advanceTimersByTime(1));
		expect(screen.queryByText('Loading the next page')).not.toBeInTheDocument();
	});

	it('does not announce a navigation that finishes before the delay', async () => {
		vi.useFakeTimers();

		const { rerender } = render(RouteLoadingAnnouncer, {
			active: true,
			message: 'Loading the next page'
		});

		await act(() => vi.advanceTimersByTime(150));
		await rerender({ active: false, message: 'Loading the next page' });
		await act(() => vi.runAllTimers());

		expect(screen.queryByText('Loading the next page')).not.toBeInTheDocument();
	});

	it('cancels stale show and clear timers across rapid transitions', async () => {
		vi.useFakeTimers();

		const { rerender } = render(RouteLoadingAnnouncer, {
			active: true,
			message: 'Loading the next page'
		});

		await act(() => vi.advanceTimersByTime(250));
		await rerender({ active: false, message: 'Loading the next page' });
		await rerender({ active: true, message: 'Loading the next page' });

		await act(() => vi.advanceTimersByTime(50));
		expect(screen.queryByText('Loading the next page')).not.toBeInTheDocument();

		await act(() => vi.advanceTimersByTime(249));
		expect(screen.queryByText('Loading the next page')).not.toBeInTheDocument();

		await act(() => vi.advanceTimersByTime(1));
		expect(screen.getByText('Loading the next page')).toBeInTheDocument();

		await rerender({ active: false, message: 'Loading the next page' });
		await act(() => vi.advanceTimersByTime(500));
		await rerender({ active: true, message: 'Loading the next page' });
		await act(() => vi.advanceTimersByTime(600));
		expect(screen.getByText('Loading the next page')).toBeInTheDocument();

		await rerender({ active: false, message: 'Loading the next page' });
		expect(screen.getByText('Loading the next page')).toBeInTheDocument();
		await act(() => vi.advanceTimersByTime(0));
		expect(screen.queryByText('Loading the next page')).not.toBeInTheDocument();
	});
});
