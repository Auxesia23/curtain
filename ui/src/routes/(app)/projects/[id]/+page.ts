import { error } from '@sveltejs/kit';
import { environmentsApi, permissionsApi, projectsApi, variablesApi } from '#lib/api/endpoints';
import type { VariableRow } from '#lib/api/types';
import { decryptText } from '#lib/crypto/project';
import { session } from '#lib/state/session.svelte';

export const load = async ({ params }) => {
	const projectId = Number(params.id);
	if (!Number.isInteger(projectId) || projectId <= 0) {
		error(404, 'Project not found');
	}

	// Ensure the session and private key are ready before unwrapping the project key.
	// (load layout & page SvelteKit berjalan paralel.)
	await session.ensureLoaded();

	const project = await projectsApi.get(projectId);
	// Unwrap the user's project key from the database using the local private key.
	const projectKey = await session.unlockProjectKey(projectId, project.encrypted_project_key);

	const [members, environments, catalog] = await Promise.all([
		projectsApi.members(projectId),
		environmentsApi.list(projectId),
		permissionsApi.catalog()
	]);

	// Decrypt variable names so the table can display them;
	// Values stay encrypted until the user selects "Reveal".
	const variablesByEnvironment: Record<number, VariableRow[]> = {};
	await Promise.all(
		environments.map(async (environment) => {
			const variables = await variablesApi.list(environment.ID);
			variablesByEnvironment[environment.ID] = await Promise.all(
				variables.map(async (variable) => ({
					...variable,
					name: await decryptText(projectKey, variable.encrypted_key)
				}))
			);
		})
	);

	return { project, members, environments, catalog, variablesByEnvironment };
};
