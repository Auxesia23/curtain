<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import EllipsisIcon from '@lucide/svelte/icons/ellipsis';
	import { toast } from 'svelte-sonner';
	import { adminUsersApi, invitesApi } from '#lib/api/endpoints';
	import type { AdminUser, InviteSummary, InviteStatus } from '#lib/api/types';
	import * as AlertDialog from '#lib/components/ui/alert-dialog';
	import { Badge, type BadgeVariant } from '#lib/components/ui/badge';
	import { Button, buttonVariants } from '#lib/components/ui/button';
	import * as DropdownMenu from '#lib/components/ui/dropdown-menu';
	import * as Table from '#lib/components/ui/table';
	import * as Tabs from '#lib/components/ui/tabs';
	import CreateInviteDialog from '#lib/components/app/create-invite-dialog.svelte';
	import { describeError } from '#lib/errors';
	import { session } from '#lib/state/session.svelte';
	import { cn } from '#lib/utils';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let busyId = $state<number | null>(null);
	let deleteTarget = $state<AdminUser | null>(null);
	let deleting = $state(false);

	const statusVariant: Record<InviteStatus, BadgeVariant> = {
		active: 'default',
		expired: 'secondary',
		revoked: 'destructive',
		exhausted: 'secondary'
	};

	function formatDate(value: string | null | undefined): string {
		if (!value) return '—';
		return new Date(value).toLocaleString();
	}

	async function revokeInvite(invite: InviteSummary) {
		busyId = invite.id;
		try {
			await invitesApi.revoke(invite.id);
			toast.success('Invite revoked');
			await invalidateAll();
		} catch (error) {
			toast.error(describeError(error));
		} finally {
			busyId = null;
		}
	}

	async function toggleDisabled(user: AdminUser) {
		busyId = user.id;
		try {
			if (user.disabled_at) {
				await adminUsersApi.enable(user.id);
				toast.success(`${user.email} re-enabled`);
			} else {
				await adminUsersApi.disable(user.id);
				toast.success(`${user.email} disabled`);
			}
			await invalidateAll();
		} catch (error) {
			toast.error(describeError(error));
		} finally {
			busyId = null;
		}
	}

	async function confirmDelete() {
		if (!deleteTarget) return;
		deleting = true;
		try {
			await adminUsersApi.remove(deleteTarget.id);
			toast.success(`${deleteTarget.email} deleted`);
			deleteTarget = null;
			await invalidateAll();
		} catch (error) {
			toast.error(describeError(error));
		} finally {
			deleting = false;
		}
	}
</script>

<svelte:head><title>Settings · Curtain</title></svelte:head>

<div class="flex flex-col gap-6">
	<div>
		<h1 class="text-2xl font-semibold tracking-tight">Settings</h1>
		<p class="text-sm text-muted-foreground">Manage invite links and instance members.</p>
	</div>

	<Tabs.Root value="invites">
		<Tabs.List>
			<Tabs.Trigger value="invites">Invites</Tabs.Trigger>
			<Tabs.Trigger value="members">Members</Tabs.Trigger>
		</Tabs.List>

		<Tabs.Content value="invites" class="flex flex-col gap-4 pt-4">
			<div class="flex flex-wrap items-start justify-between gap-3">
				<p class="max-w-xl text-sm text-muted-foreground">
					Invite links let people create an account on this instance. Links are generic — anyone
					holding the link can sign up until it expires or reaches its usage limit. Revoke a link
					once your team has joined.
				</p>
				<CreateInviteDialog />
			</div>

			<div class="rounded-lg border bg-background">
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>Status</Table.Head>
							<Table.Head>Uses</Table.Head>
							<Table.Head>Expires</Table.Head>
							<Table.Head>Created by</Table.Head>
							<Table.Head class="w-24" />
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each data.invites as invite (invite.id)}
							<Table.Row>
								<Table.Cell>
									<Badge variant={statusVariant[invite.status]}>{invite.status}</Badge>
								</Table.Cell>
								<Table.Cell class="text-sm">
									{invite.uses}/{invite.max_uses}
								</Table.Cell>
								<Table.Cell class="text-sm text-muted-foreground">
									{formatDate(invite.expires_at)}
								</Table.Cell>
								<Table.Cell class="text-sm text-muted-foreground">
									{invite.created_by_email ?? '—'}
								</Table.Cell>
								<Table.Cell class="text-right">
									{#if invite.status === 'active'}
										<Button
											variant="ghost"
											size="sm"
											disabled={busyId === invite.id}
											onclick={() => revokeInvite(invite)}
										>
											Revoke
										</Button>
									{/if}
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
				{#if data.invites.length === 0}
					<div class="p-8 text-center text-sm text-muted-foreground">
						No invite links yet. Create one to onboard your team.
					</div>
				{/if}
			</div>
		</Tabs.Content>

		<Tabs.Content value="members" class="pt-4">
			<div class="rounded-lg border bg-background">
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>Email</Table.Head>
							<Table.Head>Role</Table.Head>
							<Table.Head>Status</Table.Head>
							<Table.Head>Joined</Table.Head>
							<Table.Head class="w-12" />
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each data.users as user (user.id)}
							<Table.Row>
								<Table.Cell class="font-medium">
									<div class="flex items-center gap-2">
										{user.email}
										{#if user.id === session.user?.id}
											<Badge variant="outline">you</Badge>
										{/if}
									</div>
								</Table.Cell>
								<Table.Cell>
									<Badge variant={user.role === 'admin' ? 'default' : 'secondary'}>
										{user.role}
									</Badge>
								</Table.Cell>
								<Table.Cell>
									{#if user.disabled_at}
										<Badge variant="destructive">disabled</Badge>
									{:else}
										<Badge variant="outline">active</Badge>
									{/if}
								</Table.Cell>
								<Table.Cell class="text-sm text-muted-foreground">
									{formatDate(user.created_at)}
								</Table.Cell>
								<Table.Cell class="text-right">
									{#if user.id !== session.user?.id}
										<DropdownMenu.Root>
											<DropdownMenu.Trigger
												class={cn(buttonVariants({ variant: 'ghost', size: 'icon-sm' }))}
												aria-label={`Actions for ${user.email}`}
											>
												<EllipsisIcon />
											</DropdownMenu.Trigger>
											<DropdownMenu.Content align="end">
												<DropdownMenu.Group>
													<DropdownMenu.Item
														onSelect={() => toggleDisabled(user)}
														disabled={busyId === user.id}
													>
														{user.disabled_at ? 'Enable account' : 'Disable account'}
													</DropdownMenu.Item>
													<DropdownMenu.Item
														variant="destructive"
														onSelect={() => (deleteTarget = user)}
													>
														Delete account
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

<AlertDialog.Root
	open={deleteTarget !== null}
	onOpenChange={(next) => {
		if (!next) deleteTarget = null;
	}}
>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete account?</AlertDialog.Title>
			<AlertDialog.Description>
				{deleteTarget ? `${deleteTarget.email} will be permanently deleted.` : ''}
				Projects where this user is the only owner will be deleted too. Project keys they have
				seen are not rotated.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" onclick={confirmDelete} disabled={deleting}>
				{deleting ? 'Deleting…' : 'Delete'}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
