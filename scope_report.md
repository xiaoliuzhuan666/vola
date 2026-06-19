# Vola Scope Report

This report outlines the boundaries and current scopes of different modules in Vola.

## Features Scope

| Module | Scope | Boundary / Limits |
| --- | --- | --- |
| **Local Sync** | Sync personal/team skills to local client folders. | Only modifies files inside target directories containing `.vola-managed.json`. |
| **MCP Hub** | Gateway managing local stdio sub-processes. | Supports starting, stopping, and merging tool definitions. HTTP connection definitions are schema-compatible. |
| **OAuth / Tokens** | CLI browser login flow returning Scoped Token. | Token rotation and scoped authorizations are verified per request. |
| **Import / Export** | Heading export from platforms like Codex. | Export files are mapped into Vola domains (e.g. `memory/profile` or `projects/<name>/context.md`). |
