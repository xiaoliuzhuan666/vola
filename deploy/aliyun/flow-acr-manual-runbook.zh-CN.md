# 云效 Flow 打包并推送 ACR 镜像操作手册

本文档给客户或实施人员手动配置使用。目标流程是：

Codeup 代码仓库 -> 云效 Flow 构建 Docker 镜像 -> 推送到阿里云 ACR。

## 1. 需要提前准备

### 1.1 ACR 信息

在阿里云容器镜像服务 ACR 中确认这些信息：

```text
ACR_REGISTRY=<ACR 公网访问地址>
ACR_NAMESPACE=<命名空间>
ACR_REPOSITORY=<镜像仓库名>
ACR_USERNAME=<ACR 登录用户名>
ACR_PASSWORD=<ACR 登录密码>
PLATFORM=linux/amd64
```

示例：

```text
ACR_REGISTRY=crpi-ie94et80ojbqnl7z.cn-shanghai.personal.cr.aliyuncs.com
ACR_NAMESPACE=sxhx
ACR_REPOSITORY=vola
PLATFORM=linux/amd64
```

密码不要写进代码仓库。只填写到 Flow 环境变量里。

### 1.2 本地 Docker 登录 ACR

在本地电脑执行：

```bash
docker login --username=<ACR 登录用户名> <ACR 公网访问地址>
```

输入 ACR 登录密码。看到 `Login Succeeded` 表示登录成功。

## 2. 本地准备 ACR 基础镜像

这一步只需要在第一次配置时执行，或者基础镜像版本变化后再执行。

在项目根目录执行：

```bash
export ACR_REGISTRY=<ACR 公网访问地址>
export ACR_NAMESPACE=<命名空间>
export ACR_REPOSITORY=<镜像仓库名>
export ACR_USERNAME=<ACR 登录用户名>
export ACR_PASSWORD=<ACR 登录密码>
export PLATFORM=linux/amd64

bash deploy/aliyun/push-acr-base-images.sh
```

成功后，ACR 镜像仓库里会出现这些 tag：

```text
base-node-20-alpine
base-golang-1.25-alpine
base-alpine-3.19
```

如果本地也遇到 Docker Hub 限流，可以先设置 Docker Hub 账号：

```bash
export DOCKERHUB_USERNAME=<Docker Hub 用户名>
export DOCKERHUB_TOKEN=<Docker Hub Token>
```

再重新执行 `bash deploy/aliyun/push-acr-base-images.sh`。

## 3. 云效 Flow 页面配置

进入云效 Flow 流水线编辑页，配置一个“执行命令”任务。

### 3.1 流水线源

| 字段 | 填写 |
| --- | --- |
| 代码源 | Codeup 项目仓库 |
| 分支 | `main`，或客户实际发布分支 |
| 下载流水线源 | 下载全部流水线源 |

### 3.2 构建任务

| 字段 | 填写 |
| --- | --- |
| 任务名称 | `Build and push Vola image` |
| 构建集群 | 可访问 ACR 公网地址的构建集群，例如云效中国香港构建集群 |
| 指定构建节点 | `Linux/amd64` |
| 构建环境 | 指定容器环境 |
| 容器镜像地址 | `build-steps/alinux3` |
| 开启 Docker Daemon | 打开 |
| 超时时间 | `240` 分钟 |

### 3.3 任务命令

执行命令只填写这一行：

```bash
bash deploy/aliyun/flow-build-acr.sh
```

不要在这里粘贴 Docker daemon 配置脚本，也不要再粘贴 Docker Hub 登录脚本。构建逻辑已经放在仓库脚本里。

### 3.4 环境变量

在任务步骤的环境变量区域填写：

| 名称 | 值 |
| --- | --- |
| `ACR_REGISTRY` | ACR 公网访问地址 |
| `ACR_NAMESPACE` | ACR 命名空间 |
| `ACR_REPOSITORY` | ACR 镜像仓库名 |
| `ACR_USERNAME` | ACR 登录用户名 |
| `ACR_PASSWORD` | ACR 登录密码 |
| `PLATFORM` | `linux/amd64` |

可选变量：

| 名称 | 用途 |
| --- | --- |
| `IMAGE_TAG` | 指定镜像 tag；不填时默认使用当前 commit 短 SHA |
| `NODE_BASE_IMAGE` | 覆盖 Node 基础镜像完整地址 |
| `GO_BASE_IMAGE` | 覆盖 Golang 基础镜像完整地址 |
| `RUNTIME_BASE_IMAGE` | 覆盖运行时基础镜像完整地址 |

## 4. 保存并运行

1. 点击右上角“保存并运行”。
2. 运行配置里选择分支，例如 `main`。
3. 点击“运行”。
4. 打开任务日志。

## 5. 成功日志应该包含什么

日志里应出现：

```text
User Command Content
bash deploy/aliyun/flow-build-acr.sh

Login Succeeded
Using base images:
NODE_BASE_IMAGE=<ACR 地址>/<命名空间>/<仓库>:base-node-20-alpine
GO_BASE_IMAGE=<ACR 地址>/<命名空间>/<仓库>:base-golang-1.25-alpine
RUNTIME_BASE_IMAGE=<ACR 地址>/<命名空间>/<仓库>:base-alpine-3.19
```

构建阶段应能看到类似：

```text
load metadata for <ACR 地址>/<命名空间>/<仓库>:base-golang-1.25-alpine
load metadata for <ACR 地址>/<命名空间>/<仓库>:base-node-20-alpine
```

日志末尾应出现：

```text
pushing <ACR 地址>/<命名空间>/<仓库>:latest with docker
VOLA_IMAGE=<ACR 地址>/<命名空间>/<仓库>:<镜像 tag>
run step successfully!
```

以本次成功运行为例，最终镜像是：

```text
crpi-ie94et80ojbqnl7z.cn-shanghai.personal.cr.aliyuncs.com/sxhx/vola:40fe7f8a972a
```

## 6. 常见错误

| 现象 | 处理 |
| --- | --- |
| `429 Too Many Requests` | Flow 还在拉 Docker Hub。确认 ACR 里已有 `base-*` tags，并确认 Flow 命令是 `bash deploy/aliyun/flow-build-acr.sh`。 |
| Docker Hub timeout | 同上。不要依赖 Flow 现场拉 Docker Hub 基础镜像。 |
| `UndefinedArgInFrom` | Dockerfile 的 `ARG` 位置不对。`NODE_BASE_IMAGE`、`GO_BASE_IMAGE`、`RUNTIME_BASE_IMAGE` 要放在所有 `FROM` 前面。 |
| `No such file or directory`，并出现 `flow-build-acr.shset` | Flow 命令内容粘贴错了，两段脚本连在一起了。清空命令框，只保留 `bash deploy/aliyun/flow-build-acr.sh`。 |
| `unauthorized` 或 `Login denied` | ACR 用户名、密码或仓库权限不对。重新检查 Flow 环境变量。不要把密码写进代码仓库。 |
| 构建成功但部署机器拉不到镜像 | 确认部署机器使用的是 ACR 公网地址；如果 ACR 开了访问控制，需要放行部署机器出口 IP。 |

## 7. 交付检查清单

- [ ] ACR 已有 `base-node-20-alpine`
- [ ] ACR 已有 `base-golang-1.25-alpine`
- [ ] ACR 已有 `base-alpine-3.19`
- [ ] Flow 命令只有 `bash deploy/aliyun/flow-build-acr.sh`
- [ ] Flow 环境变量里有 ACR 登录信息，密码没有提交到代码仓库
- [ ] Flow 日志打印了 `Using base images`
- [ ] Flow 日志打印了 `VOLA_IMAGE=...`
- [ ] Flow 页面显示运行成功
