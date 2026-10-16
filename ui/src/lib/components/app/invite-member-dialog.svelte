<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { toast } from 'svelte-sonner';
	import { usersApi, projectsApi } from '#lib/api/endpoints';
	import type { ProjectRole } from '#lib/api/types';
	import { Button, buttonVariants } from '#lib/components/ui/button';
	import * as Dialog from '#lib/components/ui/dialog';
	import * as Field from '#lib/components/ui/field';
	import { Input } from '#lib/components/ui/input';
	import * as Select from '#lib/components/ui/select';
	import { Spinner } from '#lib/components/ui/spinner';
	import { importPublicKey } from '#lib/crypto/identity';
	import { wrapProjectKey } from '#lib/crypto/project';
	import { describeError } from '#lib/errors';
	import { cn } from '#lib/utils';

	let {
		projectId,
		projectKey,
		canManage = false,
		onclose
	}: {
		projectId: number;
		projectKey: CryptoKey;
		canManage?: boolean;
		onclose: () => void;
	} = $props();

	let open = $state(true);
	let email = $state('');
	let role = $state<ProjectRole>('editor');
	let error = $state('');
	let busy = $state(false);

	const roleItems = [
		{ value: 'editor', label: 'Editor' },
		{ value: 'viewer', label: 'Viewer' },
		{ value: 'owner', label: 'Owner' }
	];

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		const target = email.trim().toLowerCase();
		if (!target) return;
		busy = true;
		error = '';
		try {
			// 1) Look up the member and their public key.
			const invitee = await usersApi.lookup(target);
			// 2) Wrap the project key with the invitee's public key. The inviter's
			//    client has already decrypted the project key.
			const inviteePublicKey = await importPublicKey(invitee.public_key);
			const wrapped = await wrapProjectKey(inviteePublicKey, projectKey);
			// 3) Store the encrypted copy for the invitee.
			await projectsApi.invite(projectId, invitee.id, wrapped, role);

			toast.success(`${invitee.email} added to the project as ${role}`);
			await invalidateAll();
			// Close explicitly because programmatic closing does not trigger onOpenChange.
			onclose();
		} catch (err) {
			error = describeError(err);
		} finally {
			busy = false;
		}
	}
</script>

<Dialog.Root bind:open onOpenChange={(next) => !next && onclose()}>
	<Dialog.Content>
		<form class="grid gap-6" onsubmit={submit}>
			<Dialog.Header>
				<Dialog.Title>Invite member</Dialog.Title>
				<Dialog.Description>
					The project key will be re-encrypted with the invitee’s public key. The server never
					sees the key itself.
				</Dialog.Description>
			</Dialog.Header>

			<Field.FieldGroup>
				<Field.Field data-invalid={error ? true : undefined}>
					<Field.FieldLabel for="invite-email">Member email</Field.FieldLabel>
					<Input
						id="invite-email"
						type="email"
						bind:value={email}
						required
						autocomplete="off"
						placeholder="teammate@company.com"
						aria-invalid={error ? true : undefined}
					/>
					{#if error}
						<Field.Error>{error}</Field.Error>
					{/if}
				</Field.Field>
				<Field.Field>
					<Field.FieldLabel for="invite-role">Role</Field.FieldLabel>
					<Select.Root type="single" bind:value={role} items={roleItems}>
						<Select.Trigger class="w-full">
							<Select.Value placeholder="Select role" />
						</Select.Trigger>
						<Select.Content>
							<Select.Group>
								{#each roleItems as option (option.value)}
									<Select.Item
										value={option.value}
										label={option.label}
										disabled={option.value === 'owner' && !canManage}
									>
										{option.label}
									</Select.Item>
								{/each}
							</Select.Group>
						</Select.Content>
					</Select.Root>
					<Field.FieldDescription>
						Editor can manage content, viewer is read-only. Inviting an owner requires the
						“manage members” capability.
					</Field.FieldDescription>
				</Field.Field>
			</Field.FieldGroup>

			<Dialog.Footer>
				<Dialog.Close class={cn(buttonVariants({ variant: 'outline' }))}>Cancel</Dialog.Close>
				<Button type="submit" disabled={busy || !email.trim()}>
					{#if busy}
						<Spinner data-icon="inline-start" />
					{/if}
					Invite
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
