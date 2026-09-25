<script lang="ts">
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
	import FolderXIcon from "@lucide/svelte/icons/folder-x";
	import { onDestroy, untrack } from "svelte";

	import {
		isNotFoundApiError,
		isUnauthorizedApiError
	} from "$lib/api/errors";
	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import * as Empty from "$lib/components/ui/empty";
	import { Skeleton } from "$lib/components/ui/skeleton";
	import { Spinner } from "$lib/components/ui/spinner";
	import { copy } from "$lib/copy/en";
	import {
		GroupAccountingState,
		provideGroupAccounting
	} from "$lib/state/group-accounting.svelte";
	import {
		GroupDetailContext,
		provideGroupDetailContext
	} from "$lib/state/group-detail.svelte";
	import { pageTitle } from "$lib/utils/page-title";

	let { data, children } = $props();

	const groupDetail = provideGroupDetailContext(new GroupDetailContext());
	const accounting = provideGroupAccounting(new GroupAccountingState());

	$effect(() => {
		const groupId = data.groupId;
		untrack(() => {
			accounting.activate(groupId);
			void groupDetail.load(groupId).catch(() => {
				// The route boundary renders the state selected by the context.
			});
		});
	});

	$effect(() => {
		const errors = [
			accounting.expenses.error,
			accounting.repayments.error,
			accounting.settlements.error
		].filter((error) => error !== null);
		if (errors.length === 0) {
			return;
		}

		untrack(() => {
			if (errors.some(isUnauthorizedApiError)) {
				accounting.clear();
				groupDetail.clear();
			} else if (errors.some(isNotFoundApiError)) {
				accounting.clear();
				groupDetail.markHidden();
			}
		});
	});

	$effect(() => {
		if (groupDetail.status === 'hidden') {
			untrack(() => accounting.clear());
		}
	});

	onDestroy(() => {
		accounting.dispose();
		groupDetail.dispose();
	});

	function retryDetail(): void {
		void groupDetail.refreshDetail().catch(() => {
			// The route boundary renders the state selected by the context.
		});
	}
</script>

<svelte:head>
	<title>
		{pageTitle(groupDetail.group?.name ?? copy.routes.group.documentTitle)}
	</title>
</svelte:head>

{#if groupDetail.status !== 'hidden' && accounting.groupId !== ''}
	<div
		class="min-w-0"
		hidden={groupDetail.status !== 'ready' || groupDetail.group === null}
	>
		{@render children?.()}
	</div>
{/if}

{#if groupDetail.status !== 'ready' || groupDetail.group === null}
	<section class="mx-auto flex w-full max-w-3xl flex-col gap-6">
		{#if groupDetail.status === 'hidden'}
			<Empty.Root class="min-h-72 border">
				<Empty.Header>
					<Empty.Media variant="icon">
						<FolderXIcon />
					</Empty.Media>
					<Empty.Title>
						<h1 id="group-state-heading" tabindex="-1">
							{copy.groups.workspace.hiddenTitle}
						</h1>
					</Empty.Title>
					<Empty.Description>
						{copy.groups.workspace.hiddenDescription}
					</Empty.Description>
				</Empty.Header>
				<Empty.Content>
					<Button href="/groups" class="min-h-11">
						{copy.groups.workspace.backToGroups}
					</Button>
				</Empty.Content>
			</Empty.Root>
		{:else if groupDetail.status === 'error'}
			<div class="flex max-w-xl flex-col gap-4">
				<Alert.Root variant="destructive">
					<CircleAlertIcon data-icon="inline-start" />
					<Alert.Title>
						<h1 id="group-state-heading" tabindex="-1">
							{copy.groups.workspace.loadFailureTitle}
						</h1>
					</Alert.Title>
					<Alert.Description>
						{copy.groups.workspace.loadFailureDescription}
					</Alert.Description>
				</Alert.Root>
				<Button class="min-h-11 self-start" onclick={retryDetail}>
					{copy.groups.workspace.retry}
				</Button>
			</div>
		{:else}
			<header class="flex flex-col gap-5" aria-labelledby="group-state-heading">
				<div class="flex items-center gap-3" aria-live="polite">
					<Spinner aria-label={copy.groups.workspace.loading} />
					<h1 id="group-state-heading" tabindex="-1">
						{copy.groups.workspace.loading}
					</h1>
				</div>
				<div class="flex flex-col gap-3" aria-hidden="true">
					<Skeleton class="h-5 w-24" />
					<Skeleton class="h-8 w-3/5 max-w-96" />
					<Skeleton class="h-4 w-32" />
				</div>
			</header>
			<div class="flex flex-col gap-4" aria-hidden="true">
				<Skeleton class="h-11 w-full max-w-sm" />
				<Skeleton class="h-72 w-full" />
			</div>
		{/if}
	</section>
{/if}
