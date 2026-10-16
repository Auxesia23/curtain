<script lang="ts">
	import { Button, buttonVariants } from '#lib/components/ui/button';
	import * as DropdownMenu from '#lib/components/ui/dropdown-menu';
	import BrandLogo from '#lib/components/app/brand-logo.svelte';
	import { session } from '#lib/state/session.svelte';
	import { goto } from '$app/navigation';
	import FolderIcon from '@lucide/svelte/icons/folder';
	import LockIcon from '@lucide/svelte/icons/lock';
	import LogOutIcon from '@lucide/svelte/icons/log-out';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import { cn } from '#lib/utils';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	const user = $derived(session.user);
	const initial = $derived((user?.email?.[0] ?? '?').toUpperCase());

	async function logout() {
		await session.logout();
		await goto('/login');
	}

	async function lock() {
		session.lock();
		await goto('/unlock');
	}
</script>

<div class="flex min-h-svh flex-col bg-muted/20">
	<header class="border-b bg-background">
		<div class="mx-auto flex h-14 w-full max-w-6xl items-center justify-between px-4">
			<a href="/projects" class="flex items-center gap-2 font-semibold" aria-label="Curtain — Projects">
				<BrandLogo class="size-5" />
				<span>Curtain</span>
			</a>

			<DropdownMenu.Root>
				<DropdownMenu.Trigger
					class={cn(buttonVariants({ variant: 'ghost', size: 'icon' }), 'rounded-full')}
					aria-label="Account menu"
				>
					<span class="flex size-7 items-center justify-center rounded-full bg-muted text-xs font-medium">
						{initial}
					</span>
				</DropdownMenu.Trigger>
				<DropdownMenu.Content align="end" class="w-56">
					<DropdownMenu.Group>
						<DropdownMenu.Label class="text-xs">{user?.email}</DropdownMenu.Label>
						<DropdownMenu.Separator />
						<DropdownMenu.Item onclick={() => goto('/projects')}>
							<FolderIcon />
							Projects
						</DropdownMenu.Item>
						{#if user?.role === 'admin'}
							<DropdownMenu.Item onclick={() => goto('/settings')}>
								<SettingsIcon />
								Settings
							</DropdownMenu.Item>
						{/if}
					</DropdownMenu.Group>
					<DropdownMenu.Separator />
					<DropdownMenu.Group>
						<DropdownMenu.Item onclick={lock}>
							<LockIcon />
							Lock vault
						</DropdownMenu.Item>
						<DropdownMenu.Item variant="destructive" onclick={logout}>
							<LogOutIcon />
							Sign out
						</DropdownMenu.Item>
					</DropdownMenu.Group>
				</DropdownMenu.Content>
			</DropdownMenu.Root>
		</div>
	</header>

	<main class="mx-auto w-full max-w-6xl flex-1 px-4 py-8">
		{@render children()}
	</main>
</div>
