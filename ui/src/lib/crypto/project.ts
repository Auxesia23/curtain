/**
 * Project keys (AES-256-GCM) and variable encryption.
 *
 * - a random project key is generated in the browser when a project is created
 * - the key is wrapped with each member's public key using RSA-OAEP before upload
 * - variable names and values are encrypted with the project key using AES-GCM
 * - encrypted blobs use the format base64(iv(12) || ciphertext)
 */
import { b64decode, b64encode, concatBytes, fromUtf8, randomBytes, utf8 } from './encoding';

const PROJECT_KEY_ALGORITHM: AesKeyGenParams = { name: 'AES-GCM', length: 256 };
const VARIABLE_AAD = utf8('curtain.variable.v1');
const IV_LENGTH = 12;

export async function generateProjectKey(): Promise<CryptoKey> {
	return crypto.subtle.generateKey(PROJECT_KEY_ALGORITHM, true, ['encrypt', 'decrypt']);
}

/** Wrap a project key with a recipient's public key and return base64 for the server. */
export async function wrapProjectKey(
	recipientPublicKey: CryptoKey,
	projectKey: CryptoKey
): Promise<string> {
	const wrapped = await crypto.subtle.wrapKey('raw', projectKey, recipientPublicKey, {
		name: 'RSA-OAEP'
	});
	return b64encode(wrapped);
}

/** Unwrap a project key with the user's private key. */
export async function unwrapProjectKey(
	privateKey: CryptoKey,
	wrappedB64: string
): Promise<CryptoKey> {
	return crypto.subtle.unwrapKey(
		'raw',
		b64decode(wrappedB64),
		privateKey,
		{ name: 'RSA-OAEP' },
		PROJECT_KEY_ALGORITHM,
		true,
		['encrypt', 'decrypt']
	);
}

export async function encryptText(projectKey: CryptoKey, plaintext: string): Promise<string> {
	const iv = randomBytes(IV_LENGTH);
	const ciphertext = await crypto.subtle.encrypt(
		{ name: 'AES-GCM', iv, additionalData: VARIABLE_AAD },
		projectKey,
		utf8(plaintext)
	);
	return b64encode(concatBytes(iv, new Uint8Array(ciphertext)));
}

export async function decryptText(projectKey: CryptoKey, blobB64: string): Promise<string> {
	const blob = b64decode(blobB64);
	if (blob.length <= IV_LENGTH) {
		throw new Error('Invalid variable blob');
	}
	const iv = blob.slice(0, IV_LENGTH);
	const ciphertext = blob.slice(IV_LENGTH);
	const plaintext = await crypto.subtle.decrypt(
		{ name: 'AES-GCM', iv, additionalData: VARIABLE_AAD },
		projectKey,
		ciphertext
	);
	return fromUtf8(plaintext);
}
