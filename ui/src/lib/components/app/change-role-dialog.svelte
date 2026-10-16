<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { toast } from 'svelte-sonner';
	import { projectsApi } from '#lib/api/endpoints';
	import type { Member, PermissionCatalog, ProjectRole } from '#lib/api/types';
	import { Button, buttonVariants } from '#lib/components/ui/button';
	import * as Dialog from '#lib/components/ui/dialog';
	import * as Field from '#lib/components/ui/field';
	import * as Select from '#lib/components/ui/select';
	import { Spinner } from '#lib/components/ui/spinner';
	import { describeError } from '#lib/errors';
	import { cn } from '#lib/utils';

	let {
		projectId,
		member,
		catalog,
		onclose
	}: {
		projectId: number;
		member: Member;
		catalog: PermissionCatalog;
		onclose: () => void;
	} = $props();

	let open = $state(true);
	let role = $state<ProjectRole>('editor');
	let error = $state('');
	let busy = $state(false);

	// The component is remounted whenever the dialog opens, so initialize the role from props.
	$effect(() => {
		role = member.role;
	});

	const roleItems = $derived(catalog.roles.map((option) => ({ value: option.role, label: option.label })));
	const selectedRole = $derived(catalog.roles.find((option) => option.role === role));

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		error = '';
		try {
			await projectsApi.updateMemberRole(projectId, member.user_id, role);
			toast.success(`${member.email} is now ${role}`);
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
				<Dialog.Title>Change role</Dialog.Title>
				<Dialog.Description>
					Set the project role for {member.email}. The role determines default capabilities;
					individual overrides can be added afterwards.
				</Dialog.Description>
			</Dialog.Header>

			<Field.FieldGroup>
				<Field.Field>
					<Field.FieldLabel for="member-role">Role</Field.FieldLabel>
					<Select.Root type="single" bind:value={role} items={roleItems}>
						<Select.Trigger class="w-full">
							<Select.Value placeholder="Select role" />
						</Select.Trigger>
						<Select.Content>
							<Select.Group>
								{#each catalog.roles as option (option.role)}
									<Select.Item value={option.role} label={option.label}>
										{option.label}
									</Select.Item>
								{/each}
							</Select.Group>
						</Select.Content>
					</Select.Root>
					{#if selectedRole}
						<Field.FieldDescription>{selectedRole.description}</Field.FieldDescription>
					{/if}
				</Field.Field>
			</Field.FieldGroup>

			{#if error}
				<Field.Error>{error}</Field.Error>
			{/if}

			<Dialog.Footer>
				<Dialog.Close class={cn(buttonVariants({ variant: 'outline' }))}>Cancel</Dialog.Close>
				<Button type="submit" disabled={busy || role === member.role}>
					{#if busy}
						<Spinner data-icon="inline-start" />
					{/if}
					Save role
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
