/** Encoding helpers for base64, UTF-8, and byte-buffer concatenation. */

/** Byte buffer backed by an ArrayBuffer for WebCrypto compatibility. */
export type Bytes = Uint8Array<ArrayBuffer>;

const encoder = new TextEncoder();
const decoder = new TextDecoder();

export function utf8(text: string): Bytes {
	return new Uint8Array(encoder.encode(text));
}

export function fromUtf8(bytes: ArrayBuffer | Uint8Array): string {
	return decoder.decode(bytes);
}

export function b64encode(data: ArrayBuffer | Uint8Array): string {
	const bytes = data instanceof Uint8Array ? data : new Uint8Array(data);
	let binary = '';
	for (let i = 0; i < bytes.length; i++) {
		binary += String.fromCharCode(bytes[i]);
	}
	return btoa(binary);
}

export function b64decode(text: string): Bytes {
	const binary = atob(text);
	const bytes = new Uint8Array(binary.length);
	for (let i = 0; i < binary.length; i++) {
		bytes[i] = binary.charCodeAt(i);
	}
	return bytes;
}

export function randomBytes(length: number): Bytes {
	return crypto.getRandomValues(new Uint8Array(length));
}

export function concatBytes(...parts: Uint8Array[]): Bytes {
	const total = parts.reduce((sum, part) => sum + part.length, 0);
	const out = new Uint8Array(total);
	let offset = 0;
	for (const part of parts) {
		out.set(part, offset);
		offset += part.length;
	}
	return out;
}
