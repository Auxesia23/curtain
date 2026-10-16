import { redirect } from '@sveltejs/kit';
import { session } from '#lib/state/session.svelte';

// Protect the app area until the cookie session and local vault are ready.
export const load = async () => {
	await session.ensureLoaded();
	if (session.status === 'anonymous') {
		redirect(307, '/login');
	}
	if (session.status === 'locked') {
		redirect(307, '/unlock');
	}
};
