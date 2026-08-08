package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const lifecycleBootstrapToken = "test-bootstrap-token"

func TestRunLifecycleBootstrapSetupRestartAndShutdown(t *testing.T) {
	pelican := httptest.NewServer(http.HandlerFunc(fakePelicanHandler))
	t.Cleanup(pelican.Close)

	temporary := t.TempDir()
	secretsDirectory := filepath.Join(temporary, "secrets")
	if err := os.Mkdir(secretsDirectory, 0o700); err != nil {
		t.Fatalf("create secrets directory: %v", err)
	}
	writeLifecycleSecret(t, secretsDirectory, "bootstrap-token", lifecycleBootstrapToken)
	writeLifecycleSecret(t, secretsDirectory, "pelican-api-key", "test-pelican-api-key")

	databasePath := filepath.Join(temporary, "data", "router.db")
	proxiesDirectory := filepath.Join(temporary, "proxies")
	markerPath := filepath.Join(temporary, "control", "reload")

	configureLifecycleEnvironment(t, lifecycleConfig{
		DatabasePath:     databasePath,
		SecretsDirectory: secretsDirectory,
		ProxiesDirectory: proxiesDirectory,
		ReloadMarkerPath: markerPath,
		PelicanURL:       pelican.URL + "/api/application",
	})

	address, cancelFirst, firstDone := startLifecycleApp(t)
	client := &http.Client{Timeout: 2 * time.Second}
	waitForStatus(t, client, address, "/health", http.StatusOK)
	waitForStatus(t, client, address, "/ready", http.StatusServiceUnavailable)

	setupBody := []byte(fmt.Sprintf(`{"pelican_url":%q,"pelican_secret_name":"pelican-api-key","router_domain":"mc.example.test"}`, pelican.URL+"/api/application"))
	request, err := http.NewRequest(http.MethodPost, "http://"+address+"/api/v1/setup", bytes.NewReader(setupBody))
	if err != nil {
		t.Fatalf("create setup request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+lifecycleBootstrapToken)
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("submit setup request: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("setup status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}

	waitForStatus(t, client, address, "/ready", http.StatusOK)
	assertLifecycleFile(t, filepath.Join(proxiesDirectory, "pelican-mc-router-server-uuid.yml"))
	assertLifecycleFile(t, markerPath)
	waitForStatus(t, client, address, "/api/v1/setup", http.StatusNotFound)

	cancelFirst()
	waitForLifecycleExit(t, firstDone)

	address, cancelSecond, secondDone := startLifecycleApp(t)
	waitForStatus(t, client, address, "/health", http.StatusOK)
	waitForStatus(t, client, address, "/ready", http.StatusOK)
	waitForStatus(t, client, address, "/api/v1/setup", http.StatusNotFound)
	assertLifecycleFile(t, filepath.Join(proxiesDirectory, "pelican-mc-router-server-uuid.yml"))

	cancelSecond()
	waitForLifecycleExit(t, secondDone)
}

type lifecycleConfig struct {
	DatabasePath     string
	SecretsDirectory string
	ProxiesDirectory string
	ReloadMarkerPath string
	PelicanURL       string
}

func configureLifecycleEnvironment(t *testing.T, cfg lifecycleConfig) {
	t.Helper()
	t.Setenv("PELICAN_MC_ROUTER_DATABASE_PATH", cfg.DatabasePath)
	t.Setenv("PELICAN_MC_ROUTER_SECRETS_DIRECTORY", cfg.SecretsDirectory)
	t.Setenv("PELICAN_MC_ROUTER_SECRETS_BOOTSTRAP_TOKEN_NAME", "bootstrap-token")
	t.Setenv("PELICAN_MC_ROUTER_SERVER_HOST", "127.0.0.1")
	t.Setenv("PELICAN_MC_ROUTER_ROUTER_BACKEND", "infrared")
	t.Setenv("PELICAN_MC_ROUTER_INFRARED_PROXIES_PATH", cfg.ProxiesDirectory)
	t.Setenv("PELICAN_MC_ROUTER_INFRARED_RELOAD_MARKER_PATH", cfg.ReloadMarkerPath)
	t.Setenv("PELICAN_MC_ROUTER_PELICAN_URL", cfg.PelicanURL)
	t.Setenv("PELICAN_MC_ROUTER_PELICAN_TIMEOUT", "2s")
	t.Setenv("PELICAN_MC_ROUTER_DISCOVERY_INTERVAL", "1h")
	t.Setenv("PELICAN_MC_ROUTER_DASHBOARD_AUTH_ENABLED", "false")
}

func startLifecycleApp(t *testing.T) (string, context.CancelFunc, <-chan error) {
	t.Helper()
	port := reserveLifecyclePort(t)
	t.Setenv("PELICAN_MC_ROUTER_SERVER_PORT", fmt.Sprint(port))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx) }()
	return net.JoinHostPort("127.0.0.1", fmt.Sprint(port)), cancel, done
}

func reserveLifecyclePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve listener: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func waitForStatus(t *testing.T, client *http.Client, address, path string, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get("http://" + address + path)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == want {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s did not return status %d", path, want)
}

func waitForLifecycleExit(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not exit after cancellation")
	}
}

func writeLifecycleSecret(t *testing.T, directory, name, value string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), []byte(value+"\n"), 0o600); err != nil {
		t.Fatalf("write secret %q: %v", name, err)
	}
}

func assertLifecycleFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
}

func fakePelicanHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Header.Get("Authorization") != "Bearer test-pelican-api-key" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var data any
	switch r.URL.Path {
	case "/api/application/nodes":
		data = []any{map[string]any{"object": "node", "attributes": map[string]any{"id": 1, "uuid": "node-uuid", "name": "node", "fqdn": "127.0.0.1"}}}
	case "/api/application/servers":
		data = []any{map[string]any{"object": "server", "attributes": map[string]any{"id": 1, "uuid": "server-uuid", "identifier": "lifecycle", "name": "lifecycle", "node": 1, "allocation": 1, "egg": 1}}}
	case "/api/application/eggs":
		data = []any{map[string]any{"object": "egg", "attributes": map[string]any{"id": 1, "uuid": "egg-uuid", "name": "Minecraft", "tags": []string{"minecraft"}}}}
	case "/api/application/nodes/1/allocations":
		data = []any{map[string]any{"object": "allocation", "attributes": map[string]any{"id": 1, "ip": "127.0.0.1", "port": 25565}}}
	default:
		w.WriteHeader(http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"object": "list",
		"data":   data,
		"meta": map[string]any{"pagination": map[string]any{
			"total": 1, "count": 1, "per_page": 1, "current_page": 1, "total_pages": 1,
		}},
	})
}
