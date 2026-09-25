<script lang="ts">
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import { untrack } from "svelte";

	import {
		ApiError,
		isCsrfApiError,
		isForbiddenApiError,
		isUnauthorizedApiError
	} from "$lib/api/errors";
	import ActivityPanel from "$lib/components/groups/activity-panel.svelte";
	import BalancesPanel from "$lib/components/groups/balances-panel.svelte";
	import GroupWorkspaceHeader from "$lib/components/groups/group-workspace-header.svelte";
	import MembersPanel from "$lib/components/groups/members-panel.svelte";
	import type { MemberNotFoundResolution } from "$lib/components/groups/member-removal";
	import { Separator } from "$lib/components/ui/separator";
	import * as Tabs from "$lib/components/ui/tabs";
	import { copy } from "$lib/copy/en";
	import {
		type AccountingResourceStatus,
		useGroupAccounting
	} from "$lib/state/group-accounting.svelte";
	import { useGroupDetailContext } from "$lib/state/group-detail.svelte";
	import {
		groupViewUrl,
		normalizeGroupView,
		type GroupView
	} from "$lib/utils/group-workspace";

	const groupDetail = useGroupDetailContext();
	const accounting = useGroupAccounting();

	let memberRefreshWarning = $state(false);

	const selectedView = $derived(
		normalizeGroupView(page.url.searchParams.getAll('view'))
	);

	$effect(() => {
		const groupId = accounting.groupId;
		if (groupId === '') {
			return;
		}

		untrack(() => {
			void accounting.ensureOverview();
		});
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

	async function handleMemberRemoved(): Promise<void> {
		memberRefreshWarning = false;
		const results = await Promise.allSettled([
			groupDetail.refreshDetail(),
			accounting.refreshExpenses(),
			accounting.refreshRepayments(),
			accounting.refreshSettlements()
		]);
		const refreshed = results.every((result) => result.status === 'fulfilled');

		if (!refreshed && groupDetail.status === 'ready') {
			memberRefreshWarning = true;
		}
	}

	async function handleMemberNotFound(
		userId: string
	): Promise<MemberNotFoundResolution> {
		try {
			const detail = await groupDetail.refreshDetail();
			if (detail.group.currentUserRole !== 'owner') {
				return 'handled';
			}

			return detail.members.some((member) => member.userId === userId)
				? 'unresolved'
				: 'removed';
		} catch {
			return groupDetail.status === 'hidden' ||
				groupDetail.status === 'loading'
				? 'handled'
				: 'unresolved';
		}
	}

	async function handleMemberProtectedError(error: ApiError): Promise<boolean> {
		if (isUnauthorizedApiError(error)) {
			groupDetail.clear();
			return true;
		}
		if (!isForbiddenApiError(error) || isCsrfApiError(error)) {
			return false;
		}

		try {
			const detail = await groupDetail.refreshDetail();
			return detail.group.currentUserRole !== 'owner';
		} catch {
			return groupDetail.status === 'hidden' || groupDetail.status === 'loading';
		}
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

	function panelStatus(
		status: AccountingResourceStatus,
		error: unknown
	): 'loading' | 'ready' | 'error' {
		return error !== null ? 'error' : status === 'idle' ? 'loading' : status;
	}

	function retry(request: () => Promise<unknown>): void {
		void request().catch(() => {
			// The accounting context exposes the retry failure to its panel.
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
					status={panelStatus(
						accounting.settlements.status,
						accounting.settlements.error
					)}
					response={accounting.settlements.response}
					onRetry={() => retry(() => accounting.refreshSettlements())}
				/>
			</Tabs.Content>

			<Tabs.Content value="activity" class="min-w-0 pt-4">
				<ActivityPanel
					groupId={groupDetail.groupId}
					members={groupDetail.members}
					expenseStatus={panelStatus(
						accounting.expenses.status,
						accounting.expenses.error
					)}
					expenseResponse={accounting.expenses.response}
					repaymentStatus={panelStatus(
						accounting.repayments.status,
						accounting.repayments.error
					)}
					repaymentResponse={accounting.repayments.response}
					onRetryExpenses={() => retry(() => accounting.refreshExpenses())}
					onRetryRepayments={() =>
						retry(() => accounting.refreshRepayments())}
				/>
			</Tabs.Content>

			<Tabs.Content value="members" class="min-w-0 pt-4">
				<MembersPanel
					members={groupDetail.members}
					groupId={groupDetail.groupId}
					canManage={groupDetail.group.currentUserRole === 'owner'}
					refreshWarning={memberRefreshWarning}
					onMemberRemoved={handleMemberRemoved}
					onMemberNotFound={handleMemberNotFound}
					onProtectedError={handleMemberProtectedError}
				/>
			</Tabs.Content>
		</Tabs.Root>
	</section>
{/if}
