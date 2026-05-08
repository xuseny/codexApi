package service

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

const (
	windsurfLSDefaultBinary       = "/opt/windsurf/language_server_linux_x64"
	windsurfLSDefaultPort         = 42100
	windsurfLSDefaultCSRF         = "windsurf-api-csrf-fixed-token"
	windsurfLSDefaultAPIURL       = "https://server.self-serve.windsurf.com"
	windsurfRawGetChatMessagePath = "/exa.language_server_pb.LanguageServerService/RawGetChatMessage"
)

type windsurfLSEntry struct {
	port      int
	csrfToken string
	proxyURL  string
	cmd       *exec.Cmd
	ready     bool
}

type windsurfLSManager struct {
	mu       sync.Mutex
	nextPort int
	entries  map[string]*windsurfLSEntry
}

var defaultWindsurfLSManager = &windsurfLSManager{
	nextPort: windsurfLSDefaultPort + 1,
	entries:  make(map[string]*windsurfLSEntry),
}

func (s *OpenAIGatewayService) CloseWindsurfLanguageServers() {
	defaultWindsurfLSManager.close()
}

func ensureWindsurfLanguageServer(ctx context.Context, proxyURL string) (*windsurfLSEntry, error) {
	return defaultWindsurfLSManager.ensure(ctx, proxyURL)
}

func (m *windsurfLSManager) close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, entry := range m.entries {
		if entry != nil && entry.cmd != nil && entry.cmd.Process != nil {
			if err := entry.cmd.Process.Kill(); err != nil {
				slog.Warn("failed to stop windsurf language server", "key", key, "port", entry.port, "error", err)
			}
		}
		delete(m.entries, key)
	}
}

func (m *windsurfLSManager) ensure(ctx context.Context, proxyURL string) (*windsurfLSEntry, error) {
	if m == nil {
		return nil, errors.New("windsurf language server manager is not configured")
	}
	if externalURL := strings.TrimSpace(os.Getenv("WINDSURF_LS_URL")); externalURL != "" {
		port, err := portFromLocalHTTPURL(externalURL)
		if err != nil {
			return nil, err
		}
		return &windsurfLSEntry{port: port, csrfToken: getEnvStringLocal("WINDSURF_LS_CSRF_TOKEN", windsurfLSDefaultCSRF), ready: true}, nil
	}

	key := windsurfLSProxyKey(proxyURL)
	m.mu.Lock()
	if existing := m.entries[key]; existing != nil && existing.ready {
		m.mu.Unlock()
		return existing, nil
	}
	port := windsurfLSDefaultPort
	if key != "default" {
		port = m.nextPort
		m.nextPort++
	}
	for isTCPPortInUse(port) {
		if key == "default" {
			port = m.nextPort
			m.nextPort++
			break
		}
		port = m.nextPort
		m.nextPort++
	}
	entry := &windsurfLSEntry{
		port:      port,
		csrfToken: getEnvStringLocal("WINDSURF_LS_CSRF_TOKEN", windsurfLSDefaultCSRF),
		proxyURL:  proxyURL,
	}
	m.entries[key] = entry
	m.mu.Unlock()

	if err := startWindsurfLanguageServer(ctx, entry, key); err != nil {
		m.mu.Lock()
		delete(m.entries, key)
		m.mu.Unlock()
		return nil, err
	}
	return entry, nil
}

func startWindsurfLanguageServer(ctx context.Context, entry *windsurfLSEntry, key string) error {
	binaryPath := getEnvStringLocal("WINDSURF_LS_BINARY_PATH", windsurfLSDefaultBinary)
	if _, err := os.Stat(binaryPath); err != nil {
		return fmt.Errorf("windsurf language server binary not found at %s; set WINDSURF_LS_BINARY_PATH or mount the binary into Docker", binaryPath)
	}
	dataRoot := getEnvStringLocal("WINDSURF_LS_DATA_DIR", filepath.Join(".", "data", "windsurf-ls"))
	dataDir := filepath.Join(dataRoot, key)
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		return err
	}

	apiServerURL := getEnvStringLocal("WINDSURF_LS_API_SERVER_URL", windsurfLSDefaultAPIURL)
	args := []string{
		"--api_server_url=" + apiServerURL,
		"--server_port=" + strconv.Itoa(entry.port),
		"--csrf_token=" + entry.csrfToken,
		"--register_user_url=https://api.codeium.com/register_user/",
		"--codeium_dir=" + dataDir,
		"--database_dir=" + filepath.Join(dataDir, "db"),
		"--detect_proxy=false",
	}
	cmd := exec.CommandContext(context.Background(), binaryPath, args...)
	cmd.Env = windsurfLanguageServerEnv(entry.proxyURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start windsurf language server: %w", err)
	}
	entry.cmd = cmd
	go func() {
		err := cmd.Wait()
		if err != nil {
			slog.Warn("windsurf language server exited", "key", key, "port", entry.port, "error", err)
		}
	}()

	waitCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	for {
		if isTCPPortInUse(entry.port) {
			entry.ready = true
			return nil
		}
		select {
		case <-waitCtx.Done():
			_ = cmd.Process.Kill()
			return fmt.Errorf("windsurf language server port %d not ready: %w", entry.port, waitCtx.Err())
		case <-time.After(300 * time.Millisecond):
		}
	}
}

func newWindsurfGRPCClient() *http.Client {
	return &http.Client{
		Timeout: 0,
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				var dialer net.Dialer
				return dialer.DialContext(ctx, network, addr)
			},
		},
	}
}

func windsurfLanguageServerEnv(proxyURL string) []string {
	allow := map[string]bool{
		"HOME": true, "PATH": true, "LANG": true, "LC_ALL": true,
		"TMPDIR": true, "TMP": true, "TEMP": true,
		"SSL_CERT_FILE": true, "SSL_CERT_DIR": true,
		"HTTP_PROXY": true, "HTTPS_PROXY": true, "NO_PROXY": true,
		"http_proxy": true, "https_proxy": true, "no_proxy": true,
	}
	env := make([]string, 0, len(allow)+4)
	seen := map[string]bool{}
	for _, item := range os.Environ() {
		key, _, ok := strings.Cut(item, "=")
		if ok && allow[key] {
			env = append(env, item)
			seen[key] = true
		}
	}
	if !seen["HOME"] {
		env = append(env, "HOME=/tmp")
	}
	if proxyURL != "" {
		env = append(env,
			"HTTP_PROXY="+proxyURL,
			"HTTPS_PROXY="+proxyURL,
			"http_proxy="+proxyURL,
			"https_proxy="+proxyURL,
		)
	}
	return env
}

func windsurfLSProxyKey(proxyURL string) string {
	if strings.TrimSpace(proxyURL) == "" {
		return "default"
	}
	sum := sha1.Sum([]byte(proxyURL))
	return "proxy_" + hex.EncodeToString(sum[:])[:16]
}

func isTCPPortInUse(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func portFromLocalHTTPURL(raw string) (int, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	_, portRaw, ok := strings.Cut(trimmed, ":")
	if !ok {
		return 0, fmt.Errorf("invalid WINDSURF_LS_URL: %s", raw)
	}
	if idx := strings.LastIndex(portRaw, ":"); idx >= 0 {
		portRaw = portRaw[idx+1:]
	}
	portRaw = strings.Trim(portRaw, "/")
	port, err := strconv.Atoi(portRaw)
	if err != nil || port <= 0 {
		return 0, fmt.Errorf("invalid WINDSURF_LS_URL port: %s", raw)
	}
	return port, nil
}

func getEnvStringLocal(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
