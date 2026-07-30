<script lang="ts">
	import RegisterForm from "$lib/components/auth/register-form.svelte";
	import { copy } from "$lib/copy/en";
	import { pageTitle } from "$lib/utils/page-title";
</script>

<svelte:head>
	<title>{pageTitle(copy.routes.register.documentTitle)}</title>
</svelte:head>

<RegisterForm />
