# Tech Stack

This document lists the tech stack and tools used in Vola.

## Backend (Go)

- **Go version**: 1.21+
- **HTTP Routing**: `go-chi/chi/v5`
- **Database**: SQLite with `modernc.org/sqlite` (pure Go driver)
- **Token / JWT**: Scoped token mapping with SQLite storage and custom validation
- **UUID**: `google/uuid`

## Frontend (Vite / React)

- **Framework**: Vite + React + TypeScript
- **Styling**: Vanilla CSS (Vola UI utilizes custom design system variables under `src/index.css`)
- **Icons**: SVG paths (integrated natively or imported through setup presets)

## Client Environments

- **Claude Desktop Config**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Cursor Config**: `~/.cursor/mcp.json`
- **Trae Config**: `~/.trae/mcp.json`
- **Codebuddy / Workbuddy**: Stored locally in `~/.codebuddy` / `~/.workbuddy`
