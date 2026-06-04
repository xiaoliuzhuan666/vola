---
name: vola
description: Use Vola as the canonical hub for platform import, export, listing, and status workflows through MCP plus platform-native entrypoints.
tags:
  - vola
  - mcp
  - sync
  - portability
---

# Vola

Use this umbrella skill when the user wants to work with Vola from inside a supported platform such as Codex or Claude.

## Core Model

- Vola MCP is the supported public capability surface for current product workflows.
- This local `vola` skill is the entry layer that routes platform-native commands to the right MCP tools and local platform actions.
- Treat Vola as the canonical destination for imported data.
- Current public surface focuses on profile, memory, projects, skills, tree, token, and sync workflows.
- Roles, inbox, and collaboration remain deferred product concepts. Do not treat them as currently supported public Vola tools.

## Commands

- `ls`
  Read `/skills/vola/commands/ls.md`
- `read`
  Read `/skills/vola/commands/read.md`
- `write`
  Read `/skills/vola/commands/write.md`
- `search`
  Read `/skills/vola/commands/search.md`
- `create`
  Read `/skills/vola/commands/create.md`
- `log`
  Read `/skills/vola/commands/log.md`
- `import`
  Read `/skills/vola/commands/import.md`
- `token`
  Read `/skills/vola/commands/token.md`
- `stats`
  Read `/skills/vola/commands/stats.md`
- `export`
  Read `/skills/vola/commands/export.md`
- `status`
  Read `/skills/vola/commands/status.md`
- `help`
  Read `/skills/vola/commands/help.md`

## Quick Start

- Codex examples: `$vola help`, `$vola ls`, `$vola read profile/preferences`, `$vola status`
- Claude examples: `/vola help`, `/vola ls`, `/vola read profile/preferences`, `/vola status`

## Rules

- Use Vola MCP tools for Hub reads and writes instead of inventing local file formats.
- Preserve exact assets separately from derived summaries.
- When a platform-specific portability manual exists, read `/skills/portability/<platform>/SKILL.md` before migrating data or choosing import/export tools.
- When no platform-specific manual exists, fall back to `/skills/portability/general/SKILL.md`.
- For skills migration, especially "all skills", workspace exports, or zip-based imports, do not choose `import_skill`, `import_skills_archive`, or `prepare_skills_upload` until the portability manual has been read.
- Never silently drop unsupported or partially captured data; preserve it as notes or archive metadata.
