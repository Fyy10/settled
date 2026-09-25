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
	import { getExpense, replaceExpense } from "$lib/api/expenses";
	import type { Expense } from "$lib/api/types";
	import DeleteExpenseDialog from "$lib/components/expenses/delete-expense-dialog.svelte";
	import ExpenseForm from "$lib/components/expenses/expense-form.svelte";
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
	import {
		initializeExpenseEditDraft,
		type ExpenseDraft,
		type ExpenseInput
	} from "$lib/utils/expense-draft";
	import { pageTitle } from "$lib/utils/page-title";

	type LoadStatus = 'loading' | 'ready' | 'error' | 'unavailable';

	const groupDetail = useGroupDetailContext();
	const accounting = useGroupAccounting();
	let mutation = $state(new MutationCoordinator<'save' | 'delete'>());
	let mutationKey = '';
	let loadStatus = $state<LoadStatus>('loading');
	let expense = $state<Expense | null>(null);
	let initialDraft = $state<ExpenseDraft | null>(null);
	let loadedKey = '';
	let initializedKey = '';
	let mutationDisabledReason = $state<string | null>(null);
	let controller: AbortController | null = null;

	const routeGroupId = $derived(page.params.groupId ?? '');
	const routeExpenseId = $derived(page.params.expenseId ?? '');
	const groupPath = $derived(
		`/groups/${encodeURIComponent(routeGroupId)}`
	);
	const activityPath = $derived(`${groupPath}?view=activity`);

	$effect(() => {
		const groupId = routeGroupId;
		const expenseId = routeExpenseId;
		const key = `${groupId}:${expenseId}`;
		if (groupId === '' || expenseId === '' || loadedKey === key) {
			return;
		}
		if (mutationKey !== key) {
			mutation = new MutationCoordinator<'save' | 'delete'>();
			mutationKey = key;
		}
		loadedKey = key;
		untrack(() => startLoad(groupId, expenseId, key));
	});

	$effect(() => {
		const loadedExpense = expense;
		const members = groupDetail.members;
		const key = `${routeGroupId}:${routeExpenseId}`;
		if (
			loadStatus !== 'ready' ||
			loadedExpense === null ||
			groupDetail.status !== 'ready' ||
			groupDetail.group === null ||
			initializedKey === key
		) {
			return;
		}

		const memberIds = new Set(members.map((member) => member.userId));
		if (
			!memberIds.has(loadedExpense.paidByUserId) ||
			loadedExpense.splits.some((split) => !memberIds.has(split.userId))
		) {
			loadStatus = 'error';
			initialDraft = null;
			return;
		}

		try {
			initialDraft = initializeExpenseEditDraft(
				loadedExpense,
				members.map((member) => member.userId)
			);
			initializedKey = key;
		} catch {
			loadStatus = 'error';
			initialDraft = null;
		}
	});

	onDestroy(() => controller?.abort());

	function startLoad(
		groupId = routeGroupId,
		expenseId = routeExpenseId,
		key = `${groupId}:${expenseId}`
	): void {
		controller?.abort();
		const requestController = new AbortController();
		controller = requestController;
		loadStatus = 'loading';
		expense = null;
		initialDraft = null;
		initializedKey = '';

		void getExpense(groupId, expenseId, {
			signal: requestController.signal
		})
			.then((response) => {
				if (isCurrent(requestController, key)) {
					expense = response;
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
					if (isCurrent(requestController, key)) {
						loadStatus =
							groupDetail.status === 'ready' ? 'unavailable' : 'error';
					}
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

	function isCurrent(
		requestController: AbortController,
		key: string
	): boolean {
		return (
			!requestController.signal.aborted &&
			controller === requestController &&
			`${routeGroupId}:${routeExpenseId}` === key
		);
	}

	function save(
		input: ExpenseInput,
		options: { signal: AbortSignal }
	): Promise<Expense> {
		return replaceExpense(routeGroupId, routeExpenseId, input, options);
	}

	function finish(): ReturnType<typeof finishCommittedMutation> {
		return finishCommittedMutation({
			refresh: () => accounting.refreshAfterExpenseMutation(),
			navigate: navigateAfterRefresh
		});
	}

	function navigateAfterRefresh(): Promise<unknown> {
		const errors = [
			accounting.expenses.error,
			accounting.settlements.error
		];
		if (
			groupDetail.status !== 'ready' ||
			errors.some(
				(error) =>
					isUnauthorizedApiError(error) || isNotFoundApiError(error)
			)
		) {
			return Promise.resolve();
		}
		return goto(activityPath, { replaceState: true });
	}

	async function resolveNotFound(): Promise<'record' | 'handled'> {
		try {
			await groupDetail.refreshDetail();
		} catch {
			// The route layout owns hidden-group and session states.
		}
		if (groupDetail.status !== 'ready') {
			return 'handled';
		}

		return 'record';
	}
</script>

<svelte:head>
	<title>
		{pageTitle(
			groupDetail.group === null
				? copy.routes.editExpense.documentTitle
				: `${copy.routes.editExpense.documentTitle} · ${groupDetail.group.name}`
		)}
	</title>
</svelte:head>

{#if groupDetail.status === 'ready' && groupDetail.group !== null}
	<section
		class="mx-auto flex w-full max-w-2xl flex-col gap-5 pb-4"
		aria-labelledby="expense-page-heading"
	>
		<header class="flex flex-col gap-3">
			<Button
				href={activityPath}
				variant="ghost"
				class="min-h-11 self-start px-2"
			>
				<ArrowLeftIcon data-icon="inline-start" />
				{copy.groups.expenses.backToGroup}
			</Button>
			<div class="flex flex-col gap-1.5">
				<h1 id="expense-page-heading" tabindex="-1">
					{copy.routes.editExpense.title}
				</h1>
				<p class="text-muted-foreground">
					{copy.routes.editExpense.description}
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
						{copy.groups.expenses.recordUnavailableTitle}
					</Empty.Title>
					<Empty.Description>
						{copy.groups.expenses.recordUnavailableDescription}
					</Empty.Description>
				</Empty.Header>
				<Empty.Content>
					<Button href={activityPath} class="min-h-11">
						{copy.groups.expenses.continueToActivity}
					</Button>
				</Empty.Content>
			</Empty.Root>
		{:else if loadStatus === 'error'}
			<div class="flex flex-col gap-4">
				<Alert.Root variant="destructive">
					<CircleAlertIcon data-icon="inline-start" />
					<Alert.Title>
						{copy.groups.expenses.loadFailureTitle}
					</Alert.Title>
					<Alert.Description>
						{copy.groups.expenses.loadFailureDescription}
					</Alert.Description>
				</Alert.Root>
				<Button
					class="min-h-11 self-start"
					onclick={() => startLoad()}
				>
					{copy.groups.expenses.retry}
				</Button>
			</div>
		{:else if initialDraft !== null && expense !== null}
			<Card.Root class="overflow-visible">
				<Card.Content>
					{#key `${routeGroupId}:${routeExpenseId}`}
						<ExpenseForm
							mode="edit"
							members={groupDetail.members}
							{initialDraft}
							cancelHref={activityPath}
							{mutation}
							{save}
							onCommitted={finish}
							onNotFound={resolveNotFound}
							{mutationDisabledReason}
						/>
					{/key}
				</Card.Content>
			</Card.Root>

			<Card.Root>
				<Card.Header>
					<Card.Title>{copy.groups.expenses.delete}</Card.Title>
					<Card.Description>
						{copy.groups.expenses.deleteDescription}
					</Card.Description>
				</Card.Header>
				<Card.Content>
					<DeleteExpenseDialog
						groupId={routeGroupId}
						expenseId={routeExpenseId}
						activityHref={activityPath}
						{mutation}
						onCommitted={finish}
						onNotFound={resolveNotFound}
						{mutationDisabledReason}
					/>
				</Card.Content>
			</Card.Root>
		{:else}
			<Card.Root aria-label={copy.routes.editExpense.loadingLabel}>
				<Card.Content class="flex flex-col gap-5">
					{#each [0, 1, 2, 3] as row (row)}
						<Skeleton class="h-14 w-full" />
					{/each}
				</Card.Content>
			</Card.Root>
		{/if}
	</section>
{/if}
