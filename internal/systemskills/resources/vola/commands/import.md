# `vola import`

Use this command when the user wants to bring local or platform data into Vola.

## Public Forms

- `vola import <platform> [--dry-run] [--raw] [--zip FILE]`
- `vola import skill <local-dir> [--name NAME]`
- `vola import profile <local-file> [--category preferences|relationships|principles]`
- `vola import memory <local-file-or-dir>`
- `vola import project <local-file-or-dir> [--name NAME]`

## Notes

- Platform names can appear directly after `import`; other categories still use the explicit noun form like `import skill` or `import memory`.
- A leading `/` is optional when the user writes category-like paths.
- If the user already enabled local Git Mirror, remind them that later Hub writes and imports are mirrored there automatically, but GitHub push still requires normal Git credentials and repo setup in that directory.
