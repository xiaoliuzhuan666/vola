# Vola Agent Collaboration Framework

This document serves as the entrypoint for agents and multi-model collaboration within the Vola project.

## Agent Roles & Guidelines

Vola is a personal agentic data hub. When building features:
1. **Scope Safety**: Respect scopes (`ScopeReadSkills`, `ScopeWriteSkills`, `ScopeAdmin`). Never bypass authentication.
2. **Path Mapping Rules**: Ensure all path mappings are absolute and physically resolved without resolving arbitrary symlinks to sensitive locations.
3. **Configuration Locking**: Avoid direct, lock-free file writing. Use `platforms.SafeUpdateMcpConfig` for all local configuration updates to prevent conflicts.

## Sync & Export Conventions

- **Claude Code**: Syncs to `~/.claude/skills`.
- **Codex**: Syncs to `~/.agents/skills`.
- **Cursor**: Export-only rules mapping `SKILL.md` to `<skill-name>.mdc` with frontmatter:
  ```yaml
  ---
  description: "Vola managed skill: <name>"
  globs: "*"
  ---
  ```
- **Gemini CLI**: Export-only guidance for manual `GEMINI.md` references.
