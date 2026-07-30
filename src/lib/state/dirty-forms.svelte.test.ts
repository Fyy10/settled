import { afterEach, describe, expect, it } from 'vitest';

import {
	hasDirtyForms,
	hasBlockingForms,
	registerDirtyForm,
	type DirtyFormRegistration
} from './dirty-forms.svelte';

const registrations: DirtyFormRegistration[] = [];

afterEach(() => {
	for (const registration of registrations.splice(0)) {
		registration.unregister();
	}
});

describe('dirty-form registry', () => {
	it('tracks one stable registration across updates', () => {
		const registration = trackRegistration();
		expect(hasDirtyForms.current).toBe(false);

		registration.update(true);
		registration.update(true);
		expect(hasDirtyForms.current).toBe(true);

		registration.update(false);
		expect(hasDirtyForms.current).toBe(false);
	});

	it('stays dirty until every simultaneous registration is clean', () => {
		const first = trackRegistration(true);
		const second = trackRegistration(true);
		expect(hasDirtyForms.current).toBe(true);

		first.update(false);
		expect(hasDirtyForms.current).toBe(true);

		second.unregister();
		expect(hasDirtyForms.current).toBe(false);
	});

	it('unregisters idempotently and ignores later stale updates', () => {
		const registration = trackRegistration(true);

		registration.unregister();
		registration.unregister();
		registration.update(true);

		expect(hasDirtyForms.current).toBe(false);
	});

	it('tracks pending blockers separately from unsaved data', () => {
		const registration = trackRegistration(false);
		registration.update(false, true);
		expect(hasDirtyForms.current).toBe(false);
		expect(hasBlockingForms.current).toBe(true);

		registration.update(true, false);
		expect(hasDirtyForms.current).toBe(true);
		expect(hasBlockingForms.current).toBe(false);
	});
});

function trackRegistration(initialDirty = false): DirtyFormRegistration {
	const registration = registerDirtyForm(initialDirty);
	registrations.push(registration);
	return registration;
}
