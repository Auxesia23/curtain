<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { toast } from 'svelte-sonner';
	import { projectsApi } from '#lib/api/endpoints';
	import type { Capability, Member, PermissionCatalog, PermissionEffect } from '#lib/api/types';
	import { Button, buttonVariants } from '#lib/components/ui/button';
	import * as Dialog from '#lib/components/ui/dialog';
	import * as Field from '#lib/components/ui/field';
	import { Separator } from '#lib/components/ui/separator';
	import { Spinner } from '#lib/components/ui/spinner';
	import * as ToggleGroup from '#lib/components/ui/toggle-group';
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
	let error = $state('');
	let busy = $state(false);

	// 'default' follows the role template. Other values are explicit overrides.
	let choices = $state<Record<string, string | undefined>>({});

	// The component is remounted whenever the dialog opens, so initialize choices from props.
	$effect(() => {
		choices = Object.fromEntries(
			catalog.capabilities.map((info) => [
				info.capability,
				member.overrides.find((override) => override.capability === info.capability)?.effect ??
					'default'
			])
		);
	});

	const roleDefaults = $derived(
		catalog.roles.find((option) => option.role === member.role)?.capabilities ?? []
	);
	const groups = $derived([
		{ key: 'content', label: 'Content', items: catalog.capabilities.filter((c) => c.group === 'content') },
		{ key: 'members', label: 'Members', items: catalog.capabilities.filter((c) => c.group === 'members') },
		{ key: 'project', label: 'Project', items: catalog.capabilities.filter((c) => c.group === 'project') }
	]);

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		const overrides = Object.entries(choices)
			.filter(([, value]) => value === 'allow' || value === 'deny')
			.map(([capability, effect]) => ({
				capability: capability as Capability,
				effect: effect as PermissionEffect
			}));

		busy = true;
		error = '';
		try {
			await projectsApi.setMemberPermissions(projectId, member.user_id, overrides);
			toast.success(`Permissions updated for ${member.email}`);
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
	<Dialog.Content class="sm:max-w-lg">
		<form class="grid gap-6" onsubmit={submit}>
			<Dialog.Header>
				<Dialog.Title>Permissions</Dialog.Title>
				<Dialog.Description>
					Capabilities for {member.email}. “Default” follows the {member.role} role; explicit
					Allow/Deny always wins over the role.
				</Dialog.Description>
			</Dialog.Header>

			<div class="flex max-h-[50vh] flex-col overflow-y-auto pr-1">
				{#each groups as group, index (group.key)}
					{#if index > 0}
						<Separator class="my-3" />
					{/if}
					<span class="pb-1 text-xs font-medium tracking-wide text-muted-foreground uppercase">
						{group.label}
					</span>
					{#each group.items as info (info.capability)}
						<div class="flex items-center justify-between gap-4 py-2">
							<div class="flex min-w-0 flex-col gap-0.5">
								<span class="text-sm font-medium">{info.label}</span>
								<span class="text-xs text-muted-foreground">{info.description}</span>
								<span class="text-xs text-muted-foreground">
									Role default: {roleDefaults.includes(info.capability) ? 'allowed' : 'not allowed'}
								</span>
							</div>
							<ToggleGroup.Root
								type="single"
								bind:value={choices[info.capability]}
								variant="outline"
								size="sm"
								class="shrink-0"
								aria-label={info.label}
							>
								<ToggleGroup.Item value="default">Default</ToggleGroup.Item>
								<ToggleGroup.Item value="allow">Allow</ToggleGroup.Item>
								<ToggleGroup.Item value="deny">Deny</ToggleGroup.Item>
							</ToggleGroup.Root>
						</div>
					{/each}
				{/each}
			</div>

			{#if error}
				<Field.Error>{error}</Field.Error>
			{/if}

			<Dialog.Footer>
				<Dialog.Close class={cn(buttonVariants({ variant: 'outline' }))}>Cancel</Dialog.Close>
				<Button type="submit" disabled={busy}>
					{#if busy}
						<Spinner data-icon="inline-start" />
					{/if}
					Save permissions
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
