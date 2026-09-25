export function trackDialogViewport(node: HTMLElement): () => void {
	const viewport = window.visualViewport;
	if (viewport === null || viewport === undefined) {
		return () => {};
	}

	const update = (): void => {
		// Preserve native pinch zoom and panning instead of following its crop.
		if (viewport.scale !== 1) {
			node.style.removeProperty('--dialog-viewport-top');
			node.style.removeProperty('--dialog-viewport-height');
			return;
		}
		node.style.setProperty('--dialog-viewport-top', `${viewport.offsetTop}px`);
		node.style.setProperty('--dialog-viewport-height', `${viewport.height}px`);
	};

	update();
	viewport.addEventListener('resize', update);
	viewport.addEventListener('scroll', update);
	return () => {
		viewport.removeEventListener('resize', update);
		viewport.removeEventListener('scroll', update);
		node.style.removeProperty('--dialog-viewport-top');
		node.style.removeProperty('--dialog-viewport-height');
	};
}
