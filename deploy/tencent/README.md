# Tencent Cloud pull-only deployment

This deployment mode expects the neuDrive image to be built in Codeup/Flow and pushed to Alibaba Cloud ACR. The Tencent Cloud host only pulls and restarts containers.

## Server layout

```text
/opt/neudrive/
  config/neudrive.env
  deploy/tencent/docker-compose.yml
  deploy/tencent/pull-and-deploy.sh
```

The app is bound to loopback only:

```text
127.0.0.1:18080 -> neudrive-server:8080
```

Do not reuse the existing family-growth ports or paths:

```text
127.0.0.1:3005
127.0.0.1:8100
growth.sunningfun.cn
/api/
```

## Required env

Set these in `/opt/neudrive/config/neudrive.env`:

```text
NEUDRIVE_IMAGE=crpi-ie94et80ojbqnl7z.cn-shanghai.personal.cr.aliyuncs.com/sxhx/neudrive:<image-tag>
NEUDRIVE_HOST_PORT=18080
POSTGRES_DB=neudrive
POSTGRES_USER=neudrive
POSTGRES_PASSWORD=<server secret>
JWT_SECRET=<server secret>
VAULT_MASTER_KEY=<server secret>
PUBLIC_BASE_URL=http://127.0.0.1:18080
CORS_ORIGINS=http://127.0.0.1:18080
```

Keep ACR credentials out of this file if possible. Run `docker login` once on the server with a pull-only credential:

```bash
docker login crpi-ie94et80ojbqnl7z.cn-shanghai.personal.cr.aliyuncs.com
```

## Deploy

```bash
cd /opt/neudrive
bash deploy/tencent/pull-and-deploy.sh
```

The script runs:

```text
docker compose pull db server
docker compose up -d db server
curl http://127.0.0.1:18080/api/health
```

## Rollback

Change `NEUDRIVE_IMAGE` in `config/neudrive.env` to a previous tag, then run:

```bash
bash deploy/tencent/pull-and-deploy.sh
```
