# Vola Codeup + ACR + Tencent pull-only deployment

Last updated: 2026-05-19

## Goal

Move Vola image builds away from the Tencent Cloud host that also runs family-growth.

Target flow:

```text
Codeup repository
  -> Cloud Effect Flow builds Docker image
  -> Alibaba Cloud ACR stores image
  -> Tencent Cloud host runs docker compose pull + up
```

## Repository changes

- `deploy/aliyun/flow-build-acr.sh`: builds and pushes the Vola image to ACR.
- `deploy/aliyun/README.md`: Flow variables and build step notes.
- `deploy/tencent/docker-compose.yml`: uses `VOLA_IMAGE`; no server-side Docker build.
- `deploy/tencent/pull-and-deploy.sh`: pull-only deployment script for Tencent Cloud.
- `deploy/tencent/README.md`: server layout, env file, deploy and rollback notes.
- `vola.env.example`: adds `VOLA_IMAGE` and `VOLA_HOST_PORT`.

## Upstream sync check

Checked upstream `agi-bar/Vola.git` on 2026-05-19.

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

Note: the Docker image build does not depend on the local `internal/web/dist` directory. The root `Dockerfile` runs `npm ci`, builds `web/dist`, copies that output into `internal/web/dist`, and then builds the Linux `vola` binary. Run `make build` separately only when publishing local binaries or committing refreshed embedded web assets.

## Codeup repository

Status: repository created and pushed.

```text
Codeup web URL: https://codeup.aliyun.com/69ead0c5fa2a62bc8595a145/sxhx/vola
Git HTTPS URL: https://codeup.aliyun.com/69ead0c5fa2a62bc8595a145/sxhx/vola.git
Local remote: codeup
Visibility: private
Default branch: main
Pushed commit: 805ff50d1c4d4c16c313aec9c3c34c29f59f9913
Commit message: Prepare Vola deployment pipeline
```

Verified:

```text
git ls-remote --symref codeup HEAD refs/heads/main
git remote show codeup
```

## ACR / Flow values

Status: ACR Personal Edition instance, namespace, and private image repository are created.

```text
ACR_REGISTRY=crpi-ie94et80ojbqnl7z.cn-shanghai.personal.cr.aliyuncs.com
ACR_NAMESPACE=sxhx
ACR_REPOSITORY=vola
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
Public image path: crpi-ie94et80ojbqnl7z.cn-shanghai.personal.cr.aliyuncs.com/sxhx/vola:<tag>
VPC image path: crpi-ie94et80ojbqnl7z-vpc.cn-shanghai.personal.cr.aliyuncs.com/sxhx/vola:<tag>
```

Use the public image path from Tencent Cloud. The VPC image path is for Alibaba Cloud VPC access.

## Tencent server values

Status: server directory and pull-only compose files are in place; no Vola container has been started.

```text
APP_DIR=/opt/vola
COMPOSE_PROJECT=vola
VOLA_HOST_PORT=18080
HEALTHCHECK_URL=http://127.0.0.1:18080/api/health
```

The server env file should live at:

```text
/opt/vola/config/vola.env
```

It must include:

```text
VOLA_IMAGE=crpi-ie94et80ojbqnl7z.cn-shanghai.personal.cr.aliyuncs.com/sxhx/vola:<image-tag>
POSTGRES_DB=vola
POSTGRES_USER=vola
POSTGRES_PASSWORD=<server secret>
JWT_SECRET=<server secret>
VAULT_MASTER_KEY=<server secret>
PUBLIC_BASE_URL=http://127.0.0.1:18080
CORS_ORIGINS=http://127.0.0.1:18080
```

Current remote layout:

```text
/opt/vola/
  config/                 # created, mode 700
  deploy/tencent/
    docker-compose.yml
    pull-and-deploy.sh    # executable
```

The production env file has not been written:

```text
/opt/vola/config/vola.env
```

After the first ACR image is available, log in to ACR on the Tencent host with a pull credential:

```bash
docker login crpi-ie94et80ojbqnl7z.cn-shanghai.personal.cr.aliyuncs.com
```

Then create `/opt/vola/config/vola.env` and deploy:

```bash
cd /opt/vola
bash deploy/tencent/pull-and-deploy.sh
```

## Validation

Local checks completed:

```text
bash -n deploy/aliyun/flow-build-acr.sh
bash -n deploy/tencent/pull-and-deploy.sh
VOLA_IMAGE=crpi-ie94et80ojbqnl7z.cn-shanghai.personal.cr.aliyuncs.com/sxhx/vola:test docker compose -f deploy/tencent/docker-compose.yml --env-file vola.env.example config
git diff --check
```

Remote checks:

```text
ssh family-growth-tencent hostname
ssh family-growth-tencent 'ls -ld /opt/vola /opt/vola/config /opt/vola/deploy/tencent'
ssh family-growth-tencent 'docker compose -p vola --env-file /tmp/vola.compose-check.env -f /opt/vola/deploy/tencent/docker-compose.yml config'
```

Latest remote result:

```text
host: VM-0-13-opencloudos
compose config: ok with temporary non-production placeholder env
running Vola containers: none
```

Not validated yet:

- ACR image push from Flow.
- Tencent host `docker pull` from ACR.
- Tencent host `docker login` with the ACR Registry password.
- `/opt/vola/config/vola.env` with real production secrets.
- Vola runtime health check on `http://127.0.0.1:18080/api/health`.

## Guardrails

- Do not run `docker build` or `docker compose up --build` on the shared Tencent host.
- Do not use the root `docker-compose.yml` on the shared Tencent host.
- Do not reuse family-growth ports: `3005`, `8100`.
- Do not attach Vola to `growth.sunningfun.cn` or shared `/api/` routes.
- Keep ACR credentials, database passwords, JWT secrets, COS secrets, and vault keys outside Git.
