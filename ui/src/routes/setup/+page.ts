import { redirect } from '@sveltejs/kit';
import { instanceApi } from '#lib/api/endpoints';

// The setup page is available only while the instance has no users.
export const load = async () => {
	const instance = await instanceApi.get();
	if (!instance.needs_setup) {
		redirect(307, '/');
	}
};
