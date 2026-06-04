# `vola write`

Use this command when the user wants to create or update Vola data from text or a local file path.

## Examples

- `vola write profile/preferences ./preferences.md`
- `vola write memory "Remember this"`
- `vola write project/demo/notes.md ./notes.md`
- `vola write skill/writer/SKILL.md ./SKILL.md`

## Notes

- The second argument may be literal text, `-` for stdin, or a local file path.
- Use `--literal` when an argument that looks like a local path should be treated as plain text.
- `secret` is read-only in the current public surface.
