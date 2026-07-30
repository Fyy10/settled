<script lang="ts">
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import { onDestroy, untrack } from "svelte";

	import { listExpenses } from "$lib/api/expenses";
	import {
		isNotFoundApiError,
		isUnauthorizedApiError
	} from "$lib/api/errors";
	import { listRepayments } from "$lib/api/repayments";
	import { listSettlements } from "$lib/api/settlements";
	import type {
		ExpenseListResponse,
		RepaymentListResponse,
		SettlementListResponse
	} from "$lib/api/types";
	import ActivityPanel from "$lib/components/groups/activity-panel.svelte";
	import BalancesPanel from "$lib/components/groups/balances-panel.svelte";
	import GroupWorkspaceHeader from "$lib/components/groups/group-workspace-header.svelte";
	import MembersPanel from "$lib/components/groups/members-panel.svelte";
	import { Separator } from "$lib/components/ui/separator";
	import * as Tabs from "$lib/components/ui/tabs";
	import { copy } from "$lib/copy/en";
	import { useGroupDetailContext } from "$lib/state/group-detail.svelte";
	import {
		groupViewUrl,
		normalizeGroupView,
		type GroupView
	} from "$lib/utils/group-workspace";

	type PanelStatus = 'loading' | 'ready' | 'error';

	type PanelState<T> = {
		status: PanelStatus;
		response: T | null;
	};

	const groupDetail = useGroupDetailContext();

	let loadedGroupId = '';
	let expenseState = $state<PanelState<ExpenseListResponse>>({
		status: 'loading',
		response: null
	});
	let repaymentState = $state<PanelState<RepaymentListResponse>>({
		status: 'loading',
		response: null
	});
	let settlementState = $state<PanelState<SettlementListResponse>>({
		status: 'loading',
		response: null
	});

	let expenseController: AbortController | null = null;
	let repaymentController: AbortController | null = null;
	let settlementController: AbortController | null = null;

	const selectedView = $derived(
		normalizeGroupView(page.url.searchParams.getAll('view'))
	);

	$effect(() => {
		const groupId = groupDetail.groupId;
		const ready = groupDetail.status === 'ready' && groupDetail.group !== null;
		if (!ready || loadedGroupId === groupId) {
			return;
		}

		untrack(() => startOverviewLoad(groupId));
	});

	$effect(() => {
		const values = page.url.searchParams.getAll('view');
		const normalized = normalizeGroupView(values);
		if (values.length === 1 && values[0] === normalized) {
			return;
		}

		const target = groupViewUrl(page.url, normalized);
		untrack(() => {
			void goto(target, {
				replaceState: true,
				noScroll: true,
				keepFocus: true
			});
		});
	});

	onDestroy(abortOverviewRequests);

	function startOverviewLoad(groupId: string): void {
		loadedGroupId = groupId;
		expenseState = { status: 'loading', response: null };
		repaymentState = { status: 'loading', response: null };
		settlementState = { status: 'loading', response: null };

		void refreshExpenses(groupId);
		void refreshRepayments(groupId);
		void refreshSettlements(groupId);
	}

	async function refreshExpenses(groupId = groupDetail.groupId): Promise<void> {
		expenseController?.abort();
		const controller = new AbortController();
		expenseController = controller;
		expenseState = { status: 'loading', response: null };

		try {
			const response = await listExpenses(groupId, {
				signal: controller.signal
			});
			if (isCurrentRequest(controller, expenseController, groupId)) {
				expenseState = { status: 'ready', response };
			}
		} catch (error) {
			if (!isCurrentRequest(controller, expenseController, groupId)) {
				return;
			}
			handlePanelError(error, () => {
				expenseState = { status: 'error', response: null };
			});
		} finally {
			if (expenseController === controller) {
				expenseController = null;
			}
		}
	}

	async function refreshRepayments(groupId = groupDetail.groupId): Promise<void> {
		repaymentController?.abort();
		const controller = new AbortController();
		repaymentController = controller;
		repaymentState = { status: 'loading', response: null };

		try {
			const response = await listRepayments(groupId, {
				signal: controller.signal
			});
			if (isCurrentRequest(controller, repaymentController, groupId)) {
				repaymentState = { status: 'ready', response };
			}
		} catch (error) {
			if (!isCurrentRequest(controller, repaymentController, groupId)) {
				return;
			}
			handlePanelError(error, () => {
				repaymentState = { status: 'error', response: null };
			});
		} finally {
			if (repaymentController === controller) {
				repaymentController = null;
			}
		}
	}

	async function refreshSettlements(groupId = groupDetail.groupId): Promise<void> {
		settlementController?.abort();
		const controller = new AbortController();
		settlementController = controller;
		settlementState = { status: 'loading', response: null };

		try {
			const response = await listSettlements(groupId, {
				signal: controller.signal
			});
			if (isCurrentRequest(controller, settlementController, groupId)) {
				settlementState = { status: 'ready', response };
			}
		} catch (error) {
			if (!isCurrentRequest(controller, settlementController, groupId)) {
				return;
			}
			handlePanelError(error, () => {
				settlementState = { status: 'error', response: null };
			});
		} finally {
			if (settlementController === controller) {
				settlementController = null;
			}
		}
	}

	function handlePanelError(error: unknown, setLocalError: () => void): void {
		if (isUnauthorizedApiError(error)) {
			groupDetail.clear();
			return;
		}
		if (isNotFoundApiError(error)) {
			groupDetail.markHidden();
			return;
		}

		setLocalError();
	}

	function isCurrentRequest(
		controller: AbortController,
		currentController: AbortController | null,
		groupId: string
	): boolean {
		return (
			!controller.signal.aborted &&
			currentController === controller &&
			groupDetail.groupId === groupId &&
			groupDetail.status === 'ready'
		);
	}

	function abortOverviewRequests(): void {
		expenseController?.abort();
		repaymentController?.abort();
		settlementController?.abort();
		expenseController = null;
		repaymentController = null;
		settlementController = null;
	}

	function selectView(value: string): void {
		const view = normalizeGroupView(value);
		if (view === selectedView) {
			return;
		}

		void navigateToView(view);
	}

	async function navigateToView(view: GroupView): Promise<void> {
		await goto(groupViewUrl(page.url, view), {
			replaceState: true,
			noScroll: true,
			keepFocus: true
		});
	}
</script>

{#if groupDetail.group !== null}
	<section
		class="mx-auto flex w-full max-w-5xl flex-col gap-6 pb-[calc(6rem+env(safe-area-inset-bottom))] sm:pb-0"
		aria-labelledby="group-heading"
	>
		<GroupWorkspaceHeader group={groupDetail.group} />
		<Separator />

		<Tabs.Root value={selectedView} onValueChange={selectView}>
			<div
				class="-mx-1 overflow-x-auto px-1 py-1"
				role="region"
				aria-label={copy.groups.workspace.tabsLabel}
			>
				<Tabs.List
					variant="line"
					class="min-h-11 min-w-max justify-start"
					aria-label={copy.groups.workspace.tabsLabel}
				>
					<Tabs.Trigger value="balances" class="min-h-11 px-4">
						{copy.groups.workspace.balancesTab}
					</Tabs.Trigger>
					<Tabs.Trigger value="activity" class="min-h-11 px-4">
						{copy.groups.workspace.activityTab}
					</Tabs.Trigger>
					<Tabs.Trigger value="members" class="min-h-11 px-4">
						{copy.groups.workspace.membersTab}
					</Tabs.Trigger>
				</Tabs.List>
			</div>

			<Tabs.Content value="balances" class="min-w-0 pt-4">
				<BalancesPanel
					groupId={groupDetail.groupId}
					status={settlementState.status}
					response={settlementState.response}
					onRetry={() => void refreshSettlements()}
				/>
			</Tabs.Content>

			<Tabs.Content value="activity" class="min-w-0 pt-4">
				<ActivityPanel
					groupId={groupDetail.groupId}
					members={groupDetail.members}
					expenseStatus={expenseState.status}
					expenseResponse={expenseState.response}
					repaymentStatus={repaymentState.status}
					repaymentResponse={repaymentState.response}
					onRetryExpenses={() => void refreshExpenses()}
					onRetryRepayments={() => void refreshRepayments()}
				/>
			</Tabs.Content>

			<Tabs.Content value="members" class="min-w-0 pt-4">
				<MembersPanel members={groupDetail.members} />
			</Tabs.Content>
		</Tabs.Root>
	</section>
{/if}
