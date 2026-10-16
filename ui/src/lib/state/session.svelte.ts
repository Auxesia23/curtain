/**
 * Application session state using Svelte 5 runes.
 *
 * Cryptographic flows:
 *  - register: generate a keypair, derive keys from the password, send authKey
 *    and the encrypted private key to the server, then store the private key
 *    as non-extractable in the IndexedDB vault
 *  - login: fetch the KDF salt, derive keys, authenticate, decrypt the private
 *    key, and store it in the vault
 *  - unlock: if the session cookie is still valid but the local vault is empty,
 *    decrypt the private key again with the password
 *
 * Project keys stay in memory and are cleared on lock, logout, or reload.
 */
import { ApiError } from '#lib/api/client';
import { authApi } from '#lib/api/endpoints';
import type { UserProfile } from '#lib/api/types';
import {
	decryptPrivateKey,
	deriveAuthKey,
	deriveMasterKey,
	deriveWrapKey,
	encryptPrivateKey,
	exportPublicKey,
	generateIdentityKeypair,
	generateKDFConfig,
	importPublicKey,
	kdfConfigFromProfile,
	wipeMasterKey
} from '#lib/crypto/index';
import { unwrapProjectKey } from '#lib/crypto/project';
import { vault } from '#lib/storage/vault';

export type SessionStatus = 'loading' | 'anonymous' | 'locked' | 'ready';

export type ProgressCallback = (step: string) => void;

class SessionStore {
	status = $state<SessionStatus>('loading');
	user = $state<UserProfile | null>(null);
	privateKey = $state<CryptoKey | null>(null);

	#loaded = false;
	#loading: Promise<void> | null = null;
	#projectKeys = new Map<number, CryptoKey>();

	/**
	 * Load session status from the cookie and local vault.
	 * Safe to call repeatedly or concurrently. In-flight calls share one promise,
	 * which matters because SvelteKit loads layouts and pages in parallel.
	 */
	async ensureLoaded(): Promise<void> {
		if (this.#loaded) return;
		this.#loading ??= this.#load().finally(() => {
			this.#loading = null;
		});
		await this.#loading;
	}

	async #load(): Promise<void> {
		try {
			const user = await authApi.me();
			this.user = user;
			const identity = await vault.get();
			if (identity && identity.userId === user.id) {
				this.privateKey = identity.privateKey;
				this.status = 'ready';
			} else {
				this.status = 'locked';
			}
		} catch (error) {
			if (error instanceof ApiError && error.status === 401) {
				this.user = null;
				this.status = 'anonymous';
				this.#loaded = true;
				return;
			}
			this.status = 'anonymous';
			throw error;
		}
		this.#loaded = true;
	}

	/** Register by generating a keypair, deriving keys, and encrypting the private key. */
	async register(
		email: string,
		password: string,
		onStep?: ProgressCallback,
		inviteToken?: string
	): Promise<void> {
		const kdf = generateKDFConfig();

		onStep?.('Deriving key from password (Argon2id)…');
		const masterKeyPromise = deriveMasterKey(password, kdf);
		const keypairPromise = generateIdentityKeypair();
		const [masterKey, keypair] = await Promise.all([masterKeyPromise, keypairPromise]);

		try {
			onStep?.('Deriving encryption keys…');
			const authKey = await deriveAuthKey(masterKey);
			const wrapKey = await deriveWrapKey(masterKey);
			const publicKey = await exportPublicKey(keypair.publicKey);

			onStep?.('Encrypting private key…');
			const encryptedPrivateKey = await encryptPrivateKey(wrapKey, keypair.privateKey);

			onStep?.('Creating account…');
			const auth = await authApi.register({
				email,
				auth_key: authKey,
				kdf,
				public_key: publicKey,
				encrypted_private_key: encryptedPrivateKey,
				invite_token: inviteToken,
				client: 'web'
			});

			await this.#storeIdentity(auth.user, wrapKey, encryptedPrivateKey);
		} finally {
			wipeMasterKey(masterKey);
		}
	}

	/** Log in by deriving keys from the stored salt and decrypting the private key. */
	async login(email: string, password: string, onStep?: ProgressCallback): Promise<void> {
		onStep?.('Fetching KDF parameters…');
		const kdf = await authApi.kdf(email);

		onStep?.('Deriving key from password (Argon2id)…');
		const config = kdfConfigFromProfile(kdf);
		const masterKey = await deriveMasterKey(password, config);

		try {
			onStep?.('Verifying credentials…');
			const authKey = await deriveAuthKey(masterKey);
			const auth = await authApi.login({ email, auth_key: authKey, client: 'web' });

			onStep?.('Unlocking local vault…');
			const wrapKey = await deriveWrapKey(masterKey);
			await this.#storeIdentity(auth.user, wrapKey, auth.user.encrypted_private_key);
		} finally {
			wipeMasterKey(masterKey);
		}
	}

	/** Unlock an active session when its local vault is empty. */
	async unlock(password: string, onStep?: ProgressCallback): Promise<void> {
		const user = this.user ?? (await authApi.me());
		const config = kdfConfigFromProfile(user);

		onStep?.('Deriving key from password (Argon2id)…');
		const masterKey = await deriveMasterKey(password, config);

		try {
			onStep?.('Decrypting private key…');
			const wrapKey = await deriveWrapKey(masterKey);
			let privateKey: CryptoKey;
			try {
				privateKey = await decryptPrivateKey(wrapKey, user.encrypted_private_key);
			} catch {
				throw new Error('Incorrect password.');
			}

			await vault.set({
				userId: user.id,
				email: user.email,
				publicKey: user.public_key,
				privateKey,
				createdAt: Date.now()
			});
			this.user = user;
			this.privateKey = privateKey;
			this.status = 'ready';
		} finally {
			wipeMasterKey(masterKey);
		}
	}

	async logout(): Promise<void> {
		try {
			await authApi.logout();
		} catch {
			// The server session may have expired. Clear local state either way.
		}
		await vault.clear();
		this.#projectKeys.clear();
		this.privateKey = null;
		this.user = null;
		this.status = 'anonymous';
		this.#loaded = true;
	}

	/** Lock the vault without logging out. The private key is removed from memory. */
	lock(): void {
		this.#projectKeys.clear();
		this.privateKey = null;
		this.status = this.user ? 'locked' : 'anonymous';
	}

	rememberProjectKey(projectId: number, key: CryptoKey): void {
		this.#projectKeys.set(projectId, key);
	}

	getProjectKey(projectId: number): CryptoKey | undefined {
		return this.#projectKeys.get(projectId);
	}

	forgetProjectKey(projectId: number): void {
		this.#projectKeys.delete(projectId);
	}

	/** Unwrap the user's project key from the database and cache it in memory. */
	async unlockProjectKey(projectId: number, encryptedProjectKey: string): Promise<CryptoKey> {
		const cached = this.#projectKeys.get(projectId);
		if (cached) return cached;

		if (!this.privateKey) {
			throw new Error('Vault is locked. Unlock it with your password first.');
		}
		const key = await unwrapProjectKey(this.privateKey, encryptedProjectKey);
		this.#projectKeys.set(projectId, key);
		return key;
	}

	/** Return the user's public key for wrapping new project keys. */
	async ownPublicKey(): Promise<CryptoKey> {
		if (!this.user) throw new Error('Not signed in.');
		return importPublicKey(this.user.public_key);
	}

	async #storeIdentity(
		user: UserProfile,
		wrapKey: CryptoKey,
		encryptedPrivateKey: string
	): Promise<void> {
		// Re-import as non-extractable before storing the key in IndexedDB.
		const privateKey = await decryptPrivateKey(wrapKey, encryptedPrivateKey);
		await vault.set({
			userId: user.id,
			email: user.email,
			publicKey: user.public_key,
			privateKey,
			createdAt: Date.now()
		});
		this.user = user;
		this.privateKey = privateKey;
		this.status = 'ready';
	}
}

export const session = new SessionStore();
