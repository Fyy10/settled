import adapter from '@sveltejs/adapter-static';
import { loadEnv } from 'vite';
import { resolveApiBaseUrl } from './src/lib/config/api-base-url.ts';

const environment = loadEnv(process.env.NODE_ENV ?? 'development', '.', 'PUBLIC_');
const apiOrigin = resolveApiBaseUrl(environment.PUBLIC_API_BASE_URL, {
	production: false
});

/** @type {import('@sveltejs/kit').Config} */
const config = {
	kit: {
		serviceWorker: { register: false },
		csp: {
			mode: 'hash',
			directives: {
				'default-src': ['self'],
				'script-src': ['self'],
				'style-src': ['self', 'unsafe-inline'],
				'connect-src': ['self', apiOrigin],
				'img-src': ['self', 'data:'],
				'font-src': ['self'],
				'worker-src': ['self'],
				'manifest-src': ['self'],
				'object-src': ['none'],
				'base-uri': ['self'],
				'form-action': ['self']
			}
		},
		adapter: adapter({
			fallback: '200.html'
		}),
		prerender: {
			handleUnseenRoutes: 'ignore'
		}
	}
};

export default config;
