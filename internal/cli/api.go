package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// apiClient is a small HTTP client for the endpoints used by the CLI.
// It has no dependency on internal server packages.
type apiClient struct {
	baseURL string
	http    *http.Client
	token   string // bearer token (client "cli")
}

func newAPIClient(baseURL string) *apiClient {
	return &apiClient{
		baseURL: normalizeServer(baseURL),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// apiError represents a JSON error response from the server.
type apiError struct {
	Status  int
	Message string
}

func (e *apiError) Error() string { return e.Message }

// apiStatus returns the HTTP status when err is an *apiError, or zero otherwise.
func apiStatus(err error) int {
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		return apiErr.Status
	}
	return 0
}

func (c *apiClient) do(method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach server %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return fmt.Errorf("failed to read server response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(data))
		var payload struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &payload) == nil && payload.Error != "" {
			message = payload.Error
		}
		if message == "" {
			message = resp.Status
		}
		return &apiError{Status: resp.StatusCode, Message: message}
	}

	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("unexpected server response: %w", err)
		}
	}
	return nil
}

// ---------------------------------------------------------------- API types

type kdfResponse struct {
	KDFAlgorithm string `json:"kdf_algo"`
	KDFSalt      string `json:"kdf_salt"`
	KDFParams    string `json:"kdf_params"`
}

type userProfile struct {
	ID                  uint   `json:"id"`
	Email               string `json:"email"`
	Role                string `json:"role"`
	KDFAlgorithm        string `json:"kdf_algo"`
	KDFSalt             string `json:"kdf_salt"`
	KDFParams           string `json:"kdf_params"`
	PublicKey           string `json:"public_key"`
	EncryptedPrivateKey string `json:"encrypted_private_key"`
}

type authResponse struct {
	Token string      `json:"token"`
	User  userProfile `json:"user"`
}

type projectInfo struct {
	ID                  uint   `json:"id"`
	Name                string `json:"name"`
	Role                string `json:"role"`
	EncryptedProjectKey string `json:"encrypted_project_key"`
}

// GORM serializes gorm.Model fields with capitalized names such as "ID", while
// Name has an explicit JSON tag.
type environmentInfo struct {
	ID   uint   `json:"ID"`
	Name string `json:"name"`
}

type variableInfo struct {
	ID             uint   `json:"ID"`
	EncryptedKey   string `json:"encrypted_key"`
	EncryptedValue string `json:"encrypted_value"`
}

// ---------------------------------------------------------------- endpoints

func (c *apiClient) kdf(email string) (kdfResponse, error) {
	var out kdfResponse
	err := c.do(http.MethodGet, "/api/auth/kdf?email="+url.QueryEscape(email), nil, &out)
	return out, err
}

func (c *apiClient) login(email, authKey string) (authResponse, error) {
	var out authResponse
	body := map[string]string{"email": email, "auth_key": authKey, "client": "cli"}
	err := c.do(http.MethodPost, "/api/auth/login", body, &out)
	return out, err
}

func (c *apiClient) me() (userProfile, error) {
	var out userProfile
	err := c.do(http.MethodGet, "/api/auth/me", nil, &out)
	return out, err
}

func (c *apiClient) logout() error {
	return c.do(http.MethodPost, "/api/auth/logout", nil, nil)
}

func (c *apiClient) listProjects() ([]projectInfo, error) {
	var out []projectInfo
	err := c.do(http.MethodGet, "/api/projects", nil, &out)
	return out, err
}

func (c *apiClient) listEnvironments(projectID uint) ([]environmentInfo, error) {
	var out []environmentInfo
	err := c.do(http.MethodGet, fmt.Sprintf("/api/projects/%d/environments", projectID), nil, &out)
	return out, err
}

func (c *apiClient) listVariables(envID uint) ([]variableInfo, error) {
	var out []variableInfo
	err := c.do(http.MethodGet, fmt.Sprintf("/api/environments/%d/variables", envID), nil, &out)
	return out, err
}
