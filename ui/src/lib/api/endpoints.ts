/** Fungsi endpoint per resource. */
import { del, get, patch, post, put } from './client';
import type {
	AdminUser,
	AuthResponse,
	CreatedInvite,
	Environment,
	EnvironmentVariable,
	InstanceInfo,
	InviteInfo,
	InviteSummary,
	LoginPayload,
	Member,
	PermissionCatalog,
	PermissionEntry,
	Project,
	ProjectRole,
	RegisterPayload,
	UserLookup,
	UserProfile,
	VariablePayload
} from './types';

export const authApi = {
	kdf: (email: string) =>
		get<{ kdf_algo: string; kdf_salt: string; kdf_params: string }>(
			`/auth/kdf?email=${encodeURIComponent(email)}`
		),
	register: (payload: RegisterPayload) => post<AuthResponse>('/auth/register', payload),
	login: (payload: LoginPayload) => post<AuthResponse>('/auth/login', payload),
	logout: () => post<void>('/auth/logout'),
	me: () => get<UserProfile>('/auth/me')
};

export const usersApi = {
	lookup: (email: string) =>
		get<UserLookup>(`/users/lookup?email=${encodeURIComponent(email)}`)
};

export const projectsApi = {
	list: () => get<Project[]>('/projects'),
	create: (name: string, encryptedProjectKey: string) =>
		post<Project>('/projects', { name, encrypted_project_key: encryptedProjectKey }),
	get: (projectId: number) => get<Project>(`/projects/${projectId}`),
	remove: (projectId: number) => del<void>(`/projects/${projectId}`),
	members: (projectId: number) => get<Member[]>(`/projects/${projectId}/members`),
	invite: (projectId: number, userId: number, encryptedProjectKey: string, role: ProjectRole = 'editor') =>
		post<Member>(`/projects/${projectId}/invite`, {
			user_id: userId,
			encrypted_project_key: encryptedProjectKey,
			role
		}),
	updateMemberRole: (projectId: number, userId: number, role: ProjectRole) =>
		patch<{ user_id: number; role: ProjectRole }>(
			`/projects/${projectId}/members/${userId}`,
			{ role }
		),
	setMemberPermissions: (projectId: number, userId: number, overrides: PermissionEntry[]) =>
		put<void>(`/projects/${projectId}/members/${userId}/permissions`, { overrides }),
	removeMember: (projectId: number, userId: number) =>
		del<void>(`/projects/${projectId}/members/${userId}`)
};

export const permissionsApi = {
	catalog: () => get<PermissionCatalog>('/permissions/catalog')
};

export const environmentsApi = {
	list: (projectId: number) => get<Environment[]>(`/projects/${projectId}/environments`),
	create: (projectId: number, name: string) =>
		post<Environment>(`/projects/${projectId}/environments`, { name }),
	remove: (environmentId: number) => del<void>(`/environments/${environmentId}`)
};

export const variablesApi = {
	list: (environmentId: number) =>
		get<EnvironmentVariable[]>(`/environments/${environmentId}/variables`),
	create: (environmentId: number, payload: VariablePayload) =>
		post<EnvironmentVariable>(`/environments/${environmentId}/variables`, payload),
	update: (variableId: number, payload: VariablePayload) =>
		put<EnvironmentVariable>(`/variables/${variableId}`, payload),
	remove: (variableId: number) => del<void>(`/variables/${variableId}`)
};

export const instanceApi = {
	get: () => get<InstanceInfo>('/instance')
};

export const invitesApi = {
	/** Public endpoint used to validate a token from an /invite/<token> link. */
	validate: (token: string) => get<InviteInfo>(`/invites/${encodeURIComponent(token)}`),
	list: () => get<InviteSummary[]>('/admin/invites'),
	create: (maxUses: number, expiresInHours: number) =>
		post<CreatedInvite>('/admin/invites', { max_uses: maxUses, expires_in_hours: expiresInHours }),
	revoke: (inviteId: number) => del<void>(`/admin/invites/${inviteId}`)
};

export const adminUsersApi = {
	list: () => get<AdminUser[]>('/admin/users'),
	disable: (userId: number) => post<unknown>(`/admin/users/${userId}/disable`),
	enable: (userId: number) => post<unknown>(`/admin/users/${userId}/enable`),
	remove: (userId: number) => del<void>(`/admin/users/${userId}`)
};
