<script lang="ts">
	import { goto } from "$app/navigation";
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
	import UsersRoundIcon from "@lucide/svelte/icons/users-round";
	import { onMount } from "svelte";
	import { toast } from "svelte-sonner";

	import { listGroups } from "$lib/api/groups";
	import { isUnauthorizedApiError } from "$lib/api/errors";
	import type { GroupSummary } from "$lib/api/types";
	import CreateGroupDialog from "$lib/components/groups/create-group-dialog.svelte";
	import GroupCard from "$lib/components/groups/group-card.svelte";
	import JoinGroupDialog from "$lib/components/groups/join-group-dialog.svelte";
	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import * as Card from "$lib/components/ui/card";
	import * as Empty from "$lib/components/ui/empty";
	import { Separator } from "$lib/components/ui/separator";
	import { Skeleton } from "$lib/components/ui/skeleton";
	import { Spinner } from "$lib/components/ui/spinner";
	import { copy } from "$lib/copy/en";
	import { pageTitle } from "$lib/utils/page-title";

	type ListState = 'loading' | 'ready' | 'error';

	let listState = $state<ListState>('loading');
	let groups = $state<GroupSummary[]>([]);
	let currentController: AbortController | null = null;

	onMount(() => {
		void refreshGroups();

		return () => {
			currentController?.abort();
		};
	});

	async function refreshGroups({ preserve = false } = {}): Promise<void> {
		currentController?.abort();
		const controller = new AbortController();
		currentController = controller;

		if (!preserve) {
			listState = 'loading';
		}

		try {
			groups = await listGroups({ signal: controller.signal });
			listState = 'ready';
		} catch (error) {
			if (controller.signal.aborted || isUnauthorizedApiError(error)) {
				return;
			}
			if (!preserve) {
				listState = 'error';
			}
		} finally {
			if (currentController === controller) {
				currentController = null;
			}
		}
	}

	async function handleCreated(group: GroupSummary): Promise<void> {
		await refreshGroups({ preserve: true });
		toast.success(copy.groups.create.success);
		await goto(`/groups/${encodeURIComponent(group.id)}`);
	}

	async function handleJoined(group: GroupSummary): Promise<void> {
		await refreshGroups({ preserve: true });
		toast.success(copy.groups.join.success);
		await goto(`/groups/${encodeURIComponent(group.id)}`);
	}
</script>

<svelte:head>
	<title>{pageTitle(copy.routes.groups.documentTitle)}</title>
</svelte:head>

<section class="mx-auto flex w-full max-w-5xl flex-col gap-6" aria-labelledby="groups-heading">
	<header class="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
		<div class="flex max-w-2xl flex-col gap-2">
			<h1 id="groups-heading" tabindex="-1">{copy.routes.groups.title}</h1>
			<p class="text-muted-foreground">{copy.routes.groups.description}</p>
		</div>

		{#if listState !== 'ready' || groups.length > 0}
			<div class="flex flex-wrap gap-2">
				<CreateGroupDialog onSuccess={handleCreated} />
				<JoinGroupDialog onSuccess={handleJoined} />
			</div>
		{/if}
	</header>

	<Separator />

	{#if listState === 'loading'}
		<div class="flex flex-col gap-4">
			<div class="flex items-center gap-3" aria-live="polite">
				<Spinner aria-label={copy.groups.loading} />
				<p class="text-sm font-medium">{copy.groups.loading}</p>
			</div>

			<div class="grid gap-4 md:grid-cols-2" aria-hidden="true">
				{#each [0, 1, 2] as row (row)}
					<Card.Root>
						<Card.Header class="border-b">
							<Card.Title><Skeleton class="h-4 w-2/5" /></Card.Title>
						</Card.Header>
						<Card.Content><Skeleton class="h-4 w-1/3" /></Card.Content>
						<Card.Footer class="justify-between gap-3">
							<Skeleton class="h-3 w-1/2" />
							<Skeleton class="size-5" />
						</Card.Footer>
					</Card.Root>
				{/each}
			</div>
		</div>
	{:else if listState === 'error'}
		<div class="flex max-w-xl flex-col gap-4">
			<Alert.Root variant="destructive">
				<CircleAlertIcon data-icon="inline-start" />
				<Alert.Title>{copy.groups.loadFailureTitle}</Alert.Title>
				<Alert.Description>{copy.groups.loadFailureDescription}</Alert.Description>
			</Alert.Root>
			<Button class="min-h-11 self-start" onclick={() => void refreshGroups()}>
				{copy.groups.retry}
			</Button>
		</div>
	{:else if groups.length === 0}
		<Empty.Root class="min-h-72 border">
			<Empty.Header>
				<Empty.Media variant="icon">
					<UsersRoundIcon data-icon="inline-start" />
				</Empty.Media>
				<Empty.Title>{copy.groups.emptyTitle}</Empty.Title>
				<Empty.Description>{copy.groups.emptyDescription}</Empty.Description>
			</Empty.Header>
			<Empty.Content>
				<div class="flex flex-wrap justify-center gap-2">
					<CreateGroupDialog onSuccess={handleCreated} />
					<JoinGroupDialog onSuccess={handleJoined} />
				</div>
			</Empty.Content>
		</Empty.Root>
	{:else}
		<div class="grid gap-4 md:grid-cols-2">
			{#each groups as group (group.id)}
				<GroupCard {group} />
			{/each}
		</div>
	{/if}
</section>
