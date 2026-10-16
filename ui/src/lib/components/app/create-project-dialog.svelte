<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import { toast } from 'svelte-sonner';
	import { projectsApi } from '#lib/api/endpoints';
	import { Button, buttonVariants } from '#lib/components/ui/button';
	import * as Dialog from '#lib/components/ui/dialog';
	import * as Field from '#lib/components/ui/field';
	import { Input } from '#lib/components/ui/input';
	import { Spinner } from '#lib/components/ui/spinner';
	import { generateProjectKey, wrapProjectKey } from '#lib/crypto/project';
	import { describeError } from '#lib/errors';
	import { session } from '#lib/state/session.svelte';
	import { cn } from '#lib/utils';

	let open = $state(false);
	let name = $state('');
	let busy = $state(false);

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (!name.trim()) return;
		busy = true;
		try {
			// 1) Generate the AES-256 project key in the browser.
			const projectKey = await generateProjectKey();
			// 2) Wrap it with the owner's public key. Only ciphertext is sent.
			const publicKey = await session.ownPublicKey();
			const wrapped = await wrapProjectKey(publicKey, projectKey);
			// 3) Save the project and keep the key in memory for this session.
			const project = await projectsApi.create(name.trim(), wrapped);
			session.rememberProjectKey(project.id, projectKey);

			toast.success(`Project "${project.name}" created`);
			open = false;
			name = '';
			await invalidateAll();
		} catch (error) {
			toast.error(describeError(error));
		} finally {
			busy = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Trigger class={cn(buttonVariants())}>
		<PlusIcon data-icon="inline-start" />
		New project
	</Dialog.Trigger>
	<Dialog.Content>
		<form class="grid gap-6" onsubmit={submit}>
			<Dialog.Header>
				<Dialog.Title>New project</Dialog.Title>
				<Dialog.Description>
					The project’s AES-256 key is generated in your browser and stored on the server
					only in encrypted form.
				</Dialog.Description>
			</Dialog.Header>

			<Field.FieldGroup>
				<Field.Field>
					<Field.FieldLabel for="project-name">Project name</Field.FieldLabel>
					<Input
						id="project-name"
						bind:value={name}
						required
						autocomplete="off"
						placeholder="billing-app"
					/>
				</Field.Field>
			</Field.FieldGroup>

			<Dialog.Footer>
				<Dialog.Close class={cn(buttonVariants({ variant: 'outline' }))}>Cancel</Dialog.Close>
				<Button type="submit" disabled={busy || !name.trim()}>
					{#if busy}
						<Spinner data-icon="inline-start" />
					{/if}
					Create project
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
