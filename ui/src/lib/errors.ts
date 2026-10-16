import { ApiError } from '#lib/api/client';

/** Convert any error into a user-friendly message. */
export function describeError(error: unknown): string {
	if (error instanceof ApiError) {
		if (error.status === 401) {
			return 'Incorrect email or password.';
		}
		if (error.message) {
			return error.message;
		}
		return `Request failed (${error.status}).`;
	}
	if (error instanceof Error) {
		return error.message;
	}
	return 'Something went wrong.';
}
