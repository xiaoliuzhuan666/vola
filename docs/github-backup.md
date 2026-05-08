English | [简体中文](github-backup.zh-CN.md)

# GitHub Backup

GitHub Backup mirrors the user-visible neuDrive file tree into a Git repository. It is meant for recoverable version history: your skills, memory files, project notes, and other public Hub files can be backed up to GitHub and inspected with normal Git tools.

## What Gets Backed Up

GitHub Backup writes the same path structure shown in neuDrive, for example:

```text
skills/...
memory/...
project/...
```

Secrets are not exported. Internal account metadata, connection records, vault scope metadata, billing state, and service implementation details are not written to the backup repository.

## Hosted Mode

Hosted deployments should use GitHub App user authorization.

User flow:

1. Open `GitHub Backup`.
2. Click `Connect GitHub`.
3. After authorization, neuDrive creates or reuses a private repository named `neudrive-backup` under the current GitHub user.
4. Click `Sync now` to write the current neuDrive file tree and push it to `origin/main`.

Hosted mode keeps the ordinary user interface simple:

- Auth mode is fixed to `github_app_user`.
- Remote name is fixed to `origin`.
- Target branch is fixed to `main`.
- Auto commit and auto push are enabled.
- Manual sync is rate limited to one request per 60 seconds by default.

The manual sync cooldown can be changed with:

```bash
GIT_MIRROR_MANUAL_SYNC_COOLDOWN_SECONDS=60
```

Set it to `0` to disable the cooldown.

## Hosted Deployment

Hosted Git working trees live under:

```text
$GIT_MIRROR_HOSTED_ROOT/<user_id>
```

`GIT_MIRROR_HOSTED_ROOT` has no built-in default in hosted mode. If it is missing, sync returns:

```text
GIT_MIRROR_HOSTED_ROOT is not configured
```

Recommended Kubernetes shape:

```yaml
env:
  - name: GIT_MIRROR_HOSTED_ROOT
    value: /data/git-mirrors

volumeMounts:
  - name: git-mirror-data
    mountPath: /data/git-mirrors

volumes:
  - name: git-mirror-data
    persistentVolumeClaim:
      claimName: neudrive-git-mirror
```

The sync code creates each user directory if it does not already exist, but the process must be able to write to the mounted parent path. For multi-pod deployments, use an RWX volume or make sure only one pod runs Git mirror sync workers against a given root.

Hosted GitHub App authorization also needs:

```bash
GITHUB_APP_CLIENT_ID=...
GITHUB_APP_CLIENT_SECRET=...
GITHUB_APP_SLUG=...
PUBLIC_BASE_URL=https://your-neudrive-host
JWT_SECRET=...
```

The GitHub App must request these repository permissions:

- Administration: read and write. Required by GitHub's `POST /user/repos` API when neuDrive creates the private `neudrive-backup` repository.
- Contents: read and write. Required for Git push access to the backup repository.

After changing App permissions in GitHub, users must approve the updated permissions or reconnect GitHub before repository creation is retried.

## Local Mode

Local deployments can choose one of three auth modes:

- Local Git credentials: uses the machine's SSH key or credential helper. For GitHub SSH, use `git@github.com:owner/repo.git`.
- GitHub token: useful for HTTPS repository URLs.
- GitHub App user: same user authorization flow as hosted mode.

Local mode does not rate limit manual `Sync now` by default. You can still set `GIT_MIRROR_MANUAL_SYNC_COOLDOWN_SECONDS` if you want a local cooldown.

CLI equivalents:

```bash
neu git init --output ./neudrive-export/git-mirror
neu git pull
neu git auth github-app --device
```

## Remote Changes And Conflicts

neuDrive treats the mirror as a backup target. If the remote branch has commits that are not in the local mirror, neuDrive blocks a normal push and reports a remote conflict. Review the remote change first, then use the UI overwrite action when you intentionally want neuDrive to replace the remote branch with the current mirror state. Overwrite uses `--force-with-lease`.

## Troubleshooting

- `GIT_MIRROR_HOSTED_ROOT is not configured`: set the env var and mount a writable directory.
- Permission denied under the hosted root: fix PVC ownership with `securityContext.fsGroup` or an init container.
- GitHub HTTPS URL with local credentials: use GitHub token mode, or switch the repository URL to `git@github.com:owner/repo.git`.
- Backup contains no imported files: confirm the current neuDrive user actually owns those files. System skills may be visible even when the user's own `file_tree` is empty.
