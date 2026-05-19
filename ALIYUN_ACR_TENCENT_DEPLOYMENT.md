# neuDrive Codeup + ACR + Tencent pull-only deployment

Last updated: 2026-05-19

## Goal

Move neuDrive image builds away from the Tencent Cloud host that also runs family-growth.

Target flow:

```text
Codeup repository
  -> Cloud Effect Flow builds Docker image
  -> Alibaba Cloud ACR stores image
  -> Tencent Cloud host runs docker compose pull + up
```

## Repository changes

- `deploy/aliyun/flow-build-acr.sh`: builds and pushes the neuDrive image to ACR.
- `deploy/aliyun/README.md`: Flow variables and build step notes.
- `deploy/tencent/docker-compose.yml`: uses `NEUDRIVE_IMAGE`; no server-side Docker build.
- `deploy/tencent/pull-and-deploy.sh`: pull-only deployment script for Tencent Cloud.
- `deploy/tencent/README.md`: server layout, env file, deploy and rollback notes.
- `neudrive.env.example`: adds `NEUDRIVE_IMAGE` and `NEUDRIVE_HOST_PORT`.

## Upstream sync check

Checked upstream `agi-bar/neuDrive.git` on 2026-05-19.

Local branch:

```text
HEAD=2f255a154bcdd5ffc1328ddddb8edc5160e3eb12
origin/main=450bdacfd94e72c62086da91b4172e431e2636e9
```

Upstream had 5 newer commits:

```text
437ab3c Use triage SDK for feedback launch
1f2b7e6 Fix GitHub App permission approval flow
08cf5e5 Add gated feedback entry points
7f202f6 Fix feedback launch CI and unify public heroes
450bdac Preserve feedback intent through login
```

Decision:

- Absorbed the GitHub App permission approval flow fix because it affects existing GitHub Backup behavior.
- Did not absorb the feedback launch changes. They are product-surface changes and should be reviewed separately before merging into this customized fork.

Validated:

```text
go test ./internal/localgitsync
go test ./internal/api -run 'TestHostedGitMirrorDefaultBackupRepo(DisconnectsStaleGitHubAppConnection|CreatesAndReuses|RequiresGitHubAppConnection)'
cd web && npm run build
git diff --check -- internal/api/errors.go internal/api/git_mirror.go internal/localgitsync/github_app.go internal/api/sqlite_shared_test.go internal/localgitsync/github_app_test.go web/src/pages/GitMirrorPage.tsx
```

Note: `make build` was not run after the web build in this deployment pass, so the embedded `internal/web/dist` bundle should be refreshed before a production release commit.

## Codeup repository

Status: repository created, not pushed yet.

```text
Codeup web URL: https://codeup.aliyun.com/69ead0c5fa2a62bc8595a145/sxhx/neudrive
Git HTTPS URL: https://codeup.aliyun.com/69ead0c5fa2a62bc8595a145/sxhx/neudrive.git
Local remote: codeup
Visibility: private
Initial refs: empty
```

Reason not pushed yet:

- The local neuDrive worktree contains many existing uncommitted changes from ongoing secondary development.
- Pushing the current branch directly would mix deployment scaffolding, upstream cherry-picks, generated web assets, and unrelated feature work into Codeup.
- Create a clean release commit or a dedicated Codeup branch before pushing.

## ACR / Flow values

Status: ACR Personal Edition instance, namespace, and private image repository are created.

```text
ACR_REGISTRY=crpi-ie94et80ojbqnl7z.cn-shanghai.personal.cr.aliyuncs.com
ACR_NAMESPACE=sxhx
ACR_REPOSITORY=neudrive
ACR_REPOSITORY_TYPE=private
ACR_REPOSITORY_SOURCE=local repository
IMAGE_TAG=<commit-sha-or-release-tag>
PLATFORM=linux/amd64
```

Sensitive values are not recorded in this file:

```text
ACR_USERNAME=<stored in Flow variable group>
ACR_PASSWORD=<stored in Flow variable group>
```

Manual actions still required in Alibaba Cloud:

1. Put `ACR_USERNAME` and `ACR_PASSWORD` into Cloud Effect Flow variables or a protected variable group.
2. Configure Flow to use the Codeup repository and run:

```bash
bash deploy/aliyun/flow-build-acr.sh
```

Created ACR repository:

```text
Public registry host: crpi-ie94et80ojbqnl7z.cn-shanghai.personal.cr.aliyuncs.com
Public image path: crpi-ie94et80ojbqnl7z.cn-shanghai.personal.cr.aliyuncs.com/sxhx/neudrive:<tag>
VPC image path: crpi-ie94et80ojbqnl7z-vpc.cn-shanghai.personal.cr.aliyuncs.com/sxhx/neudrive:<tag>
```

Use the public image path from Tencent Cloud. The VPC image path is for Alibaba Cloud VPC access.

## Tencent server values

Status: server directory and pull-only compose files are in place; no neuDrive container has been started.

```text
APP_DIR=/opt/neudrive
COMPOSE_PROJECT=neudrive
NEUDRIVE_HOST_PORT=18080
HEALTHCHECK_URL=http://127.0.0.1:18080/api/health
```

The server env file should live at:

```text
/opt/neudrive/config/neudrive.env
```

It must include:

```text
NEUDRIVE_IMAGE=crpi-ie94et80ojbqnl7z.cn-shanghai.personal.cr.aliyuncs.com/sxhx/neudrive:<image-tag>
POSTGRES_DB=neudrive
POSTGRES_USER=neudrive
POSTGRES_PASSWORD=<server secret>
JWT_SECRET=<server secret>
VAULT_MASTER_KEY=<server secret>
PUBLIC_BASE_URL=http://127.0.0.1:18080
CORS_ORIGINS=http://127.0.0.1:18080
```

Current remote layout:

```text
/opt/neudrive/
  config/                 # created, mode 700
  deploy/tencent/
    docker-compose.yml
    pull-and-deploy.sh    # executable
```

The production env file has not been written:

```text
/opt/neudrive/config/neudrive.env
```

After the first ACR image is available, log in to ACR on the Tencent host with a pull credential:

```bash
docker login crpi-ie94et80ojbqnl7z.cn-shanghai.personal.cr.aliyuncs.com
```

Then create `/opt/neudrive/config/neudrive.env` and deploy:

```bash
cd /opt/neudrive
bash deploy/tencent/pull-and-deploy.sh
```

## Validation

Local checks completed:

```text
bash -n deploy/aliyun/flow-build-acr.sh
bash -n deploy/tencent/pull-and-deploy.sh
NEUDRIVE_IMAGE=crpi-ie94et80ojbqnl7z.cn-shanghai.personal.cr.aliyuncs.com/sxhx/neudrive:test docker compose -f deploy/tencent/docker-compose.yml --env-file neudrive.env.example config
git diff --check
```

Remote checks:

```text
ssh family-growth-tencent hostname
ssh family-growth-tencent 'ls -ld /opt/neudrive /opt/neudrive/config /opt/neudrive/deploy/tencent'
ssh family-growth-tencent 'docker compose -p neudrive --env-file /tmp/neudrive.compose-check.env -f /opt/neudrive/deploy/tencent/docker-compose.yml config'
```

Latest remote result:

```text
host: VM-0-13-opencloudos
compose config: ok with temporary non-production placeholder env
running neuDrive containers: none
```

Not validated yet:

- ACR image push from Flow.
- Tencent host `docker pull` from ACR.
- Tencent host `docker login` with the ACR Registry password.
- `/opt/neudrive/config/neudrive.env` with real production secrets.
- neuDrive runtime health check on `http://127.0.0.1:18080/api/health`.

## Guardrails

- Do not run `docker build` or `docker compose up --build` on the shared Tencent host.
- Do not use the root `docker-compose.yml` on the shared Tencent host.
- Do not reuse family-growth ports: `3005`, `8100`.
- Do not attach neuDrive to `growth.sunningfun.cn` or shared `/api/` routes.
- Keep ACR credentials, database passwords, JWT secrets, COS secrets, and vault keys outside Git.
