/**
 * Small HTTP client for the Go API.
 *
 * Web authentication uses an HttpOnly cookie, so every request includes
 * credentials. The CLI uses a bearer header instead.
 */

export class ApiError extends Error {
	readonly status: number;

	constructor(status: number, message: string) {
		super(message);
		this.name = 'ApiError';
		this.status = status;
	}
}

export interface RequestOptions {
	method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
	body?: unknown;
	signal?: AbortSignal;
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
	const { method = 'GET', body, signal } = options;
	const headers: Record<string, string> = {};
	if (body !== undefined) {
		headers['Content-Type'] = 'application/json';
	}

	const response = await fetch(`/api${path}`, {
		method,
		headers,
		credentials: 'include',
		body: body === undefined ? undefined : JSON.stringify(body),
		signal
	});

	if (response.status === 204) {
		return undefined as T;
	}

	let data: unknown = null;
	const text = await response.text();
	if (text) {
		try {
			data = JSON.parse(text);
		} catch {
			data = text;
		}
	}

	if (!response.ok) {
		const message =
			typeof data === 'object' && data !== null && 'error' in data
				? String((data as { error: unknown }).error)
				: `Request failed (${response.status})`;
		throw new ApiError(response.status, message);
	}

	return data as T;
}

export const get = <T>(path: string, options?: RequestOptions) => request<T>(path, options);
export const post = <T>(path: string, body?: unknown, options?: RequestOptions) =>
	request<T>(path, { ...options, method: 'POST', body });
export const put = <T>(path: string, body?: unknown, options?: RequestOptions) =>
	request<T>(path, { ...options, method: 'PUT', body });
export const patch = <T>(path: string, body?: unknown, options?: RequestOptions) =>
	request<T>(path, { ...options, method: 'PATCH', body });
export const del = <T>(path: string, options?: RequestOptions) =>
	request<T>(path, { ...options, method: 'DELETE' });
