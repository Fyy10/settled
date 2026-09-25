import { svelteTesting } from '@testing-library/svelte/vite';
import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { SvelteKitPWA } from '@vite-pwa/sveltekit';
import { loadEnv } from 'vite';
import process from 'node:process';
import { defineConfig } from 'vitest/config';

import { resolveApiBaseUrl } from './src/lib/config/api-base-url';

export default defineConfig(({ command, mode }) => {
	const environment = loadEnv(mode, '.', 'PUBLIC_');

	const apiOrigin = resolveApiBaseUrl(environment.PUBLIC_API_BASE_URL, {
		production: false
	});
	// SvelteKit's separately loaded CSP config must use the same resolved origin.
	process.env.PUBLIC_API_BASE_URL = apiOrigin;

	return {
		plugins: [{
			name: 'settled-production-api-origin',
			apply: 'build',
			buildStart() {
				resolveApiBaseUrl(environment.PUBLIC_API_BASE_URL, { production: true });
			}
		}, tailwindcss(), sveltekit(), svelteTesting(), SvelteKitPWA({
			base: '/',
			buildBase: '/',
			scope: '/',
			registerType: 'prompt',
			injectRegister: null,
			strategies: 'generateSW',
			kit: { adapterFallback: '200.html', spa: true },
			manifest: {
				name: 'Settled', short_name: 'Settled', id: '/', start_url: '/', scope: '/',
				display: 'standalone', background_color: '#fafcfa', theme_color: '#2f6f58',
				icons: [192, 512].flatMap((size) => [
					{ src: `/icons/settled-${size}.png`, sizes: `${size}x${size}`, type: 'image/png', purpose: 'any' },
					{ src: `/icons/settled-maskable-${size}.png`, sizes: `${size}x${size}`, type: 'image/png', purpose: 'maskable' }
				])
			},
			workbox: {
				globPatterns: ['client/_app/immutable/**/*.{js,css,woff,woff2,svg}', 'client/icons/*.png', 'client/favicon.ico', 'prerendered/pages/**/*.html'],
				globIgnores: ['**/__data.json', '**/server/**'],
				navigateFallback: '/200.html',
				navigateFallbackDenylist: [/^\/api(?:\/|$)/, /^\/_app\//, /\.[^/]+$/],
				cleanupOutdatedCaches: true,
				skipWaiting: false,
				clientsClaim: true,
				runtimeCaching: (['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD'] as const).map((method) => ({
					method,
					// Workbox serializes this callback; it must not close over build variables.
					urlPattern: ({ url, request }) => /^\/api(?:\/|$)/.test(url.pathname) ||
						(url.origin !== self.location.origin && request.credentials === 'include'),
					handler: 'NetworkOnly' as const
				}))
			}
		})],
		test: {
			environment: 'jsdom',
			setupFiles: ['./src/tests/setup.ts']
		}
	};
});
