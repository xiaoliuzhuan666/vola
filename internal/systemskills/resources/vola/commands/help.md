# `vola help`

Use this command when the user asks what Vola can do from the current platform.

## Public Forms

- `vola help`
- `vola help roots`
- `vola help <command>`
- `vola help project`

## Explain

- Vola now uses a root-directory mental model:
  - `vola ls [path]`
  - `vola read <path>`
  - `vola write <path> <content-or-file>`
  - `vola search <query> [path]`
  - `vola create project <name>`
  - `vola log project/<name> ...`
  - `vola import <category> <src>`
  - `vola token create --kind sync|skills-upload`
  - `vola stats`
- The external roots are `profile`, `memory`, `project`, `skill`, `secret`, and `platform`.
- A leading `/` is optional. `project/demo` and `/project/demo` are equivalent.
- `vola platform ...`, `connect`, `disconnect`, `export`, and `status` remain operational commands.
- Git Mirror is configured through the Vola dashboard rather than a dedicated CLI command.
- Once the user has enabled Git Mirror, later Hub writes and imports keep syncing into the same directory automatically.

## Good Guidance

- Start with `vola help` when the user needs the whole mental model.
- Use `vola help roots` when the user is confused about `profile / memory / project / skill / secret / platform`.
- `vola help project`, `vola help memory`, and similar root names should resolve back to the path model guidance.
- Use `vola help write` or `vola help import` when the user needs one concrete workflow with examples.
- For Claude/Codex embedded usage, mirror the same guidance with `/vola help ...` or `$vola help ...`.
