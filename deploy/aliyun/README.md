# Codeup + Flow + ACR image build

This path keeps Vola image builds away from the production Tencent Cloud host.

For a customer-facing step-by-step manual, see
[flow-acr-manual-runbook.zh-CN.md](flow-acr-manual-runbook.zh-CN.md).

## Flow setup

1. Use the Codeup repository as the Flow source.
2. Create an Alibaba Cloud Container Registry repository, for example:
   - registry: `crpi-ie94et80ojbqnl7z.cn-shanghai.personal.cr.aliyuncs.com`
   - namespace: `sxhx`
   - repository: `vola`
3. Store these values as Flow variables or a variable group:

```text
ACR_REGISTRY=crpi-ie94et80ojbqnl7z.cn-shanghai.personal.cr.aliyuncs.com
ACR_NAMESPACE=sxhx
ACR_REPOSITORY=vola
ACR_USERNAME=<registry username>
ACR_PASSWORD=<registry password>
IMAGE_TAG=<commit sha or release tag>
PLATFORM=linux/amd64
```

`ACR_PASSWORD` should be the registry password or a RAM credential suitable for pushing images. Do not commit it.

4. Before the first Flow build, mirror the Docker Hub base images into the same ACR repository:

```bash
export ACR_REGISTRY=crpi-ie94et80ojbqnl7z.cn-shanghai.personal.cr.aliyuncs.com
export ACR_NAMESPACE=sxhx
export ACR_REPOSITORY=vola
export ACR_USERNAME=<registry username>
export ACR_PASSWORD=<registry password>
export PLATFORM=linux/amd64

bash deploy/aliyun/push-acr-base-images.sh
```

By default this pushes these tags into the existing `sxhx/vola` repository:

```text
base-node-20-alpine
base-golang-1.25-alpine
base-alpine-3.19
```

If Docker Hub rate limits the local machine too, set `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` before running the script.

5. Add a command step:

```bash
bash deploy/aliyun/flow-build-acr.sh
```

The script builds the root `Dockerfile` using the mirrored ACR base images, pushes both `:${IMAGE_TAG}` and `:latest`, and prints the exact `VOLA_IMAGE=...` value for the server env file. It also prints `NEUDRIVE_IMAGE=...` so older deployment automation can keep reading the legacy name; new deployments should use `VOLA_IMAGE`.

Optional overrides:

```text
BASE_IMAGE_REGISTRY=<registry for base images>
BASE_IMAGE_NAMESPACE=<namespace for base images>
BASE_IMAGE_REPOSITORY=<repository for base images>
NODE_BASE_IMAGE=<full node base image>
GO_BASE_IMAGE=<full golang base image>
RUNTIME_BASE_IMAGE=<full runtime base image>
```

## Notes

- The Tencent Cloud server should not run `docker build` for Vola.
- For ACR Personal Edition, use the repository page's public endpoint as `ACR_REGISTRY`. The VPC endpoint is only for Alibaba Cloud VPC access and should not be used from the Tencent Cloud host.
- If ACR Enterprise Edition uses public access control, allow the Flow build cluster egress address in ACR. If a private build cluster is used, push through the VPC address when configured in Flow.
- The Dockerfile builds a Linux image. For Tencent Lighthouse/CVM x86 hosts, keep `PLATFORM=linux/amd64`.
- If Flow reports Docker Hub `429 Too Many Requests`, make sure the `base-*` image tags above exist in ACR and that `flow-build-acr.sh` is being used.
