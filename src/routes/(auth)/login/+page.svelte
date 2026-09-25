<script lang="ts">
	import LoginForm from "$lib/components/auth/login-form.svelte";
	import { copy } from "$lib/copy/en";
	import { pageTitle } from "$lib/utils/page-title";
</script>

<svelte:head>
	<title>{pageTitle(copy.routes.login.documentTitle)}</title>
</svelte:head>

<LoginForm />
