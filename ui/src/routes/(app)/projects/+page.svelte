<script lang="ts">
	import FolderIcon from '@lucide/svelte/icons/folder';
	import { Badge } from '#lib/components/ui/badge';
	import { Button } from '#lib/components/ui/button';
	import * as Card from '#lib/components/ui/card';
	import * as Empty from '#lib/components/ui/empty';
	import CreateProjectDialog from '#lib/components/app/create-project-dialog.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head><title>Projects · Curtain</title></svelte:head>

<div class="flex flex-col gap-6">
	<div class="flex flex-wrap items-start justify-between gap-4">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Projects</h1>
			<p class="text-sm text-muted-foreground">
				Every environment variable is encrypted in your browser.
			</p>
		</div>
		<CreateProjectDialog />
	</div>

	{#if data.projects.length === 0}
		<Empty.Root>
			<Empty.Header>
				<Empty.Media variant="icon">
					<FolderIcon />
				</Empty.Media>
				<Empty.Title>No projects yet</Empty.Title>
				<Empty.Description>
					Create your first project. The project key is generated in your browser and the
					server only receives an encrypted copy.
				</Empty.Description>
			</Empty.Header>
			<Empty.Content>
				<CreateProjectDialog />
			</Empty.Content>
		</Empty.Root>
	{:else}
		<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
			{#each data.projects as project (project.id)}
				<Card.Root>
					<Card.Header>
						<Card.Title>{project.name}</Card.Title>
						<Card.Description>
							{project.role === 'owner' ? 'You own this project' : 'You are a member of this project'}
						</Card.Description>
						<Card.Action>
							<Badge variant={project.role === 'owner' ? 'default' : 'secondary'}>
								{project.role}
							</Badge>
						</Card.Action>
					</Card.Header>
					<Card.Footer>
						<Button href={`/projects/${project.id}`} variant="outline" size="sm" class="w-full">
							Open project
						</Button>
					</Card.Footer>
				</Card.Root>
			{/each}
		</div>
	{/if}
</div>
