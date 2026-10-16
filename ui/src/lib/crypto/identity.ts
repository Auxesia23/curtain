/**
 * User identity based on an RSA-OAEP 2048-bit keypair with SHA-256.
 *
 * - public_key = base64(SPKI DER), stored by the server and shared with other members
 * - the private key is encrypted with AES-256-GCM and wrapKey before upload
 * - after login, the private key is imported with `extractable: false` and stored
 *   in IndexedDB so JavaScript cannot export it
 */
import { b64decode, b64encode, concatBytes, randomBytes, utf8 } from './encoding';

export const RSA_ALGORITHM: RsaHashedKeyGenParams = {
	name: 'RSA-OAEP',
	modulusLength: 2048,
	publicExponent: new Uint8Array([0x01, 0x00, 0x01]),
	hash: 'SHA-256'
};

const PRIVATE_KEY_AAD = utf8('curtain.private-key.v1');
const IV_LENGTH = 12;

export async function generateIdentityKeypair(): Promise<CryptoKeyPair> {
	return crypto.subtle.generateKey(RSA_ALGORITHM, true, ['wrapKey', 'unwrapKey', 'encrypt', 'decrypt']);
}

export async function exportPublicKey(publicKey: CryptoKey): Promise<string> {
	const spki = await crypto.subtle.exportKey('spki', publicKey);
	return b64encode(spki);
}

export async function importPublicKey(publicKeyB64: string): Promise<CryptoKey> {
	return crypto.subtle.importKey('spki', b64decode(publicKeyB64), RSA_ALGORITHM, true, [
		'wrapKey',
		'encrypt'
	]);
}

/** Encrypt a PKCS#8 private key with wrapKey and return base64(iv || ciphertext). */
export async function encryptPrivateKey(
	wrapKey: CryptoKey,
	privateKey: CryptoKey
): Promise<string> {
	const pkcs8 = await crypto.subtle.exportKey('pkcs8', privateKey);
	const iv = randomBytes(IV_LENGTH);
	const ciphertext = await crypto.subtle.encrypt(
		{ name: 'AES-GCM', iv, additionalData: PRIVATE_KEY_AAD },
		wrapKey,
		pkcs8
	);
	return b64encode(concatBytes(iv, new Uint8Array(ciphertext)));
}

/**
 * Decrypt the private key received from the server and import it as
 * non-extractable so JavaScript cannot export it later.
 */
export async function decryptPrivateKey(wrapKey: CryptoKey, blobB64: string): Promise<CryptoKey> {
	const blob = b64decode(blobB64);
	if (blob.length <= IV_LENGTH) {
		throw new Error('Invalid private key blob');
	}
	const iv = blob.slice(0, IV_LENGTH);
	const ciphertext = blob.slice(IV_LENGTH);
	const pkcs8 = await crypto.subtle.decrypt(
		{ name: 'AES-GCM', iv, additionalData: PRIVATE_KEY_AAD },
		wrapKey,
		ciphertext
	);
	return crypto.subtle.importKey('pkcs8', pkcs8, RSA_ALGORITHM, false, ['unwrapKey', 'decrypt']);
}
