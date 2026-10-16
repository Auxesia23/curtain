#!/usr/bin/env node
/**
 * End-to-end tests for the Curtain API and client-side cryptography.
 *
 * This script reimplements the browser's cryptographic primitives
 * (WebCrypto + Argon2id from hash-wasm) and exercises the full flow:
 * first-run admin setup, invitation-only registration, cookie login, CLI tokens,
 * project keys, project invitations, variable encryption/decryption, and
 * admin user management.
 *
 * Run:
 *   go run ./cmd/api --addr :8099 --db /tmp/curtain-e2e.db
 *   BASE=http://localhost:8099 npm run e2e
 */
import { argon2id } from 'hash-wasm';

const BASE = process.env.BASE ?? 'http://localhost:8080';
const API = `${BASE}/api`;
const RUN = Date.now().toString(36);

const encoder = new TextEncoder();
const decoder = new TextDecoder();

const RSA_ALGORITHM = {
	name: 'RSA-OAEP',
	modulusLength: 2048,
	publicExponent: new Uint8Array([0x01, 0x00, 0x01]),
	hash: 'SHA-256'
};
const PRIVATE_KEY_AAD = encoder.encode('curtain.private-key.v1');
const VARIABLE_AAD = encoder.encode('curtain.variable.v1');

const b64 = (data) =>
	Buffer.from(data instanceof Uint8Array ? data : new Uint8Array(data)).toString('base64');
const unb64 = (text) => new Uint8Array(Buffer.from(text, 'base64'));
const randomBytes = (length) => crypto.getRandomValues(new Uint8Array(length));

function concat(...parts) {
	const total = parts.reduce((sum, part) => sum + part.length, 0);
	const out = new Uint8Array(total);
	let offset = 0;
	for (const part of parts) {
		out.set(part, offset);
		offset += part.length;
	}
	return out;
}

// ---------------------------------------------------------------- kripto

async function deriveMasterKey(password, kdf) {
	const result = await argon2id({
		password,
		salt: unb64(kdf.salt),
		parallelism: kdf.params.p,
		iterations: kdf.params.t,
		memorySize: kdf.params.m,
		hashLength: kdf.params.hash_length,
		outputType: 'binary'
	});
	return new Uint8Array(result);
}

async function hkdfBits(masterKey, info) {
	const ikm = await crypto.subtle.importKey('raw', masterKey, 'HKDF', false, ['deriveBits']);
	const bits = await crypto.subtle.deriveBits(
		{ name: 'HKDF', hash: 'SHA-256', salt: encoder.encode('curtain.kdf.v1'), info: encoder.encode(info) },
		ikm,
		256
	);
	return new Uint8Array(bits);
}

const deriveAuthKey = async (masterKey) => b64(await hkdfBits(masterKey, 'auth-key-v1'));

async function deriveWrapKey(masterKey) {
	const raw = await hkdfBits(masterKey, 'private-key-wrap-v1');
	return crypto.subtle.importKey('raw', raw, { name: 'AES-GCM' }, false, ['encrypt', 'decrypt']);
}

async function encryptPrivateKey(wrapKey, privateKey) {
	const pkcs8 = await crypto.subtle.exportKey('pkcs8', privateKey);
	const iv = randomBytes(12);
	const ciphertext = await crypto.subtle.encrypt(
		{ name: 'AES-GCM', iv, additionalData: PRIVATE_KEY_AAD },
		wrapKey,
		pkcs8
	);
	return b64(concat(iv, new Uint8Array(ciphertext)));
}

async function decryptPrivateKey(wrapKey, blobB64) {
	const blob = unb64(blobB64);
	const pkcs8 = await crypto.subtle.decrypt(
		{ name: 'AES-GCM', iv: blob.slice(0, 12), additionalData: PRIVATE_KEY_AAD },
		wrapKey,
		blob.slice(12)
	);
	return crypto.subtle.importKey('pkcs8', pkcs8, RSA_ALGORITHM, false, ['unwrapKey', 'decrypt']);
}

const importPublicKey = (publicKeyB64) =>
	crypto.subtle.importKey('spki', unb64(publicKeyB64), RSA_ALGORITHM, true, ['wrapKey', 'encrypt']);

async function wrapProjectKey(publicKey, projectKey) {
	const wrapped = await crypto.subtle.wrapKey('raw', projectKey, publicKey, { name: 'RSA-OAEP' });
	return b64(wrapped);
}

async function unwrapProjectKey(privateKey, wrappedB64) {
	return crypto.subtle.unwrapKey(
		'raw',
		unb64(wrappedB64),
		privateKey,
		{ name: 'RSA-OAEP' },
		{ name: 'AES-GCM', length: 256 },
		true,
		['encrypt', 'decrypt']
	);
}

async function encryptText(projectKey, plaintext) {
	const iv = randomBytes(12);
	const ciphertext = await crypto.subtle.encrypt(
		{ name: 'AES-GCM', iv, additionalData: VARIABLE_AAD },
		projectKey,
		encoder.encode(plaintext)
	);
	return b64(concat(iv, new Uint8Array(ciphertext)));
}

async function decryptText(projectKey, blobB64) {
	const blob = unb64(blobB64);
	const plaintext = await crypto.subtle.decrypt(
		{ name: 'AES-GCM', iv: blob.slice(0, 12), additionalData: VARIABLE_AAD },
		projectKey,
		blob.slice(12)
	);
	return decoder.decode(plaintext);
}

// ---------------------------------------------------------------- HTTP

function makeClient(name) {
	let cookie = null;
	let token = null;

	return {
		name,
		get cookie() {
			return cookie;
		},
		get token() {
			return token;
		},
		setToken(value) {
			token = value;
		},
		async request(method, path, body, { sendOrigin = true } = {}) {
			const headers = {};
			if (body !== undefined) headers['content-type'] = 'application/json';
			if (cookie) {
				headers.cookie = cookie;
				if (sendOrigin) headers.origin = BASE;
			}
			if (token) headers.authorization = `Bearer ${token}`;

			const response = await fetch(API + path, {
				method,
				headers,
				body: body === undefined ? undefined : JSON.stringify(body)
			});

			const setCookie = response.headers.getSetCookie?.() ?? [];
			if (setCookie.length > 0) {
				cookie = setCookie[0].split(';')[0];
			}

			const text = await response.text();
			let data = null;
			if (text) {
				try {
					data = JSON.parse(text);
				} catch {
					data = text;
				}
			}
			return { status: response.status, data, setCookie };
		}
	};
}

// ---------------------------------------------------------------- test util

let passed = 0;
let failed = 0;

function check(condition, label, detail = '') {
	if (condition) {
		passed++;
		console.log(`  ✔ ${label}`);
	} else {
		failed++;
		console.error(`  ✘ ${label}${detail ? ` — ${detail}` : ''}`);
	}
}

function section(title) {
	console.log(`\n== ${title}`);
}

// ---------------------------------------------------------------- alur

async function registerUser(client, email, password, inviteToken) {
	const kdf = {
		algorithm: 'argon2id',
		salt: b64(randomBytes(16)),
		params: { m: 65536, t: 3, p: 1, hash_length: 32 }
	};
	const keypair = await crypto.subtle.generateKey(RSA_ALGORITHM, true, [
		'wrapKey',
		'unwrapKey',
		'encrypt',
		'decrypt'
	]);
	const masterKey = await deriveMasterKey(password, kdf);
	const authKey = await deriveAuthKey(masterKey);
	const wrapKey = await deriveWrapKey(masterKey);
	const publicKey = b64(await crypto.subtle.exportKey('spki', keypair.publicKey));
	const encryptedPrivateKey = await encryptPrivateKey(wrapKey, keypair.privateKey);

	const res = await client.request('POST', '/auth/register', {
		email,
		auth_key: authKey,
		kdf,
		public_key: publicKey,
		encrypted_private_key: encryptedPrivateKey,
		invite_token: inviteToken,
		client: 'web'
	});

	return { res, kdf, keypair, wrapKey, publicKey };
}

async function main() {
	const aliceEmail = `alice-${RUN}@example.com`;
	const bobEmail = `bob-${RUN}@example.com`;
	const carolEmail = `carol-${RUN}@example.com`;
	const daveEmail = `dave-${RUN}@example.com`;
	const alicePassword = 'sandi-rahasia-alice';
	const bobPassword = 'sandi-rahasia-bob';
	const carolPassword = 'sandi-rahasia-carol';
	const davePassword = 'sandi-rahasia-dave';

	const anon = makeClient('anon');
	const alice = makeClient('alice-web');
	const bobWeb = makeClient('bob-web');
	const bobCli = makeClient('bob-cli');

	section('First-run setup: user pertama jadi admin (tanpa invite)');
	const before = await anon.request('GET', '/instance');
	check(before.status === 200 && before.data.needs_setup === true, 'GET /instance -> needs_setup=true');

	const aliceReg = await registerUser(alice, aliceEmail, alicePassword);
	check(aliceReg.res.status === 201, 'register user pertama -> 201', JSON.stringify(aliceReg.res.data));
	check(aliceReg.res.data?.user?.role === 'admin', 'user pertama mendapat role admin');
	check(
		aliceReg.res.setCookie.some((c) => /httponly/i.test(c) && c.startsWith('curtain_session=')),
		'cookie sesi httpOnly diterima'
	);

	const after = await anon.request('GET', '/instance');
	check(after.data.needs_setup === false, 'GET /instance -> needs_setup=false setelah admin ada');

	const intruder = await registerUser(anon, `intruder-${RUN}@example.com`, 'sandi-rahasia-x');
	check(intruder.res.status === 403, 'register tanpa invite -> 403', String(intruder.res.status));

	section('Sesi web & profil');
	const me = await alice.request('GET', '/auth/me');
	check(me.status === 200 && me.data.email === aliceEmail, 'GET /auth/me dengan cookie -> 200');
	check(me.data.role === 'admin', 'profil berisi role admin');
	check(
		typeof me.data.encrypted_private_key === 'string' && me.data.kdf_salt === aliceReg.kdf.salt,
		'profil berisi salt KDF + encrypted private key'
	);

	section('Login salah & proteksi CSRF');
	const wrong = await alice.request('POST', '/auth/login', {
		email: aliceEmail,
		auth_key: b64(randomBytes(32)),
		client: 'web'
	});
	check(wrong.status === 401, 'login dengan auth_key salah -> 401', String(wrong.status));

	const csrf = await alice.request('POST', '/auth/logout', undefined, { sendOrigin: false });
	check(csrf.status === 403, 'request cookie tanpa Origin valid -> 403 (CSRF)', String(csrf.status));

	section('Admin: invite link & pendaftaran invite-only');
	const inviteRes = await alice.request('POST', '/admin/invites', {
		max_uses: 5,
		expires_in_hours: 24
	});
	check(inviteRes.status === 201, 'admin membuat invite -> 201', JSON.stringify(inviteRes.data));
	const inviteToken = inviteRes.data.token;
	check(typeof inviteToken === 'string' && inviteToken.length > 20, 'respons berisi token + url');

	const inviteInfo = await anon.request('GET', `/invites/${inviteToken}`);
	check(
		inviteInfo.status === 200 && inviteInfo.data.valid === true && inviteInfo.data.remaining_uses === 5,
		'validasi invite publik -> valid dengan 5 sisa pemakaian'
	);

	const bobReg = await registerUser(bobWeb, bobEmail, bobPassword, inviteToken);
	check(bobReg.res.status === 201, 'bob register dengan invite -> 201');
	check(bobReg.res.data?.user?.role === 'member', 'bob mendapat role member');

	const bobAdminAttempt = await bobWeb.request('GET', '/admin/users');
	check(bobAdminAttempt.status === 403, 'member tidak bisa akses endpoint admin -> 403');

	const limitedInvite = await alice.request('POST', '/admin/invites', { max_uses: 1, expires_in_hours: 1 });
	const limitedToken = limitedInvite.data.token;
	const carol = makeClient('carol-web');
	const carolReg = await registerUser(carol, carolEmail, carolPassword, limitedToken);
	check(carolReg.res.status === 201, 'carol register memakai invite max_uses=1 -> 201');

	const dave = makeClient('dave-web');
	const daveReg = await registerUser(dave, daveEmail, davePassword, limitedToken);
	check(daveReg.res.status === 403, 'invite habis dipakai -> 403', String(daveReg.res.status));

	const revokeTarget = await alice.request('POST', '/admin/invites', { max_uses: 3, expires_in_hours: 1 });
	const revokeRes = await alice.request('DELETE', `/admin/invites/${revokeTarget.data.id}`);
	check(revokeRes.status === 204, 'admin revoke invite -> 204');
	const afterRevoke = await anon.request('GET', `/invites/${revokeTarget.data.token}`);
	check(afterRevoke.data.valid === false, 'invite yang di-revoke tidak valid lagi');

	const inviteList = await alice.request('GET', '/admin/invites');
	check(
		inviteList.status === 200 && inviteList.data.length >= 3,
		'admin melihat daftar invite + status',
		JSON.stringify(inviteList.data?.map?.((i) => i.status))
	);
	check(
		inviteList.data.every((i) => i.token === undefined),
		'daftar invite tidak membocorkan token mentah'
	);

	section('Login ulang (dekripsi private key dari salt tersimpan)');
	const kdfInfo = await alice.request('GET', `/auth/kdf?email=${encodeURIComponent(aliceEmail)}`);
	check(kdfInfo.status === 200, 'GET /auth/kdf -> 200');
	const masterKey = await deriveMasterKey(alicePassword, {
		algorithm: kdfInfo.data.kdf_algo,
		salt: kdfInfo.data.kdf_salt,
		params: JSON.parse(kdfInfo.data.kdf_params)
	});
	const aliceLogin = await alice.request('POST', '/auth/login', {
		email: aliceEmail,
		auth_key: await deriveAuthKey(masterKey),
		client: 'web'
	});
	check(aliceLogin.status === 200, 'login ulang -> 200', JSON.stringify(aliceLogin.data));
	const alicePrivateKey = await decryptPrivateKey(
		await deriveWrapKey(masterKey),
		aliceLogin.data.user.encrypted_private_key
	);
	check(alicePrivateKey.extractable === false, 'private key di-import non-extractable');

	section('Project & variable (enkripsi browser)');
	const projectKey = await crypto.subtle.generateKey({ name: 'AES-GCM', length: 256 }, true, [
		'encrypt',
		'decrypt'
	]);
	const alicePublicKey = await importPublicKey(aliceLogin.data.user.public_key);
	const projectRes = await alice.request('POST', '/projects', {
		name: `e2e-${RUN}`,
		encrypted_project_key: await wrapProjectKey(alicePublicKey, projectKey)
	});
	check(projectRes.status === 201, 'buat project -> 201', JSON.stringify(projectRes.data));
	const projectId = projectRes.data.id;

	const envRes = await alice.request('POST', `/projects/${projectId}/environments`, {
		name: 'production'
	});
	check(envRes.status === 201, 'buat environment -> 201', JSON.stringify(envRes.data));
	const environmentId = envRes.data.ID;

	const secretName = 'DATABASE_URL';
	const secretValue = `postgres://alice:${RUN}@db.internal:5432/app`;
	const variableRes = await alice.request('POST', `/environments/${environmentId}/variables`, {
		encrypted_key: await encryptText(projectKey, secretName),
		encrypted_value: await encryptText(projectKey, secretValue)
	});
	check(variableRes.status === 201, 'simpan variable terenkripsi -> 201');
	const variableId = variableRes.data.ID;
	check(
		!JSON.stringify(variableRes.data).includes(secretValue),
		'server tidak menerima plaintext nilai variable'
	);

	section('Sesi CLI bob (bearer token)');
	const bobKdf = await bobCli.request('GET', `/auth/kdf?email=${encodeURIComponent(bobEmail)}`);
	const bobMasterKey = await deriveMasterKey(bobPassword, {
		algorithm: bobKdf.data.kdf_algo,
		salt: bobKdf.data.kdf_salt,
		params: JSON.parse(bobKdf.data.kdf_params)
	});
	const bobCliLogin = await bobCli.request('POST', '/auth/login', {
		email: bobEmail,
		auth_key: await deriveAuthKey(bobMasterKey),
		client: 'cli'
	});
	check(bobCliLogin.status === 200, 'login CLI -> 200');
	check(typeof bobCliLogin.data.token === 'string', 'login CLI mengembalikan token di body');
	check(bobCliLogin.setCookie.length === 0, 'login CLI tidak mengirim cookie sesi');
	bobCli.setToken(bobCliLogin.data.token);
	const bobPrivateKey = await decryptPrivateKey(
		await deriveWrapKey(bobMasterKey),
		bobCliLogin.data.user.encrypted_private_key
	);
	check((await bobCli.request('GET', '/auth/me')).status === 200, 'GET /auth/me dengan bearer -> 200');

	section('Invite project: bagikan project key ke public key bob');
	const lookup = await alice.request('GET', `/users/lookup?email=${encodeURIComponent(bobEmail)}`);
	check(lookup.status === 200, 'lookup bob -> 200');
	const inviteProjectRes = await alice.request('POST', `/projects/${projectId}/invite`, {
		user_id: lookup.data.id,
		encrypted_project_key: await wrapProjectKey(
			await importPublicKey(lookup.data.public_key),
			projectKey
		)
	});
	check(inviteProjectRes.status === 201, 'invite bob ke project -> 201');

	section('Bob membuka project key & mendekripsi variable');
	const bobProjects = await bobCli.request('GET', '/projects');
	check(
		bobProjects.status === 200 && bobProjects.data.some((p) => p.id === projectId),
		'bob melihat project yang di-invite'
	);
	const bobProject = await bobCli.request('GET', `/projects/${projectId}`);
	const bobProjectKey = await unwrapProjectKey(bobPrivateKey, bobProject.data.encrypted_project_key);

	const bobVariables = await bobCli.request('GET', `/environments/${environmentId}/variables`);
	const row = bobVariables.data.find((v) => v.ID === variableId);
	check((await decryptText(bobProjectKey, row.encrypted_key)) === secretName, 'nama variable terdekripsi oleh bob');
	check((await decryptText(bobProjectKey, row.encrypted_value)) === secretValue, 'nilai variable terdekripsi oleh bob');

	section('Role & kapabilitas project (Fase 2)');
	// Carol already has an account from the invitation section; add her as a viewer.
	const carolLookup = await alice.request('GET', `/users/lookup?email=${encodeURIComponent(carolEmail)}`);
	const carolInvite = await alice.request('POST', `/projects/${projectId}/invite`, {
		user_id: carolLookup.data.id,
		encrypted_project_key: await wrapProjectKey(await importPublicKey(carolLookup.data.public_key), projectKey),
		role: 'viewer'
	});
	check(carolInvite.status === 201, 'invite carol sebagai viewer -> 201');

	const carolCli = makeClient('carol-cli');
	const carolKdf = await carolCli.request('GET', `/auth/kdf?email=${encodeURIComponent(carolEmail)}`);
	const carolMasterKey = await deriveMasterKey(carolPassword, {
		algorithm: carolKdf.data.kdf_algo,
		salt: carolKdf.data.kdf_salt,
		params: JSON.parse(carolKdf.data.kdf_params)
	});
	const carolLogin = await carolCli.request('POST', '/auth/login', {
		email: carolEmail,
		auth_key: await deriveAuthKey(carolMasterKey),
		client: 'cli'
	});
	carolCli.setToken(carolLogin.data.token);

	const viewerRead = await carolCli.request('GET', `/environments/${environmentId}/variables`);
	check(viewerRead.status === 200, 'viewer boleh membaca variable -> 200');
	const viewerWrite = await carolCli.request('POST', `/environments/${environmentId}/variables`, {
		encrypted_key: await encryptText(projectKey, 'VIEWER_KEY'),
		encrypted_value: await encryptText(projectKey, 'nope')
	});
	check(viewerWrite.status === 403, 'viewer tidak boleh membuat variable -> 403', String(viewerWrite.status));
	const viewerDeleteEnv = await carolCli.request('DELETE', `/environments/${environmentId}`);
	check(viewerDeleteEnv.status === 403, 'viewer tidak boleh hapus environment -> 403');

	const editorWrite = await bobCli.request('POST', `/environments/${environmentId}/variables`, {
		encrypted_key: await encryptText(projectKey, 'EDITOR_KEY'),
		encrypted_value: await encryptText(projectKey, 'boleh')
	});
	check(editorWrite.status === 201, 'editor boleh membuat variable -> 201');
	const editorInvite = await bobCli.request('POST', `/projects/${projectId}/invite`, {
		user_id: carolLookup.data.id,
		encrypted_project_key: 'x'
	});
	check(editorInvite.status === 403, 'editor tidak boleh invite member -> 403');
	const editorManage = await bobCli.request('PATCH', `/projects/${projectId}/members/${carolLookup.data.id}`, {
		role: 'viewer'
	});
	check(editorManage.status === 403, 'editor tidak boleh mengubah role member -> 403');

	const promote = await alice.request('PATCH', `/projects/${projectId}/members/${carolLookup.data.id}`, {
		role: 'editor'
	});
	check(promote.status === 200, 'owner mengubah carol menjadi editor -> 200');
	const promotedWrite = await carolCli.request('POST', `/environments/${environmentId}/variables`, {
		encrypted_key: await encryptText(projectKey, 'PROMOTED_KEY'),
		encrypted_value: await encryptText(projectKey, 'yes')
	});
	check(promotedWrite.status === 201, 'carol (editor) boleh membuat variable -> 201');

	const denyOverride = await alice.request('PUT', `/projects/${projectId}/members/${carolLookup.data.id}/permissions`, {
		overrides: [{ capability: 'var.create', effect: 'deny' }]
	});
	check(denyOverride.status === 204, 'owner menulis override deny var.create -> 204');
	const deniedWrite = await carolCli.request('POST', `/environments/${environmentId}/variables`, {
		encrypted_key: await encryptText(projectKey, 'DENIED_KEY'),
		encrypted_value: await encryptText(projectKey, 'no')
	});
	check(deniedWrite.status === 403, 'override deny menang atas role editor -> 403', String(deniedWrite.status));

	const resetOverride = await alice.request('PUT', `/projects/${projectId}/members/${carolLookup.data.id}/permissions`, {
		overrides: []
	});
	check(resetOverride.status === 204, 'override di-reset -> 204');

	const deleteTarget = await alice.request('POST', `/environments/${environmentId}/variables`, {
		encrypted_key: await encryptText(projectKey, 'DELETE_ME'),
		encrypted_value: await encryptText(projectKey, 'bye')
	});
	check(deleteTarget.status === 201, 'alice membuat variable untuk uji delete -> 201');

	const bobLookup = await alice.request('GET', `/users/lookup?email=${encodeURIComponent(bobEmail)}`);
	const bobDenyDelete = await alice.request('PUT', `/projects/${projectId}/members/${bobLookup.data.id}/permissions`, {
		overrides: [{ capability: 'var.delete', effect: 'deny' }]
	});
	check(bobDenyDelete.status === 204, 'owner menulis override deny var.delete untuk bob -> 204');
	const deniedDelete = await bobCli.request('DELETE', `/variables/${deleteTarget.data.ID}`);
	check(deniedDelete.status === 403, 'override deny var.delete menahan editor -> 403', String(deniedDelete.status));

	await alice.request('PUT', `/projects/${projectId}/members/${bobLookup.data.id}/permissions`, {
		overrides: []
	});
	const allowedDelete = await bobCli.request('DELETE', `/variables/${deleteTarget.data.ID}`);
	check(allowedDelete.status === 204, 'setelah override dihapus, editor boleh hapus variable -> 204');

	const lastOwnerDemote = await alice.request('PATCH', `/projects/${projectId}/members/${aliceLogin.data.user.id}`, {
		role: 'editor'
	});
	check(lastOwnerDemote.status === 409, 'owner terakhir tidak bisa di-demote -> 409', String(lastOwnerDemote.status));
	const lastOwnerRemove = await alice.request('DELETE', `/projects/${projectId}/members/${aliceLogin.data.user.id}`);
	check(lastOwnerRemove.status === 409, 'owner terakhir tidak bisa di-remove -> 409');

	const catalog = await bobCli.request('GET', '/permissions/catalog');
	check(
		catalog.status === 200 && catalog.data.capabilities.length === 8 && catalog.data.roles.length === 3,
		'katalog kapabilitas -> 8 kapabilitas, 3 role'
	);

	section('Update variable & logout');
	const updatedValue = `postgres://alice:${RUN}@db2.internal:5432/app`;
	const updateRes = await alice.request('PUT', `/variables/${variableId}`, {
		encrypted_key: await encryptText(projectKey, secretName),
		encrypted_value: await encryptText(projectKey, updatedValue)
	});
	check(updateRes.status === 200, 'update variable -> 200');

	const logoutRes = await bobCli.request('POST', '/auth/logout');
	check(logoutRes.status === 204, 'logout CLI -> 204');
	check((await bobCli.request('GET', '/auth/me')).status === 401, 'token lama tidak berlaku setelah logout');

	section('Admin: disable/enable & hapus user');
	const users = await alice.request('GET', '/admin/users');
	const bobId = users.data.find((u) => u.email === bobEmail)?.id;
	const carolId = users.data.find((u) => u.email === carolEmail)?.id;
	check(typeof bobId === 'number' && typeof carolId === 'number', 'admin melihat daftar user');

	const bobRelogin = await bobCli.request('POST', '/auth/login', {
		email: bobEmail,
		auth_key: await deriveAuthKey(bobMasterKey),
		client: 'cli'
	});
	bobCli.setToken(bobRelogin.data.token);

	const disableRes = await alice.request('POST', `/admin/users/${bobId}/disable`);
	check(disableRes.status === 200, 'admin disable bob -> 200');
	check(
		(await bobCli.request('GET', '/auth/me')).status === 401,
		'sesi bob langsung tidak berlaku setelah disable'
	);
	const disabledLogin = await bobCli.request('POST', '/auth/login', {
		email: bobEmail,
		auth_key: await deriveAuthKey(bobMasterKey),
		client: 'cli'
	});
	check(disabledLogin.status === 403, 'login user disabled -> 403', String(disabledLogin.status));

	const enableRes = await alice.request('POST', `/admin/users/${bobId}/enable`);
	check(enableRes.status === 200, 'admin enable bob -> 200');
	const reLogin = await bobCli.request('POST', '/auth/login', {
		email: bobEmail,
		auth_key: await deriveAuthKey(bobMasterKey),
		client: 'cli'
	});
	check(reLogin.status === 200, 'login setelah enable -> 200');
	bobCli.setToken(reLogin.data.token);

	const selfDisable = await alice.request('POST', `/admin/users/${aliceLogin.data.user.id}/disable`);
	check(selfDisable.status === 409, 'admin tidak bisa disable dirinya sendiri -> 409');

	const deleteRes = await alice.request('DELETE', `/admin/users/${carolId}`);
	check(deleteRes.status === 204, 'admin hapus user -> 204');
	const usersAfter = await alice.request('GET', '/admin/users');
	check(
		!usersAfter.data.some((u) => u.email === carolEmail),
		'user yang dihapus tidak ada di daftar'
	);

	section('Owner menghapus project');
	const memberDelete = await bobCli.request('DELETE', `/projects/${projectId}`);
	check(memberDelete.status === 403, 'member tidak bisa hapus project -> 403', String(memberDelete.status));

	const ownerDelete = await alice.request('DELETE', `/projects/${projectId}`);
	check(ownerDelete.status === 204, 'owner hapus project -> 204', String(ownerDelete.status));

	const goneProject = await alice.request('GET', `/projects/${projectId}`);
	check(goneProject.status === 404, 'project yang dihapus -> 404');
	const goneVariables = await bobCli.request('GET', `/environments/${environmentId}/variables`);
	check(goneVariables.status === 404, 'environment ikut terhapus -> 404');

	console.log(`\n${passed} passed, ${failed} failed`);
	if (failed > 0) {
		process.exit(1);
	}
}

main().catch((error) => {
	console.error(error);
	process.exit(1);
});
