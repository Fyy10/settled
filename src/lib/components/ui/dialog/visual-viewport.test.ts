import { afterEach, describe, expect, it, vi } from 'vitest';

import { trackDialogViewport } from './visual-viewport';

afterEach(() => vi.unstubAllGlobals());

describe('dialog visual viewport', () => {
	it('tracks keyboard resize and viewport pan, then removes listeners on close', () => {
		const viewport = Object.assign(new EventTarget(), {
			scale: 1,
			height: 700,
			offsetTop: 0
		});
		vi.stubGlobal('visualViewport', viewport);
		const node = document.createElement('div');
		const stop = trackDialogViewport(node);
		expect(node.style.getPropertyValue('--dialog-viewport-height')).toBe('700px');

		viewport.height = 280;
		viewport.offsetTop = 120;
		viewport.dispatchEvent(new Event('resize'));
		expect(node.style.getPropertyValue('--dialog-viewport-height')).toBe('280px');
		expect(node.style.getPropertyValue('--dialog-viewport-top')).toBe('120px');

		viewport.offsetTop = 140;
		viewport.dispatchEvent(new Event('scroll'));
		expect(node.style.getPropertyValue('--dialog-viewport-top')).toBe('140px');
		stop();
		viewport.dispatchEvent(new Event('resize'));
		expect(node.style.getPropertyValue('--dialog-viewport-height')).toBe('');
		expect(node.style.getPropertyValue('--dialog-viewport-top')).toBe('');
	});

	it('leaves pinch zoom native and resumes keyboard sizing when zoom returns', () => {
		const viewport = Object.assign(new EventTarget(), {
			scale: 1,
			height: 700,
			offsetTop: 0
		});
		vi.stubGlobal('visualViewport', viewport);
		const node = document.createElement('div');
		const stop = trackDialogViewport(node);
		viewport.scale = 2;
		viewport.dispatchEvent(new Event('resize'));
		expect(node.style.getPropertyValue('--dialog-viewport-height')).toBe('');
		viewport.scale = 1;
		viewport.dispatchEvent(new Event('resize'));
		expect(node.style.getPropertyValue('--dialog-viewport-height')).toBe('700px');
		stop();
	});

	it('keeps the CSS dvh fallback when VisualViewport is unavailable', () => {
		vi.stubGlobal('visualViewport', undefined);
		const node = document.createElement('div');
		const stop = trackDialogViewport(node);
		expect(node.style.length).toBe(0);
		expect(stop).not.toThrow();
	});
});
