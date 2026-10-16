#!/usr/bin/env node
/**
 * End-to-end UI tests using a real browser (headless Playwright Chromium).
 *
 * Flow: first-run admin setup, create an invitation in Settings, register Bob
 * through /invite/<token>, then exercise project flows (create a project,
 * environment, and variable; reveal it; reload the vault; invite a project
 * member; decrypt a variable; and log out and back in).
 *
 * Run with a server that embeds the latest UI build:
 *   make build && ./bin/curtain-server --addr :8099 --db /tmp/curtain-ui.db
 *   BASE=http://localhost:8099 npm run e2e:ui
 */
import { chromium } from 'playwright';

const BASE = process.env.BASE ?? 'http://localhost:8099';
const RUN = Date.now().toString(36);
const aliceEmail = `alice-ui-${RUN}@example.com`;
const bobEmail = `bob-ui-${RUN}@example.com`;
const password = 'sandi-rahasia-ui';
const projectName = `ui-project-${RUN}`;
const variableName = 'DATABASE_URL';
const variableValue = `postgres://alice:${RUN}@db.internal:5432/app`;

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

const browserErrors = [];

function watch(page, label) {
	page.on('pageerror', (error) => browserErrors.push(`[${label}] ${error.message}`));
	page.on('console', (message) => {
		if (message.type() === 'error' && !message.text().includes('401')) {
			browserErrors.push(`[${label}] console: ${message.text()}`);
		}
	});
}

async function fillCredentials(page, email) {
	await page.getByLabel('Email', { exact: true }).fill(email);
	await page.getByLabel('Password', { exact: true }).fill(password);
	await page.getByLabel('Confirm password', { exact: true }).fill(password);
	await page.locator('form button[type="submit"]').click();
	await page.waitForURL('**/projects', { timeout: 120_000 });
}

/** First-run setup: instance kosong -> user pertama jadi admin. */
async function setupAdmin(page, email) {
	await page.goto(`${BASE}/setup`);
	await page.getByText('Set up Curtain').waitFor({ timeout: 15_000 });
	await fillCredentials(page, email);
}

async function registerViaInvite(page, email, inviteLink) {
	await page.goto(inviteLink);
	await page.getByText('Create your Curtain account').waitFor({ timeout: 15_000 });
	await fillCredentials(page, email);
}

async function login(page, email) {
	await page.goto(`${BASE}/login`);
	await page.getByLabel('Email', { exact: true }).fill(email);
	await page.getByLabel('Password', { exact: true }).fill(password);
	await page.getByRole('button', { name: 'Sign in' }).click();
	await page.waitForURL('**/projects', { timeout: 120_000 });
}

async function createInviteLink(page) {
	await page.goto(`${BASE}/settings`);
	await page.getByRole('heading', { name: 'Settings' }).waitFor({ timeout: 15_000 });
	await page.getByRole('button', { name: 'Create invite' }).click();
	await page.getByLabel('Max uses').fill('5');
	await page.getByLabel('Expires in (days)').fill('7');
	await page.getByRole('button', { name: 'Create link' }).click();
	const link = (await page.locator('code').first().innerText()).trim();
	await page.getByRole('button', { name: 'Done' }).click();
	return link;
}

async function main() {
	const browser = await chromium.launch();

	try {
		const aliceContext = await browser.newContext();
		const alice = await aliceContext.newPage();
		watch(alice, 'alice');

		section('First-run setup lewat UI (user pertama jadi admin)');
		await setupAdmin(alice, aliceEmail);
		check(alice.url().endsWith('/projects'), 'setup -> halaman projects');
		await alice.locator('[aria-label="Account menu"]').waitFor({ timeout: 15_000 });

		await alice.locator('[aria-label="Account menu"]').click();
		await alice.getByRole('menuitem', { name: 'Settings' }).waitFor({ timeout: 10_000 });
		check(true, 'admin melihat menu Settings');
		await alice.keyboard.press('Escape');

		section('Vault lokal (IndexedDB, private key non-extractable)');
		const vault = await alice.evaluate(async () => {
			const db = await new Promise((resolve, reject) => {
				const request = indexedDB.open('curtain');
				request.onsuccess = () => resolve(request.result);
				request.onerror = () => reject(request.error);
			});
			const tx = db.transaction('vault', 'readonly');
			const record = await new Promise((resolve, reject) => {
				const request = tx.objectStore('vault').get('identity');
				request.onsuccess = () => resolve(request.result);
				request.onerror = () => reject(request.error);
			});
			db.close();
			return record
				? {
						userId: record.userId,
						extractable: record.privateKey.extractable,
						type: record.privateKey.type
					}
				: null;
		});
		check(vault !== null, 'identitas tersimpan di IndexedDB');
		check(vault?.extractable === false, 'private key disimpan dengan extractable=false');
		check(vault?.type === 'private', 'objek tersimpan berupa CryptoKey private');

		section('Buat project, environment, dan variable');
		await alice.getByRole('button', { name: 'New project' }).first().click();
		await alice.getByLabel('Project name').fill(projectName);
		await alice.getByRole('button', { name: 'Create project' }).click();
		await alice.getByRole('link', { name: 'Open project' }).first().click();
		await alice.waitForURL(/\/projects\/\d+$/);
		const projectUrl = alice.url();
		check(true, 'project dibuat dan dibuka');

		await alice.getByRole('button', { name: 'Environment', exact: true }).click();
		await alice.getByLabel('Environment name').fill('production');
		await alice.getByRole('button', { name: 'Create environment' }).click();
		await alice.getByText('production').first().waitFor({ timeout: 10_000 });
		check(true, 'environment "production" dibuat');

		const envTrigger = alice.locator('[data-slot="select-trigger"]');
		const envTriggerText = (await envTrigger.innerText()).trim();
		check(
			envTriggerText.includes('production') && !/^\d+$/.test(envTriggerText),
			'dropdown environment menampilkan nama (bukan id)',
			envTriggerText
		);

		await alice.getByRole('button', { name: 'New variable', exact: true }).click();
		await alice.getByLabel('Name', { exact: true }).fill(variableName);
		await alice.getByLabel('Value', { exact: true }).fill(variableValue);
		await alice.getByRole('button', { name: 'Add variable' }).click();
		await alice.getByText(variableName).waitFor({ timeout: 10_000 });
		check(true, 'variable tersimpan & nama terdekripsi di tabel');

		// Regression: the dialog should open again after a successful add.
		await alice.getByRole('button', { name: 'New variable', exact: true }).click();
		await alice.getByRole('dialog').waitFor({ timeout: 10_000 });
		check(true, 'dialog variable bisa dibuka lagi setelah sukses');
		await alice.getByRole('dialog').getByRole('button', { name: 'Close' }).click();
		await alice.getByRole('dialog').waitFor({ state: 'detached', timeout: 10_000 });

		section('Reveal nilai variable (dekripsi di browser)');
		await alice.getByRole('button', { name: `Actions for ${variableName}` }).click();
		await alice.getByRole('menuitem', { name: 'Show' }).click();
		await alice.getByText(variableValue).waitFor({ timeout: 10_000 });
		check(true, 'variable value decrypted after "Show"');

		section('Reload: vault lokal dipakai ulang');
		await alice.reload();
		try {
			await alice.getByText(variableName).waitFor({ timeout: 15_000 });
			check(!alice.url().includes('/unlock'), 'tidak diminta unlock setelah reload (vault IndexedDB)');
		} catch (error) {
			console.error('DEBUG url:', alice.url());
			console.error('DEBUG body:', (await alice.locator('body').innerText()).slice(0, 500));
			console.error('DEBUG errors:', browserErrors.slice(-8));
			throw error;
		}

		section('Admin membuat invite link lewat Settings');
		const inviteLink = await createInviteLink(alice);
		check(inviteLink.includes('/invite/'), 'invite link dibuat', inviteLink);
		await alice.getByRole('button', { name: 'Revoke' }).first().waitFor({ timeout: 10_000 });
		check(true, 'invite muncul di daftar dengan status aktif');

		// Regression: reopening the invite dialog should show the form, not the old link.
		await alice.getByRole('button', { name: 'Create invite' }).click();
		await alice.getByLabel('Max uses').waitFor({ timeout: 10_000 });
		check(true, 'dialog create invite kembali ke form saat dibuka ulang');
		await alice.getByRole('dialog').getByRole('button', { name: 'Close' }).click();
		await alice.getByRole('dialog').waitFor({ state: 'detached', timeout: 10_000 });

		// Regression: navigate to Projects from the profile menu without clicking the logo.
		await alice.getByRole('button', { name: 'Account menu' }).click();
		await alice.getByRole('menuitem', { name: 'Projects' }).click();
		await alice.waitForURL('**/projects', { timeout: 10_000 });
		check(true, 'dropdown profil punya navigasi ke Projects');
		check(
			(await alice.locator('a[aria-label="Curtain — Projects"] svg').count()) === 1,
			'logo brand tampil di header'
		);

		section('Bob daftar lewat invite link');
		const bobContext = await browser.newContext();
		const bob = await bobContext.newPage();
		watch(bob, 'bob');
		await registerViaInvite(bob, bobEmail, inviteLink);
		await bob.getByText('No projects yet').waitFor({ timeout: 10_000 });
		check(true, 'bob terdaftar via invite & belum punya project');

		await bob.locator('[aria-label="Account menu"]').click();
		await bob.getByRole('menuitem', { name: 'Lock vault' }).waitFor({ timeout: 10_000 });
		check(
			(await bob.getByRole('menuitem', { name: 'Settings' }).count()) === 0,
			'bob (member) tidak melihat menu Settings'
		);
		await bob.keyboard.press('Escape');

		section('Invite bob ke project (project key di-wrap ulang)');
		await alice.goto(projectUrl);
		await alice.getByRole('button', { name: /Invite member/ }).click();
		await alice.getByLabel('Member email').fill(bobEmail);
		await alice
			.getByRole('dialog')
			.locator('[data-slot="select-trigger"]')
			.click();
		await alice.getByRole('option', { name: 'Viewer' }).click();
		await alice.getByRole('button', { name: 'Invite', exact: true }).click();
		await alice.getByRole('tab', { name: 'Members' }).click();
		await alice.getByRole('cell', { name: bobEmail, exact: true }).waitFor({ timeout: 15_000 });
		await alice.getByText('you', { exact: true }).waitFor({ timeout: 10_000 });
		check(true, 'bob di-invite sebagai viewer dan muncul di daftar anggota');

		await alice.getByRole('button', { name: /Invite member/ }).click();
		await alice.getByRole('dialog').waitFor({ timeout: 10_000 });
		check(true, 'dialog invite bisa dibuka lagi setelah sukses');
		await alice.getByRole('dialog').getByRole('button', { name: 'Close' }).click();
		await alice.getByRole('dialog').waitFor({ state: 'detached', timeout: 10_000 });

		section('Bob membuka project & mendekripsi variable');
		await bob.reload();
		await bob.getByRole('link', { name: 'Open project' }).first().click();
		await bob.waitForURL(/\/projects\/\d+$/);
		await bob.getByText(variableName).waitFor({ timeout: 15_000 });
		await bob.getByRole('button', { name: `Actions for ${variableName}` }).click();
		await bob.getByRole('menuitem', { name: 'Show' }).click();
		await bob.getByText(variableValue).waitFor({ timeout: 10_000 });
		check(true, 'bob berhasil mendekripsi variable memakai project key hasil invite');
		check(
			(await bob.getByRole('button', { name: 'Project actions' }).count()) === 0,
			'bob (viewer) tidak melihat aksi hapus project'
		);
		check(
			(await bob.getByRole('button', { name: 'New variable', exact: true }).count()) === 0,
			'viewer tidak melihat tombol New variable'
		);
		check(
			(await bob.getByRole('button', { name: 'Environment actions' }).count()) === 0,
			'viewer tidak melihat aksi hapus environment'
		);
		check(
			(await bob.getByRole('button', { name: 'Environment', exact: true }).count()) === 0,
			'viewer tidak melihat tombol Create environment'
		);
		check(
			(await bob.getByRole('button', { name: /Invite member/ }).count()) === 0,
			'viewer tidak melihat tombol Invite member'
		);

		section('Owner mengubah role & permissions (Fase 2)');
		await alice.getByRole('tab', { name: 'Members' }).click();
		await alice.getByRole('button', { name: `Actions for ${bobEmail}` }).click();
		await alice.getByRole('menuitem', { name: 'Change role…' }).click();
		await alice
			.getByRole('dialog')
			.locator('[data-slot="select-trigger"]')
			.click();
		await alice.getByRole('option', { name: 'Editor' }).click();
		await alice.getByRole('button', { name: 'Save role' }).click();
		await alice.getByText('1 override').waitFor({ state: 'detached', timeout: 5_000 }).catch(() => {});
		check(true, 'owner mengubah role bob menjadi editor');

		await alice.getByRole('button', { name: `Actions for ${bobEmail}` }).click();
		await alice.getByRole('menuitem', { name: 'Permissions…' }).click();
		const permissionsDialog = alice.getByRole('dialog');
		await permissionsDialog.getByRole('group', { name: 'Delete variables' }).getByRole('radio', { name: 'Deny' }).click();
		await permissionsDialog.getByRole('button', { name: 'Save permissions' }).click();
		await alice.getByText('1 override').waitFor({ timeout: 10_000 });
		check(true, 'override deny var.delete tersimpan untuk bob');

		await alice.getByRole('button', { name: `Actions for ${bobEmail}` }).click();
		await alice.getByRole('menuitem', { name: 'Permissions…' }).click();
		await alice.getByRole('dialog').waitFor({ timeout: 10_000 });
		check(true, 'dialog permissions bisa dibuka lagi setelah sukses');
		await alice.getByRole('dialog').getByRole('button', { name: 'Close' }).click();
		await alice.getByRole('dialog').waitFor({ state: 'detached', timeout: 10_000 });

		section('Bob (editor dengan deny var.delete) di UI');
		await bob.reload();
		await bob.getByText(variableName).waitFor({ timeout: 15_000 });
		check(
			(await bob.getByRole('button', { name: 'New variable', exact: true }).count()) === 1,
			'editor melihat tombol New variable'
		);
		await bob.getByRole('button', { name: `Actions for ${variableName}` }).click();
		await bob.getByRole('menuitem', { name: 'Edit' }).waitFor({ timeout: 10_000 });
		check(
			(await bob.getByRole('menuitem', { name: 'Delete' }).count()) === 0,
			'override deny menyembunyikan aksi Delete untuk bob'
		);
		await bob.keyboard.press('Escape');

		section('Logout & login ulang (KDF salt dari server)');
		await bob.getByRole('button', { name: 'Account menu' }).click();
		await bob.getByRole('menuitem', { name: 'Sign out' }).click();
		await bob.waitForURL('**/login', { timeout: 15_000 });
		check(true, 'logout mengembalikan ke halaman login');

		await login(bob, bobEmail);
		await bob.getByRole('link', { name: 'Open project' }).first().click();
		await bob.getByText(variableName).waitFor({ timeout: 15_000 });
		check(true, 'login ulang (salt KDF dari server) berhasil membuka vault');

		section('Owner menghapus project lewat UI');
		await alice.goto(projectUrl);
		await alice.getByRole('button', { name: 'Project actions' }).click();
		await alice.getByRole('menuitem', { name: 'Delete project' }).click();
		await alice.getByRole('button', { name: 'Delete project', exact: true }).click();
		await alice.waitForURL('**/projects', { timeout: 15_000 });
		await alice.getByText('No projects yet').waitFor({ timeout: 10_000 });
		check(true, 'project terhapus dan daftar project kosong');

		await aliceContext.close();
		await bobContext.close();
	} finally {
		await browser.close();
	}

	if (browserErrors.length > 0) {
		console.error('\nError di browser:');
		for (const error of browserErrors) console.error(' -', error);
		failed += browserErrors.length;
	}

	console.log(`\n${passed} passed, ${failed} failed`);
	if (failed > 0) process.exit(1);
}

main().catch((error) => {
	console.error(error);
	process.exit(1);
});
