package main

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDesktopAutoStartDefaultsAndSavedPreference(t *testing.T) {
	for _, tc := range []struct {
		name     string
		contents string
		want     bool
	}{
		{name: "first launch", want: true},
		{name: "legacy settings", contents: `{"port":0}`, want: true},
		{name: "saved disabled", contents: `{"autoStart":false}`, want: false},
		{name: "saved enabled", contents: `{"autoStart":true}`, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if tc.contents != "" {
				mustWrite(t, path, tc.contents)
			}
			settings, err := loadDesktopSettings(path)
			if err != nil {
				t.Fatal(err)
			}
			if settings.AutoStart != tc.want {
				t.Fatalf("AutoStart = %v, want %v", settings.AutoStart, tc.want)
			}
		})
	}
}

func testController(t *testing.T) *desktopController {
	t.Helper()
	base := t.TempDir()
	c := &desktopController{settings: desktopSettings{Directory: t.TempDir()}, configPath: filepath.Join(base, "settings.json"), statePath: filepath.Join(base, "service.json")}
	t.Cleanup(c.stop)
	return c
}

func TestDesktopServiceLifecycle(t *testing.T) {
	c := testController(t)
	mustWrite(t, filepath.Join(c.settings.Directory, "hello.txt"), "hello desktop")
	if c.status().Running {
		t.Fatal("service must be stopped initially")
	}
	if err := c.start(); err != nil {
		t.Fatal(err)
	}
	firstToken := c.handler.token
	status := c.status()
	if !status.Running || status.Port == 0 || status.LocalURL == "" {
		t.Fatalf("invalid running status: %+v", status)
	}
	if err := c.start(); err != nil {
		t.Fatal(err)
	}
	if c.handler.token != firstToken {
		t.Fatal("repeated start replaced a running service")
	}
	response, err := http.Get(status.LocalURL + "download?path=hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || string(data) != "hello desktop" {
		t.Fatalf("download: %s, %v", data, err)
	}
	c.stop()
	if c.status().Running {
		t.Fatal("service is still running after stop")
	}
	if _, err := os.Stat(c.statePath); !os.IsNotExist(err) {
		t.Fatalf("instance lock not removed: %v", err)
	}
	u, _ := url.Parse(status.LocalURL)
	if conn, err := net.DialTimeout("tcp", u.Host, time.Second); err == nil {
		conn.Close()
		t.Fatal("listener is still open after stop")
	}
	c.stop()
	if err := c.start(); err != nil {
		t.Fatal(err)
	}
	if c.handler.token == firstToken {
		t.Fatal("restart reused old access token")
	}
}

func TestDesktopSettingsApplyAndPersist(t *testing.T) {
	c := testController(t)
	if err := c.start(); err != nil {
		t.Fatal(err)
	}
	settings := desktopSettings{Directory: t.TempDir(), Protected: true, AutoStart: true}
	if err := c.configure(settings); err != nil {
		t.Fatal(err)
	}
	root, _ := c.handler.roots()
	if root != settings.Directory || !c.handler.isProtected() {
		t.Fatal("running service did not apply settings")
	}
	loaded, err := loadDesktopSettings(c.configPath)
	if err != nil || loaded != settings {
		t.Fatalf("saved settings: %+v, %v", loaded, err)
	}
	settings.Port = 9000
	if err := c.configure(settings); err == nil {
		t.Fatal("changed port while running")
	}
	settings.Port, settings.Directory = 0, filepath.Join(t.TempDir(), "missing")
	if err := c.configure(settings); err == nil {
		t.Fatal("accepted missing directory")
	}
	if c.settings != loaded {
		t.Fatal("failed setting change altered settings")
	}
}

func TestDesktopStartFailureDoesNotLeakInstanceLock(t *testing.T) {
	c := testController(t)
	listener, err := net.Listen("tcp4", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	c.settings.Port = listener.Addr().(*net.TCPAddr).Port
	if err := c.start(); err == nil {
		t.Fatal("start succeeded on occupied port")
	}
	if c.status().Running {
		t.Fatal("failed service marked running")
	}
	if _, err := os.Stat(c.statePath); !os.IsNotExist(err) {
		t.Fatalf("failed start left a lock: %v", err)
	}
	c.settings.Port = 0
	if err := c.start(); err != nil {
		t.Fatal(err)
	}
}

func TestDesktopDoesNotTakeOverExistingService(t *testing.T) {
	c := testController(t)
	if err := c.start(); err != nil {
		t.Fatal(err)
	}
	other := testController(t)
	other.statePath = c.statePath
	if err := other.start(); err == nil {
		t.Fatal("took over existing service")
	}
	if other.status().Running {
		t.Fatal("reported another instance as owned")
	}
	other.stop()
	if !c.status().Running {
		t.Fatal("stopped another instance's service")
	}
}

func TestDesktopWebPageCannotManageService(t *testing.T) {
	c := testController(t)
	if err := c.start(); err != nil {
		t.Fatal(err)
	}
	handler := c.handler.routes(c.port)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "127.0.0.1:43210"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if strings.Contains(response.Body.String(), `action="/stop"`) || strings.Contains(response.Body.String(), `action="/directory"`) {
		t.Fatal("desktop service exposed web management UI")
	}
	for _, path := range []string{"/directory", "/security", "/stop"} {
		request := httptest.NewRequest(http.MethodPost, path, nil)
		request.RemoteAddr = "127.0.0.1:43210"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s management endpoint available", path)
		}
	}
}

func TestDesktopStartsBeforeFirstCommandAndEOFCleansUp(t *testing.T) {
	// Isolate the same cache-based single-instance lock used by the real worker.
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("HOME", cache)
	t.Setenv("LocalAppData", cache)
	config := filepath.Join(t.TempDir(), "settings.json")
	settings := desktopSettings{Directory: t.TempDir(), Protected: true, AutoStart: true}
	if err := saveDesktopSettings(config, settings); err != nil {
		t.Fatal(err)
	}
	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	t.Cleanup(func() {
		inputWriter.Close()
		outputReader.Close()
	})
	done := make(chan error, 1)
	go func() { done <- runDesktop(inputReader, outputWriter, config); outputWriter.Close() }()
	statePath, err := serviceStatePath()
	if err != nil {
		t.Fatal(err)
	}
	// No status request has been sent. Sharing must already be available.
	client := &http.Client{Timeout: time.Second}
	started := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		state, _, err := readServiceState(statePath)
		if err == nil && state.ControlURL != "" {
			response, err := client.Head(state.ControlURL + "health")
			if err == nil {
				response.Body.Close()
				started = response.StatusCode == http.StatusOK
				if started {
					break
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !started {
		t.Fatal("service did not start before the first UI command")
	}
	if _, err := io.WriteString(inputWriter, "{\"action\":\"status\"}\n"); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(outputReader)
	if !scanner.Scan() {
		t.Fatal("missing pipe response")
	}
	var response desktopResponse
	if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Status.Running || response.Status.Settings != settings {
		t.Fatalf("autostart settings not restored: %+v", response)
	}
	inputWriter.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("EOF did not stop worker")
	}
	u, _ := url.Parse(response.Status.LocalURL)
	if conn, err := net.DialTimeout("tcp", u.Host, time.Second); err == nil {
		conn.Close()
		t.Fatal("EOF left sharing enabled")
	}
}
