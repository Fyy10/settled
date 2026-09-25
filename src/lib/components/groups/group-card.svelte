<script lang="ts">
	import ChevronRightIcon from "@lucide/svelte/icons/chevron-right";

	import type { GroupSummary } from "$lib/api/types";
	import { Badge } from "$lib/components/ui/badge";
	import * as Card from "$lib/components/ui/card";
	import { copy } from "$lib/copy/en";
	import {
		formatGroupUpdatedAt,
		formatMemberCount
	} from "$lib/utils/group";

	let { group }: { group: GroupSummary } = $props();
</script>

<a
	href={`/groups/${encodeURIComponent(group.id)}`}
	class="block h-full rounded-xl outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
>
	<Card.Root class="h-full">
		<Card.Header class="min-w-0 border-b">
			<Card.Title class="min-w-0">
				<span class="sr-only">Open </span>
				<span class="break-words">{group.name}</span>
			</Card.Title>
			{#if group.currentUserRole === 'owner'}
				<Card.Action>
					<Badge variant="secondary">{copy.groups.owner}</Badge>
				</Card.Action>
			{/if}
		</Card.Header>

		<Card.Content class="flex-1">
			<p>{formatMemberCount(group.memberCount)}</p>
		</Card.Content>

		<Card.Footer class="justify-between gap-3">
			<time class="text-xs text-muted-foreground" datetime={group.updatedAt}>
				{copy.groups.updated} {formatGroupUpdatedAt(group.updatedAt)}
			</time>
			<ChevronRightIcon data-icon="inline-end" aria-hidden="true" />
		</Card.Footer>
	</Card.Root>
</a>
