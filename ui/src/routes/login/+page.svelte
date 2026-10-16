<script lang="ts">
	import { goto } from '$app/navigation';
	import BrandLogo from '#lib/components/app/brand-logo.svelte';
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
	import * as Alert from '#lib/components/ui/alert';
	import { Button } from '#lib/components/ui/button';
	import * as Card from '#lib/components/ui/card';
	import * as Field from '#lib/components/ui/field';
	import { Input } from '#lib/components/ui/input';
	import { Spinner } from '#lib/components/ui/spinner';
	import { describeError } from '#lib/errors';
	import { session } from '#lib/state/session.svelte';

	let email = $state('');
	let password = $state('');
	let error = $state('');
	let progress = $state('');
	let busy = $state(false);

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		error = '';
		progress = 'Preparing…';
		try {
			await session.login(email.trim(), password, (step) => (progress = step));
			await goto('/projects');
		} catch (err) {
			error = describeError(err);
			progress = '';
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head><title>Sign in · Curtain</title></svelte:head>

<div class="flex min-h-svh items-center justify-center px-4 py-10">
	<div class="mx-auto w-full max-w-sm">
		<Card.Root>
			<Card.Header>
				<div class="flex items-center gap-2">
					<BrandLogo class="size-5" />
					<Card.Title>Curtain</Card.Title>
				</div>
				<Card.Description>Sign in to unlock your environment vault.</Card.Description>
			</Card.Header>
			<Card.Content>
				<form onsubmit={submit} class="flex flex-col gap-6">
					{#if error}
						<Alert.Root variant="destructive">
							<TriangleAlertIcon />
							<Alert.Title>Sign-in failed</Alert.Title>
							<Alert.Description>{error}</Alert.Description>
						</Alert.Root>
					{/if}

					<Field.FieldGroup>
						<Field.Field>
							<Field.FieldLabel for="email">Email</Field.FieldLabel>
							<Input
								id="email"
								type="email"
								bind:value={email}
								required
								autocomplete="username"
								placeholder="you@company.com"
							/>
						</Field.Field>
						<Field.Field>
							<Field.FieldLabel for="password">Password</Field.FieldLabel>
							<Input
								id="password"
								type="password"
								bind:value={password}
								required
								autocomplete="current-password"
							/>
						</Field.Field>
					</Field.FieldGroup>

					<Button type="submit" disabled={busy} class="w-full">
						{#if busy}
							<Spinner data-icon="inline-start" />
							{progress || 'Working…'}
						{:else}
							Sign in
						{/if}
					</Button>
				</form>
			</Card.Content>
			<Card.Footer class="justify-center text-xs text-muted-foreground">
				Registration is invite-only. Ask an admin for an invite link.
			</Card.Footer>
		</Card.Root>
	</div>
</div>
