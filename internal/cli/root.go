package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"browser-manager/internal/browser"
	"browser-manager/internal/server"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "browser-manager",
	Short: "Central browser manager for AI agents",
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the browser manager server",
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		browserPath, _ := cmd.Flags().GetString("browser")
		profile, _ := cmd.Flags().GetString("profile")
		noHeadless, _ := cmd.Flags().GetBool("no-headless")

		manager := browser.NewBrowserManager(browserPath, profile, !noHeadless)
		if err := manager.Start(); err != nil {
			log.Fatal(err)
		}

		status := manager.Status()
		log.Printf("Browser started (PID: %v, debug port: %v)", status.PID, status.DebugPort)

		srv := server.NewServer(fmt.Sprintf(":%d", port), manager)

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			log.Println("Received shutdown signal")
			manager.Stop()
			srv.Stop()
			os.Exit(0)
		}()

		if err := srv.Start(); err != nil {
			log.Fatal(err)
		}
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get browser status",
	Run: func(cmd *cobra.Command, args []string) {
		serverURL, _ := cmd.Flags().GetString("server-url")
		url := serverURL + "/api/status"

		body, _, err := sendRequest(http.MethodGet, url)
		if err != nil {
			fmt.Printf("Error: unable to connect to server at %s\n", serverURL)
			os.Exit(1)
		}

		var pretty bytes.Buffer
		if err := json.Indent(&pretty, body, "", "  "); err != nil {
			fmt.Printf("Error: failed to parse response: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(pretty.String())
	},
}

var wsURLCmd = &cobra.Command{
	Use:   "ws-url",
	Short: "Get CDP WebSocket URL",
	Run: func(cmd *cobra.Command, args []string) {
		serverURL, _ := cmd.Flags().GetString("server-url")
		url := serverURL + "/api/ws-url"

		body, statusCode, err := sendRequest(http.MethodGet, url)
		if err != nil {
			fmt.Printf("Error: unable to connect to server at %s\n", serverURL)
			os.Exit(1)
		}

		if statusCode == http.StatusOK {
			var result map[string]interface{}
			if err := json.Unmarshal(body, &result); err != nil {
				fmt.Printf("Error: failed to parse response: %v\n", err)
				os.Exit(1)
			}
			if wsURL, ok := result["ws_url"].(string); ok {
				fmt.Println(wsURL)
			}
			return
		}

		var errResp map[string]interface{}
		if err := json.Unmarshal(body, &errResp); err != nil {
			fmt.Printf("Error: request failed with status %d\n", statusCode)
			os.Exit(1)
		}
		if errMsg, ok := errResp["error"].(string); ok {
			fmt.Printf("Error: %s\n", errMsg)
		} else {
			fmt.Printf("Error: request failed with status %d\n", statusCode)
		}
		os.Exit(1)
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the browser",
	Run: func(cmd *cobra.Command, args []string) {
		serverURL, _ := cmd.Flags().GetString("server-url")
		url := serverURL + "/api/restart"

		body, _, err := sendRequest(http.MethodPost, url)
		if err != nil {
			fmt.Printf("Error: unable to connect to server at %s\n", serverURL)
			os.Exit(1)
		}

		var pretty bytes.Buffer
		if err := json.Indent(&pretty, body, "", "  "); err != nil {
			fmt.Printf("Error: failed to parse response: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(pretty.String())
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the browser and server",
	Run: func(cmd *cobra.Command, args []string) {
		serverURL, _ := cmd.Flags().GetString("server-url")
		url := serverURL + "/api/stop"

		_, _, err := sendRequest(http.MethodPost, url)
		if err != nil {
			fmt.Printf("Error: unable to connect to server at %s\n", serverURL)
			os.Exit(1)
		}

		fmt.Println("Server shutting down...")
	},
}

func init() {
	serverCmd.Flags().Int("port", 9292, "server listen port")
	serverCmd.Flags().String("browser", "google-chrome", "path to browser executable")
	serverCmd.Flags().String("profile", "", "browser profile/user-data-dir path")
	serverCmd.Flags().Bool("no-headless", false, "run browser in headed mode (with GUI)")

	statusCmd.Flags().String("server-url", "http://localhost:9292", "server URL")
	wsURLCmd.Flags().String("server-url", "http://localhost:9292", "server URL")
	restartCmd.Flags().String("server-url", "http://localhost:9292", "server URL")
	stopCmd.Flags().String("server-url", "http://localhost:9292", "server URL")

	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(wsURLCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(stopCmd)
}

// Execute runs the root cobra command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// sendRequest creates an HTTP request with the given method and URL,
// sends it with a 5-second timeout, and returns the response body,
// status code, and any error.
func sendRequest(method, url string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response body: %w", err)
	}

	return buf.Bytes(), resp.StatusCode, nil
}
