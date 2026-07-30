export type CommittedMutationResult = {
	refresh: 'succeeded' | 'failed';
	navigation: 'succeeded' | 'failed';
};

export async function finishCommittedMutation(options: {
	refresh: () => Promise<unknown>;
	navigate: () => Promise<unknown>;
}): Promise<CommittedMutationResult> {
	let refresh: CommittedMutationResult['refresh'] = 'succeeded';
	let navigation: CommittedMutationResult['navigation'] = 'succeeded';

	try {
		await options.refresh();
	} catch {
		refresh = 'failed';
	}

	try {
		await options.navigate();
	} catch {
		navigation = 'failed';
	}

	return { refresh, navigation };
}
