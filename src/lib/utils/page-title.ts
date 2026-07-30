import { copy } from '$lib/copy/en';

export function pageTitle(label?: string): string {
	return label ? `${label} · ${copy.appName}` : copy.appName;
}
