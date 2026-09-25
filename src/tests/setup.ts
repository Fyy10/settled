import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';
import { afterAll } from 'vitest';

afterAll(async () => {
	cleanup();
	await tick();
	// Bits UI restores body styles on a 24 ms timer after the last dialog unmounts.
	// Let that cleanup finish before Vitest destroys this file's jsdom document.
	await new Promise((resolve) => setTimeout(resolve, 50));
});
