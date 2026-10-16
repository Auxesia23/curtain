<script lang="ts">
	import BrandLogo from '#lib/components/app/brand-logo.svelte';
	import * as Card from '#lib/components/ui/card';
	import RegisterForm from '#lib/components/app/register-form.svelte';
</script>

<svelte:head><title>Set up · Curtain</title></svelte:head>

<div class="flex min-h-svh items-center justify-center px-4 py-10">
	<div class="mx-auto w-full max-w-sm">
		<Card.Root>
			<Card.Header>
				<div class="flex items-center gap-2">
					<BrandLogo class="size-5" />
					<Card.Title>Set up Curtain</Card.Title>
				</div>
				<Card.Description>
					This instance has no accounts yet. Create the admin account — you can invite your team
					afterwards.
				</Card.Description>
			</Card.Header>
			<Card.Content>
				<RegisterForm submitLabel="Create admin account" />
			</Card.Content>
			<Card.Footer class="text-xs text-muted-foreground">
				Your keypair is generated in this browser. The private key never leaves it in plaintext.
			</Card.Footer>
		</Card.Root>
	</div>
</div>
