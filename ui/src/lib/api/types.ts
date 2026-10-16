/** Tipe payload API (cerminan DTO di backend Go internal/dto). */

export interface KDFParams {
	m: number;
	t: number;
	p: number;
	hash_length: number;
}

export interface KDFConfig {
	algorithm: 'argon2id';
	salt: string;
	params: KDFParams;
}

export interface UserProfile {
	id: number;
	email: string;
	role: 'admin' | 'member';
	kdf_algo: string;
	kdf_salt: string;
	kdf_params: string;
	public_key: string;
	encrypted_private_key: string;
}

export interface AuthResponse {
	token?: string;
	user: UserProfile;
}

export interface RegisterPayload {
	email: string;
	auth_key: string;
	kdf: KDFConfig;
	public_key: string;
	encrypted_private_key: string;
	invite_token?: string;
	client: 'web' | 'cli';
}

export interface LoginPayload {
	email: string;
	auth_key: string;
	client: 'web' | 'cli';
}

export interface UserLookup {
	id: number;
	email: string;
	public_key: string;
}

// --- Role & kapabilitas project (Fase 2) ---

export type ProjectRole = 'owner' | 'editor' | 'viewer';

export type Capability =
	| 'env.create'
	| 'env.delete'
	| 'var.create'
	| 'var.update'
	| 'var.delete'
	| 'member.invite'
	| 'member.manage'
	| 'project.delete';

export type PermissionEffect = 'allow' | 'deny';

export interface PermissionEntry {
	capability: Capability;
	effect: PermissionEffect;
}

export interface CapabilityInfo {
	capability: Capability;
	label: string;
	description: string;
	group: 'content' | 'members' | 'project';
}

export interface RoleInfo {
	role: ProjectRole;
	label: string;
	description: string;
	capabilities: Capability[];
}

export interface PermissionCatalog {
	capabilities: CapabilityInfo[];
	roles: RoleInfo[];
}

export interface Project {
	id: number;
	name: string;
	role: ProjectRole;
	encrypted_project_key: string;
	/** The caller's effective capabilities, included only in GetProject responses. */
	capabilities?: Capability[];
}

export interface Member {
	user_id: number;
	email: string;
	public_key: string;
	role: ProjectRole;
	overrides: PermissionEntry[];
	capabilities: Capability[];
}

export interface Environment {
	ID: number;
	CreatedAt: string;
	UpdatedAt: string;
	project_id: number;
	name: string;
}

export interface EnvironmentVariable {
	ID: number;
	CreatedAt: string;
	UpdatedAt: string;
	environment_id: number;
	encrypted_key: string;
	encrypted_value: string;
}

export interface VariablePayload {
	encrypted_key: string;
	encrypted_value: string;
}

/** A variable with its name decrypted in the browser for display. */
export interface VariableRow extends EnvironmentVariable {
	name: string;
}

// --- Instance, invitations, and administration ---

export interface InstanceInfo {
	needs_setup: boolean;
}

export interface InviteInfo {
	valid: boolean;
	expires_at: string | null;
	remaining_uses: number;
}

export type InviteStatus = 'active' | 'expired' | 'revoked' | 'exhausted';

export interface InviteSummary {
	id: number;
	max_uses: number;
	uses: number;
	expires_at: string | null;
	revoked_at: string | null;
	status: InviteStatus;
	created_by: number;
	created_by_email?: string;
	created_at: string;
}

/** Creation response. The raw `token` and `url` are returned only once. */
export interface CreatedInvite extends InviteSummary {
	token: string;
	url: string;
}

export interface AdminUser {
	id: number;
	email: string;
	role: 'admin' | 'member';
	disabled_at: string | null;
	created_at: string;
}
