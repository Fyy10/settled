import {
	act,
	fireEvent,
	render,
	screen,
	waitFor
} from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '$lib/api/errors';
import { groupDetailFixture } from '../../../tests/fixtures/api-contract';

import GroupCodeSection from './group-code-section.svelte';

const mocks = vi.hoisted(() => ({
	getGroupJoinCode: vi.fn()
}));

vi.mock('$lib/api/groups', () => ({
	getGroupJoinCode: mocks.getGroupJoinCode
}));
vi.mock('$lib/config/public', () => ({
	API_BASE_URL: 'http://localhost:8080'
}));

let intersectionCallback: IntersectionObserverCallback;
let clipboardDescriptor: PropertyDescriptor | undefined;

beforeEach(() => {
	mocks.getGroupJoinCode.mockReset().mockResolvedValue('ABCD1234');
	clipboardDescriptor = Object.getOwnPropertyDescriptor(navigator, 'clipboard');
	vi.stubGlobal(
		'IntersectionObserver',
		class {
			constructor(callback: IntersectionObserverCallback) {
				intersectionCallback = callback;
			}

			observe(): void {}
			disconnect(): void {}
			unobserve(): void {}
			takeRecords(): IntersectionObserverEntry[] {
				return [];
			}
		}
	);
});

afterEach(() => {
	vi.unstubAllGlobals();
	if (clipboardDescriptor === undefined) {
		Reflect.deleteProperty(navigator, 'clipboard');
	} else {
		Object.defineProperty(navigator, 'clipboard', clipboardDescriptor);
	}
});

describe('GroupCodeSection', () => {
	it('requests the private code only after the section becomes visible', async () => {
		render(GroupCodeSection, {
			groupId: groupDetailFixture.group.id,
			onProtectedError: vi.fn()
		});

		expect(mocks.getGroupJoinCode).not.toHaveBeenCalled();
		await revealSection();

		expect(
			await screen.findByRole('textbox', { name: 'Group code' })
		).toHaveValue('ABCD1234');
		expect(mocks.getGroupJoinCode).toHaveBeenCalledOnce();
		expect(mocks.getGroupJoinCode.mock.calls[0][0]).toBe(
			groupDetailFixture.group.id
		);
		expect(mocks.getGroupJoinCode.mock.calls[0][1].signal).toBeInstanceOf(
			AbortSignal
		);
	});

	it('selects and focuses the read-only code when clipboard access fails', async () => {
		const writeText = vi.fn().mockRejectedValue(new Error('Denied'));
		setClipboard(writeText);
		render(GroupCodeSection, {
			groupId: groupDetailFixture.group.id,
			onProtectedError: vi.fn()
		});
		await revealSection();
		const input = await screen.findByRole<HTMLInputElement>('textbox', {
			name: 'Group code'
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Copy code' }));

		expect(writeText).toHaveBeenCalledWith('ABCD1234');
		expect(
			await screen.findByText(
				'Copy failed. The code is selected so you can copy it manually.'
			)
		).toBeInTheDocument();
		await waitFor(() => expect(input).toHaveFocus());
		expect(input.selectionStart).toBe(0);
		expect(input.selectionEnd).toBe('ABCD1234'.length);
		expect(input).toHaveAttribute('readonly');
	});

	it('announces a successful copy without changing the button label', async () => {
		const writeText = vi.fn().mockResolvedValue(undefined);
		setClipboard(writeText);
		render(GroupCodeSection, {
			groupId: groupDetailFixture.group.id,
			onProtectedError: vi.fn()
		});
		await revealSection();
		await screen.findByRole('textbox', { name: 'Group code' });

		await fireEvent.click(screen.getByRole('button', { name: 'Copy code' }));

		expect(await screen.findByText('Code copied')).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Copy code' })).toBeInTheDocument();
	});

	it('clears the code and delegates an authoritative owner-role failure', async () => {
		const forbidden = new ApiError({
			status: 403,
			code: 'forbidden',
			message: 'Forbidden.',
			fields: {}
		});
		const onProtectedError = vi.fn().mockResolvedValue(true);
		mocks.getGroupJoinCode.mockRejectedValue(forbidden);
		render(GroupCodeSection, {
			groupId: groupDetailFixture.group.id,
			onProtectedError
		});

		await revealSection();

		await waitFor(() =>
			expect(onProtectedError).toHaveBeenCalledWith(forbidden)
		);
		expect(screen.queryByRole('textbox', { name: 'Group code' })).not.toBeInTheDocument();
		expect(screen.queryByText('ABCD1234')).not.toBeInTheDocument();
	});
});

async function revealSection(): Promise<void> {
	await act(() => {
		intersectionCallback(
			[{ isIntersecting: true } as IntersectionObserverEntry],
			{} as IntersectionObserver
		);
	});
}

function setClipboard(writeText: (value: string) => Promise<void>): void {
	Object.defineProperty(navigator, 'clipboard', {
		configurable: true,
		value: { writeText }
	});
}
