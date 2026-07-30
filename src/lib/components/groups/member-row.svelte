<script lang="ts">
	import type { Snippet } from "svelte";

	import type { GroupMember } from "$lib/api/types";
	import { Badge } from "$lib/components/ui/badge";
	import { copy } from "$lib/copy/en";
	import { formatMemberJoinedAt } from "$lib/utils/group-workspace";

	import MemberAvatar from "./member-avatar.svelte";

	let {
		member,
		actions
	}: {
		member: GroupMember;
		actions?: Snippet;
	} = $props();
</script>

<div class="flex min-w-0 items-start gap-3 py-4">
	<MemberAvatar displayName={member.displayName} size="lg" />
	<div class="flex min-w-0 flex-1 flex-col gap-1">
		<div class="flex min-w-0 flex-wrap items-center gap-2">
			<span class="font-semibold break-words">{member.displayName}</span>
			{#if member.role === 'owner'}
				<Badge variant="secondary">{copy.groups.workspace.owner}</Badge>
			{/if}
		</div>
		<span class="break-all text-sm text-muted-foreground">{member.email}</span>
		<span class="text-xs text-muted-foreground">
			{copy.groups.workspace.members.joined}
			{formatMemberJoinedAt(member.joinedAt)}
		</span>
	</div>
	{#if actions}
		<div class="shrink-0">
			{@render actions()}
		</div>
	{/if}
</div>
