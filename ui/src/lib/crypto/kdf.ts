/**
 * Argon2id key derivation and derived keys.
 *
 * Design, also documented in the README:
 *
 *   masterKey  = Argon2id(password, salt, {m,t,p}, 32 bytes)
 *   authKey    = HKDF-SHA256(masterKey, info="auth-key-v1") -> sent to the server
 *   wrapKey    = HKDF-SHA256(masterKey, info="private-key-wrap-v1") -> AES-GCM key
 *
 * Domain separation keeps the server-verified authKey separate from the key
 * that encrypts the private key. The server and TLS intermediaries never get
 * the key needed to unlock the user's vault.
 */
import { argon2id } from 'hash-wasm';
import { b64decode, b64encode, randomBytes, utf8, type Bytes } from './encoding';

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

/** Default Argon2id parameters: 64 MiB and 3 iterations, following OWASP guidance. */
export const DEFAULT_KDF_PARAMS: KDFParams = { m: 65536, t: 3, p: 1, hash_length: 32 };

const KDF_SALT_INFO = 'curtain.kdf.v1';
const AUTH_KEY_INFO = 'auth-key-v1';
const WRAP_KEY_INFO = 'private-key-wrap-v1';

/** Create KDF settings for registration with a random 16-byte salt. */
export function generateKDFConfig(): KDFConfig {
	return {
		algorithm: 'argon2id',
		salt: b64encode(randomBytes(16)),
		params: { ...DEFAULT_KDF_PARAMS }
	};
}

/**
 * Derive a master key from the password with Argon2id.
 * This is intentionally expensive and uses about 64 MiB of memory.
 */
export async function deriveMasterKey(password: string, kdf: KDFConfig): Promise<Bytes> {
	const result = await argon2id({
		password,
		salt: b64decode(kdf.salt),
		parallelism: kdf.params.p,
		iterations: kdf.params.t,
		memorySize: kdf.params.m,
		hashLength: kdf.params.hash_length,
		outputType: 'binary'
	});
	// Copy into a new buffer backed by ArrayBuffer for WebCrypto compatibility.
	return new Uint8Array(result);
}

async function hkdfBits(masterKey: Bytes, info: string): Promise<Bytes> {
	const ikm = await crypto.subtle.importKey('raw', masterKey, 'HKDF', false, ['deriveBits']);
	const bits = await crypto.subtle.deriveBits(
		{
			name: 'HKDF',
			hash: 'SHA-256',
			salt: utf8(KDF_SALT_INFO),
			info: utf8(info)
		},
		ikm,
		256
	);
	return new Uint8Array(bits);
}

/** Return the base64 authKey sent to the server during registration and login. */
export async function deriveAuthKey(masterKey: Bytes): Promise<string> {
	return b64encode(await hkdfBits(masterKey, AUTH_KEY_INFO));
}

/** Derive the non-extractable AES-GCM key used to encrypt the private key. */
export async function deriveWrapKey(masterKey: Bytes): Promise<CryptoKey> {
	const raw = await hkdfBits(masterKey, WRAP_KEY_INFO);
	return crypto.subtle.importKey('raw', raw, { name: 'AES-GCM' }, false, ['encrypt', 'decrypt']);
}

/** Clear the master key from memory after use. */
export function wipeMasterKey(masterKey: Uint8Array): void {
	masterKey.fill(0);
}

/** Rebuild KDFConfig from the user profile returned by the server. */
export function kdfConfigFromProfile(profile: {
	kdf_algo?: string;
	kdf_salt: string;
	kdf_params: string;
}): KDFConfig {
	const algorithm = (profile.kdf_algo || 'argon2id') as KDFConfig['algorithm'];
	if (algorithm !== 'argon2id') {
		throw new Error(`Unsupported KDF algorithm: ${algorithm}`);
	}
	let params: KDFParams = { ...DEFAULT_KDF_PARAMS };
	if (profile.kdf_params) {
		try {
			params = { ...params, ...(JSON.parse(profile.kdf_params) as Partial<KDFParams>) };
		} catch {
			throw new Error('Invalid KDF parameters from server');
		}
	}
	return { algorithm, salt: profile.kdf_salt, params };
}
