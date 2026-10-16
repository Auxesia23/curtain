import { invitesApi } from '#lib/api/endpoints';

// Validate the invitation link so the page can show an appropriate error when invalid.
export const load = async ({ params }) => {
	const info = await invitesApi.validate(params.token);
	return { token: params.token, info };
};
