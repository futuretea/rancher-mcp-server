//go:build integration

// Package integration exercises the built server against real Rancher
// deployments started with Docker.
//
// Run:
//
//	go test -tags=integration -timeout 90m ./test/integration/...
//
// Environment:
//
//	RANCHER_TEST_VERSIONS        comma-separated versions (default 2.13.3,2.14.3,2.15.1)
//	RANCHER_TEST_ADMIN_PASSWORD  bootstrap admin password for the throwaway container
//	RANCHER_TEST_KEEP            set to 1 to keep containers after the run
package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

const (
	defaultVersions      = "2.13.3,2.14.3,2.15.1"
	defaultAdminPassword = "RancherIntegration1!"
	testRedirectURI      = "http://localhost:9999/callback"
)

var serverBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "rancher-mcp-it")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create temp dir: %v\n", err)
		os.Exit(1)
	}
	serverBin = filepath.Join(dir, "rancher-mcp-server")
	build := exec.Command("go", "build", "-o", serverBin, "./cmd/rancher-mcp-server")
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build server binary: %v\n%s", err, out)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func adminPassword() string {
	if pw := os.Getenv("RANCHER_TEST_ADMIN_PASSWORD"); pw != "" {
		return pw
	}
	return defaultAdminPassword
}

func testVersions() []string {
	raw := os.Getenv("RANCHER_TEST_VERSIONS")
	if raw == "" {
		raw = defaultVersions
	}
	versions := []string{}
	for _, v := range strings.Split(raw, ",") {
		if v = strings.TrimSpace(v); v != "" {
			versions = append(versions, v)
		}
	}
	return versions
}

// supportsConfigurableScopes reports whether the version has OIDCClient.spec.scopes
// (added in Rancher 2.14 by rancher/rancher#53016).
func supportsConfigurableScopes(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) < 2 || parts[0] != "2" {
		return false
	}
	minor, err := strconv.Atoi(parts[1])
	return err == nil && minor >= 14
}

type rancherEnv struct {
	t            *testing.T
	version      string
	container    string
	baseURL      string
	adminToken   string
	apiToken     string
	clientID     string
	clientSecret string
	kubePort     int
}

func docker(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func startRancher(t *testing.T, version string, port int) *rancherEnv {
	t.Helper()
	kubePort := freePort(t)
	env := &rancherEnv{
		t:         t,
		version:   version,
		container: "rancher-it-" + strings.ReplaceAll(version, ".", "-"),
		baseURL:   fmt.Sprintf("https://localhost:%d", port),
		kubePort:  kubePort,
	}
	t.Cleanup(func() {
		if os.Getenv("RANCHER_TEST_KEEP") == "" {
			_ = exec.Command("docker", "rm", "-f", env.container).Run()
		}
	})

	_ = exec.Command("docker", "rm", "-f", env.container).Run()
	docker(t, "run", "-d", "--name", env.container, "--privileged", "--restart=unless-stopped",
		"-p", fmt.Sprintf("%d:443", port),
		"-p", fmt.Sprintf("%d:6443", kubePort),
		"-e", "CATTLE_BOOTSTRAP_PASSWORD="+adminPassword(),
		"rancher/rancher:v"+version)

	env.waitPing()
	env.login()
	env.setServerURL()
	env.ensureOIDCProvider()
	env.createOIDCClient()
	env.createAPIToken()
	return env
}

func (e *rancherEnv) waitPing() {
	e.t.Helper()
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		if status, body := e.request(http.MethodGet, "/ping", "", nil); status == http.StatusOK && strings.TrimSpace(string(body)) == "pong" {
			return
		}
		time.Sleep(5 * time.Second)
	}
	e.t.Fatalf("Rancher %s did not answer /ping at %s", e.version, e.baseURL)
}

func (e *rancherEnv) request(method, path, token string, body any) (int, []byte) {
	e.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			e.t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, e.baseURL+path, reader)
	if err != nil {
		e.t.Fatalf("build request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := insecureClient().Do(req)
	if err != nil {
		return 0, nil
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw
}

func (e *rancherEnv) mustRequest(method, path, token string, body any) []byte {
	e.t.Helper()
	status, raw := e.request(method, path, token, body)
	if status < 200 || status > 299 {
		e.t.Fatalf("%s %s returned %d: %s", method, path, status, raw)
	}
	return raw
}

func (e *rancherEnv) login() {
	e.t.Helper()
	raw := e.mustRequest(http.MethodPost, "/v3-public/localProviders/local?action=login", "", map[string]any{
		"username":     "admin",
		"password":     adminPassword(),
		"responseType": "json",
	})
	var parsed struct {
		Token              string `json:"token"`
		MustChangePassword bool   `json:"mustChangePassword"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		e.t.Fatalf("decode login response: %v", err)
	}
	if parsed.MustChangePassword {
		newPassword := adminPassword() + "-changed"
		e.mustRequest(http.MethodPost, "/v3/users/admin?action=setpassword", parsed.Token, map[string]any{"newPassword": newPassword})
		raw = e.mustRequest(http.MethodPost, "/v3-public/localProviders/local?action=login", "", map[string]any{
			"username":     "admin",
			"password":     newPassword,
			"responseType": "json",
		})
		if err := json.Unmarshal(raw, &parsed); err != nil {
			e.t.Fatalf("decode login response after password change: %v", err)
		}
	}
	if parsed.Token == "" {
		e.t.Fatal("Rancher login returned an empty token")
	}
	e.adminToken = parsed.Token
}

func (e *rancherEnv) setServerURL() {
	e.t.Helper()
	// Without server-url the issued tokens carry a relative "/oidc" issuer,
	// which no external verifier can match.
	e.mustRequest(http.MethodPut, "/v3/settings/server-url", e.adminToken, map[string]any{"value": e.baseURL})
}

func (e *rancherEnv) ensureOIDCProvider() {
	e.t.Helper()
	raw := e.mustRequest(http.MethodGet, "/v3/features/oidc-provider", e.adminToken, nil)
	var feature struct {
		Value bool `json:"value"`
	}
	if err := json.Unmarshal(raw, &feature); err != nil {
		e.t.Fatalf("decode oidc-provider feature: %v", err)
	}
	if feature.Value {
		return
	}
	e.mustRequest(http.MethodPut, "/v3/features/oidc-provider", e.adminToken, map[string]any{"value": true})
	// Enabling the flag stops Rancher; the container restart policy brings it back.
	time.Sleep(20 * time.Second)
	e.waitPing()
	e.login()
}

func (e *rancherEnv) createOIDCClient() {
	e.t.Helper()
	scopes := ""
	if supportsConfigurableScopes(e.version) {
		scopes = "\n  scopes:\n    - openid\n    - profile\n    - offline_access\n    - rancher:mcp"
	}
	manifest := fmt.Sprintf(`apiVersion: management.cattle.io/v3
kind: OIDCClient
metadata:
  name: mcp-integration
spec:
  description: "rancher-mcp-server integration test"
  redirectURIs:
    - %q
  tokenExpirationSeconds: 3600
  refreshTokenExpirationSeconds: 86400%s
`, testRedirectURI, scopes)

	cmd := exec.Command("docker", "exec", "-i", e.container, "kubectl",
		"--kubeconfig", "/etc/rancher/k3s/k3s.yaml", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	if out, err := cmd.CombinedOutput(); err != nil {
		e.t.Fatalf("create OIDCClient: %v\n%s", err, out)
	}
	time.Sleep(5 * time.Second)

	e.clientID = e.kubectl("get", "oidcclient", "mcp-integration", "-o", "jsonpath={.status.clientID}")
	if e.clientID == "" {
		e.t.Fatal("OIDCClient status has no clientID")
	}
	secret := e.kubectl("get", "secret", "-n", "cattle-oidc-client-secrets", e.clientID,
		"-o", "jsonpath={.data.client-secret-1}")
	decoded, err := base64.StdEncoding.DecodeString(secret)
	if err != nil || len(decoded) == 0 {
		e.t.Fatalf("decode OIDC client secret: %v", err)
	}
	e.clientSecret = string(decoded)
}

// kubeconfig writes a kubeconfig for the Rancher local cluster that points at
// the published k3s API port. The embedded certificate is not valid for the
// host port mapping, so verification is skipped in the test copy.
func (e *rancherEnv) kubeconfig() string {
	e.t.Helper()
	raw := docker(e.t, "exec", e.container, "cat", "/etc/rancher/k3s/k3s.yaml")

	kept := make([]string, 0, 32)
	for _, line := range strings.Split(raw, "\n") {
		if strings.Contains(line, "certificate-authority-data:") {
			continue
		}
		kept = append(kept, line)
	}
	raw = strings.Join(kept, "\n")
	raw = strings.Replace(raw, "https://127.0.0.1:6443", fmt.Sprintf("https://localhost:%d", e.kubePort), 1)
	raw = strings.Replace(raw, "server:", "insecure-skip-tls-verify: true\n    server:", 1)

	path := filepath.Join(e.t.TempDir(), "kubeconfig.yaml")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		e.t.Fatalf("write kubeconfig: %v", err)
	}
	return path
}

func (e *rancherEnv) kubectl(args ...string) string {
	e.t.Helper()
	full := append([]string{"exec", e.container, "kubectl", "--kubeconfig", "/etc/rancher/k3s/k3s.yaml"}, args...)
	out, err := exec.Command("docker", full...).CombinedOutput()
	if err != nil {
		e.t.Fatalf("kubectl %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func (e *rancherEnv) createAPIToken() {
	e.t.Helper()
	raw := e.mustRequest(http.MethodPost, "/v3/tokens", e.adminToken, map[string]any{
		"type":        "token",
		"description": "integration test",
	})
	var parsed struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		e.t.Fatalf("decode token response: %v", err)
	}
	if parsed.Token == "" {
		e.t.Fatal("Rancher API token creation returned an empty token")
	}
	e.apiToken = parsed.Token
}

// oidcAccessToken drives the authorization-code + PKCE flow without a browser.
// Rancher's /oidc/authorize accepts the Rancher API token in the Authorization header.
func (e *rancherEnv) oidcAccessToken(scopes string) string {
	e.t.Helper()
	verifier := randomURLSafe(48)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	authorizeURL := fmt.Sprintf("%s/oidc/authorize?response_type=code&client_id=%s&redirect_uri=%s&scope=%s&code_challenge=%s&code_challenge_method=S256&state=integration",
		e.baseURL, e.clientID, urlQueryEscape(testRedirectURI), urlQueryEscape(scopes), challenge)

	req, err := http.NewRequest(http.MethodGet, authorizeURL, nil)
	if err != nil {
		e.t.Fatalf("build authorize request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.apiToken)
	client := insecureClient()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		e.t.Fatalf("authorize request: %v", err)
	}
	defer resp.Body.Close()
	location := resp.Header.Get("Location")
	code := queryParam(location, "code")
	if code == "" {
		body, _ := io.ReadAll(resp.Body)
		e.t.Fatalf("authorize did not return a code (status %d, location %q, body %s)", resp.StatusCode, location, body)
	}

	form := url.Values{}
	for key, value := range map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"redirect_uri":  testRedirectURI,
		"client_id":     e.clientID,
		"client_secret": e.clientSecret,
		"code_verifier": verifier,
	} {
		form.Set(key, value)
	}
	tokenResp, err := insecureClient().PostForm(e.baseURL+"/oidc/token", form)
	if err != nil {
		e.t.Fatalf("token exchange: %v", err)
	}
	defer tokenResp.Body.Close()
	raw, _ := io.ReadAll(tokenResp.Body)
	var parsed struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		e.t.Fatalf("decode token response: %v (body %s)", err, raw)
	}
	if parsed.AccessToken == "" {
		e.t.Fatalf("token exchange returned no access_token: %s", raw)
	}
	return parsed.AccessToken
}

// accessTokenScope decodes the scope claim of a JWT access token.
func accessTokenScope(t *testing.T, token string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		t.Fatalf("token is not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode JWT payload: %v", err)
	}
	var claims struct {
		Scope any `json:"scope"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("decode JWT claims: %v", err)
	}
	return fmt.Sprint(claims.Scope)
}

func insecureClient() *http.Client {
	// #nosec G402 -- test-only client against a local throwaway container.
	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	return &http.Client{Timeout: 30 * time.Second, Transport: transport}
}

func randomURLSafe(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// startMCPServer runs the built binary and returns its base URL.
func startMCPServer(t *testing.T, args ...string) string {
	t.Helper()
	port := freePort(t)
	full := append([]string{"--port", strconv.Itoa(port)}, args...)
	cmd := exec.Command(serverBin, full...)
	logFile, err := os.CreateTemp("", "rancher-mcp-it-server-*.log")
	if err != nil {
		t.Fatalf("create server log: %v", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		if t.Failed() {
			if raw, err := os.ReadFile(logFile.Name()); err == nil {
				t.Logf("server log:\n%s", raw)
			}
		}
		_ = os.Remove(logFile.Name())
	})

	baseURL := fmt.Sprintf("http://localhost:%d", port)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return baseURL
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("MCP server did not become ready on %s", baseURL)
	return ""
}

// mcpStatus posts a minimal JSON-RPC initialize request and returns the HTTP
// status, used to assert auth rejections before any MCP session exists.
func mcpStatus(t *testing.T, baseURL string, headers map[string]string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+"/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	if err != nil {
		t.Fatalf("build MCP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("MCP request: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

func callTool(t *testing.T, baseURL string, headers map[string]string, tool string, args map[string]any) (*mcpgo.CallToolResult, error) {
	t.Helper()
	client, err := mcpclient.NewStreamableHttpClient(baseURL+"/mcp",
		mcptransport.WithHTTPHeaders(headers))
	if err != nil {
		t.Fatalf("NewStreamableHttpClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	if err := client.Start(ctx); err != nil {
		return nil, fmt.Errorf("client.Start: %w", err)
	}
	if _, err := client.Initialize(ctx, mcpgo.InitializeRequest{Params: mcpgo.InitializeParams{
		ProtocolVersion: mcpgo.LATEST_PROTOCOL_VERSION,
		ClientInfo:      mcpgo.Implementation{Name: "rancher-mcp-server-integration", Version: "0.0.0"},
		Capabilities:    mcpgo.ClientCapabilities{},
	}}); err != nil {
		return nil, fmt.Errorf("client.Initialize: %w", err)
	}
	return client.CallTool(ctx, mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{
		Name:      tool,
		Arguments: args,
	}})
}

func urlQueryEscape(value string) string {
	return strings.NewReplacer(":", "%3A", "/", "%2F", " ", "%20").Replace(value)
}

func queryParam(rawURL, key string) string {
	idx := strings.Index(rawURL, "?")
	if idx < 0 {
		return ""
	}
	for _, pair := range strings.Split(rawURL[idx+1:], "&") {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 && parts[0] == key {
			return parts[1]
		}
	}
	return ""
}
