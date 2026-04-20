# Browser Manager — Central browser server for AI agents

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Overview

Browser Manager is a Go CLI that acts as a central server for AI agents to interact with a browser via the Chrome DevTools Protocol (CDP). Instead of each agent spawning its own browser, they talk to a shared server that manages a single browser instance and exposes its CDP WebSocket URL through a simple REST API.

## Why?

When running multiple AI agents that need browser access, each one typically spawns its own Chrome process. This wastes memory, competes for resources, and makes lifecycle management difficult — who owns which browser? When should it restart?

Browser Manager solves this by running **one** browser instance and sharing it. Agents fetch the WebSocket URL from the server and connect directly via CDP. The server only handles browser lifecycle (start, stop, restart); all actual browser interaction (navigate, click, screenshot) happens over CDP between the agent and the browser directly — no proxy overhead.

## Architecture

```
┌──────────┐     ┌────────────┐     ┌──────────────────┐     ┌─────────┐
│ AI Agent │────>│  REST API   │────>│  Browser Manager  │────>│ Chrome  │
│          │     │  (port 9292)│     │    (server)       │     │  (CDP)  │
└──────────┘     └────────────┘     └──────────────────┘     └─────────┘
      │                                                         ^
      │              CDP WebSocket (direct)                     │
      └─────────────────────────────────────────────────────────┘
```

The server manages browser lifecycle only. Once an agent has the WebSocket URL from `/api/ws-url`, it connects directly to Chrome over CDP — the server is not in the data path.

## Installation

### From source

```bash
go install browser-manager/cmd/browser-manager@latest
```

Or build manually:

```bash
git clone https://github.com/your-org/browser-manager.git
cd browser-manager
go build -o browser-manager ./cmd/browser-manager
```

### Requirements

- **Go** 1.24+
- **Chrome** or **Chromium** installed and available in `$PATH`

## Quick Start

```bash
# Start server with default Chrome (headless)
browser-manager server

# Start with a custom browser path and profile
browser-manager server --browser /usr/bin/chromium --profile ~/.config/my-profile

# Start in headed mode (with GUI)
browser-manager server --no-headless

# From another terminal, get the CDP WebSocket URL
browser-manager ws-url
# ws://127.0.0.1:38479/devtools/browser/a1b2c3d4-...
```

## CLI Reference

### `browser-manager server`

Start the browser manager server. Launches Chrome and exposes the REST API.

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `9292` | Server listen port |
| `--browser` | `google-chrome` | Path to browser executable |
| `--profile` | *(empty)* | Browser user-data-dir path |
| `--no-headless` | `false` | Run browser in headed mode (with GUI) |

### `browser-manager status`

Get the current browser status from a running server.

| Flag | Default | Description |
|------|---------|-------------|
| `--server-url` | `http://localhost:9292` | URL of the running server |

### `browser-manager ws-url`

Get the CDP WebSocket URL for direct browser connection.

| Flag | Default | Description |
|------|---------|-------------|
| `--server-url` | `http://localhost:9292` | URL of the running server |

### `browser-manager restart`

Restart the browser instance (server stays running).

| Flag | Default | Description |
|------|---------|-------------|
| `--server-url` | `http://localhost:9292` | URL of the running server |

### `browser-manager stop`

Stop the browser and shut down the server.

| Flag | Default | Description |
|------|---------|-------------|
| `--server-url` | `http://localhost:9292` | URL of the running server |

## REST API

No authentication required. JSON request/response.

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/status` | Get browser status (PID, debug port, WS URL) |
| `GET` | `/api/ws-url` | Get CDP WebSocket URL |
| `POST` | `/api/restart` | Restart the browser instance |
| `POST` | `/api/stop` | Stop the browser and shut down the server |

### Response examples

**`GET /api/status`**

```json
{
  "running": true,
  "pid": 12345,
  "debug_port": 38479,
  "ws_url": "ws://127.0.0.1:38479/devtools/browser/...",
  "browser_path": "/usr/bin/google-chrome",
  "profile": ""
}
```

**`GET /api/ws-url`**

```json
{
  "ws_url": "ws://127.0.0.1:38479/devtools/browser/..."
}
```

**`POST /api/restart`**

```json
{
  "running": true,
  "pid": 67890,
  "debug_port": 42157,
  "ws_url": "ws://127.0.0.1:42157/devtools/browser/...",
  "browser_path": "/usr/bin/google-chrome",
  "profile": ""
}
```

**`POST /api/stop`**

```json
{
  "message": "server shutting down"
}
```

## Using with Puppeteer

```javascript
import puppeteer from 'puppeteer';

// Fetch the CDP WebSocket URL from the server
const resp = await fetch('http://localhost:9292/api/ws-url');
const { ws_url } = await resp.json();

// Connect directly to the shared browser
const browser = await puppeteer.connect({ browserWSEndpoint: ws_url });
const page = await browser.newPage();
await page.goto('https://example.com');
console.log(await page.title());

// Close the page, but NOT the browser — the server owns it
await page.close();
browser.disconnect();
```

## Using with curl

```bash
# Check browser status
curl http://localhost:9292/api/status

# Get CDP WebSocket URL
curl http://localhost:9292/api/ws-url

# Restart the browser
curl -X POST http://localhost:9292/api/restart

# Stop the server
curl -X POST http://localhost:9292/api/stop
```

## Agent Skills

The `skills/browser-manager/` directory follows the [agentskills.io](https://agentskills.io) format. AI agents can load `skills/browser-manager/SKILL.md` to learn how to interact with the browser manager API — checking status, obtaining WebSocket URLs, and connecting via CDP.

## How It Works

1. **Launches Chrome** with `--remote-debugging-port=0`, which tells Chrome to pick a random free port for its DevTools endpoint.
2. **Discovers the debug port** by reading Chrome's stderr output, which contains a line like `DevTools listening on ws://127.0.0.1:PORT/devtools/browser/...`.
3. **Exposes the CDP WebSocket URL** via a REST API on a configurable port (default `9292`).
4. **Clients connect directly** to the browser via CDP using the WebSocket URL — the server is not a proxy and adds no latency to browser interactions.
5. **Single browser instance** is shared by all clients. The server monitors process health and supports restart/stop operations.


