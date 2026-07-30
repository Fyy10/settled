<script lang="ts">
	import UsersRoundIcon from "@lucide/svelte/icons/users-round";

	import type { GroupMember } from "$lib/api/types";
	import * as Card from "$lib/components/ui/card";
	import * as Empty from "$lib/components/ui/empty";
	import { Separator } from "$lib/components/ui/separator";
	import { copy } from "$lib/copy/en";

	import MemberRow from "./member-row.svelte";

	let { members }: { members: readonly GroupMember[] } = $props();
</script>

<Card.Root aria-labelledby="members-heading">
	<Card.Header>
		<Card.Title>
			<h2 id="members-heading">{copy.groups.workspace.members.title}</h2>
		</Card.Title>
		<Card.Description>
			{copy.groups.workspace.members.description}
		</Card.Description>
	</Card.Header>

	<Card.Content>
		{#if members.length === 0}
			<Empty.Root class="min-h-56">
				<Empty.Header>
					<Empty.Media variant="icon">
						<UsersRoundIcon />
					</Empty.Media>
					<Empty.Title>{copy.groups.workspace.members.emptyTitle}</Empty.Title>
					<Empty.Description>
						{copy.groups.workspace.members.emptyDescription}
					</Empty.Description>
				</Empty.Header>
			</Empty.Root>
		{:else}
			<ul>
				{#each members as member, index (member.userId)}
					<li>
						{#if index > 0}
							<Separator />
						{/if}
						<MemberRow {member} />
					</li>
				{/each}
			</ul>
		{/if}
	</Card.Content>
</Card.Root>
