package browser

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// BrowserStatus represents the current state of the browser.
type BrowserStatus struct {
	Running     bool   `json:"running"`
	PID         int    `json:"pid"`
	DebugPort   int    `json:"debug_port"`
	WSURL       string `json:"ws_url"`
	BrowserPath string `json:"browser_path"`
	Profile     string `json:"profile"`
}

// BrowserManager manages a single browser (Chrome/Chromium) instance.
type BrowserManager struct {
	browserPath string
	profile     string
	headless    bool
	cmd         *exec.Cmd
	debugPort   int
	wsURL       string
	mu          sync.Mutex
	cancel      context.CancelFunc
}

// devToolsLineRegex matches the Chrome stderr line announcing the DevTools
// WebSocket URL and captures the debug port number.
var devToolsLineRegex = regexp.MustCompile(`ws://127\.0\.0\.1:(\d+)/`)

// fullWSURLRegex extracts the complete WebSocket URL from a line of output.
var fullWSURLRegex = regexp.MustCompile(`ws://127\.0\.0\.1:\d+/[^\s]+`)

// NewBrowserManager creates a new BrowserManager for the given browser
// executable path and optional user profile directory.
func NewBrowserManager(browserPath, profile string, headless bool) *BrowserManager {
	return &BrowserManager{
		browserPath: browserPath,
		profile:     profile,
		headless:    headless,
	}
}

// Start launches the browser process and blocks until the DevTools debugging
// port is discovered (or a 10-second timeout is reached).
func (bm *BrowserManager) Start() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.isRunningLocked() {
		return fmt.Errorf("browser is already running (pid: %d)", bm.cmd.Process.Pid)
	}

	// Build command-line arguments.
	args := []string{
		"--remote-debugging-port=0",
		"--no-first-run",
		"--no-default-browser-check",
		"--no-sandbox",
		"--disable-gpu",
		"--disable-dev-shm-usage",
	}
	if bm.headless {
		args = append(args, "--headless=new")
	}
	if bm.profile != "" {
		args = append(args, fmt.Sprintf("--user-data-dir=%s", bm.profile))
	}

	// Create a cancellable context for the browser process.
	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, bm.browserPath, args...)

	// Pipe stderr so we can read the DevTools listening URL.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start browser: %w", err)
	}

	bm.cmd = cmd
	bm.cancel = cancel

	// Channels used by the stderr-reading goroutine to communicate results.
	wsURLCh := make(chan string, 1)
	errCh := make(chan error, 1)

	// Read stderr line-by-line looking for the DevTools WebSocket URL.
	// After finding it, keep draining stderr so the pipe doesn't block the browser.
	go func() {
		defer close(wsURLCh)
		defer close(errCh)

		scanner := bufio.NewScanner(stderr)
		found := false
		for scanner.Scan() {
			line := scanner.Text()
			if !found {
				if fullMatch := fullWSURLRegex.FindString(line); fullMatch != "" {
					wsURLCh <- fullMatch
					found = true
				}
			}
			// Continue reading to keep the pipe drained.
		}
		if !found {
			if scanErr := scanner.Err(); scanErr != nil {
				errCh <- fmt.Errorf("error reading browser stderr: %w", scanErr)
			} else {
				errCh <- fmt.Errorf("browser process exited before DevTools URL was found")
			}
		}
	}()

	// Wait up to 10 seconds for the DevTools URL to appear.
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer timeoutCancel()

	select {
	case wsURL := <-wsURLCh:
		portStr := devToolsLineRegex.FindStringSubmatch(wsURL)
		if len(portStr) < 2 {
			// Should not happen since fullWSURLRegex already matched, but be safe.
			bm.killAndReset()
			return fmt.Errorf("failed to extract port from DevTools URL: %s", wsURL)
		}
		port, err := strconv.Atoi(portStr[1])
		if err != nil {
			bm.killAndReset()
			return fmt.Errorf("failed to parse debug port %q: %w", portStr[1], err)
		}
		bm.debugPort = port
		bm.wsURL = wsURL

		// Reap the child process in the background when it eventually exits.
		go func() { _ = cmd.Wait() }()

		return nil

	case readErr := <-errCh:
		bm.killAndReset()
		return fmt.Errorf("failed to discover DevTools port: %w", readErr)

	case <-timeoutCtx.Done():
		bm.killAndReset()
		return fmt.Errorf("timeout waiting for DevTools port (10s)")
	}
}

// Stop terminates the browser process and resets internal state.
func (bm *BrowserManager) Stop() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if !bm.isRunningLocked() {
		return nil
	}

	bm.killAndReset()
	return nil
}

// Restart stops the current browser instance and starts a fresh one.
func (bm *BrowserManager) Restart() error {
	if err := bm.Stop(); err != nil {
		return fmt.Errorf("failed to stop browser: %w", err)
	}
	return bm.Start()
}

// Status returns a snapshot of the current browser state. If the browser is
// running, it attempts to query the CDP /json/version endpoint for the latest
// webSocketDebuggerUrl; on failure it falls back to the stored URL.
func (bm *BrowserManager) Status() BrowserStatus {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	status := BrowserStatus{
		BrowserPath: bm.browserPath,
		Profile:     bm.profile,
	}

	if !bm.isRunningLocked() {
		return status
	}

	status.Running = true
	status.PID = bm.cmd.Process.Pid
	status.DebugPort = bm.debugPort
	status.WSURL = bm.wsURL

	// Try to refresh the WebSocket URL from the live CDP endpoint.
	if bm.debugPort > 0 {
		if freshURL, err := fetchWSURLFromCDP(bm.debugPort); err == nil {
			status.WSURL = freshURL
		}
	}

	return status
}

// IsRunning reports whether the managed browser process is still alive.
func (bm *BrowserManager) IsRunning() bool {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	return bm.isRunningLocked()
}

// isRunningLocked is the lock-free internal helper that checks process liveness.
// Caller must hold bm.mu.
func (bm *BrowserManager) isRunningLocked() bool {
	if bm.cmd == nil || bm.cmd.Process == nil {
		return false
	}
	return syscall.Kill(bm.cmd.Process.Pid, 0) == nil
}

// killAndReset terminates the browser process and clears all state.
// Caller must hold bm.mu.
func (bm *BrowserManager) killAndReset() {
	if bm.cancel != nil {
		bm.cancel()
		bm.cancel = nil
	}
	if bm.cmd != nil && bm.cmd.Process != nil {
		_ = bm.cmd.Process.Kill()
		// Wait must be called on the exec.Cmd to release OS resources.
		// We don't care about the error here since we're forcibly killing.
		_, _ = bm.cmd.Process.Wait()
	}
	bm.cmd = nil
	bm.debugPort = 0
	bm.wsURL = ""
}

// fetchWSURLFromCDP queries the Chrome DevTools Protocol HTTP endpoint
// /json/version to obtain the current webSocketDebuggerUrl.
func fetchWSURLFromCDP(port int) (string, error) {
	endpoint := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return "", fmt.Errorf("CDP /json/version request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("CDP /json/version returned HTTP %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode CDP response: %w", err)
	}

	wsURL, ok := result["webSocketDebuggerUrl"].(string)
	if !ok || wsURL == "" {
		return "", fmt.Errorf("webSocketDebuggerUrl not found in CDP response")
	}

	return wsURL, nil
}
