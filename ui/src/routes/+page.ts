import { redirect } from '@sveltejs/kit';
import { instanceApi } from '#lib/api/endpoints';
import { session } from '#lib/state/session.svelte';

// Landing page: first-run setup, atau arahkan sesuai status sesi (SPA).
export const load = async () => {
	const instance = await instanceApi.get();
	if (instance.needs_setup) {
		redirect(307, '/setup');
	}

	await session.ensureLoaded();
	switch (session.status) {
		case 'ready':
			redirect(307, '/projects');
		case 'locked':
			redirect(307, '/unlock');
		default:
			redirect(307, '/login');
	}
};
