<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import CheckIcon from '@lucide/svelte/icons/check';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import EllipsisIcon from '@lucide/svelte/icons/ellipsis';
	import EyeIcon from '@lucide/svelte/icons/eye';
	import EyeOffIcon from '@lucide/svelte/icons/eye-off';
	import FolderIcon from '@lucide/svelte/icons/folder';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import UserPlusIcon from '@lucide/svelte/icons/user-plus';
	import { toast } from 'svelte-sonner';
	import { environmentsApi, projectsApi, variablesApi } from '#lib/api/endpoints';
	import type { Capability, Environment, Member, VariableRow } from '#lib/api/types';
	import * as AlertDialog from '#lib/components/ui/alert-dialog';
	import { Badge } from '#lib/components/ui/badge';
	import { Button, buttonVariants } from '#lib/components/ui/button';
	import * as DropdownMenu from '#lib/components/ui/dropdown-menu';
	import * as Empty from '#lib/components/ui/empty';
	import * as Select from '#lib/components/ui/select';
	import { Spinner } from '#lib/components/ui/spinner';
	import * as Table from '#lib/components/ui/table';
	import * as Tabs from '#lib/components/ui/tabs';
	import CreateEnvironmentDialog from '#lib/components/app/create-environment-dialog.svelte';
	import ChangeRoleDialog from '#lib/components/app/change-role-dialog.svelte';
	import InviteMemberDialog from '#lib/components/app/invite-member-dialog.svelte';
	import MemberPermissionsDialog from '#lib/components/app/member-permissions-dialog.svelte';
	import VariableFormDialog from '#lib/components/app/variable-form-dialog.svelte';
	import { decryptText, encryptText } from '#lib/crypto/project';
	import { describeError } from '#lib/errors';
	import { session } from '#lib/state/session.svelte';
	import { cn } from '#lib/utils';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let tab = $state('variables');
	let selectedEnvId = $state('');
	let revealed = $state<Record<number, string>>({});
	let revealingId = $state<number | null>(null);
	let variableEditor = $state<{
		mode: 'create' | 'edit';
		row: VariableRow | null;
		name: string;
		value: string;
	} | null>(null);
	let deleteVariable = $state<VariableRow | null>(null);
	let deleteEnvironment = $state<Environment | null>(null);
	let deleteProjectOpen = $state(false);
	let deleting = $state(false);
	let deletingProject = $state(false);
	let inviteOpen = $state(false);
	let roleTarget = $state<Member | null>(null);
	let permissionsTarget = $state<Member | null>(null);
	let removeTarget = $state<Member | null>(null);
	let removingMember = $state(false);

	// The server-calculated capabilities for the current user, used to gate actions.
	const can = (capability: Capability) => (data.project.capabilities ?? []).includes(capability);

	const projectKey = $derived(session.getProjectKey(data.project.id));
	const selectedEnvironment = $derived(
		data.environments.find((environment) => String(environment.ID) === selectedEnvId) ?? null
	);
	// bits-ui uses this list to resolve the label before Content mounts.
	// Without it, the trigger displays the raw value.
	const environmentItems = $derived(
		data.environments.map((environment) => ({
			value: String(environment.ID),
			label: environment.name
		}))
	);
	const currentVariables = $derived(
		selectedEnvId ? (data.variablesByEnvironment[Number(selectedEnvId)] ?? []) : []
	);

	// Keep the selected environment valid after data is refreshed.
	$effect(() => {
		if (
			data.environments.length > 0 &&
			!data.environments.some((environment) => String(environment.ID) === selectedEnvId)
		) {
			selectedEnvId = String(data.environments[0].ID);
		}
	});

	async function toggleReveal(row: VariableRow) {
		if (revealed[row.ID] !== undefined) {
			const next = { ...revealed };
			delete next[row.ID];
			revealed = next;
			return;
		}
		if (!projectKey) {
			toast.error('Project key is unavailable, reload the page.');
			return;
		}
		revealingId = row.ID;
		try {
			const plain = await decryptText(projectKey, row.encrypted_value);
			revealed = { ...revealed, [row.ID]: plain };
		} catch (error) {
			toast.error(`Failed to decrypt value: ${describeError(error)}`);
		} finally {
			revealingId = null;
		}
	}

	async function copyValue(row: VariableRow) {
		try {
			if (!projectKey) throw new Error('Project key is unavailable');
			const plain = revealed[row.ID] ?? (await decryptText(projectKey, row.encrypted_value));
			await navigator.clipboard.writeText(plain);
			toast.success('Value copied to clipboard');
		} catch (error) {
			toast.error(describeError(error));
		}
	}

	async function openEdit(row: VariableRow) {
		try {
			if (!projectKey) throw new Error('Project key is unavailable');
			const value = await decryptText(projectKey, row.encrypted_value);
			variableEditor = { mode: 'edit', row, name: row.name, value };
		} catch (error) {
			toast.error(describeError(error));
		}
	}

	function openCreate() {
		variableEditor = { mode: 'create', row: null, name: '', value: '' };
	}

	async function saveVariable(payload: { name: string; value: string }) {
		if (!projectKey) throw new Error('Project key is unavailable');
		if (!selectedEnvironment) throw new Error('No environment selected');

		// Encrypt the variable name and value with the project key before sending them.
		const encrypted_key = await encryptText(projectKey, payload.name);
		const encrypted_value = await encryptText(projectKey, payload.value);

		if (variableEditor?.mode === 'edit' && variableEditor.row) {
			await variablesApi.update(variableEditor.row.ID, { encrypted_key, encrypted_value });
		} else {
			await variablesApi.create(selectedEnvironment.ID, { encrypted_key, encrypted_value });
		}
		toast.success('Variable saved');
		await invalidateAll();
	}

	async function confirmDeleteVariable() {
		if (!deleteVariable) return;
		deleting = true;
		try {
			await variablesApi.remove(deleteVariable.ID);
			toast.success(`Variable "${deleteVariable.name}" deleted`);
			deleteVariable = null;
			await invalidateAll();
		} catch (error) {
			toast.error(describeError(error));
		} finally {
			deleting = false;
		}
	}

	async function confirmDeleteEnvironment() {
		if (!deleteEnvironment) return;
		deleting = true;
		try {
			await environmentsApi.remove(deleteEnvironment.ID);
			toast.success(`Environment "${deleteEnvironment.name}" deleted`);
			deleteEnvironment = null;
			await invalidateAll();
		} catch (error) {
			toast.error(describeError(error));
		} finally {
			deleting = false;
		}
	}

	async function confirmDeleteProject() {
		deletingProject = true;
		try {
			await projectsApi.remove(data.project.id);
			session.forgetProjectKey(data.project.id);
			toast.success(`Project "${data.project.name}" deleted`);
			await goto('/projects');
		} catch (error) {
			toast.error(describeError(error));
		} finally {
			deletingProject = false;
		}
	}

	async function confirmRemoveMember() {
		if (!removeTarget) return;
		removingMember = true;
		try {
			await projectsApi.removeMember(data.project.id, removeTarget.user_id);
			toast.success(`${removeTarget.email} removed from the project`);
			removeTarget = null;
			await invalidateAll();
		} catch (error) {
			toast.error(describeError(error));
		} finally {
			removingMember = false;
		}
	}
</script>

<svelte:head><title>{data.project.name} · Curtain</title></svelte:head>

<div class="flex flex-col gap-6">
	<div class="flex flex-wrap items-center justify-between gap-4">
		<div class="flex items-center gap-3">
			<Button href="/projects" variant="ghost" size="icon-sm" aria-label="Back">
				<ArrowLeftIcon />
			</Button>
			<div>
				<div class="flex items-center gap-2">
					<h1 class="text-2xl font-semibold tracking-tight">{data.project.name}</h1>
					<Badge variant={data.project.role === 'owner' ? 'default' : 'secondary'}>
						{data.project.role}
					</Badge>
				</div>
				<p class="text-sm text-muted-foreground">
					All variables are encrypted with a project key that only exists in members’ browsers.
				</p>
			</div>
		</div>
		{#if can('member.invite') || can('project.delete')}
			<div class="flex items-center gap-2">
				{#if can('member.invite')}
					<Button onclick={() => (inviteOpen = true)}>
						<UserPlusIcon data-icon="inline-start" />
						Invite member
					</Button>
				{/if}
				{#if can('project.delete')}
					<DropdownMenu.Root>
						<DropdownMenu.Trigger
							class={cn(buttonVariants({ variant: 'outline', size: 'icon' }))}
							aria-label="Project actions"
						>
							<EllipsisIcon />
						</DropdownMenu.Trigger>
						<DropdownMenu.Content align="end" class="w-44">
							<DropdownMenu.Group>
								<DropdownMenu.Item
									variant="destructive"
									onSelect={() => (deleteProjectOpen = true)}
								>
									<Trash2Icon />
									Delete project
								</DropdownMenu.Item>
							</DropdownMenu.Group>
						</DropdownMenu.Content>
					</DropdownMenu.Root>
				{/if}
			</div>
		{/if}
	</div>

	<Tabs.Root bind:value={tab}>
		<Tabs.List>
			<Tabs.Trigger value="variables">Variables</Tabs.Trigger>
			<Tabs.Trigger value="members">Members</Tabs.Trigger>
		</Tabs.List>

		<Tabs.Content value="variables" class="flex flex-col gap-4 pt-4">
			{#if data.environments.length === 0}
				<Empty.Root>
					<Empty.Header>
						<Empty.Media variant="icon">
							<FolderIcon />
						</Empty.Media>
						<Empty.Title>No environments yet</Empty.Title>
						<Empty.Description>
							{#if can('env.create')}
								Create your first environment (e.g. development) to start storing variables.
							{:else}
								No environments have been created yet. Ask a project owner to create one.
							{/if}
						</Empty.Description>
					</Empty.Header>
					{#if can('env.create')}
						<Empty.Content>
							<CreateEnvironmentDialog projectId={data.project.id} />
						</Empty.Content>
					{/if}
				</Empty.Root>
			{:else}
				<div class="flex flex-wrap items-center gap-2">
					<Select.Root type="single" bind:value={selectedEnvId} items={environmentItems}>
						<Select.Trigger class="w-56">
							<Select.Value placeholder="Select environment" />
						</Select.Trigger>
						<Select.Content>
							<Select.Group>
								{#each data.environments as environment (environment.ID)}
									<Select.Item value={String(environment.ID)} label={environment.name}>
										{environment.name}
									</Select.Item>
								{/each}
							</Select.Group>
						</Select.Content>
					</Select.Root>
					{#if can('env.create')}
						<CreateEnvironmentDialog projectId={data.project.id} />
					{/if}

					<div class="ml-auto flex items-center gap-2">
						{#if can('var.create')}
							<Button size="sm" onclick={openCreate}>
								<PlusIcon data-icon="inline-start" />
								New variable
							</Button>
						{/if}
						{#if can('env.delete')}
							<DropdownMenu.Root>
								<DropdownMenu.Trigger
									class={cn(buttonVariants({ variant: 'outline', size: 'icon-sm' }))}
									aria-label="Environment actions"
								>
									<EllipsisIcon />
								</DropdownMenu.Trigger>
								<DropdownMenu.Content align="end">
									<DropdownMenu.Group>
										<DropdownMenu.Item
											variant="destructive"
											onSelect={() =>
												selectedEnvironment && (deleteEnvironment = selectedEnvironment)}
										>
											<Trash2Icon />
											Delete environment
										</DropdownMenu.Item>
									</DropdownMenu.Group>
								</DropdownMenu.Content>
							</DropdownMenu.Root>
						{/if}
					</div>
				</div>

				<div class="rounded-lg border bg-background">
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head>Name</Table.Head>
								<Table.Head>Value</Table.Head>
								<Table.Head class="w-12" />
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each currentVariables as row (row.ID)}
								<Table.Row>
									<Table.Cell class="font-mono text-xs font-medium">{row.name}</Table.Cell>
									<Table.Cell class="max-w-0 font-mono text-xs">
										<div class="flex items-center gap-2">
											{#if revealingId === row.ID}
												<Spinner />
											{/if}
											<span class="truncate">{revealed[row.ID] ?? '••••••••'}</span>
										</div>
									</Table.Cell>
									<Table.Cell class="text-right">
										<DropdownMenu.Root>
											<DropdownMenu.Trigger
												class={cn(buttonVariants({ variant: 'ghost', size: 'icon-sm' }))}
												aria-label={`Actions for ${row.name}`}
											>
												<EllipsisIcon />
											</DropdownMenu.Trigger>
											<DropdownMenu.Content align="end">
												<DropdownMenu.Group>
													<DropdownMenu.Item onSelect={() => toggleReveal(row)}>
														{#if revealed[row.ID] !== undefined}
															<EyeOffIcon />
															Hide
														{:else}
															<EyeIcon />
															Show
														{/if}
													</DropdownMenu.Item>
													<DropdownMenu.Item onSelect={() => copyValue(row)}>
														<CopyIcon />
														Copy value
													</DropdownMenu.Item>
													{#if can('var.update')}
														<DropdownMenu.Item onSelect={() => openEdit(row)}>
															<PencilIcon />
															Edit
														</DropdownMenu.Item>
													{/if}
													{#if can('var.delete')}
														<DropdownMenu.Item
															variant="destructive"
															onSelect={() => (deleteVariable = row)}
														>
															<Trash2Icon />
															Delete
														</DropdownMenu.Item>
													{/if}
												</DropdownMenu.Group>
											</DropdownMenu.Content>
										</DropdownMenu.Root>
									</Table.Cell>
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
					{#if currentVariables.length === 0}
						<div class="p-8 text-center text-sm text-muted-foreground">
							No variables in this environment yet.
						</div>
					{/if}
				</div>
			{/if}
		</Tabs.Content>

		<Tabs.Content value="members" class="pt-4">
			<div class="rounded-lg border bg-background">
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>Email</Table.Head>
							<Table.Head>Role</Table.Head>
							<Table.Head>Permissions</Table.Head>
							<Table.Head class="w-12" />
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each data.members as member (member.user_id)}
							<Table.Row>
								<Table.Cell class="font-medium">
									<div class="flex items-center gap-2">
										{member.email}
										{#if member.user_id === session.user?.id}
											<Badge variant="outline">you</Badge>
										{/if}
									</div>
								</Table.Cell>
								<Table.Cell>
									<Badge variant={member.role === 'owner' ? 'default' : 'secondary'}>
										{member.role}
									</Badge>
								</Table.Cell>
								<Table.Cell>
									{#if member.overrides.length > 0}
										<Badge variant="outline">
											{member.overrides.length} override{member.overrides.length > 1 ? 's' : ''}
										</Badge>
									{:else}
										<span class="text-xs text-muted-foreground">role default</span>
									{/if}
								</Table.Cell>
								<Table.Cell class="text-right">
									{#if can('member.manage')}
										<DropdownMenu.Root>
											<DropdownMenu.Trigger
												class={cn(buttonVariants({ variant: 'ghost', size: 'icon-sm' }))}
												aria-label={`Actions for ${member.email}`}
											>
												<EllipsisIcon />
											</DropdownMenu.Trigger>
											<DropdownMenu.Content align="end" class="w-48">
												<DropdownMenu.Group>
													<DropdownMenu.Item onSelect={() => (roleTarget = member)}>
														Change role…
													</DropdownMenu.Item>
													<DropdownMenu.Item onSelect={() => (permissionsTarget = member)}>
														Permissions…
													</DropdownMenu.Item>
													<DropdownMenu.Item
														variant="destructive"
														onSelect={() => (removeTarget = member)}
													>
														Remove member
													</DropdownMenu.Item>
												</DropdownMenu.Group>
											</DropdownMenu.Content>
										</DropdownMenu.Root>
									{/if}
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</div>
		</Tabs.Content>
	</Tabs.Root>
</div>

{#if variableEditor}
	<VariableFormDialog
		mode={variableEditor.mode}
		initialName={variableEditor.name}
		initialValue={variableEditor.value}
		onsave={saveVariable}
		onclose={() => (variableEditor = null)}
	/>
{/if}

{#if inviteOpen && projectKey}
	<InviteMemberDialog
		projectId={data.project.id}
		{projectKey}
		canManage={can('member.manage')}
		onclose={() => (inviteOpen = false)}
	/>
{/if}

{#if roleTarget}
	<ChangeRoleDialog
		projectId={data.project.id}
		member={roleTarget}
		catalog={data.catalog}
		onclose={() => (roleTarget = null)}
	/>
{/if}

{#if permissionsTarget}
	<MemberPermissionsDialog
		projectId={data.project.id}
		member={permissionsTarget}
		catalog={data.catalog}
		onclose={() => (permissionsTarget = null)}
	/>
{/if}

<AlertDialog.Root
	open={deleteVariable !== null}
	onOpenChange={(next) => {
		if (!next) deleteVariable = null;
	}}
>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete variable?</AlertDialog.Title>
			<AlertDialog.Description>
				{deleteVariable ? `"${deleteVariable.name}" will be permanently deleted.` : ''}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" onclick={confirmDeleteVariable} disabled={deleting}>
				{deleting ? 'Deleting…' : 'Delete'}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root
	open={deleteEnvironment !== null}
	onOpenChange={(next) => {
		if (!next) deleteEnvironment = null;
	}}
>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete environment?</AlertDialog.Title>
			<AlertDialog.Description>
				{deleteEnvironment
					? `All variables in "${deleteEnvironment.name}" will be deleted too.`
					: ''}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action
				variant="destructive"
				onclick={confirmDeleteEnvironment}
				disabled={deleting}
			>
				{deleting ? 'Deleting…' : 'Delete'}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root bind:open={deleteProjectOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete project?</AlertDialog.Title>
			<AlertDialog.Description>
				"{data.project.name}" and all of its environments, variables, and memberships will be
				permanently deleted. Project keys held by members are not rotated.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action
				variant="destructive"
				onclick={confirmDeleteProject}
				disabled={deletingProject}
			>
				{deletingProject ? 'Deleting…' : 'Delete project'}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root
	open={removeTarget !== null}
	onOpenChange={(next) => {
		if (!next) removeTarget = null;
	}}
>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Remove member?</AlertDialog.Title>
			<AlertDialog.Description>
				{removeTarget ? `${removeTarget.email} will lose access immediately.` : ''}
				Their copy of the project key is deleted; keys they have already seen are not rotated.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action
				variant="destructive"
				onclick={confirmRemoveMember}
				disabled={removingMember}
			>
				{removingMember ? 'Removing…' : 'Remove member'}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
