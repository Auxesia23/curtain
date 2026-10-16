<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import LinkIcon from '@lucide/svelte/icons/link';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import { toast } from 'svelte-sonner';
	import { invitesApi } from '#lib/api/endpoints';
	import type { CreatedInvite } from '#lib/api/types';
	import * as Alert from '#lib/components/ui/alert';
	import { Button, buttonVariants } from '#lib/components/ui/button';
	import * as Dialog from '#lib/components/ui/dialog';
	import * as Field from '#lib/components/ui/field';
	import { Input } from '#lib/components/ui/input';
	import { Spinner } from '#lib/components/ui/spinner';
	import { describeError } from '#lib/errors';
	import { cn } from '#lib/utils';

	let open = $state(false);
	let maxUses = $state(5);
	let expiresDays = $state(7);
	let error = $state('');
	let busy = $state(false);
	let created = $state<CreatedInvite | null>(null);

	const shareUrl = $derived(
		created ? new URL(created.url, window.location.origin).toString() : ''
	);

	function reset() {
		created = null;
		error = '';
		maxUses = 5;
		expiresDays = 7;
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		error = '';
		try {
			created = await invitesApi.create(Number(maxUses), Number(expiresDays) * 24);
			await invalidateAll();
		} catch (err) {
			error = describeError(err);
		} finally {
			busy = false;
		}
	}

	async function copyLink() {
		if (!shareUrl) return;
		try {
			await navigator.clipboard.writeText(shareUrl);
			toast.success('Invite link copied');
		} catch {
			toast.error('Could not copy — select the link manually.');
		}
	}
</script>

<Dialog.Root
	bind:open
	onOpenChange={(next) => {
		if (!next) reset();
	}}
>
	<Dialog.Trigger class={cn(buttonVariants())}>
		<PlusIcon data-icon="inline-start" />
		Create invite
	</Dialog.Trigger>
	<Dialog.Content>
		{#if created}
			<Dialog.Header>
				<Dialog.Title>Invite link created</Dialog.Title>
				<Dialog.Description>
					Copy it now — the token is shown only once and can’t be recovered later.
				</Dialog.Description>
			</Dialog.Header>

			<Alert.Root>
				<LinkIcon />
				<Alert.Title>Share this link</Alert.Title>
				<Alert.Description>
					<code class="text-xs break-all">{shareUrl}</code>
				</Alert.Description>
			</Alert.Root>

			<Dialog.Footer>
				<Button variant="outline" onclick={copyLink}>
					<CopyIcon data-icon="inline-start" />
					Copy link
				</Button>
				<Dialog.Close class={cn(buttonVariants())}>Done</Dialog.Close>
			</Dialog.Footer>
		{:else}
			<form class="grid gap-6" onsubmit={submit}>
				<Dialog.Header>
					<Dialog.Title>Create invite link</Dialog.Title>
					<Dialog.Description>
						The link is generic: anyone who has it can create an account until it expires or
						reaches its usage limit. Revoke it once your team has joined.
					</Dialog.Description>
				</Dialog.Header>

				<Field.FieldGroup>
					<Field.Field>
						<Field.FieldLabel for="invite-max-uses">Max uses</Field.FieldLabel>
						<Input
							id="invite-max-uses"
							type="number"
							min={1}
							max={100}
							bind:value={maxUses}
							required
						/>
					</Field.Field>
					<Field.Field>
						<Field.FieldLabel for="invite-expires">Expires in (days)</Field.FieldLabel>
						<Input
							id="invite-expires"
							type="number"
							min={1}
							max={365}
							bind:value={expiresDays}
							required
						/>
					</Field.Field>
				</Field.FieldGroup>

				{#if error}
					<Field.Error>{error}</Field.Error>
				{/if}

				<Dialog.Footer>
					<Dialog.Close class={cn(buttonVariants({ variant: 'outline' }))}>Cancel</Dialog.Close>
					<Button type="submit" disabled={busy}>
						{#if busy}
							<Spinner data-icon="inline-start" />
						{/if}
						Create link
					</Button>
				</Dialog.Footer>
			</form>
		{/if}
	</Dialog.Content>
</Dialog.Root>
