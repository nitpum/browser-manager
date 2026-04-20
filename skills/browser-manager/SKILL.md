---
name: browser-manager
description: Control a shared browser instance via a central server. Get CDP WebSocket URLs, check browser status, restart or stop the browser. Use when you need to interact with a web browser, scrape pages, automate browser tasks, take screenshots, fill forms, navigate web pages, or debug web applications.
---

# Browser Manager

## When to use this skill

Use this skill when you need to:
- Open and interact with web pages
- Scrape web content
- Fill out web forms
- Take screenshots of web pages
- Automate browser tasks
- Test web applications
- Debug web pages
- Navigate to URLs and extract information

Do NOT use this skill when:
- You only need to make simple API calls (use curl/fetch instead)
- You need to process local files (use file tools instead)

## Prerequisites

The browser-manager server must be running. Start it with:

```bash
browser-manager server --port 9292 --browser /usr/bin/google-chrome --profile /path/to/profile
```

Flags:
- `--port`: Server listen port (default: 9292)
- `--browser`: Path to browser executable (default: "google-chrome")
- `--profile`: Browser profile/user-data-dir path (optional)

## Workflow

### Step 1: Check server status

Before doing anything, verify the server is running and the browser is available:

```bash
curl http://localhost:9292/api/status
```

Expected response:
```json
{
  "running": true,
  "pid": 12345,
  "debug_port": 9222,
  "ws_url": "ws://127.0.0.1:9222/devtools/browser/...",
  "browser_path": "/usr/bin/google-chrome",
  "profile": "/path/to/profile"
}
```

If `"running": false`, the browser needs to be started first.

### Step 2: Get the CDP WebSocket URL

```bash
curl http://localhost:9292/api/ws-url
```

Response:
```json
{
  "ws_url": "ws://127.0.0.1:9222/devtools/browser/..."
}
```

### Step 3: Connect via CDP

Use the `ws_url` to connect to the browser via CDP (Chrome DevTools Protocol). You can:

1. **Direct CDP WebSocket connection**: Connect to the ws_url using a WebSocket client and send CDP commands
2. **Use a CDP library**: Use libraries like:
   - Python: `pychrome`, `playwright`, or `selenium` with CDP support
   - Node.js: `chrome-remote-interface`, `puppeteer`
   - Go: `chromedp`

### Step 4: Interact with the browser

Once connected via CDP, you can:
- Create new tabs/pages
- Navigate to URLs
- Execute JavaScript
- Take screenshots
- Fill forms and click elements
- Extract page content
- Monitor network requests

Example CDP commands to create a new tab and navigate:

**Via HTTP (no WebSocket needed):**
```bash
# List open tabs
curl http://localhost:9222/json/list

# Create new tab
curl -X PUT http://localhost:9222/json/new?https://example.com
```

**Via WebSocket:**
Send JSON messages like:
```json
{"id": 1, "method": "Target.createTarget", "params": {"url": "https://example.com"}}
```

## API Quick Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/status` | Browser status (running, PID, debug port, WS URL) |
| GET | `/api/ws-url` | CDP WebSocket debug URL |
| POST | `/api/restart` | Restart browser instance |
| POST | `/api/stop` | Stop browser and server |

All endpoints are at the server URL (default: `http://localhost:9292`).

## Common Patterns

### Restart an unresponsive browser

```bash
curl -X POST http://localhost:9292/api/restart
```

### Stop everything

```bash
curl -X POST http://localhost:9292/api/stop
```

### Get browser debug port for direct HTTP access

```bash
# Get status to find the debug port
curl -s http://localhost:9292/api/status | jq '.debug_port'
# Then use Chrome's DevTools HTTP endpoints
curl http://localhost:<debug_port>/json/list
```

## Important Notes

- The server manages only **one browser instance** at a time
- The CDP connection is **direct** — you connect to the browser's WebSocket, not through the server
- All tab/page management is done via CDP, not the server API
- The server API is only for lifecycle management (status, restart, stop)
- No authentication is required

## Detailed API Reference

See [api-reference.md](references/api-reference.md) for complete endpoint documentation.
