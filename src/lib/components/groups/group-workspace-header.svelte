<script lang="ts">
	import { goto } from "$app/navigation";
	import ArrowLeftIcon from "@lucide/svelte/icons/arrow-left";
	import EllipsisIcon from "@lucide/svelte/icons/ellipsis";
	import HandCoinsIcon from "@lucide/svelte/icons/hand-coins";
	import PlusIcon from "@lucide/svelte/icons/plus";
	import SettingsIcon from "@lucide/svelte/icons/settings";

	import type { GroupSummary } from "$lib/api/types";
	import { Badge } from "$lib/components/ui/badge";
	import { Button } from "$lib/components/ui/button";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
	import { copy } from "$lib/copy/en";
	import { formatMemberCount } from "$lib/utils/group";

	let { group }: { group: GroupSummary } = $props();

	const groupPath = $derived(`/groups/${encodeURIComponent(group.id)}`);
	const addExpensePath = $derived(`${groupPath}/expenses/new`);
	const recordPaymentPath = $derived(`${groupPath}/repayments/new`);
	const settingsPath = $derived(`${groupPath}/settings`);
</script>

<header class="flex flex-col gap-5" aria-labelledby="group-heading">
	<div class="flex min-w-0 items-start justify-between gap-3">
		<div class="flex min-w-0 flex-1 flex-col gap-3">
			<Button
				href="/groups"
				variant="ghost"
				class="min-h-11 self-start px-2"
			>
				<ArrowLeftIcon data-icon="inline-start" />
				{copy.groups.workspace.groups}
			</Button>

			<div class="flex min-w-0 flex-col gap-2">
				<h1 id="group-heading" tabindex="-1" class="break-words">
					{group.name}
				</h1>
				<div class="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
					<span>{formatMemberCount(group.memberCount)}</span>
					{#if group.currentUserRole === 'owner'}
						<Badge variant="secondary">{copy.groups.workspace.owner}</Badge>
					{/if}
				</div>
			</div>
		</div>

		<div class="hidden flex-wrap items-center justify-end gap-2 sm:flex">
			<Button href={addExpensePath} class="min-h-11">
				<PlusIcon data-icon="inline-start" />
				{copy.groups.workspace.addExpense}
			</Button>
			<Button href={recordPaymentPath} variant="outline" class="min-h-11">
				<HandCoinsIcon data-icon="inline-start" />
				{copy.groups.workspace.recordPayment}
			</Button>
			{#if group.currentUserRole === 'owner'}
				<DropdownMenu.Root>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<Button
								{...props}
								variant="ghost"
								size="icon-lg"
								aria-label={copy.groups.workspace.openMenu}
							>
								<EllipsisIcon data-icon="icon-only" />
							</Button>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end">
						<DropdownMenu.Group>
							<DropdownMenu.Item
								class="min-h-11"
								onSelect={() => void goto(settingsPath)}
							>
								<SettingsIcon data-icon="inline-start" />
								{copy.groups.workspace.settings}
							</DropdownMenu.Item>
						</DropdownMenu.Group>
					</DropdownMenu.Content>
				</DropdownMenu.Root>
			{/if}
		</div>

		{#if group.currentUserRole === 'owner'}
			<div class="sm:hidden">
				<DropdownMenu.Root>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<Button
								{...props}
								variant="ghost"
								size="icon-lg"
								aria-label={copy.groups.workspace.openMenu}
							>
								<EllipsisIcon data-icon="icon-only" />
							</Button>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end">
						<DropdownMenu.Group>
							<DropdownMenu.Item
								class="min-h-11"
								onSelect={() => void goto(settingsPath)}
							>
								<SettingsIcon data-icon="inline-start" />
								{copy.groups.workspace.settings}
							</DropdownMenu.Item>
						</DropdownMenu.Group>
					</DropdownMenu.Content>
				</DropdownMenu.Root>
			</div>
		{/if}
	</div>
</header>

<nav
	class="fixed inset-x-0 bottom-0 z-20 border-t bg-background px-4 pt-3 pb-[calc(env(safe-area-inset-bottom)+0.75rem)] sm:hidden"
	aria-label={copy.groups.workspace.actionsLabel}
>
	<div class="mx-auto flex w-full max-w-lg gap-2">
		<Button href={addExpensePath} class="min-h-11 min-w-0 flex-1">
			{copy.groups.workspace.addExpense}
		</Button>
		<Button
			href={recordPaymentPath}
			variant="outline"
			class="min-h-11 min-w-0 flex-1"
		>
			{copy.groups.workspace.recordPayment}
		</Button>
	</div>
</nav>
