<script lang="ts">
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import ArrowLeftIcon from "@lucide/svelte/icons/arrow-left";
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
	import FileX2Icon from "@lucide/svelte/icons/file-x-2";
	import { onDestroy, untrack } from "svelte";

	import {
		isNotFoundApiError,
		isUnauthorizedApiError
	} from "$lib/api/errors";
	import {
		getRepayment,
		replaceRepayment
	} from "$lib/api/repayments";
	import type {
		Repayment,
		ReplaceRepaymentInput
	} from "$lib/api/types";
	import DeleteRepaymentDialog from "$lib/components/repayments/delete-repayment-dialog.svelte";
	import RepaymentForm from "$lib/components/repayments/repayment-form.svelte";
	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import * as Card from "$lib/components/ui/card";
	import * as Empty from "$lib/components/ui/empty";
	import { Skeleton } from "$lib/components/ui/skeleton";
	import { copy } from "$lib/copy/en";
	import { useGroupAccounting } from "$lib/state/group-accounting.svelte";
	import { useGroupDetailContext } from "$lib/state/group-detail.svelte";
	import { MutationCoordinator } from "$lib/state/mutation-coordinator.svelte";
	import { finishCommittedMutation } from "$lib/utils/committed-mutation";
	import { pageTitle } from "$lib/utils/page-title";
	import {
		initializeRepaymentEditDraft,
		type RepaymentDraft
	} from "$lib/utils/repayment-draft";

	type LoadStatus = 'loading' | 'ready' | 'error' | 'unavailable';

	const groupDetail = useGroupDetailContext();
	const accounting = useGroupAccounting();
	let mutation = $state(
		new MutationCoordinator<'save' | 'delete'>()
	);
	let mutationKey = '';
	let loadStatus = $state<LoadStatus>('loading');
	let repayment = $state<Repayment | null>(null);
	let initialDraft = $state<RepaymentDraft | null>(null);
	let loadedKey = '';
	let initializedKey = '';
	let mutationDisabledReason = $state<string | null>(null);
	let controller: AbortController | null = null;
	let resolutionController: AbortController | null = null;

	const routeGroupId = $derived(page.params.groupId ?? '');
	const routeRepaymentId = $derived(page.params.repaymentId ?? '');
	const routeKey = $derived(
		`${routeGroupId}\u0000${routeRepaymentId}`
	);
	const groupPath = $derived(
		`/groups/${encodeURIComponent(routeGroupId)}`
	);
	const balancesPath = $derived(`${groupPath}?view=balances`);
	const activityPath = $derived(`${groupPath}?view=activity`);

	$effect(() => {
		const groupId = routeGroupId;
		const repaymentId = routeRepaymentId;
		const key = routeKey;
		if (
			groupId === '' ||
			repaymentId === '' ||
			loadedKey === key
		) {
			return;
		}
		if (mutationKey !== key) {
			mutation = new MutationCoordinator<'save' | 'delete'>();
			mutationKey = key;
		}
		loadedKey = key;
		untrack(() => startLoad(groupId, repaymentId, key));
	});

	$effect(() => {
		const loadedRepayment = repayment;
		const members = groupDetail.members;
		const key = routeKey;
		if (
			loadStatus !== 'ready' ||
			loadedRepayment === null ||
			groupDetail.status !== 'ready' ||
			groupDetail.group === null ||
			groupDetail.groupId !== routeGroupId ||
			initializedKey === key
		) {
			return;
		}

		const memberIds = new Set(
			members.map((member) => member.userId)
		);
		if (
			!memberIds.has(loadedRepayment.fromUserId) ||
			!memberIds.has(loadedRepayment.toUserId)
		) {
			loadStatus = 'error';
			initialDraft = null;
			return;
		}

		try {
			initialDraft =
				initializeRepaymentEditDraft(loadedRepayment);
			initializedKey = key;
		} catch {
			loadStatus = 'error';
			initialDraft = null;
		}
	});

	onDestroy(() => {
		controller?.abort();
		resolutionController?.abort();
	});

	function startLoad(
		groupId = routeGroupId,
		repaymentId = routeRepaymentId,
		key = `${groupId}\u0000${repaymentId}`
	): void {
		controller?.abort();
		resolutionController?.abort();
		const requestController = new AbortController();
		controller = requestController;
		loadStatus = 'loading';
		repayment = null;
		initialDraft = null;
		initializedKey = '';

		void getRepayment(groupId, repaymentId, {
			signal: requestController.signal
		})
			.then((response) => {
				if (isCurrent(requestController, key)) {
					repayment = response;
					loadStatus = 'ready';
				}
			})
			.catch(async (error: unknown) => {
				if (!isCurrent(requestController, key)) {
					return;
				}
				if (isUnauthorizedApiError(error)) {
					return;
				}
				if (isNotFoundApiError(error)) {
					await resolveInitialNotFound(
						requestController,
						key
					);
					return;
				}
				loadStatus = 'error';
			})
			.finally(() => {
				if (controller === requestController) {
					controller = null;
				}
			});
	}

	async function resolveInitialNotFound(
		requestController: AbortController,
		key: string
	): Promise<void> {
		try {
			await groupDetail.refreshDetail();
		} catch {
			if (
				isCurrent(requestController, key) &&
				groupDetail.status === 'ready'
			) {
				loadStatus = 'error';
			}
			return;
		}
		if (!isCurrent(requestController, key)) {
			return;
		}
		loadStatus =
			groupDetail.status === 'ready' &&
			groupDetail.groupId === routeGroupId
				? 'unavailable'
				: 'error';
	}

	function isCurrent(
		requestController: AbortController,
		key: string
	): boolean {
		return (
			!requestController.signal.aborted &&
			controller === requestController &&
			routeKey === key
		);
	}

	function save(
		input: ReplaceRepaymentInput,
		options: { signal: AbortSignal }
	): Promise<Repayment> {
		return replaceRepayment(
			routeGroupId,
			routeRepaymentId,
			input,
			options
		);
	}

	function finishSave(): ReturnType<
		typeof finishCommittedMutation
	> {
		return finishCommittedMutation({
			refresh: () => accounting.refreshAfterRepaymentMutation(),
			navigate: () => navigateAfterRefresh(balancesPath)
		});
	}

	function finishDelete(): ReturnType<
		typeof finishCommittedMutation
	> {
		return finishCommittedMutation({
			refresh: () => accounting.refreshAfterRepaymentMutation(),
			navigate: () => navigateAfterRefresh(activityPath)
		});
	}

	function navigateAfterRefresh(destination: string): Promise<unknown> {
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
		return goto(destination, { replaceState: true });
	}

	async function resolveSaveNotFound(): Promise<
		'record' | 'handled' | 'retryable'
	> {
		const key = routeKey;
		try {
			await groupDetail.refreshDetail();
		} catch {
			// Continue only if the layout still has authoritative group access.
		}
		if (
			groupDetail.status !== 'ready' ||
			groupDetail.groupId !== routeGroupId ||
			routeKey !== key
		) {
			return 'handled';
		}

		resolutionController?.abort();
		const requestController = new AbortController();
		resolutionController = requestController;
		try {
			await getRepayment(routeGroupId, routeRepaymentId, {
				signal: requestController.signal
			});
			return routeKey === key ? 'retryable' : 'handled';
		} catch (error) {
			if (
				requestController.signal.aborted ||
				routeKey !== key ||
				isUnauthorizedApiError(error)
			) {
				return 'handled';
			}
			if (isNotFoundApiError(error)) {
				loadStatus = 'unavailable';
				repayment = null;
				initialDraft = null;
				return 'handled';
			}
			return 'retryable';
		} finally {
			if (resolutionController === requestController) {
				resolutionController = null;
			}
		}
	}

	async function resolveDeleteNotFound(): Promise<
		'record' | 'handled'
	> {
		try {
			await groupDetail.refreshDetail();
		} catch {
			// The layout owns hidden-group and session states.
		}
		if (
			groupDetail.status === 'ready' &&
			groupDetail.groupId === routeGroupId
		) {
			loadStatus = 'unavailable';
			repayment = null;
			initialDraft = null;
		}
		return 'handled';
	}
</script>

<svelte:head>
	<title>
		{pageTitle(
			groupDetail.group === null
				? copy.routes.editPayment.documentTitle
				: `${copy.routes.editPayment.documentTitle} · ${groupDetail.group.name}`
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
				href={activityPath}
				variant="ghost"
				class="min-h-11 self-start px-2"
			>
				<ArrowLeftIcon data-icon="inline-start" />
				{copy.groups.repayments.backToGroup}
			</Button>
			<div class="flex flex-col gap-1.5">
				<h1 id="repayment-page-heading" tabindex="-1">
					{copy.routes.editPayment.title}
				</h1>
				<p class="text-muted-foreground">
					{copy.routes.editPayment.description}
				</p>
			</div>
		</header>

		{#if loadStatus === 'unavailable'}
			<Empty.Root class="min-h-72 border">
				<Empty.Header>
					<Empty.Media variant="icon">
						<FileX2Icon />
					</Empty.Media>
					<Empty.Title>
						{copy.groups.repayments.recordUnavailableTitle}
					</Empty.Title>
					<Empty.Description>
						{copy.groups.repayments.recordUnavailableDescription}
					</Empty.Description>
				</Empty.Header>
				<Empty.Content>
					<Button href={activityPath} class="min-h-11">
						{copy.groups.repayments.continueToActivity}
					</Button>
				</Empty.Content>
			</Empty.Root>
		{:else if loadStatus === 'error'}
			<div class="flex flex-col gap-4">
				<Alert.Root variant="destructive">
					<CircleAlertIcon data-icon="inline-start" />
					<Alert.Title>
						{copy.groups.repayments.loadFailureTitle}
					</Alert.Title>
					<Alert.Description>
						{copy.groups.repayments.loadFailureDescription}
					</Alert.Description>
				</Alert.Root>
				<Button
					class="min-h-11 self-start"
					onclick={() => startLoad()}
				>
					{copy.groups.repayments.retry}
				</Button>
			</div>
		{:else if initialDraft !== null && repayment !== null}
			<Card.Root class="overflow-visible">
				<Card.Content>
					{#key routeKey}
						<RepaymentForm
							mode="edit"
							members={groupDetail.members}
							{initialDraft}
							cancelHref={activityPath}
							balancesHref={balancesPath}
							{mutation}
							{save}
							onCommitted={finishSave}
							onNotFound={resolveSaveNotFound}
							{mutationDisabledReason}
						/>
					{/key}
				</Card.Content>
			</Card.Root>

			<Card.Root>
				<Card.Header>
					<Card.Title>
						{copy.groups.repayments.delete}
					</Card.Title>
					<Card.Description>
						{copy.groups.repayments.deleteDescription}
					</Card.Description>
				</Card.Header>
				<Card.Content>
					<DeleteRepaymentDialog
						groupId={routeGroupId}
						repaymentId={routeRepaymentId}
						activityHref={activityPath}
						{mutation}
						onCommitted={finishDelete}
						onNotFound={resolveDeleteNotFound}
						{mutationDisabledReason}
					/>
				</Card.Content>
			</Card.Root>
		{:else}
			<Card.Root
				aria-label={copy.routes.editPayment.loadingLabel}
			>
				<Card.Content class="flex flex-col gap-5">
					{#each [0, 1, 2, 3, 4] as row (row)}
						<Skeleton class="h-14 w-full" />
					{/each}
				</Card.Content>
			</Card.Root>
		{/if}
	</section>
{/if}
