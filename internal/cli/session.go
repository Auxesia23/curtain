package cli

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// loadSession reads global config and keyring credentials, then creates an API
// client with the bearer token.
func loadSession() (*fileConfig, *credentialBlob, *apiClient, error) {
	cfg, err := loadFileConfig()
	if err != nil {
		return nil, nil, nil, err
	}
	if cfg == nil || cfg.Server == "" {
		return nil, nil, nil, errors.New("not logged in: run `curtain login` first")
	}
	creds, err := loadCredentials(cfg.Server)
	if err != nil {
		return nil, nil, nil, err
	}
	if creds == nil {
		return nil, nil, nil, errors.New("credentials not found in the OS keyring: run `curtain login`")
	}
	client := newAPIClient(cfg.Server)
	client.token = creds.Token
	return cfg, creds, client, nil
}

// sessionError wraps API errors and suggests logging in again for 401 responses.
func sessionError(action string, err error) error {
	if apiStatus(err) == 401 {
		return fmt.Errorf("%s: CLI session is no longer valid — run `curtain login`", action)
	}
	return fmt.Errorf("%s: %w", action, err)
}

// fetchedEnv contains decrypted variables and their project/environment context.
type fetchedEnv struct {
	ProjectID   uint
	ProjectName string
	EnvID       uint
	EnvName     string
	Vars        map[string]string
	Warnings    []string
}

// fetchEnv runs the complete read flow: fetch the profile, open the private key,
// find the project, unwrap its key, find the environment, and decrypt variables.
// projectRef can be a numeric ID or project name.
func fetchEnv(client *apiClient, creds *credentialBlob, projectRef, envName string) (*fetchedEnv, error) {
	client.token = creds.Token

	profile, err := client.me()
	if err != nil {
		return nil, sessionError("fetching profile", err)
	}

	wrapKey, err := b64Decode(creds.WrapKey)
	if err != nil {
		return nil, fmt.Errorf("invalid wrap_key in the keyring: %w (try `curtain login` again)", err)
	}
	privateKey, err := decryptPrivateKey(wrapKey, profile.EncryptedPrivateKey)
	if err != nil {
		return nil, err
	}

	projects, err := client.listProjects()
	if err != nil {
		return nil, sessionError("fetching projects", err)
	}
	project, err := resolveProject(projects, projectRef)
	if err != nil {
		return nil, err
	}

	projectKey, err := unwrapProjectKey(privateKey, project.EncryptedProjectKey)
	if err != nil {
		return nil, err
	}

	envs, err := client.listEnvironments(project.ID)
	if err != nil {
		return nil, sessionError("fetching environments", err)
	}
	env, err := resolveEnvironment(envs, envName)
	if err != nil {
		return nil, err
	}

	rows, err := client.listVariables(env.ID)
	if err != nil {
		return nil, sessionError("fetching variables", err)
	}
	vars, warnings := decryptAll(projectKey, rows)

	return &fetchedEnv{
		ProjectID:   project.ID,
		ProjectName: project.Name,
		EnvID:       env.ID,
		EnvName:     env.Name,
		Vars:        vars,
		Warnings:    warnings,
	}, nil
}

// resolveProject finds a project by numeric ID or unique name.
func resolveProject(projects []projectInfo, ref string) (projectInfo, error) {
	if len(projects) == 0 {
		return projectInfo{}, errors.New("no accessible projects — create one via the web UI first")
	}
	if id, err := strconv.ParseUint(ref, 10, 64); err == nil {
		for _, p := range projects {
			if p.ID == uint(id) {
				return p, nil
			}
		}
		return projectInfo{}, fmt.Errorf(
			"project id %d not found (it may have been deleted or you are no longer a member); accessible: %s",
			id, describeProjects(projects),
		)
	}

	var matches []projectInfo
	for _, p := range projects {
		if p.Name == ref {
			matches = append(matches, p)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return projectInfo{}, fmt.Errorf("project %q not found; accessible: %s", ref, describeProjects(projects))
	default:
		ids := make([]string, 0, len(matches))
		for _, p := range matches {
			ids = append(ids, strconv.FormatUint(uint64(p.ID), 10))
		}
		return projectInfo{}, fmt.Errorf("multiple projects named %q — pick one with --project <id>: %s", ref, strings.Join(ids, ", "))
	}
}

func describeProjects(projects []projectInfo) string {
	parts := make([]string, 0, len(projects))
	for _, p := range projects {
		parts = append(parts, fmt.Sprintf("%s (id %d)", p.Name, p.ID))
	}
	return strings.Join(parts, ", ")
}

// resolveEnvironment finds an environment by name, which is unique per project.
func resolveEnvironment(envs []environmentInfo, name string) (environmentInfo, error) {
	if len(envs) == 0 {
		return environmentInfo{}, errors.New("this project has no environments yet — create one via the web UI first")
	}
	for _, e := range envs {
		if e.Name == name {
			return e, nil
		}
	}
	names := make([]string, 0, len(envs))
	for _, e := range envs {
		names = append(names, e.Name)
	}
	return environmentInfo{}, fmt.Errorf("environment %q not found; available: %s", name, strings.Join(names, ", "))
}

// decryptAll decrypts every variable. Invalid names or rows that fail to decrypt
// are skipped with a warning and do not abort the run.
func decryptAll(projectKey []byte, rows []variableInfo) (map[string]string, []string) {
	vars := make(map[string]string, len(rows))
	var warnings []string

	for _, row := range rows {
		name, err := decryptVariable(projectKey, row.EncryptedKey)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skipping variable #%d: %v", row.ID, err))
			continue
		}
		value, err := decryptVariable(projectKey, row.EncryptedValue)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skipping variable %q: %v", name, err))
			continue
		}
		if name == "" || strings.ContainsAny(name, "=\x00") {
			warnings = append(warnings, fmt.Sprintf("skipping variable with invalid name: %q", name))
			continue
		}
		if strings.ContainsRune(value, 0) {
			warnings = append(warnings, fmt.Sprintf("skipping variable %q: value contains NUL", name))
			continue
		}
		if _, dup := vars[name]; dup {
			warnings = append(warnings, fmt.Sprintf("duplicate variable %q — keeping the last value", name))
		}
		vars[name] = value
	}
	return vars, warnings
}

// buildEnv merges vault values into the process environment. Vault values take
// precedence over existing variables with the same name.
func buildEnv(base []string, overrides map[string]string) []string {
	keys := make([]string, 0, len(overrides))
	for k := range overrides {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	index := make(map[string]int, len(base))
	for i, kv := range base {
		if eq := strings.IndexByte(kv, '='); eq > 0 {
			index[kv[:eq]] = i
		}
	}

	out := base
	for _, k := range keys {
		if i, ok := index[k]; ok {
			out[i] = k + "=" + overrides[k]
			continue
		}
		index[k] = len(out)
		out = append(out, k+"="+overrides[k])
	}
	return out
}
