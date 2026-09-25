<script lang="ts">
	import type { GroupMember } from "$lib/api/types";
	import * as Alert from "$lib/components/ui/alert";
	import { Checkbox } from "$lib/components/ui/checkbox";
	import * as Field from "$lib/components/ui/field";
	import * as InputGroup from "$lib/components/ui/input-group";
	import * as ToggleGroup from "$lib/components/ui/toggle-group";
	import { copy } from "$lib/copy/en";
	import {
		modeSwitchDiscardsManualShares,
		participantRemovalDiscardsManualShare,
		previewExpenseSplits,
		switchExpenseSplitMode,
		updateExpenseParticipantSelection,
		type ExpenseDraft,
		type ExpenseSplitMode
	} from "$lib/utils/expense-draft";
	import {
		formatBasisPointsAsPercentageInput,
		formatUsd,
		parseUsdToCents
	} from "$lib/utils/money";

	let {
		ref = $bindable(null),
		draft,
		members,
		disabled = false,
		error = '',
		errorParticipantUserId = undefined,
		editMode = false,
		onChange
	}: {
		ref?: HTMLElement | null;
		draft: ExpenseDraft;
		members: readonly GroupMember[];
		disabled?: boolean;
		error?: string;
		errorParticipantUserId?: string;
		editMode?: boolean;
		onChange: (draft: ExpenseDraft) => void;
	} = $props();

	const selectedIds = $derived(
		new Set(draft.participants.map((participant) => participant.userId))
	);
	const participantById = $derived(
		new Map(
			draft.participants.map((participant) => [
				participant.userId,
				participant
			])
		)
	);
	const memberById = $derived(
		new Map(members.map((member) => [member.userId, member]))
	);
	const preview = $derived(previewExpenseSplits(draft));
	let controlRevision = $state(0);

	function changeMode(value: string): void {
		if (
			value !== 'equal' &&
			value !== 'exact' &&
			value !== 'percentage'
		) {
			return;
		}
		const nextMode: ExpenseSplitMode = value;
		if (
			modeSwitchDiscardsManualShares(draft, nextMode) &&
			!globalThis.confirm(copy.groups.expenses.discardManualShares)
		) {
			controlRevision += 1;
			return;
		}

		onChange(switchExpenseSplitMode(draft, nextMode));
	}

	function changeParticipant(userId: string, checked: boolean): void {
		if (
			!checked &&
			participantRemovalDiscardsManualShare(draft, userId) &&
			!globalThis.confirm(copy.groups.expenses.removeManualShare)
		) {
			controlRevision += 1;
			return;
		}

		const nextIds = new Set(selectedIds);
		if (checked) {
			nextIds.add(userId);
		} else {
			nextIds.delete(userId);
		}
		onChange(
			updateExpenseParticipantSelection(
				draft,
				members
					.filter((member) => nextIds.has(member.userId))
					.map((member) => member.userId)
			)
		);
	}

	function changeExact(userId: string, exactAmount: string): void {
		onChange({
			...draft,
			participants: draft.participants.map((participant) =>
				participant.userId === userId
					? { ...participant, exactAmount }
					: { ...participant }
			)
		});
	}

	function changePercentage(userId: string, percentage: string): void {
		onChange({
			...draft,
			participants: draft.participants.map((participant) =>
				participant.userId === userId
					? { ...participant, percentage }
					: { ...participant }
			)
		});
	}

	function memberName(userId: string): string {
		return (
			memberById.get(userId)?.displayName ??
			copy.groups.workspace.unknownMember
		);
	}

	function reviewHeadline(): string {
		if (draft.splitMode === 'equal') {
			if (!preview.isValid) {
				return `${draft.participants.length} ${draft.participants.length === 1 ? 'person' : 'people'}`;
			}
			const amounts = preview.splits.map((split) => split.amountCents);
			const minimum = Math.min(...amounts);
			const maximum = Math.max(...amounts);
			return minimum === maximum
				? `${draft.participants.length} ${draft.participants.length === 1 ? 'person' : 'people'} · ${formatUsd(minimum)} each`
				: `${draft.participants.length} people · ${formatUsd(minimum)}–${formatUsd(maximum)} each`;
		}

		if (draft.splitMode === 'exact') {
			const assigned =
				preview.assignedCents === null ? '$0.00' : formatUsd(preview.assignedCents);
			const amountResult = parseUsdToCents(draft.amount);
			const total = amountResult.ok ? formatUsd(amountResult.value) : '$0.00';
			const remaining =
				preview.amountCents === null || preview.assignedCents === null
					? null
					: preview.amountCents - preview.assignedCents;
			return remaining === null
				? `${assigned} ${copy.groups.expenses.assigned}`
				: `${assigned} of ${total} ${copy.groups.expenses.assigned} · ${formatSignedUsd(remaining)} ${copy.groups.expenses.remaining}`;
		}

		const assigned = preview.assignedBasisPoints ?? 0;
		const remaining = 10_000 - assigned;
		return `${formatBasisPointTotal(assigned)}% ${copy.groups.expenses.assigned} · ${formatSignedPercentage(remaining)} ${copy.groups.expenses.remaining}`;
	}

	function formatSignedUsd(cents: number): string {
		return cents < 0 ? `−${formatUsd(Math.abs(cents))}` : formatUsd(cents);
	}

	function formatSignedPercentage(basisPoints: number): string {
		return basisPoints < 0
			? `−${formatBasisPointTotal(Math.abs(basisPoints))}%`
			: `${formatBasisPointTotal(basisPoints)}%`;
	}

	function formatBasisPointTotal(basisPoints: number): string {
		const whole = Math.floor(basisPoints / 100);
		const fraction = String(basisPoints % 100).padStart(2, '0');
		return `${whole}.${fraction}`;
	}
</script>

<div bind:this={ref} class="flex flex-col gap-6">
	<Field.FieldSet>
		<Field.FieldLegend>{copy.groups.expenses.splitMethodLabel}</Field.FieldLegend>
		{#if editMode}
			<Field.FieldDescription>
				{copy.groups.expenses.editSplitDescription}
			</Field.FieldDescription>
		{/if}
		{#key `${draft.splitMode}:${controlRevision}`}
			<ToggleGroup.Root
				type="single"
				value={draft.splitMode}
				onValueChange={changeMode}
				variant="outline"
				class="grid w-full grid-cols-3"
				disabled={disabled}
				aria-label={copy.groups.expenses.splitMethodLabel}
			>
				<ToggleGroup.Item value="equal" class="min-h-11 min-w-0 px-2">
					{copy.groups.expenses.equalMode}
				</ToggleGroup.Item>
				<ToggleGroup.Item value="exact" class="min-h-11 min-w-0 px-2">
					{copy.groups.expenses.exactMode}
				</ToggleGroup.Item>
				<ToggleGroup.Item value="percentage" class="min-h-11 min-w-0 px-2">
					{copy.groups.expenses.percentageMode}
				</ToggleGroup.Item>
			</ToggleGroup.Root>
		{/key}
	</Field.FieldSet>

	<Field.FieldSet
		aria-invalid={error ? true : undefined}
		aria-describedby={error ? 'expense-split-error' : undefined}
		data-invalid={error ? true : undefined}
		data-disabled={disabled ? true : undefined}
	>
		<Field.FieldLegend>{copy.groups.expenses.participantsLegend}</Field.FieldLegend>
		<Field.FieldDescription>
			{copy.groups.expenses.participantsDescription}
		</Field.FieldDescription>

		<div class="grid gap-2" data-slot="checkbox-group">
			{#each members as member, index (member.userId)}
				{@const participant = participantById.get(member.userId)}
				<div class="rounded-lg border bg-card p-3">
					<div class="flex min-h-11 items-center gap-3">
						{#key `${member.userId}:${selectedIds.has(member.userId)}:${controlRevision}`}
							<Checkbox
								id={`expense-participant-${index}`}
								checked={selectedIds.has(member.userId)}
								onCheckedChange={(checked) =>
									changeParticipant(member.userId, checked)}
								disabled={disabled}
								aria-invalid={error &&
									(errorParticipantUserId === undefined ||
										errorParticipantUserId === member.userId)
									? true
									: undefined}
								aria-describedby={error &&
									(errorParticipantUserId === undefined ||
										errorParticipantUserId === member.userId)
									? 'expense-split-error'
									: undefined}
							/>
						{/key}
						<label
							for={`expense-participant-${index}`}
							class="flex min-h-11 min-w-0 flex-1 cursor-pointer items-center font-medium"
						>
							<span class="min-w-0 break-words">{member.displayName}</span>
						</label>
					</div>

					{#if participant && draft.splitMode === 'exact'}
						<Field.Field
							class="mt-2 pl-7"
							data-invalid={error &&
								(errorParticipantUserId === undefined ||
									errorParticipantUserId === member.userId)
								? true
								: undefined}
							data-disabled={disabled ? true : undefined}
						>
							<Field.FieldLabel for={`expense-exact-${index}`}>
								{copy.groups.expenses.exactShareLabel}
								{member.displayName}
							</Field.FieldLabel>
							<InputGroup.Root>
								<InputGroup.Addon>$</InputGroup.Addon>
								<InputGroup.Input
									id={`expense-exact-${index}`}
									data-split-input={member.userId}
									inputmode="decimal"
									autocomplete="off"
									value={participant.exactAmount}
									oninput={(event) =>
										changeExact(
											member.userId,
											event.currentTarget.value
										)}
									disabled={disabled}
									aria-invalid={error &&
										(errorParticipantUserId === undefined ||
											errorParticipantUserId === member.userId)
										? true
										: undefined}
									aria-describedby={error &&
										(errorParticipantUserId === undefined ||
											errorParticipantUserId === member.userId)
										? 'expense-split-error'
										: undefined}
								/>
							</InputGroup.Root>
						</Field.Field>
					{:else if participant && draft.splitMode === 'percentage'}
						<Field.Field
							class="mt-2 pl-7"
							data-invalid={error &&
								(errorParticipantUserId === undefined ||
									errorParticipantUserId === member.userId)
								? true
								: undefined}
							data-disabled={disabled ? true : undefined}
						>
							<Field.FieldLabel for={`expense-percentage-${index}`}>
								{copy.groups.expenses.percentageShareLabel}
								{member.displayName}
							</Field.FieldLabel>
							<InputGroup.Root>
								<InputGroup.Input
									id={`expense-percentage-${index}`}
									data-split-input={member.userId}
									inputmode="decimal"
									autocomplete="off"
									value={participant.percentage}
									oninput={(event) =>
										changePercentage(
											member.userId,
											event.currentTarget.value
										)}
									disabled={disabled}
									aria-invalid={error &&
										(errorParticipantUserId === undefined ||
											errorParticipantUserId === member.userId)
										? true
										: undefined}
									aria-describedby={error &&
										(errorParticipantUserId === undefined ||
											errorParticipantUserId === member.userId)
										? 'expense-split-error'
										: undefined}
								/>
								<InputGroup.Addon align="inline-end">%</InputGroup.Addon>
							</InputGroup.Root>
						</Field.Field>
					{/if}
				</div>
			{/each}
		</div>
		<Field.FieldError
			id="expense-split-error"
			errors={error ? [{ message: error }] : []}
		/>
	</Field.FieldSet>

	<Alert.Root
		variant={preview.isValid ? 'default' : 'destructive'}
		class="bg-muted/35"
	>
		<Alert.Title>{copy.groups.expenses.reviewTitle}</Alert.Title>
		<Alert.Description class="flex flex-col gap-3">
			<p class="font-medium tabular-nums text-foreground">
				{reviewHeadline()}
			</p>
			{#if preview.splits.length > 0}
				<ul class="grid gap-1.5">
					{#each preview.splits as split (split.userId)}
						<li class="flex items-baseline justify-between gap-4">
							<span class="min-w-0 break-words">{memberName(split.userId)}</span>
							<span class="shrink-0 font-medium tabular-nums">
								{formatUsd(split.amountCents)}
							</span>
						</li>
					{/each}
				</ul>
			{/if}
			{#if preview.errors.length > 0}
				<ul class="list-disc pl-4">
					{#each [...new Set(preview.errors.map((item) => item.message))] as message (message)}
						<li>{message}</li>
					{/each}
				</ul>
			{/if}
		</Alert.Description>
	</Alert.Root>
</div>
