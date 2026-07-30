import { svelteTesting } from '@testing-library/svelte/vite';
import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { loadEnv } from 'vite';
import { defineConfig } from 'vitest/config';

import { resolveApiBaseUrl } from './src/lib/config/api-base-url';

export default defineConfig(({ command, mode }) => {
	const environment = loadEnv(mode, '.', 'PUBLIC_');

	resolveApiBaseUrl(environment.PUBLIC_API_BASE_URL, { production: command === 'build' });

	return {
		plugins: [tailwindcss(), sveltekit(), svelteTesting()],
		test: {
			environment: 'jsdom',
			setupFiles: ['./src/tests/setup.ts']
		}
	};
});
