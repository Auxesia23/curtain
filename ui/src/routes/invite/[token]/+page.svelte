<script lang="ts">
	import BrandLogo from '#lib/components/app/brand-logo.svelte';
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
	import * as Alert from '#lib/components/ui/alert';
	import { Button } from '#lib/components/ui/button';
	import * as Card from '#lib/components/ui/card';
	import RegisterForm from '#lib/components/app/register-form.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const expiryLabel = $derived(
		data.info.expires_at ? new Date(data.info.expires_at).toLocaleString() : 'never'
	);
</script>

<svelte:head><title>Invite · Curtain</title></svelte:head>

<div class="flex min-h-svh items-center justify-center px-4 py-10">
	<div class="mx-auto w-full max-w-sm">
		{#if data.info.valid}
			<Card.Root>
				<Card.Header>
					<div class="flex items-center gap-2">
						<BrandLogo class="size-5" />
						<Card.Title>Create your Curtain account</Card.Title>
					</div>
					<Card.Description>
						You were invited to this instance. Invite valid until {expiryLabel}
						({data.info.remaining_uses} uses left).
					</Card.Description>
				</Card.Header>
				<Card.Content>
					<RegisterForm inviteToken={data.token} />
				</Card.Content>
				<Card.Footer class="text-xs text-muted-foreground">
					Your keypair is generated in this browser. The private key never leaves it in plaintext.
				</Card.Footer>
			</Card.Root>
		{:else}
			<Card.Root>
				<Card.Header>
					<div class="flex items-center gap-2">
						<BrandLogo class="size-5" />
						<Card.Title>Invite link not valid</Card.Title>
					</div>
				</Card.Header>
				<Card.Content>
					<Alert.Root variant="destructive">
						<TriangleAlertIcon />
						<Alert.Title>This invite link is expired, revoked, or fully used</Alert.Title>
						<Alert.Description>
							Ask an instance admin for a fresh link, or sign in if you already have an account.
						</Alert.Description>
					</Alert.Root>
				</Card.Content>
				<Card.Footer class="justify-center">
					<Button href="/login" variant="outline" class="w-full">Go to sign in</Button>
				</Card.Footer>
			</Card.Root>
		{/if}
	</div>
</div>
