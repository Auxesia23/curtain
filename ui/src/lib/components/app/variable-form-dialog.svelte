<script lang="ts">
	import { Button, buttonVariants } from '#lib/components/ui/button';
	import * as Dialog from '#lib/components/ui/dialog';
	import * as Field from '#lib/components/ui/field';
	import { Input } from '#lib/components/ui/input';
	import { Spinner } from '#lib/components/ui/spinner';
	import { describeError } from '#lib/errors';
	import { cn } from '#lib/utils';

	let {
		mode = 'create',
		initialName = '',
		initialValue = '',
		onsave,
		onclose
	}: {
		mode?: 'create' | 'edit';
		initialName?: string;
		initialValue?: string;
		onsave: (payload: { name: string; value: string }) => Promise<void>;
		onclose: () => void;
	} = $props();

	let open = $state(true);
	let name = $state('');
	let value = $state('');
	let error = $state('');
	let busy = $state(false);

	// The component is remounted whenever the editor opens, so initialize from props.
	$effect(() => {
		name = initialName;
		value = initialValue;
	});

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (!name.trim()) {
			error = 'Name is required.';
			return;
		}
		busy = true;
		error = '';
		try {
			await onsave({ name: name.trim(), value });
			// Close explicitly because programmatic closing does not trigger onOpenChange.
			// The parent must unmount this instance before the next one opens.
			onclose();
		} catch (err) {
			error = describeError(err);
		} finally {
			busy = false;
		}
	}
</script>

<Dialog.Root
	bind:open
	onOpenChange={(next) => {
		if (!next) onclose();
	}}
>
	<Dialog.Content>
		<form class="grid gap-6" onsubmit={submit}>
			<Dialog.Header>
				<Dialog.Title>{mode === 'edit' ? 'Edit variable' : 'New variable'}</Dialog.Title>
				<Dialog.Description>
					The name and value are encrypted in your browser with the project key before they are sent.
				</Dialog.Description>
			</Dialog.Header>

			<Field.FieldGroup>
				<Field.Field data-invalid={error ? true : undefined}>
					<Field.FieldLabel for="variable-name">Name</Field.FieldLabel>
					<Input
						id="variable-name"
						bind:value={name}
						required
						autocomplete="off"
						placeholder="DATABASE_URL"
						aria-invalid={error ? true : undefined}
					/>
				</Field.Field>
				<Field.Field>
					<Field.FieldLabel for="variable-value">Value</Field.FieldLabel>
					<Input
						id="variable-value"
						bind:value={value}
						required
						autocomplete="off"
						placeholder="postgres://…"
					/>
				</Field.Field>
			</Field.FieldGroup>

			{#if error}
				<Field.Error>{error}</Field.Error>
			{/if}

			<Dialog.Footer>
				<Dialog.Close class={cn(buttonVariants({ variant: 'outline' }))}>Cancel</Dialog.Close>
				<Button type="submit" disabled={busy || !name.trim()}>
					{#if busy}
						<Spinner data-icon="inline-start" />
					{/if}
					{mode === 'edit' ? 'Save changes' : 'Add variable'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
