<script lang="ts">
	import * as Avatar from "$lib/components/ui/avatar";
	import { memberInitials } from "$lib/utils/group-workspace";

	let {
		displayName,
		size = 'default',
		decorative = true
	}: {
		displayName: string;
		size?: 'sm' | 'default' | 'lg';
		decorative?: boolean;
	} = $props();
</script>

<Avatar.Root
	{size}
	aria-hidden={decorative ? 'true' : undefined}
	aria-label={decorative ? undefined : displayName}
	role={decorative ? undefined : 'img'}
>
	<Avatar.Fallback>{memberInitials(displayName)}</Avatar.Fallback>
</Avatar.Root>
