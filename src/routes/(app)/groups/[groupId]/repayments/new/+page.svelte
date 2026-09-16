<script lang="ts">
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import ArrowLeftIcon from "@lucide/svelte/icons/arrow-left";
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";

	import {
		isNotFoundApiError,
		isUnauthorizedApiError
	} from "$lib/api/errors";
	import { createRepayment } from "$lib/api/repayments";
	import type {
		Repayment,
		ReplaceRepaymentInput
	} from "$lib/api/types";
	import RepaymentForm from "$lib/components/repayments/repayment-form.svelte";
	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import * as Card from "$lib/components/ui/card";
	import { Skeleton } from "$lib/components/ui/skeleton";
	import { copy } from "$lib/copy/en";
	import { authState } from "$lib/state/auth.svelte";
	import { useGroupAccounting } from "$lib/state/group-accounting.svelte";
	import { useGroupDetailContext } from "$lib/state/group-detail.svelte";
	import { MutationCoordinator } from "$lib/state/mutation-coordinator.svelte";
	import { finishCommittedMutation } from "$lib/utils/committed-mutation";
	import { currentLocalDate } from "$lib/utils/dates";
	import { pageTitle } from "$lib/utils/page-title";
	import {
		applySettlementRepaymentPrefill,
		createRepaymentDraft,
		type RepaymentDraft
	} from "$lib/utils/repayment-draft";

	const groupDetail = useGroupDetailContext();
	const accounting = useGroupAccounting();
	let mutation = $state(
		new MutationCoordinator<'save' | 'delete'>()
	);
	let mutationKey = '';
	let initializedKey = $state('');
	let initialDraft = $state<RepaymentDraft | null>(null);
	let memberMismatch = $state(false);
	let mutationDisabledReason = $state<string | null>(null);

	const routeGroupId = $derived(page.params.groupId ?? '');
	const routeKey = $derived(
		`${page.params.groupId ?? ''}\u0000${page.url.searchParams.toString()}`
	);
	const groupPath = $derived(
		`/groups/${encodeURIComponent(routeGroupId)}`
	);
	const balancesPath = $derived(`${groupPath}?view=balances`);

	$effect(() => {
		const groupId = routeGroupId;
		const key = routeKey;
		const members = groupDetail.members;

		if (mutationKey !== key) {
			mutation = new MutationCoordinator<'save' | 'delete'>();
			mutationKey = key;
			initialDraft = null;
			initializedKey = '';
			memberMismatch = false;
		}
		if (
			groupId === '' ||
			groupDetail.groupId !== groupId ||
			groupDetail.status !== 'ready' ||
			groupDetail.group === null ||
			members.length === 0 ||
			initializedKey === key
		) {
			return;
		}

		const currentUserId =
			authState.current.status === 'authenticated'
				? authState.current.user.id
				: '';
		if (!members.some((member) => member.userId === currentUserId)) {
			memberMismatch = true;
			initializedKey = key;
			return;
		}

		const draft = createRepaymentDraft({
			fromUserId: currentUserId,
			repaymentDate: currentLocalDate()
		});
		initialDraft = applySettlementRepaymentPrefill(
			draft,
			page.url.searchParams,
			members.map((member) => member.userId)
		);
		initializedKey = key;
	});

	function save(
		input: ReplaceRepaymentInput,
		options: { signal: AbortSignal }
	): Promise<Repayment> {
		return createRepayment(routeGroupId, input, options);
	}

	function finish(): ReturnType<typeof finishCommittedMutation> {
		return finishCommittedMutation({
			refresh: () => accounting.refreshAfterRepaymentMutation(),
			navigate: navigateAfterRefresh
		});
	}

	function navigateAfterRefresh(): Promise<unknown> {
		const errors = [
			accounting.repayments.error,
			accounting.settlements.error
		];
		if (
			groupDetail.status !== 'ready' ||
			groupDetail.groupId !== routeGroupId ||
			errors.some(
				(error) =>
					isUnauthorizedApiError(error) ||
					isNotFoundApiError(error)
			)
		) {
			return Promise.resolve();
		}
		return goto(balancesPath, { replaceState: true });
	}

	async function resolveNotFound(): Promise<
		'handled' | 'retryable'
	> {
		try {
			await groupDetail.refreshDetail();
		} catch {
			// The layout owns hidden-group and session states.
		}
		return groupDetail.status === 'ready' &&
			groupDetail.groupId === routeGroupId
			? 'retryable'
			: 'handled';
	}
</script>

<svelte:head>
	<title>
		{pageTitle(
			groupDetail.group === null
				? copy.routes.recordPayment.documentTitle
				: `${copy.routes.recordPayment.documentTitle} · ${groupDetail.group.name}`
		)}
	</title>
</svelte:head>

{#if groupDetail.status === 'ready' && groupDetail.group !== null && groupDetail.groupId === routeGroupId}
	<section
		class="mx-auto flex w-full max-w-2xl flex-col gap-5 pb-4"
		aria-labelledby="repayment-page-heading"
	>
		<header class="flex flex-col gap-3">
			<Button
				href={balancesPath}
				variant="ghost"
				class="min-h-11 self-start px-2"
			>
				<ArrowLeftIcon data-icon="inline-start" />
				{copy.groups.repayments.backToGroup}
			</Button>
			<div class="flex flex-col gap-1.5">
				<h1 id="repayment-page-heading" tabindex="-1">
					{copy.routes.recordPayment.title}
				</h1>
				<p class="text-muted-foreground">
					{copy.routes.recordPayment.description}
				</p>
			</div>
		</header>

		<Card.Root class="overflow-visible">
			<Card.Content>
				{#if memberMismatch}
					<Alert.Root variant="destructive">
						<CircleAlertIcon data-icon="inline-start" />
						<Alert.Title>
							{copy.groups.repayments.memberMismatch}
						</Alert.Title>
					</Alert.Root>
				{:else if initialDraft !== null && initializedKey === routeKey}
					{#key initializedKey}
						<RepaymentForm
							mode="create"
							members={groupDetail.members}
							{initialDraft}
							cancelHref={balancesPath}
							balancesHref={balancesPath}
							{mutation}
							{save}
							onCommitted={finish}
							onNotFound={resolveNotFound}
							{mutationDisabledReason}
						/>
					{/key}
				{:else}
					<div
						class="flex flex-col gap-5"
						aria-label={copy.routes.recordPayment.loadingLabel}
					>
						{#each [0, 1, 2, 3, 4] as row (row)}
							<Skeleton class="h-14 w-full" />
						{/each}
					</div>
				{/if}
			</Card.Content>
		</Card.Root>
	</section>
{/if}
