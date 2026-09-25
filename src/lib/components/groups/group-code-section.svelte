<script lang="ts">
	import CheckIcon from "@lucide/svelte/icons/check";
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
	import CopyIcon from "@lucide/svelte/icons/copy";
	import { onDestroy, onMount, tick } from "svelte";

	import { getGroupJoinCode } from "$lib/api/groups";
	import {
		ApiError,
		isForbiddenApiError,
		isNotFoundApiError,
		isUnauthorizedApiError
	} from "$lib/api/client";
	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import * as Card from "$lib/components/ui/card";
	import * as InputGroup from "$lib/components/ui/input-group";
	import { Skeleton } from "$lib/components/ui/skeleton";
	import { Spinner } from "$lib/components/ui/spinner";
	import { copy } from "$lib/copy/en";

	type CodeStatus = 'idle' | 'loading' | 'ready' | 'error';

	let {
		groupId,
		onProtectedError
	}: {
		groupId: string;
		onProtectedError: (error: ApiError) => boolean | Promise<boolean>;
	} = $props();

	let section: HTMLElement | null = $state(null);
	let codeInput: HTMLInputElement | null = $state(null);
	let status = $state<CodeStatus>('idle');
	let joinCode = $state('');
	let copyError = $state(false);
	let copyAnnouncement = $state('');
	let observer: IntersectionObserver | null = null;
	let controller: AbortController | null = null;

	onMount(() => {
		if (typeof IntersectionObserver === 'undefined') {
			void loadCode();
			return;
		}

		observer = new IntersectionObserver((entries) => {
			if (!entries.some((entry) => entry.isIntersecting)) {
				return;
			}

			observer?.disconnect();
			observer = null;
			void loadCode();
		});
		if (section !== null) {
			observer.observe(section);
		}
	});

	onDestroy(() => {
		observer?.disconnect();
		controller?.abort();
		joinCode = '';
	});

	async function loadCode(): Promise<void> {
		if (status === 'loading' || status === 'ready') {
			return;
		}

		status = 'loading';
		copyError = false;
		controller?.abort();
		controller = new AbortController();
		try {
			joinCode = await getGroupJoinCode(groupId, {
				signal: controller.signal
			});
			status = 'ready';
		} catch (error) {
			if (controller.signal.aborted) {
				return;
			}
			joinCode = '';
			if (
				error instanceof ApiError &&
				(isForbiddenApiError(error) ||
					isNotFoundApiError(error) ||
					isUnauthorizedApiError(error))
			) {
				try {
					if (await onProtectedError(error)) {
						return;
					}
				} catch {
					// Fall through to the retryable read error.
				}
			}
			status = 'error';
		}
	}

	async function copyCode(): Promise<void> {
		copyError = false;
		copyAnnouncement = '';
		try {
			if (navigator.clipboard === undefined) {
				throw new Error('Clipboard API is unavailable.');
			}
			await navigator.clipboard.writeText(joinCode);
			copyAnnouncement = copy.groups.settings.code.copied;
		} catch {
			copyError = true;
			await tick();
			codeInput?.focus();
			codeInput?.select();
		}
	}
</script>

<Card.Root bind:ref={section} aria-labelledby="group-code-heading">
	<Card.Header>
		<Card.Title>
			<h2 id="group-code-heading">{copy.groups.settings.code.title}</h2>
		</Card.Title>
		<Card.Description>{copy.groups.settings.code.description}</Card.Description>
	</Card.Header>
	<Card.Content class="flex flex-col gap-3">
		{#if status === 'ready'}
			<InputGroup.Root class="h-11">
				<InputGroup.Input
					bind:ref={codeInput}
					value={joinCode}
					aria-label={copy.groups.settings.code.title}
					readonly
					spellcheck={false}
					class="h-11 select-all font-mono text-base tabular-nums"
				/>
				<InputGroup.Addon align="inline-end">
					<InputGroup.Button
						class="h-9 px-3"
						onclick={() => void copyCode()}
					>
						{#if copyAnnouncement}
							<CheckIcon data-icon="inline-start" />
						{:else}
							<CopyIcon data-icon="inline-start" />
						{/if}
						{copy.groups.settings.code.copy}
					</InputGroup.Button>
				</InputGroup.Addon>
			</InputGroup.Root>
			<span class="sr-only" aria-live="polite">{copyAnnouncement}</span>
			{#if copyError}
				<Alert.Root variant="destructive">
					<CircleAlertIcon data-icon="inline-start" />
					<Alert.Title>{copy.groups.settings.code.copyFailure}</Alert.Title>
				</Alert.Root>
			{/if}
		{:else if status === 'error'}
			<Alert.Root variant="destructive">
				<CircleAlertIcon data-icon="inline-start" />
				<Alert.Title>{copy.groups.settings.code.loadFailure}</Alert.Title>
			</Alert.Root>
			<Button
				variant="outline"
				class="min-h-11 self-start"
				onclick={() => void loadCode()}
			>
				{copy.groups.settings.code.retry}
			</Button>
		{:else if status === 'loading'}
			<div class="flex min-h-11 items-center gap-3" aria-live="polite">
				<Spinner aria-label={copy.groups.settings.code.loading} />
				<span class="text-sm font-medium">{copy.groups.settings.code.loading}</span>
			</div>
		{:else}
			<Skeleton class="h-11 w-full" aria-hidden="true" />
		{/if}
	</Card.Content>
</Card.Root>
