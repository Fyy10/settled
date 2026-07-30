export type DirtyFormRegistration = {
	update(dirty: boolean, blocked?: boolean): void;
	unregister(): void;
};

type DirtyFormsReadable = {
	readonly current: boolean;
};

const dirtyFormTokens = new Set<symbol>();
const blockingFormTokens = new Set<symbol>();
let dirtyFormCount = $state(0);
let blockingFormCount = $state(0);

export const hasDirtyForms: DirtyFormsReadable = {
	get current() {
		return dirtyFormCount > 0;
	}
};

export const hasBlockingForms: DirtyFormsReadable = {
	get current() {
		return blockingFormCount > 0;
	}
};

export function registerDirtyForm(
	initialDirty = false,
	initialBlocked = false
): DirtyFormRegistration {
	const token = Symbol('dirty-form');
	let registered = true;

	setTokenDirty(token, initialDirty);
	setTokenBlocked(token, initialBlocked);

	return {
		update(dirty, blocked = false) {
			if (!registered) {
				return;
			}

			setTokenDirty(token, dirty);
			setTokenBlocked(token, blocked);
		},
		unregister() {
			if (!registered) {
				return;
			}

			registered = false;
			setTokenDirty(token, false);
			setTokenBlocked(token, false);
		}
	};
}

function setTokenBlocked(token: symbol, blocked: boolean): void {
	const wasBlocked = blockingFormTokens.has(token);
	if (blocked === wasBlocked) {
		return;
	}
	if (blocked) {
		blockingFormTokens.add(token);
	} else {
		blockingFormTokens.delete(token);
	}
	blockingFormCount = blockingFormTokens.size;
}

function setTokenDirty(token: symbol, dirty: boolean): void {
	const wasDirty = dirtyFormTokens.has(token);
	if (dirty === wasDirty) {
		return;
	}

	if (dirty) {
		dirtyFormTokens.add(token);
	} else {
		dirtyFormTokens.delete(token);
	}
	dirtyFormCount = dirtyFormTokens.size;
}
