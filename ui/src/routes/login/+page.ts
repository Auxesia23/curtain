import { redirect } from '@sveltejs/kit';
import { session } from '#lib/state/session.svelte';

export const load = async () => {
	await session.ensureLoaded();
	if (session.status === 'ready') {
		redirect(307, '/projects');
	}
};
