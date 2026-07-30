export const backendParticipantA = '00000000-0000-4000-8000-000000000001';
export const backendParticipantB = '00000000-0000-4000-8000-000000000002';
export const backendParticipantC = '00000000-0000-4000-8000-000000000003';
export const backendParticipantD = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa';

export const backendExpenseSplitFixtures = {
	equalRemainder: {
		amountCents: 10,
		participantUserIds: [backendParticipantC, backendParticipantA, backendParticipantB],
		splits: [
			{ userId: backendParticipantC, amountCents: 4 },
			{ userId: backendParticipantA, amountCents: 3 },
			{ userId: backendParticipantB, amountCents: 3 }
		]
	},
	percentageRemainder: {
		amountCents: 5_400,
		percentageSplits: [
			{ userId: backendParticipantC, percentageBasisPoints: 3_334 },
			{ userId: backendParticipantA, percentageBasisPoints: 3_333 },
			{ userId: backendParticipantB, percentageBasisPoints: 3_333 }
		],
		splits: [
			{ userId: backendParticipantC, amountCents: 1_801 },
			{ userId: backendParticipantA, amountCents: 1_800 },
			{ userId: backendParticipantB, amountCents: 1_799 }
		]
	},
	crossModeRemainder: {
		amountCents: 10_003,
		participantUserIds: [
			backendParticipantC,
			backendParticipantA,
			backendParticipantD,
			backendParticipantB
		],
		splits: [
			{ userId: backendParticipantC, amountCents: 2_501 },
			{ userId: backendParticipantA, amountCents: 2_501 },
			{ userId: backendParticipantD, amountCents: 2_501 },
			{ userId: backendParticipantB, amountCents: 2_500 }
		]
	},
	maximumSafePercentage: {
		amountCents: Number.MAX_SAFE_INTEGER,
		percentageSplits: [
			{ userId: backendParticipantA, percentageBasisPoints: 5_000 },
			{ userId: backendParticipantB, percentageBasisPoints: 5_000 }
		],
		splits: [
			{ userId: backendParticipantA, amountCents: 4_503_599_627_370_496 },
			{ userId: backendParticipantB, amountCents: 4_503_599_627_370_495 }
		]
	}
} as const;
