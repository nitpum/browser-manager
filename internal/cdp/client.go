package cdp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// CDPClient communicates with a Chrome DevTools Protocol HTTP endpoint.
type CDPClient struct {
	host string
	port int
}

// NewCDPClient creates a new CDPClient that talks to the Chrome DevTools
// Protocol endpoint at host:port.
func NewCDPClient(host string, port int) *CDPClient {
	return &CDPClient{
		host: host,
		port: port,
	}
}

// GetWebSocketURL queries the CDP /json/version endpoint and returns the
// webSocketDebuggerUrl for connecting to the browser over WebSocket.
func (c *CDPClient) GetWebSocketURL() (string, error) {
	result, err := c.GetVersion()
	if err != nil {
		return "", err
	}

	wsURL, ok := result["webSocketDebuggerUrl"].(string)
	if !ok || wsURL == "" {
		return "", fmt.Errorf("webSocketDebuggerUrl not found in CDP response")
	}

	return wsURL, nil
}

// GetVersion queries the CDP /json/version endpoint and returns the full
// version information as a map.
func (c *CDPClient) GetVersion() (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("http://%s:%d/json/version", c.host, c.port)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("CDP /json/version request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CDP /json/version returned HTTP %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode CDP response: %w", err)
	}

	return result, nil
}
