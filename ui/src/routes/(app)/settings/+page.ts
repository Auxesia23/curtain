import { redirect } from '@sveltejs/kit';
import { adminUsersApi, invitesApi } from '#lib/api/endpoints';
import { session } from '#lib/state/session.svelte';

// Admin page for invitation and user management.
export const load = async () => {
	await session.ensureLoaded();
	if (session.user?.role !== 'admin') {
		redirect(307, '/projects');
	}

	const [invites, users] = await Promise.all([invitesApi.list(), adminUsersApi.list()]);
	return { invites, users };
};
