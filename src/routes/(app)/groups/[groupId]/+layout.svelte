<script lang="ts">
	let { children } = $props();
</script>

<div class="min-w-0">
	{@render children?.()}
</div>
