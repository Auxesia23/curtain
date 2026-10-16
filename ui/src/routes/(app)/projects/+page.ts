import { projectsApi } from '#lib/api/endpoints';

export const load = async () => {
	return { projects: await projectsApi.list() };
};
