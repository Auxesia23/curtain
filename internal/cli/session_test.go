package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"curtain/internal/authz"
	"curtain/internal/database"
	"curtain/internal/handlers"
	"curtain/internal/middleware"
	"curtain/internal/routes"
	"curtain/internal/services"
)

// testInstance runs the real Curtain server in-process with a temporary SQLite
// database so the CLI can be tested end-to-end against the actual API.
type testInstance struct {
	server    *httptest.Server
	client    *apiClient
	creds     *credentialBlob
	email     string
	projectID uint
	envID     uint
}

func newTestInstance(t *testing.T) *testInstance {
	t.Helper()

	db, err := database.Connect(filepath.Join(t.TempDir(), "curtain-test.db"))
	if err != nil {
		t.Fatalf("database.Connect: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("database handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	enforcer, err := authz.NewEnforcer(db)
	if err != nil {
		t.Fatalf("authz.NewEnforcer: %v", err)
	}

	router := routes.NewRouter(
		handlers.NewAuthHandler(services.NewAuthService(db), handlers.CookieSecureAuto),
		handlers.NewInviteHandler(services.NewInviteService(db)),
		handlers.NewUserHandler(services.NewUserService(db, enforcer)),
		handlers.NewProjectHandler(services.NewProjectService(db, enforcer)),
		handlers.NewEnvironmentHandler(services.NewEnvironmentService(db)),
		handlers.NewVariableHandler(services.NewVariableService(db)),
		handlers.NewPermissionHandler(),
		middleware.NewAuthorize(db, enforcer),
		middleware.NewAuthenticator(db),
		nil,
	)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)

	inst := &testInstance{
		server: ts,
		client: newAPIClient(ts.URL),
		email:  "cli-test@example.com",
	}
	inst.register(t)
	return inst
}

// register creates the first user (automatically an admin) using the client crypto flow.
func (inst *testInstance) register(t *testing.T) {
	t.Helper()

	vectors := loadVectors(t)

	var res authResponse
	err := inst.client.do(http.MethodPost, "/api/auth/register", map[string]any{
		"email":                 inst.email,
		"auth_key":              vectors.AuthKey,
		"kdf":                   vectors.KDF,
		"public_key":            vectors.PublicKey,
		"encrypted_private_key": vectors.EncryptedPrivateKey,
		"client":                "cli",
	}, &res)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if res.Token == "" {
		t.Fatal("register tidak mengembalikan token CLI")
	}
	inst.client.token = res.Token
	inst.creds = &credentialBlob{Email: inst.email, Token: res.Token, WrapKey: vectors.WrapKey}
}

// seedProject creates a project and development environment with encrypted test vectors.
func (inst *testInstance) seedProject(t *testing.T) {
	t.Helper()

	vectors := loadVectors(t)

	var project projectInfo
	err := inst.client.do(http.MethodPost, "/api/projects", map[string]any{
		"name":                  "cli-app",
		"encrypted_project_key": vectors.WrappedProjectKey,
	}, &project)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	inst.projectID = project.ID

	var env environmentInfo
	err = inst.client.do(http.MethodPost, fmt.Sprintf("/api/projects/%d/environments", project.ID), map[string]any{
		"name": "development",
	}, &env)
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}
	inst.envID = env.ID

	for _, variable := range vectors.Variables {
		var created variableInfo
		err = inst.client.do(http.MethodPost, fmt.Sprintf("/api/environments/%d/variables", env.ID), map[string]any{
			"encrypted_key":   variable.EncryptedKey,
			"encrypted_value": variable.EncryptedValue,
		}, &created)
		if err != nil {
			t.Fatalf("create variable %q: %v", variable.Key, err)
		}
	}
}

func TestFetchEnvEndToEnd(t *testing.T) {
	inst := newTestInstance(t)
	vectors := loadVectors(t)
	expected := make(map[string]string, len(vectors.Variables))
	for _, variable := range vectors.Variables {
		expected[variable.Key] = variable.Value
	}
	inst.seedProject(t)

	// Resolve by project ID, as stored in .curtain.json.
	fetched, err := fetchEnv(inst.client, inst.creds, strconv.FormatUint(uint64(inst.projectID), 10), "development")
	if err != nil {
		t.Fatalf("fetchEnv by id: %v", err)
	}
	if fetched.ProjectName != "cli-app" || fetched.EnvName != "development" {
		t.Fatalf("konteks tidak sesuai: %+v", fetched)
	}
	if len(fetched.Warnings) != 0 {
		t.Fatalf("tidak boleh ada warning: %v", fetched.Warnings)
	}
	if len(fetched.Vars) != len(expected) {
		t.Fatalf("jumlah variable: got %d want %d", len(fetched.Vars), len(expected))
	}
	for name, value := range expected {
		if got, ok := fetched.Vars[name]; !ok || got != value {
			t.Fatalf("variable %q: got %q ok=%v want %q", name, got, ok, value)
		}
	}

	// Resolve by project name.
	fetched, err = fetchEnv(inst.client, inst.creds, "cli-app", "development")
	if err != nil {
		t.Fatalf("fetchEnv by name: %v", err)
	}
	if fetched.ProjectID != inst.projectID {
		t.Fatalf("project id: got %d want %d", fetched.ProjectID, inst.projectID)
	}
}

func TestFetchEnvErrors(t *testing.T) {
	inst := newTestInstance(t)
	inst.seedProject(t)
	ref := strconv.FormatUint(uint64(inst.projectID), 10)

	if _, err := fetchEnv(inst.client, inst.creds, ref, "staging"); err == nil ||
		!strings.Contains(err.Error(), "environment") {
		t.Fatalf("environment tidak ada: err=%v", err)
	}
	if _, err := fetchEnv(inst.client, inst.creds, "999", "development"); err == nil ||
		!strings.Contains(err.Error(), "project id 999") {
		t.Fatalf("project tidak ada: err=%v", err)
	}

	expired := *inst.creds
	expired.Token = "token-palsu"
	if _, err := fetchEnv(inst.client, &expired, ref, "development"); err == nil ||
		!strings.Contains(err.Error(), "curtain login") {
		t.Fatalf("token invalid harus mengarahkan login ulang: err=%v", err)
	}
}

// TestLoginFlow checks the CLI login flow (KDF -> derive -> login) against the real API.
func TestLoginFlow(t *testing.T) {
	inst := newTestInstance(t)

	kdf, err := inst.client.kdf(inst.email)
	if err != nil {
		t.Fatalf("kdf: %v", err)
	}
	params, err := parseKDFParams(kdf.KDFParams)
	if err != nil {
		t.Fatalf("parseKDFParams: %v", err)
	}

	// Full login flow: password and server salt -> masterKey -> authKey.
	vectors := loadVectors(t)
	masterKey, err := deriveMasterKey(vectors.Password, kdf.KDFSalt, params)
	if err != nil {
		t.Fatalf("deriveMasterKey: %v", err)
	}
	defer wipeBytes(masterKey)
	authKey, err := deriveAuthKey(masterKey)
	if err != nil {
		t.Fatalf("deriveAuthKey: %v", err)
	}
	res, err := inst.client.login(inst.email, authKey)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.Token == "" {
		t.Fatal("login tidak mengembalikan token")
	}

	wrapKey, err := deriveWrapKey(masterKey)
	if err != nil {
		t.Fatalf("deriveWrapKey: %v", err)
	}
	// The keyring key must still decrypt the private key returned by login.
	if _, err := decryptPrivateKey(wrapKey, res.User.EncryptedPrivateKey); err != nil {
		t.Fatalf("decryptPrivateKey via login: %v", err)
	}

	if _, err := inst.client.login(inst.email, b64Encode(make([]byte, 32))); err == nil {
		t.Fatal("login dengan authKey salah seharusnya gagal")
	} else if apiStatus(err) != 401 {
		t.Fatalf("status login salah: %v", err)
	}
}

func TestDecryptAllFiltering(t *testing.T) {
	vectors := loadVectors(t)
	projectKey, err := b64Decode(vectors.ProjectKey)
	if err != nil {
		t.Fatalf("decode project key: %v", err)
	}
	first, second := vectors.Variables[0], vectors.Variables[1]

	rows := []variableInfo{
		{ID: 1, EncryptedKey: first.EncryptedKey, EncryptedValue: first.EncryptedValue},
		{ID: 2, EncryptedKey: first.EncryptedKey, EncryptedValue: second.EncryptedValue},
		{ID: 3, EncryptedKey: second.EncryptedKey, EncryptedValue: "not-base64-!!!"},
	}
	vars, warnings := decryptAll(projectKey, rows)

	if vars[first.Key] != second.Value {
		t.Fatalf("duplicate variable should keep the last value: %q", vars[first.Key])
	}
	if len(vars) != 1 {
		t.Fatalf("expected one valid variable: %v", vars)
	}
	if len(warnings) != 2 {
		t.Fatalf("expected two warnings: %v", warnings)
	}
}

func TestBuildEnvOverride(t *testing.T) {
	base := []string{"PATH=/bin", "FOO=lama", "BAR=tetap"}
	env := buildEnv(base, map[string]string{"FOO": "baru", "BAZ": "tambah"})

	want := map[string]string{"PATH": "/bin", "FOO": "baru", "BAR": "tetap", "BAZ": "tambah"}
	got := make(map[string]string, len(env))
	for _, kv := range env {
		name, value, _ := strings.Cut(kv, "=")
		got[name] = value
	}
	if len(got) != len(want) {
		t.Fatalf("jumlah env: got %v want %v", got, want)
	}
	for name, value := range want {
		if got[name] != value {
			t.Fatalf("env %q: got %q want %q", name, got[name], value)
		}
	}
}

func TestResolveProjectAmbiguous(t *testing.T) {
	projects := []projectInfo{
		{ID: 1, Name: "app"},
		{ID: 2, Name: "app"},
		{ID: 3, Name: "web"},
	}
	if _, err := resolveProject(projects, "app"); err == nil ||
		!strings.Contains(err.Error(), "pick one with --project") ||
		!strings.Contains(err.Error(), "1, 2") {
		t.Fatalf("nama ambigu harus meminta id: %v", err)
	}
	got, err := resolveProject(projects, "2")
	if err != nil || got.ID != 2 {
		t.Fatalf("resolve by id: got=%+v err=%v", got, err)
	}
	if _, err := resolveProject(projects, "web"); err != nil {
		t.Fatalf("resolve nama unik: %v", err)
	}
}
