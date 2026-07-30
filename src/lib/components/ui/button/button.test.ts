import { describe, expect, it } from 'vitest';

import { buttonVariants } from './button.svelte';

type Oklch = readonly [lightness: number, chroma: number, hue: number];
type FileSystem = {
	readFileSync(path: string, encoding: 'utf8'): string;
};

const fileSystem = (
	globalThis as typeof globalThis & {
		process: {
			getBuiltinModule(name: 'node:fs'): FileSystem;
		};
	}
).process.getBuiltinModule('node:fs');
const layoutCss = fileSystem.readFileSync('src/routes/layout.css', 'utf8');

function themeToken(selector: ':root' | '.dark', name: string): Oklch {
	const themeBlock = layoutCss.match(
		new RegExp(`${selector === ':root' ? ':root' : '\\.dark'}\\s*\\{([\\s\\S]*?)\\}`)
	)?.[1];
	const value = themeBlock?.match(new RegExp(`--${name}:\\s*oklch\\(([^)]+)\\)`))?.[1];
	const channels = value?.trim().split(/\s+/).map(Number);

	if (!channels || channels.length !== 3 || channels.some(Number.isNaN)) {
		throw new Error(`Could not read --${name} from ${selector}.`);
	}

	return channels as unknown as Oklch;
}

function relativeLuminance([lightness, chroma, hue]: Oklch): number {
	const hueRadians = (hue * Math.PI) / 180;
	const a = chroma * Math.cos(hueRadians);
	const b = chroma * Math.sin(hueRadians);
	const lRoot = lightness + 0.3963377774 * a + 0.2158037573 * b;
	const mRoot = lightness - 0.1055613458 * a - 0.0638541728 * b;
	const sRoot = lightness - 0.0894841775 * a - 1.291485548 * b;
	const l = lRoot ** 3;
	const m = mRoot ** 3;
	const s = sRoot ** 3;
	const clamp = (channel: number) => Math.min(1, Math.max(0, channel));
	const red = clamp(4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s);
	const green = clamp(-1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s);
	const blue = clamp(-0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s);

	return 0.2126 * red + 0.7152 * green + 0.0722 * blue;
}

function contrast(first: Oklch, second: Oklch): number {
	const firstLuminance = relativeLuminance(first);
	const secondLuminance = relativeLuminance(second);

	return (
		(Math.max(firstLuminance, secondLuminance) + 0.05) /
		(Math.min(firstLuminance, secondLuminance) + 0.05)
	);
}

describe('button focus style contract', () => {
	it('uses the global two-color focus indicator without suppressing its outline', () => {
		const focusBlock = layoutCss.match(
			/:where\(a, button, input, select, textarea, \[tabindex\]\):focus-visible\s*\{([\s\S]*?)\}/
		)?.[1];

		expect(focusBlock).toContain('box-shadow: 0 0 0 2px var(--background);');
		expect(focusBlock).toContain('outline: 2px solid var(--foreground);');
		expect(focusBlock).toContain('outline-offset: 2px;');

		for (const variant of ['default', 'destructive'] as const) {
			const classes = buttonVariants({ variant });
			expect(classes).not.toContain('outline-none');
			expect(classes).not.toMatch(/focus-visible:(?:outline|ring)/);
		}
	});

	it.each([':root', '.dark'] as const)(
		'keeps both layers above 3:1 for primary and destructive buttons in %s',
		(selector) => {
			const background = themeToken(selector, 'background');
			const foreground = themeToken(selector, 'foreground');

			expect(contrast(foreground, background)).toBeGreaterThanOrEqual(3);
			expect(contrast(background, themeToken(selector, 'primary'))).toBeGreaterThanOrEqual(3);
			expect(contrast(background, themeToken(selector, 'destructive'))).toBeGreaterThanOrEqual(3);
		}
	);
});
