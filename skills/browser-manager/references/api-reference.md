# Browser Manager API Reference

Base URL: `http://localhost:9292` (configurable via `--port` flag)

## Authentication

None required.

## Endpoints

### GET /api/status

Returns the current browser status.

**Request:**
```bash
curl http://localhost:9292/api/status
```

**Response (200 OK) — Browser running:**
```json
{
  "running": true,
  "pid": 12345,
  "debug_port": 9222,
  "ws_url": "ws://127.0.0.1:9222/devtools/browser/abc-123-def",
  "browser_path": "/usr/bin/google-chrome",
  "profile": "/home/user/.config/browser-manager/profile"
}
```

**Response (200 OK) — Browser not running:**
```json
{
  "running": false,
  "pid": 0,
  "debug_port": 0,
  "ws_url": "",
  "browser_path": "/usr/bin/google-chrome",
  "profile": "/home/user/.config/browser-manager/profile"
}
```

**Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `running` | boolean | Whether the browser process is alive |
| `pid` | integer | Browser process ID (0 if not running) |
| `debug_port` | integer | Chrome remote debugging port (0 if not running) |
| `ws_url` | string | CDP WebSocket URL (empty if not running) |
| `browser_path` | string | Path to browser executable |
| `profile` | string | Browser profile/user-data-dir path |

---

### GET /api/ws-url

Returns the CDP WebSocket debug URL for direct browser connection.

**Request:**
```bash
curl http://localhost:9292/api/ws-url
```

**Response (200 OK):**
```json
{
  "ws_url": "ws://127.0.0.1:9222/devtools/browser/abc-123-def"
}
```

**Response (503 Service Unavailable) — Browser not running:**
```json
{
  "error": "browser not running"
}
```

**Usage:**
Use the `ws_url` to connect to the browser via Chrome DevTools Protocol. This is a direct WebSocket connection to the browser — no proxy.

---

### POST /api/restart

Restarts the browser instance. Kills the existing process and launches a new one with the same configuration.

**Request:**
```bash
curl -X POST http://localhost:9292/api/restart
```

**Response (200 OK):**
```json
{
  "running": true,
  "pid": 67890,
  "debug_port": 9223,
  "ws_url": "ws://127.0.0.1:9223/devtools/browser/xyz-456-uvw",
  "browser_path": "/usr/bin/google-chrome",
  "profile": "/home/user/.config/browser-manager/profile"
}
```

**Response (500 Internal Server Error):**
```json
{
  "error": "failed to start browser: exit status 1"
}
```

**Notes:**
- The debug port may change after restart (since `--remote-debugging-port=0` uses a random port)
- All existing CDP connections will be disconnected
- Any open tabs/pages will be lost (unless the profile preserves state)

---

### POST /api/stop

Stops the browser process and shuts down the server.

**Request:**
```bash
curl -X POST http://localhost:9292/api/stop
```

**Response (200 OK):**
```json
{
  "message": "server shutting down"
}
```

**Notes:**
- This is a graceful shutdown — the response is sent before the server stops
- After this call, the server will no longer be reachable
- To resume, restart the server with `browser-manager server`

---

## Error Handling

All endpoints return errors in this format:

```json
{
  "error": "description of what went wrong"
}
```

Common HTTP status codes:
- `200` — Success
- `500` — Internal server error (browser launch failure, etc.)
- `503` — Service unavailable (browser not running)

## CLI Equivalents

Each API endpoint has a corresponding CLI command:

| API Endpoint | CLI Command |
|---|---|
| `GET /api/status` | `browser-manager status` |
| `GET /api/ws-url` | `browser-manager ws-url` |
| `POST /api/restart` | `browser-manager restart` |
| `POST /api/stop` | `browser-manager stop` |

CLI commands accept a `--server-url` flag (default: `http://localhost:9292`) to specify the server address.
