<script lang="ts">
	import { goto } from '$app/navigation';
	import LockIcon from '@lucide/svelte/icons/lock';
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
	import * as Alert from '#lib/components/ui/alert';
	import { Button } from '#lib/components/ui/button';
	import * as Card from '#lib/components/ui/card';
	import * as Field from '#lib/components/ui/field';
	import { Input } from '#lib/components/ui/input';
	import { Separator } from '#lib/components/ui/separator';
	import { Spinner } from '#lib/components/ui/spinner';
	import { describeError } from '#lib/errors';
	import { session } from '#lib/state/session.svelte';

	let password = $state('');
	let error = $state('');
	let progress = $state('');
	let busy = $state(false);

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		error = '';
		progress = 'Unlocking…';
		try {
			await session.unlock(password, (step) => (progress = step));
			await goto('/projects');
		} catch (err) {
			error = describeError(err);
			progress = '';
		} finally {
			busy = false;
		}
	}

	async function logout() {
		await session.logout();
		await goto('/login');
	}
</script>

<svelte:head><title>Unlock · Curtain</title></svelte:head>

<div class="flex min-h-svh items-center justify-center px-4 py-10">
	<div class="mx-auto w-full max-w-sm">
		<Card.Root>
			<Card.Header>
				<div class="flex items-center gap-2">
					<LockIcon class="size-5" />
					<Card.Title>Vault locked</Card.Title>
				</div>
				<Card.Description>
					Enter your password to unlock the private key for
					<span class="font-medium text-foreground">{session.user?.email}</span>.
				</Card.Description>
			</Card.Header>
			<Card.Content>
				<form onsubmit={submit} class="flex flex-col gap-6">
					{#if error}
						<Alert.Root variant="destructive">
							<TriangleAlertIcon />
							<Alert.Title>Unlock failed</Alert.Title>
							<Alert.Description>{error}</Alert.Description>
						</Alert.Root>
					{/if}

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

					<Button type="submit" disabled={busy} class="w-full">
						{#if busy}
							<Spinner data-icon="inline-start" />
							{progress || 'Working…'}
						{:else}
							Unlock
						{/if}
					</Button>
				</form>
			</Card.Content>
			<Card.Footer class="flex flex-col gap-4">
				<Separator />
				<Button variant="ghost" size="sm" onclick={logout} class="w-full">
					Sign in with a different account
				</Button>
			</Card.Footer>
		</Card.Root>
	</div>
</div>
