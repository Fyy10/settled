<script lang="ts">
	import UsersRoundIcon from "@lucide/svelte/icons/users-round";

	import type { ApiError } from "$lib/api/errors";
	import type { GroupMember } from "$lib/api/types";
	import * as Alert from "$lib/components/ui/alert";
	import * as Card from "$lib/components/ui/card";
	import * as Empty from "$lib/components/ui/empty";
	import { Separator } from "$lib/components/ui/separator";
	import { copy } from "$lib/copy/en";

	import type { MemberNotFoundResolution } from "./member-removal";
	import MemberRow from "./member-row.svelte";
	import RemoveMemberDialog from "./remove-member-dialog.svelte";

	let {
		members,
		groupId = '',
		canManage = false,
		refreshWarning = false,
		onMemberRemoved = async () => {},
		onMemberNotFound = async () => 'unresolved' as const,
		onProtectedError = async () => false
	}: {
		members: readonly GroupMember[];
		groupId?: string;
		canManage?: boolean;
		refreshWarning?: boolean;
		onMemberRemoved?: (userId: string) => void | Promise<void>;
		onMemberNotFound?: (
			userId: string
		) => MemberNotFoundResolution | Promise<MemberNotFoundResolution>;
		onProtectedError?: (error: ApiError) => boolean | Promise<boolean>;
	} = $props();

	let heading: HTMLHeadingElement | null = $state(null);
</script>

<Card.Root aria-labelledby="members-heading">
	<Card.Header>
		<Card.Title>
			<h2 bind:this={heading} id="members-heading" tabindex="-1">
				{copy.groups.workspace.members.title}
			</h2>
		</Card.Title>
		<Card.Description>
			{copy.groups.workspace.members.description}
		</Card.Description>
	</Card.Header>

	<Card.Content>
		{#if refreshWarning}
			<Alert.Root class="mb-3">
				<Alert.Title>
					{copy.groups.workspace.members.removeRefreshFailure}
				</Alert.Title>
			</Alert.Root>
		{/if}
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
						<MemberRow {member}>
							{#snippet actions()}
								{#if canManage && member.role !== 'owner'}
									<RemoveMemberDialog
										{groupId}
										{member}
										onCommitted={onMemberRemoved}
										onNotFound={onMemberNotFound}
										{onProtectedError}
										onCloseFocus={() => heading?.focus()}
									/>
								{/if}
							{/snippet}
						</MemberRow>
					</li>
				{/each}
			</ul>
		{/if}
	</Card.Content>
</Card.Root>
