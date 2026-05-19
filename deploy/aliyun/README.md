# Codeup + Flow + ACR image build

This path keeps neuDrive image builds away from the production Tencent Cloud host.

## Flow setup

1. Use the Codeup repository as the Flow source.
2. Create an Alibaba Cloud Container Registry repository, for example:
   - registry: `crpi-ie94et80ojbqnl7z.cn-shanghai.personal.cr.aliyuncs.com`
   - namespace: `sxhx`
   - repository: `neudrive`
3. Store these values as Flow variables or a variable group:

```text
ACR_REGISTRY=crpi-ie94et80ojbqnl7z.cn-shanghai.personal.cr.aliyuncs.com
ACR_NAMESPACE=sxhx
ACR_REPOSITORY=neudrive
ACR_USERNAME=<registry username>
ACR_PASSWORD=<registry password>
IMAGE_TAG=<commit sha or release tag>
PLATFORM=linux/amd64
```

`ACR_PASSWORD` should be the registry password or a RAM credential suitable for pushing images. Do not commit it.

4. Add a command step:

```bash
bash deploy/aliyun/flow-build-acr.sh
```

The script builds the root `Dockerfile`, pushes both `:${IMAGE_TAG}` and `:latest`, and prints the exact `NEUDRIVE_IMAGE=...` value for the server env file.

## Notes

- The Tencent Cloud server should not run `docker build` for neuDrive.
- For ACR Personal Edition, use the repository page's public endpoint as `ACR_REGISTRY`. The VPC endpoint is only for Alibaba Cloud VPC access and should not be used from the Tencent Cloud host.
- If ACR Enterprise Edition uses public access control, allow the Flow build cluster egress address in ACR. If a private build cluster is used, push through the VPC address when configured in Flow.
- The Dockerfile builds a Linux image. For Tencent Lighthouse/CVM x86 hosts, keep `PLATFORM=linux/amd64`.
