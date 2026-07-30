export type MutationPhase =
	| 'idle'
	| 'pending'
	| 'committed'
	| 'ambiguous'
	| 'blocked';

export class MutationCoordinator<Operation extends string = string> {
	phase = $state<MutationPhase>('idle');
	operation = $state<Operation | null>(null);

	begin(operation: Operation): boolean {
		if (this.phase !== 'idle') {
			return false;
		}
		this.operation = operation;
		this.phase = 'pending';
		return true;
	}

	reset(): void {
		if (this.phase !== 'pending') {
			return;
		}
		this.operation = null;
		this.phase = 'idle';
	}

	markCommitted(): void {
		this.phase = 'committed';
	}

	markAmbiguous(): void {
		this.phase = 'ambiguous';
	}

	markBlocked(): void {
		this.phase = 'blocked';
	}
}
