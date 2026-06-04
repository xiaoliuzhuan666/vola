English | [简体中文](github-backup.zh-CN.md)

# GitHub Backup

GitHub Backup mirrors the user-visible Vola file tree into a Git repository. It is meant for recoverable version history: your skills, memory files, project notes, and other public Hub files can be backed up to GitHub and inspected with normal Git tools.

## Storage Model

Vola data has three layers:

- Primary storage: hosted deployments usually use Postgres, and local mode usually uses SQLite. The current Hub state is written here first.
- Git working tree: Vola writes the user-visible file tree into a Git working copy so it can create version history.
- Remote backup: GitHub Backup pushes the Git working tree to GitHub. WebDAV, S3-compatible, OSS, and R2 targets upload a Vola export zip so the backup leaves the current machine or server.

GitHub Backup does not replace database backups. It stores the user-visible file tree and is useful for recovering skills, team library files, memory files, project notes, and similar content. Accounts, connections, billing, vault scope metadata, and secrets still require database backups or service configuration for recovery.

The current remote backup options are GitHub version history and WebDAV / S3-compatible export archive upload. R2, OSS, and similar object stores should be configured through an endpoint that supports S3 Signature Version 4.

## What Gets Backed Up

GitHub Backup writes the same path structure shown in Vola, for example:

```text
skills/...
team/...
memory/...
project/...
```

Secrets are not exported. Internal account metadata, connection records, vault scope metadata, billing state, and service implementation details are not written to the backup repository.

## Recovery

After GitHub sync has succeeded, clone the remote repository to inspect historical versions:

```bash
git clone https://github.com/<owner>/vola-backup.git
```

To recover a single Skill, memory file, or project file, retrieve the relevant path from Git history and write it back through Vola import or sync commands. Full service recovery still needs a database backup because GitHub Backup does not contain internal account data or secrets.

You can import a recovered local file tree with:

```bash
neu sync push --source ./recovered-vola-files
```

Check the directory before running the command so it contains only the files you want to write back into Vola.

If the backup was uploaded to WebDAV or an S3-compatible target, download the matching `vola-export-*.zip`. This zip is a Vola export archive containing recoverable file tree content, profile, memory, projects, roles, inbox data, and vault scope metadata. It does not contain secret plaintext.

Recovery flow:

1. Download the zip from WebDAV or object storage.
2. Unzip it and review the `export/` directory.
3. Import the files you need back into Vola, or prepare a local directory and run `neu sync push --source <dir>`.

Full service recovery still needs a database backup. GitHub / WebDAV / S3-compatible backups cover user-visible data and portable export archives, not login sessions, connection tokens, billing state, or secret plaintext.

## WebDAV / S3-Compatible External Backups

You can add a target from the `External backup targets` section at the bottom of `GitHub Backup`.

WebDAV targets need:

- A WebDAV folder URL, such as a Nextcloud folder, Nutstore folder, or self-hosted WebDAV directory.
- Username.
- Password or app password. It is not shown again after saving.

When you upload, Vola creates `vola-export-YYYYMMDD-HHMMSSZ.zip` and uploads it with WebDAV `PUT`. If the object path includes nested folders, the service tries to create them with `MKCOL`.

S3-compatible targets need:

- Endpoint, such as Cloudflare R2, MinIO, or an OSS endpoint that supports the S3 API.
- Bucket.
- Region. R2 can use `auto`; other services should use the region required by that provider.
- Optional prefix.
- Access key ID and secret access key. The secret is not shown again after saving.
- URL style. Path-style is the default: `<endpoint>/<bucket>/<prefix>/<object>`. Turn off Path-style URL when a provider requires virtual-hosted style.

S3-compatible upload uses AWS Signature Version 4. Object names use the same `vola-export-YYYYMMDD-HHMMSSZ.zip` format.

## Hosted Mode

Hosted deployments should use GitHub App user authorization.

User flow:

1. Open `GitHub Backup`.
2. Click `Connect GitHub`.
3. After authorization, Vola creates or reuses a private repository named `vola-backup` under the current GitHub user.
4. Click `Sync now` to write the current Vola file tree and push it to `origin/main`.

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

The app itself has no built-in `GIT_MIRROR_HOSTED_ROOT` default in hosted mode. If it is missing, sync returns:

```text
GIT_MIRROR_HOSTED_ROOT is not configured
```

`deploy/prod/deploy.sh` writes `/data/git-mirrors` into the production ConfigMap by default. If you deploy another way, set this environment variable yourself and mount a writable directory.

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
      claimName: vola-git-mirrors
```

The sync code creates each user directory if it does not already exist, but the process must be able to write to the mounted parent path. For multi-pod deployments, use an RWX volume or make sure only one pod runs Git mirror sync workers against a given root.

Hosted GitHub App authorization also needs:

```bash
GITHUB_APP_CLIENT_ID=...
GITHUB_APP_CLIENT_SECRET=...
GITHUB_APP_SLUG=...
PUBLIC_BASE_URL=https://your-vola-host
JWT_SECRET=...
```

The GitHub App must request these repository permissions:

- Administration: read and write. Required by GitHub's `POST /user/repos` API when Vola creates the private `vola-backup` repository.
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
neu git init --output ./vola-export/git-mirror
neu git pull
neu git auth github-app --device
```

## Remote Changes And Conflicts

Vola treats the mirror as a backup target. If the remote branch has commits that are not in the local mirror, Vola blocks a normal push and reports a remote conflict. Review the remote change first, then use the UI overwrite action when you intentionally want Vola to replace the remote branch with the current mirror state. Overwrite uses `--force-with-lease`.

## Troubleshooting

- `GIT_MIRROR_HOSTED_ROOT is not configured`: set the env var and mount a writable directory.
- Permission denied under the hosted root: fix PVC ownership with `securityContext.fsGroup` or an init container.
- GitHub HTTPS URL with local credentials: use GitHub token mode, or switch the repository URL to `git@github.com:owner/repo.git`.
- Backup contains no imported files: confirm the current Vola user actually owns those files. System skills may be visible even when the user's own `file_tree` is empty.
