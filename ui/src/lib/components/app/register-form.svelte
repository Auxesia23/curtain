<script lang="ts">
	import { goto } from '$app/navigation';
	import KeyRoundIcon from '@lucide/svelte/icons/key-round';
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
	import * as Alert from '#lib/components/ui/alert';
	import { Button } from '#lib/components/ui/button';
	import * as Field from '#lib/components/ui/field';
	import { Input } from '#lib/components/ui/input';
	import { Spinner } from '#lib/components/ui/spinner';
	import { describeError } from '#lib/errors';
	import { session } from '#lib/state/session.svelte';

	let {
		inviteToken = '',
		submitLabel = 'Generate keys & sign up'
	}: { inviteToken?: string; submitLabel?: string } = $props();

	let email = $state('');
	let password = $state('');
	let confirm = $state('');
	let error = $state('');
	let progress = $state('');
	let busy = $state(false);

	function validate(): string | null {
		if (password.length < 8) return 'Password must be at least 8 characters.';
		if (password !== confirm) return 'Passwords do not match.';
		return null;
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		const invalid = validate();
		if (invalid) {
			error = invalid;
			return;
		}
		busy = true;
		error = '';
		progress = 'Preparing…';
		try {
			await session.register(
				email.trim(),
				password,
				(step) => (progress = step),
				inviteToken || undefined
			);
			await goto('/projects');
		} catch (err) {
			error = describeError(err);
			progress = '';
		} finally {
			busy = false;
		}
	}
</script>

<form onsubmit={submit} class="grid gap-6">
	{#if error}
		<Alert.Root variant="destructive">
			<TriangleAlertIcon />
			<Alert.Title>Sign-up failed</Alert.Title>
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
				minlength={8}
				autocomplete="new-password"
			/>
			<Field.FieldDescription>
				At least 8 characters. This password unlocks your private key — there is no password
				reset.
			</Field.FieldDescription>
		</Field.Field>
		<Field.Field>
			<Field.FieldLabel for="confirm">Confirm password</Field.FieldLabel>
			<Input
				id="confirm"
				type="password"
				bind:value={confirm}
				required
				autocomplete="new-password"
			/>
		</Field.Field>
	</Field.FieldGroup>

	<Button type="submit" disabled={busy} class="w-full">
		{#if busy}
			<Spinner data-icon="inline-start" />
			{progress || 'Working…'}
		{:else}
			<KeyRoundIcon data-icon="inline-start" />
			{submitLabel}
		{/if}
	</Button>
</form>
