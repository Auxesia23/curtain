/**
 * Local IndexedDB vault for a user's cryptographic identity.
 *
 * It stores the private key as a non-extractable CryptoKey, re-imported from
 * the encrypted blob returned by the server, along with public metadata.
 * Because `extractable` is false, JavaScript cannot export the private key,
 * even if an XSS vulnerability occurs after the page loads.
 *
 * The vault is cleared on logout.
 */
import { createStore, del, get, set } from 'idb-keyval';

export interface VaultIdentity {
	userId: number;
	email: string;
	publicKey: string;
	privateKey: CryptoKey;
	createdAt: number;
}

const VAULT_KEY = 'identity';
const store = createStore('curtain', 'vault');

export const vault = {
	async get(): Promise<VaultIdentity | undefined> {
		return get<VaultIdentity>(VAULT_KEY, store);
	},
	async set(identity: VaultIdentity): Promise<void> {
		await set(VAULT_KEY, identity, store);
	},
	async clear(): Promise<void> {
		await del(VAULT_KEY, store);
	}
};
