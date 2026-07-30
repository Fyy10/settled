<script lang="ts">
	import { goto } from "$app/navigation";
	import ArrowLeftIcon from "@lucide/svelte/icons/arrow-left";
	import { untrack } from "svelte";
	import { toast } from "svelte-sonner";

	import {
		ApiError,
		isCsrfApiError,
		isForbiddenApiError,
		isNotFoundApiError,
		isUnauthorizedApiError
	} from "$lib/api/client";
	import type { GroupSummary } from "$lib/api/types";
	import DissolveGroupDialog from "$lib/components/groups/dissolve-group-dialog.svelte";
	import GroupCodeSection from "$lib/components/groups/group-code-section.svelte";
	import RenameGroupForm from "$lib/components/groups/rename-group-form.svelte";
	import { Button } from "$lib/components/ui/button";
	import { Separator } from "$lib/components/ui/separator";
	import { copy } from "$lib/copy/en";
	import { useGroupDetailContext } from "$lib/state/group-detail.svelte";
	import { useGroupListContext } from "$lib/state/group-list.svelte";
	import { pageTitle } from "$lib/utils/page-title";

	const groupDetail = useGroupDetailContext();
	const groupList = useGroupListContext();
	let redirecting = $state(false);
	let redirectFailed = $state(false);

	const groupPath = $derived(
		`/groups/${encodeURIComponent(groupDetail.groupId)}`
	);

	$effect(() => {
		const group = groupDetail.group;
		if (
			groupDetail.status !== 'ready' ||
			group === null ||
			group.currentUserRole === 'owner' ||
			redirecting
		) {
			return;
		}

		redirecting = true;
		untrack(() => {
			void redirectToGroup();
		});
	});

	async function redirectToGroup(): Promise<void> {
		try {
			await goto(groupPath, { replaceState: true });
		} catch {
			redirecting = false;
			redirectFailed = true;
		}
	}

	async function handleRenamed(group: GroupSummary): Promise<boolean> {
		groupDetail.updateGroup(group);
		const results = await Promise.allSettled([
			groupDetail.refreshDetail(),
			groupList.refresh({ preserve: true })
		]);

		return results.every((result) => result.status === 'fulfilled');
	}

	async function handleProtectedError(error: ApiError): Promise<boolean> {
		if (isUnauthorizedApiError(error)) {
			groupDetail.clear();
			return true;
		}
		if (isNotFoundApiError(error)) {
			groupDetail.markHidden();
			return true;
		}
		if (!isForbiddenApiError(error) || isCsrfApiError(error)) {
			return false;
		}

		try {
			const detail = await groupDetail.refreshDetail();
			if (detail.group.currentUserRole !== 'owner') {
				redirecting = true;
				await redirectToGroup();
				return true;
			}
		} catch {
			return groupDetail.status === 'hidden' || groupDetail.status === 'loading';
		}

		return false;
	}

	async function handleDissolved(): Promise<void> {
		groupDetail.markHidden();
		toast.success(copy.groups.settings.danger.success);
		const [listResult] = await Promise.allSettled([
			groupList.refresh({ preserve: true })
		]);
		if (listResult.status === 'rejected') {
			toast.error(copy.groups.settings.danger.refreshFailure);
		}
		try {
			await goto('/groups', { replaceState: true });
		} catch {
			toast.error(copy.groups.settings.danger.navigationFailure);
		}
	}
</script>

<svelte:head>
	<title>
		{pageTitle(
			groupDetail.group === null
				? copy.routes.settings.documentTitle
				: `${copy.routes.settings.documentTitle} · ${groupDetail.group.name}`
		)}
	</title>
</svelte:head>

{#if
	groupDetail.status === 'ready' &&
	groupDetail.group !== null &&
	groupDetail.group.currentUserRole === 'owner'
}
	<section
		class="mx-auto flex w-full max-w-3xl flex-col gap-6"
		aria-labelledby="settings-heading"
	>
		<header class="flex flex-col gap-3">
			<Button href={groupPath} variant="ghost" class="min-h-11 self-start px-2">
				<ArrowLeftIcon data-icon="inline-start" />
				{copy.groups.settings.backToGroup}
			</Button>
			<div class="flex flex-col gap-2">
				<h1 id="settings-heading" tabindex="-1">
					{copy.groups.settings.title}
				</h1>
				<p class="text-muted-foreground">{copy.groups.settings.description}</p>
			</div>
		</header>

		<Separator />

		<div class="flex flex-col gap-6">
			<RenameGroupForm
				group={groupDetail.group}
				onSaved={handleRenamed}
				onProtectedError={handleProtectedError}
			/>
			{#key groupDetail.group.id}
				<GroupCodeSection
					groupId={groupDetail.group.id}
					onProtectedError={handleProtectedError}
				/>
			{/key}
			<DissolveGroupDialog
				group={groupDetail.group}
				onDissolved={handleDissolved}
				onProtectedError={handleProtectedError}
			/>
		</div>
	</section>
{:else if groupDetail.status === 'ready'}
	<div class="mx-auto flex w-full max-w-3xl flex-col gap-4">
		<p class={redirectFailed ? 'text-muted-foreground' : 'sr-only'} role="status">
			{redirectFailed
				? copy.groups.settings.redirectFailure
				: copy.groups.settings.redirecting}
		</p>
		{#if redirectFailed}
			<Button href={groupPath} class="min-h-11 self-start">
				{copy.groups.settings.backToGroup}
			</Button>
		{/if}
	</div>
{/if}
