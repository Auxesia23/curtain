# Curtain

Curtain is a small end-to-end encrypted environment variable manager for teams.
The server stores encrypted data, but it does not receive the keys needed to read it.

I built this as a learning project inspired by Infisical. I wanted to understand
how end-to-end encryption works in a real application, especially how symmetric
and asymmetric encryption can work together. Curtain is not a fork of Infisical
or a feature-for-feature replacement.

The name comes from the curtain released by Sefirah Castle in the *Lord of the
Mysteries* novel, also known as LOTM. I liked the image of a curtain keeping
things hidden, so it felt right for a project about secrets.

I describe Curtain as E2EE with a zero-knowledge server design. Here, zero
knowledge means the server does not have the decryption keys for user private
keys, project keys, or environment variables. It is not a zero-knowledge proof
protocol.

- **Backend**: Go, chi, GORM, SQLite, and Casbin
- **Frontend**: SvelteKit SPA embedded in the Go server binary
- **Web cryptography**: WebCrypto and Argon2id in the browser
- **CLI**: decrypts variables locally, then injects them into a child process

## How the encryption works

The short version is that Curtain uses symmetric encryption for the data and
asymmetric encryption to share the symmetric project key with each member.

1. **A password derives two separate keys.** The client runs Argon2id on the
   password to get a master key. HKDF-SHA256 derives an `authKey` and a
   `wrapKey` from that master key. Different HKDF info strings keep their
   purposes separate.
2. **The server gets an authentication value, not the password.** The client
   sends `authKey` over HTTPS. The server hashes it with Argon2id for login
   verification. The server never gets the master key or `wrapKey`.
3. **The private key is encrypted on the client.** The client creates an RSA
   keypair. It encrypts the private key with AES-256-GCM using `wrapKey`, then
   stores the encrypted private key on the server. After login, the client
   derives `wrapKey` again and decrypts the private key locally.
4. **Each project has a random symmetric key.** The creator's client generates
   an AES-256 project key. For each member, that key is encrypted with that
   member's public RSA key. Each member therefore has their own encrypted copy
   of the project key in the database. The server cannot unwrap any of them.
5. **Variable names and values are encrypted with the project key.** The
   browser encrypts them with AES-256-GCM before sending them to the server.
   The CLI follows the same flow and decrypts locally when running a command.

This is the part I wanted to learn most: AES is good for encrypting the actual
data, while RSA lets the client share the AES project key without sending it in
plaintext.

## What this MVP does not protect

Curtain is intentionally simple, and its current access model has important
limits:

- Every active project member can read every environment in that project.
  `viewer` means read-only, not hidden from production secrets. Roles and
  Casbin capabilities control writes and management actions, not cryptographic
  read access.
- Project names, environment names, email addresses, membership details, and
  authorization policies are metadata visible to the server. Variable names and
  values are encrypted.
- Removing a member stops future API access, but it cannot erase a project key
  or ciphertext that member already copied. Curtain does not rotate project
  keys yet.
- There is no built-in password reset or key recovery. If a user loses their
  password, they cannot decrypt their existing private key. They may need a
  new account and fresh project invitations. A project with no other owner may
  be lost.
- There is no audit log or variable history yet.
- The server uses one SQLite database and is designed to run as one instance.
  Back up the database and test restoring it before using Curtain for important
  secrets.
- The first person to register on an empty instance becomes its admin. Do not
  expose a new instance publicly before completing first-run setup.

For now, Curtain fits a small team where everyone in a project is trusted with
all of that project's environments. Per-environment read isolation and project
key rotation are future work.

## Run Curtain

Start the Go API and the SvelteKit development server in separate terminals:

```bash
make dev-api
make dev-ui
```

The API listens on `:8080`; the UI dev server listens on `:5173` and proxies `/api` to the API.

Build and run the single server binary (the UI is embedded):

```bash
make build
./bin/curtain-server
```

The server creates `./curtain.db` and applies migrations automatically. Runtime configuration uses command-line flags only:

| Flag | Default | Description |
| --- | --- | --- |
| `--addr` | `:8080` | HTTP listen address |
| `--db` | `./curtain.db` | SQLite database file path |
| `--cookie-secure` | `auto` | Session cookie `Secure` attribute: `auto`, `true`, or `false` |
| `--version` | | Print the server version and exit |

For example:

```bash
./bin/curtain-server --addr :8099 --db /tmp/curtain.db --cookie-secure true
```

SQLite uses a pure-Go driver (modernc), so the server can be built with `CGO_ENABLED=0`. The application uses SQLite only and does not read runtime configuration from environment variables.

> There is no data migration for older project-permission schemas. Start with a fresh database; delete the selected SQLite file if it contains old Curtain data.

## CLI (`curtain run`)

`bin/curtain` is the client CLI. It fetches encrypted variables, decrypts them locally, and injects them into a child process. Project, environment, and variable management remains in the web UI.

```bash
make build-cli
# Or build directly:
CGO_ENABLED=0 go build -o bin/curtain ./cmd/cli

./bin/curtain login --server https://vault.example.com
cd ~/proj/app
./bin/curtain init
./bin/curtain run -- npm run dev
```

- `login` derives keys locally with Argon2id and stores the **CLI token + wrapKey** in the OS keyring (Secret Service, macOS Keychain, or Windows Credential Manager). The password is never stored. CLI sessions last 30 days with sliding expiration.
- `init` writes `.curtain.json` (`project_id` + `environment`, no secrets; safe to commit). `run` searches parent directories for this file.
- `run` fetches the latest variables on every execution, merges them into the current process environment (vault values take precedence), and replaces itself with the requested command. Exit codes and signals are preserved. Secrets are kept in memory and the child process environment, never written to disk.
- `curtain logout` revokes the server session and removes local credentials.

`run` accepts `--env`/`-e <name>` and `--project`/`-p <name|id>` to override `.curtain.json`, for example `curtain run -e staging -- npm run dev`. Run `curtain -h` or `curtain run -h` for all options.

## First-run setup and invitations

- **First run**: when an instance has no users, the first person to open the web UI can create an account and becomes the instance admin. The server logs a warning while there are no users.
- **Registration is then invite-only**: an admin creates an invitation in **Settings → Invites**. Links can be reused up to a configured limit, expire, and can be revoked. The raw token is shown only once; SQLite stores its SHA-256 hash.
- **Instance admins** can create/revoke invitations, list users, disable/enable accounts (which revokes their sessions), and delete accounts. A project is deleted with its data if the removed user was its only owner. Project keys previously seen by that user are not rotated.
- Onboarding flow: an admin creates an invitation → sends it to a teammate → the teammate opens `/invite/<token>` and registers → a project owner invites them to a project.

## Project roles and capabilities

Each project member has a **role** (default capabilities) and may receive per-capability overrides. Project membership implies read access because every member holds the project key; capabilities govern write and management actions.

| Capability | owner | editor | viewer |
| --- | :-: | :-: | :-: |
| `env.create` / `env.delete` | ✓ | ✓ | – |
| `var.create` / `var.update` / `var.delete` | ✓ | ✓ | – |
| `member.invite` | ✓ | – | – |
| `member.manage` | ✓ | – | – |
| `project.delete` | ✓ | – | – |

- Per-member overrides can `allow` or `deny` a capability. Deny takes precedence over allow and the role template.
- The last project owner cannot be demoted or removed (409).
- Inviting someone as an `owner` requires `member.manage`, not just `member.invite`.
- **Instance admins must still be project members** to access project routes. For a member who is also an instance admin, management capabilities (`member.*`, `project.delete`) bypass project capability policy; content capabilities do not. An admin who is not a member cannot read project data or manage that project through its project routes.
- The UI hides actions the current user cannot perform, but the server remains authoritative.

Casbin's GORM adapter stores role policies, project-scoped role bindings, and member overrides in `casbin_rule` within the same SQLite file. Policies load at startup and changes are persisted automatically. `project_user_permissions` is no longer used; an old copy is dropped rather than migrated. `project_users` remains the membership, wrapped-key, and displayed-role table, while Casbin enforces project access.

## Authentication

There are two client types, using the same session mechanism:

- **Web**: the session token is sent in the `curtain_session` cookie (`HttpOnly`, `SameSite=Lax`, `Secure` automatically enabled for HTTPS). The raw token is never exposed to JavaScript.
- **CLI** (`bin/curtain`): `POST /api/auth/login` with `"client": "cli"` returns a token in the JSON body, used as `Authorization: Bearer <token>`.

Sessions are opaque, not JWTs. SQLite stores `SHA-256(token)`. Web sessions last 7 days and CLI sessions 30 days; both use sliding expiration and can be revoked via `POST /api/auth/logout`.

## Cryptography specification

All encrypted bytes are sent as standard padded base64. `||` means byte concatenation.

### 1. Key derivation (client)

```text
masterKey  = Argon2id(password_utf8, salt=KDF salt, m=65536 KiB, t=3, p=1, out=32 bytes)
authKey    = HKDF-SHA256(ikm=masterKey, salt="curtain.kdf.v1", info="auth-key-v1", len=32)
wrapKey    = HKDF-SHA256(ikm=masterKey, salt="curtain.kdf.v1", info="private-key-wrap-v1", len=32)
```

- `authKey` (base64) is the only password-derived value sent to the server during registration/login. The server stores `Argon2id_serverside(authKey)` in PHC format.
- `wrapKey` never leaves the client. It encrypts and decrypts the private key. Domain separation ensures the server cannot unlock the vault even if it has `authKey`.

### 2. User identity

```text
keypair                 = RSA-OAEP 2048, SHA-256, e=65537 (wraps project keys)
public_key              = base64(SPKI DER)
private_key             = base64(PKCS#8 DER)
encrypted_private_key   = base64(iv(12) || AES-256-GCM(wrapKey, iv, private_key_pkcs8, aad="curtain.private-key.v1"))
```

At registration, the client sends `email`, `auth_key`, `kdf {algorithm, salt, params}`, `public_key`, and `encrypted_private_key`. At login, the client derives `wrapKey` again using the same salt (from `GET /api/auth/kdf?email=`), decrypts the private key, imports it as a non-extractable `CryptoKey`, and stores it in IndexedDB. JavaScript cannot export the private key afterward.

### 3. Project key

```text
project_key            = random AES-256-GCM key generated by the browser
encrypted_project_key = base64(RSA-OAEP(recipient_public_key, raw project_key))
```

The server stores one encrypted copy per member in `project_users.encrypted_project_key`.

- **Create project**: generate a project key and wrap it with the creator's public key.
- **Invite member**: decrypt the inviter's wrapped project key locally, wrap it with the invitee's public key (`GET /api/users/lookup?email=`), then submit it to `POST /api/projects/{id}/invite`.

### 4. Variables

```text
encrypted_key   = base64(iv(12) || AES-256-GCM(project_key, iv, variable_name_utf8, aad="curtain.variable.v1"))
encrypted_value = base64(iv(12) || AES-256-GCM(project_key, iv, variable_value_utf8, aad="curtain.variable.v1"))
```

Variable names are encrypted too; the server stores only ciphertext.

## API endpoints

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| GET | `/api/instance` | public | `{ needs_setup }` for first-run setup |
| POST | `/api/auth/register` | public (rate-limited) | Requires `invite_token` except during first-run setup |
| POST | `/api/auth/login` | public (rate-limited) | `client: "web"` (cookie) or `"cli"` (token) |
| GET | `/api/auth/kdf?email=` | public (rate-limited) | User KDF salt and parameters |
| POST | `/api/auth/logout` | session | Revoke the session and clear the cookie |
| GET | `/api/auth/me` | session | Profile and `encrypted_private_key` |
| GET | `/api/invites/{token}` | public (rate-limited) | Validate an invitation |
| GET/POST | `/api/admin/invites` | admin | List/create invitations (raw token returned once) |
| DELETE | `/api/admin/invites/{id}` | admin | Revoke an invitation |
| GET | `/api/admin/users` | admin | List users and account status |
| POST | `/api/admin/users/{id}/disable` or `/enable` | admin | Suspend/reactivate an account |
| DELETE | `/api/admin/users/{id}` | admin | Delete an account |
| GET | `/api/users/lookup?email=` | session | User ID and public key for project invitations |
| GET/POST | `/api/projects` | session | List/create projects |
| GET | `/api/projects/{id}` | member | Project details and the caller's `encrypted_project_key` |
| GET | `/api/projects/{id}/members` | member | Members, roles, overrides, and effective capabilities |
| POST | `/api/projects/{id}/invite` | `member.invite` | Add a member with their wrapped project key |
| PATCH | `/api/projects/{id}/members/{userID}` | `member.manage` | Change a member's role |
| PUT | `/api/projects/{id}/members/{userID}/permissions` | `member.manage` | Replace capability overrides |
| DELETE | `/api/projects/{id}/members/{userID}` | `member.manage` | Remove a member |
| DELETE | `/api/projects/{id}` | `project.delete` | Delete the project and its environments/variables/memberships |
| GET | `/api/permissions/catalog` | session | Capability catalog and role templates |
| GET/POST | `/api/projects/{id}/environments` | member / `env.create` | List/create environments |
| GET | `/api/environments/{id}/variables` | member | List opaque variables |
| POST | `/api/environments/{id}/variables` | `var.create` | Create an opaque variable |
| PUT | `/api/variables/{id}` | `var.update` | Update a variable |
| DELETE | `/api/variables/{id}` | `var.delete` | Delete a variable |
| DELETE | `/api/environments/{id}` | `env.delete` | Delete an environment |
| GET | `/health` | public | Health check |
| GET | `/*` | public | SPA fallback to `index.html` |

Additional protections: per-IP authentication rate limiting (20 requests/minute, burst 20) and CSRF protection (unsafe cookie-authenticated requests must be same-origin).

## Operations and security notes

- **HTTPS is required in production.** `crypto.subtle` (WebCrypto) is available only in a secure context: HTTPS or `http://localhost`. Browsers will not expose the cryptographic API if the binary is accessed over plain HTTP via a LAN IP. Put the server behind a TLS reverse proxy. If that proxy does not forward `X-Forwarded-Proto: https`, use `--cookie-secure true`.
- **Passwords cannot be reset.** The server stores the KDF salt and encrypted private key, but the keys cannot be derived without the password. Keep the password safe (or plan a recovery-key feature).
- **`GET /api/auth/kdf` returns 404 for unknown emails**, a deliberate account-enumeration trade-off in the pre-login flow; rate limiting reduces abuse.
- **A CLI token is not data access by itself.** The CLI still derives keys and decrypts variables locally. Go implementation: `internal/cli`; cross-language reference: `ui/scripts/e2e.mjs`; test vectors: `internal/cli/testdata/vectors.json`, generated by `ui/scripts/cli-vectors.mjs`. Run `go test ./internal/cli` to verify byte-level compatibility.
- **CLI credentials use the OS keyring.** The session token and `wrapKey` (equivalent to the private key) are stored through Secret Service, Keychain, or Credential Manager. On headless Linux without Secret Service/KWallet, `curtain login` explains the issue; run it from a desktop session.

## Directory structure

```text
ui.go                    Embedded SPA and HTTP handler (root package `curtain`)
cmd/api                  Server entry point (`bin/curtain-server`)
cmd/cli                  CLI entry point (`bin/curtain`)
internal/cli              CLI crypto, OS keyring, HTTP client, and commands
internal/crypto           Argon2id PHC hashing for auth keys
internal/token            Opaque session token generation and hashing
internal/authz            Capability catalog, Casbin model, and GORM adapter setup
internal/models           User, Session, Project, ProjectUser, Environment, Variable
internal/dto              API request and response types
internal/services         Business logic
internal/middleware       Authentication, authorization, CSRF, and rate limiting
internal/routes           chi routing
ui/                       SvelteKit SPA + shadcn-svelte
ui/scripts/e2e.mjs        API and cryptography end-to-end tests (`npm run e2e`)
ui/scripts/cli-vectors.mjs CLI test vector generator (`npm run vectors`)
```

## Testing

`make test` (or `go test ./...`) runs Go unit and integration tests against the real chi router using temporary SQLite databases. This includes cross-language crypto vectors and authorization tests.

The JavaScript end-to-end scripts require a running server with the latest embedded UI:

1. **`npm run e2e`**: API and crypto flow without a browser (Node WebCrypto + hash-wasm), including registration, login, project keys, invitations, encrypted variables, and project permissions.
2. **`npm run e2e:ui`**: complete browser flow with Playwright Chromium, including UI registration, IndexedDB vault, reveal variable, reload, invitation, and login. Install Chromium once with `npx playwright install chromium`.

```bash
# Terminal 1
make build
./bin/curtain-server --addr :8099 --db /tmp/curtain-e2e.db

# Terminal 2 (BASE is an optional test-script setting, not server configuration)
cd ui
BASE=http://localhost:8099 npm run e2e
BASE=http://localhost:8099 npm run e2e:ui
```
