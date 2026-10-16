<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import { toast } from 'svelte-sonner';
	import { environmentsApi } from '#lib/api/endpoints';
	import { Button, buttonVariants } from '#lib/components/ui/button';
	import * as Dialog from '#lib/components/ui/dialog';
	import * as Field from '#lib/components/ui/field';
	import { Input } from '#lib/components/ui/input';
	import { Spinner } from '#lib/components/ui/spinner';
	import { describeError } from '#lib/errors';
	import { cn } from '#lib/utils';

	let { projectId }: { projectId: number } = $props();

	let open = $state(false);
	let name = $state('');
	let busy = $state(false);

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (!name.trim()) return;
		busy = true;
		try {
			const environment = await environmentsApi.create(projectId, name.trim());
			toast.success(`Environment "${environment.name}" created`);
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
	<Dialog.Trigger class={cn(buttonVariants({ variant: 'outline', size: 'sm' }))}>
		<PlusIcon data-icon="inline-start" />
		Environment
	</Dialog.Trigger>
	<Dialog.Content>
		<form class="grid gap-6" onsubmit={submit}>
			<Dialog.Header>
				<Dialog.Title>New environment</Dialog.Title>
				<Dialog.Description>
					A namespace for variables, e.g. development, staging, or production.
				</Dialog.Description>
			</Dialog.Header>

			<Field.FieldGroup>
				<Field.Field>
					<Field.FieldLabel for="environment-name">Environment name</Field.FieldLabel>
					<Input
						id="environment-name"
						bind:value={name}
						required
						autocomplete="off"
						placeholder="development"
					/>
				</Field.Field>
			</Field.FieldGroup>

			<Dialog.Footer>
				<Dialog.Close class={cn(buttonVariants({ variant: 'outline' }))}>Cancel</Dialog.Close>
				<Button type="submit" disabled={busy || !name.trim()}>
					{#if busy}
						<Spinner data-icon="inline-start" />
					{/if}
					Create environment
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
