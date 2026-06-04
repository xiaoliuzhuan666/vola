# Vola 自定义域名配置：Nginx 命令版

更新时间：2026-05-20

本文用于当前腾讯云独立部署形态：

```text
远端目录：/opt/vola
Compose 项目：vola
应用容器：vola-server
数据库容器：vola-postgres
应用监听：127.0.0.1:18080 -> vola-server:8080
Nginx 对外监听：80 / 443
```

当前目标域名：

```text
https://driver.sunningfun.cn
```

不要开放 `18080` 到公网。`18080` 只给服务器本机 Nginx 访问。

## 当前完成状态

2026-05-20 已完成自定义域名配置，并通过以下验证：

```text
dig +short driver.sunningfun.cn -> 110.40.228.66
http://driver.sunningfun.cn/ -> 301 到 https://driver.sunningfun.cn/
https://driver.sunningfun.cn/ -> HTTP 200
https://driver.sunningfun.cn/api/health -> ok: true，storage: postgres
nginx -t -> successful
vola-server -> 127.0.0.1:18080->8080/tcp
容器内 PUBLIC_BASE_URL -> https://driver.sunningfun.cn
容器内 CORS_ORIGINS -> https://driver.sunningfun.cn
站点配置已补充 client_max_body_size 2048m
反向代理已补充 X-Forwarded-Host、X-Forwarded-Proto、600s proxy timeout
```

远端备份文件：

```text
/opt/vola/config/vola.env.bak-20260520134847
/www/server/panel/vhost/nginx/driver.sunningfun.cn.conf.bak-20260520135242
/www/server/panel/vhost/nginx/proxy/driver.sunningfun.cn/736703f33dd3502ed9167dc49d000f7e_driver.sunningfun.cn.conf.bak-20260520135242
```

## 1. 准备信息

把下面占位值换成真实值：

```text
DOMAIN=driver.sunningfun.cn
SERVER_IP=110.40.228.66
UPSTREAM=http://127.0.0.1:18080
```

`SERVER_IP` 以腾讯云 Lighthouse/CVM 控制台显示的公网 IP 为准。

## 2. DNS 解析

在 DNSPod 或域名所在 DNS 控制台添加 A 记录：

```text
主机记录：driver
记录类型：A
线路类型：默认
记录值：110.40.228.66
TTL：600
```

说明：

- `driver` 对应 `driver.sunningfun.cn`。
- 如果要用根域名 `example.com`，主机记录通常填 `@`。
- DNS 记录只写 IP，不写端口。
- 如果域名在中国大陆对外访问，通常需要 ICP 备案。

本地确认解析：

```bash
dig +short driver.sunningfun.cn
```

看到腾讯云服务器公网 IP 后继续。

## 3. 确认服务器端口

在腾讯云安全组或 Lighthouse 防火墙里确认开放：

```text
80/tcp
443/tcp
```

不要开放：

```text
18080/tcp
5432/tcp
```

在服务器确认 Vola 本机访问正常：

```bash
curl -fsS http://127.0.0.1:18080/api/health
curl -I http://127.0.0.1:18080/
docker compose -p vola ps
```

预期：

```text
/api/health 返回 ok: true
根页面返回 HTTP 200
vola-server 和 vola-postgres 为 running
```

## 4. 更新 Vola 外部地址配置

如果域名正式启用，建议把远端环境文件里的外部地址改成域名。这样 GitHub App OAuth、回调地址、跨域来源等场景不会继续使用本机地址。

远端文件：

```text
/opt/vola/config/vola.env
```

建议值：

```text
PUBLIC_BASE_URL=https://driver.sunningfun.cn
CORS_ORIGINS=https://driver.sunningfun.cn
```

改完后重启 Vola：

```bash
cd /opt/vola
bash deploy/tencent/pull-and-deploy.sh
```

再确认本机健康检查：

```bash
curl -fsS http://127.0.0.1:18080/api/health
```

## 5. HTTP 反向代理配置

新建独立 Nginx 配置文件。宝塔常见路径：

```text
/www/server/panel/vhost/nginx/vola.conf
```

如果不是宝塔 Nginx，请放到当前 Nginx 的 `conf.d` 或站点配置目录。

HTTP 配置：

```nginx
server {
    listen 80;
    server_name driver.sunningfun.cn;

    client_max_body_size 2048m;

    access_log /www/wwwlogs/vola.access.log;
    error_log /www/wwwlogs/vola.error.log;

    location / {
        proxy_pass http://127.0.0.1:18080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 600s;
        proxy_send_timeout 600s;
    }
}
```

检查并重载 Nginx：

```bash
nginx -t
nginx -s reload
```

外网验证：

```bash
curl -I http://driver.sunningfun.cn/
curl -fsS http://driver.sunningfun.cn/api/health
```

## 6. HTTPS 证书

推荐路径：

1. 在腾讯云 SSL 证书控制台申请免费证书。
2. 申请域名填 `driver.sunningfun.cn`。
3. 用 DNS 验证完成签发。
4. 下载 Nginx 格式证书。
5. 上传到服务器，例如：

```text
/www/server/panel/vhost/cert/vola/fullchain.pem
/www/server/panel/vhost/cert/vola/privkey.key
```

HTTPS 配置：

```nginx
server {
    listen 80;
    server_name driver.sunningfun.cn;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name driver.sunningfun.cn;

    ssl_certificate /www/server/panel/vhost/cert/vola/fullchain.pem;
    ssl_certificate_key /www/server/panel/vhost/cert/vola/privkey.key;
    ssl_protocols TLSv1.2 TLSv1.3;

    client_max_body_size 2048m;

    access_log /www/wwwlogs/vola.access.log;
    error_log /www/wwwlogs/vola.error.log;

    location / {
        proxy_pass http://127.0.0.1:18080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 600s;
        proxy_send_timeout 600s;
    }
}
```

检查并重载：

```bash
nginx -t
nginx -s reload
```

HTTPS 验证：

```bash
curl -I https://driver.sunningfun.cn/
curl -fsS https://driver.sunningfun.cn/api/health
```

## 7. 验收清单

```text
[x] DNS A 记录指向腾讯云公网 IP
[x] 腾讯云安全组开放 80 和 443
[x] 18080 未开放公网
[x] http://127.0.0.1:18080/api/health 正常
[x] Nginx 独立 vhost 文件已创建
[x] nginx -t 通过
[x] http://driver.sunningfun.cn/ 返回 301 到 HTTPS
[x] https://driver.sunningfun.cn/ 返回 200
[x] https://driver.sunningfun.cn/api/health 返回 ok: true
[x] /opt/vola/config/vola.env 已设置 PUBLIC_BASE_URL 和 CORS_ORIGINS
```

## 8. 不要改的东西

- 不要改 `growth.sunningfun.cn`。
- 不要复用已有 `/api/` 公共路径。
- 不要占用 `3005`、`8100` 等已有项目端口。
- 不要对 Vola 使用 `docker compose up --build`。
- 不要把数据库、JWT、Vault、ACR 密码写入文档。

## 9. 参考

- 腾讯云 DNSPod A 记录文档：https://staticintl.cloudcachetci.com/doc/pdf/product/pdf/1295_76971_zh.pdf
- 腾讯云 SSL 证书部署到服务器概览：https://cloud.tencent.com/document/product/400/4143
- 宝塔反向代理文档：https://docs.bt.cn/user-guide/site/php/site-config/reverse-proxy
- 宝塔 SSL 证书部署文档：https://docs.bt.cn/10.0/getting-started/deploy-ssl
