# Vola 自定义域名配置：宝塔面板版

更新时间：2026-05-20

本文用于在已安装宝塔面板的腾讯云服务器上，通过可视化界面给 Vola 配置自定义域名。

当前 Vola 部署信息：

```text
远端目录：/opt/vola
Compose 项目：vola
容器：vola-server、vola-postgres
应用本机地址：http://127.0.0.1:18080
公网入口：宝塔 Nginx 80 / 443
```

当前域名：

```text
driver.sunningfun.cn
```

本文按当前线上域名记录：`driver.sunningfun.cn`。

## 当前完成状态

2026-05-20 已完成第 10 步，并通过以下验证：

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

## 1. DNS 解析

进入域名 DNS 控制台，通常是腾讯云 DNSPod：

```text
DNSPod 控制台 -> 我的域名 -> 选择域名 -> 记录管理 -> 添加记录
```

添加 A 记录：

```text
主机记录：driver
记录类型：A
线路类型：默认
记录值：110.40.228.66
TTL：600
```

说明：

- `driver` 表示完整域名是 `driver.sunningfun.cn`。
- `110.40.228.66` 以腾讯云服务器控制台显示的公网 IP 为准。
- DNS 里不填 `18080`。
- 如果使用中国大陆服务器对外提供网站访问，域名通常需要 ICP 备案。

等待解析生效后，在本地终端确认：

```bash
dig +short driver.sunningfun.cn
```

返回服务器公网 IP 后继续。

## 2. 腾讯云防火墙

进入腾讯云 Lighthouse/CVM 控制台，确认安全组或防火墙开放：

```text
80/tcp
443/tcp
```

不要开放：

```text
18080/tcp
5432/tcp
```

`18080` 只允许服务器本机访问，外网访问交给宝塔 Nginx。

## 3. 确认 Vola 服务

在宝塔面板：

```text
终端 -> 输入服务器命令
```

执行：

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

如果这里不正常，先处理 Vola 容器，不要继续改宝塔网站配置。

## 4. 宝塔添加站点

进入宝塔面板：

```text
网站 -> 添加站点
```

填写：

```text
域名：driver.sunningfun.cn
备注：vola
根目录：/www/wwwroot/vola
FTP：不创建
数据库：不创建
PHP 版本：纯静态 / 不使用 PHP
```

说明：

- 根目录只是给宝塔创建站点用，Vola 真实服务仍在 Docker 容器里。
- 不要把站点目录设成 `/opt/vola`。
- 不要改已有 family-growth、model-pay-platform 的站点。

## 5. 宝塔设置反向代理

进入刚创建的站点：

```text
网站 -> driver.sunningfun.cn -> 设置 -> 反向代理 -> 添加反向代理
```

填写：

```text
代理名称：vola
目标 URL：http://127.0.0.1:18080
发送域名：$host
内容替换：留空
缓存：关闭
```

保存后启用反向代理。

如果宝塔界面没有显示“发送域名”，填真实域名也可以：

```text
driver.sunningfun.cn
```

## 6. 宝塔配置文件参数

进入：

```text
网站 -> driver.sunningfun.cn -> 设置 -> 配置文件
```

在该站点的 `server { ... }` 内确认有上传大小和代理超时配置。没有的话添加：

```nginx
client_max_body_size 2048m;
proxy_read_timeout 600s;
proxy_send_timeout 600s;
```

反向代理的 `location /` 最好包含这些请求头：

```nginx
proxy_set_header Host $host;
proxy_set_header X-Real-IP $remote_addr;
proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
proxy_set_header X-Forwarded-Host $host;
proxy_set_header X-Forwarded-Proto $scheme;
proxy_set_header Upgrade $http_upgrade;
proxy_set_header Connection "upgrade";
```

宝塔自动生成的反向代理配置通常已经包含一部分请求头。不要大范围改宝塔生成的配置，只确认缺少的关键项。

保存后进入：

```text
软件商店 -> Nginx -> 设置 -> 配置修改
```

或使用宝塔终端执行：

```bash
nginx -t
nginx -s reload
```

## 7. HTTP 验证

在本地或服务器执行：

```bash
curl -I http://driver.sunningfun.cn/
curl -fsS http://driver.sunningfun.cn/api/health
```

预期：

```text
根页面返回 200
/api/health 返回 ok: true
```

如果返回 502，常见原因：

- Vola 容器没启动。
- 目标 URL 填错，不是 `http://127.0.0.1:18080`。
- 服务器本机 `curl http://127.0.0.1:18080/api/health` 不正常。

## 8. 宝塔启用 HTTPS

两种方式都可以。

推荐方式 A：腾讯云 SSL 证书

```text
腾讯云 SSL 证书控制台 -> 申请免费证书 -> 填 driver.sunningfun.cn -> DNS 验证 -> 下载 Nginx 证书
```

然后在宝塔：

```text
网站 -> driver.sunningfun.cn -> 设置 -> SSL -> 其他证书
```

把 Nginx 证书内容粘贴到：

```text
证书 PEM
私钥 KEY
```

点击：

```text
保存并启用证书
```

方式 B：宝塔 Let's Encrypt

```text
网站 -> driver.sunningfun.cn -> 设置 -> SSL -> Let's Encrypt
```

选择域名 `driver.sunningfun.cn` 后申请。若 HTTP 验证失败，改用腾讯云 SSL 证书的 DNS 验证方式。

启用证书后，打开：

```text
强制 HTTPS
```

再执行：

```bash
nginx -t
nginx -s reload
```

## 9. HTTPS 验证

执行：

```bash
curl -I https://driver.sunningfun.cn/
curl -fsS https://driver.sunningfun.cn/api/health
```

预期：

```text
https://driver.sunningfun.cn/ 返回 HTTP 200
https://driver.sunningfun.cn/api/health 返回 ok: true
```

浏览器访问：

```text
https://driver.sunningfun.cn
```

确认页面能打开，地址栏证书有效。

## 10. 更新 Vola 外部地址

域名和 HTTPS 正常后，在服务器修改：

```text
/opt/vola/config/vola.env
```

建议值：

```text
PUBLIC_BASE_URL=https://driver.sunningfun.cn
CORS_ORIGINS=https://driver.sunningfun.cn
```

保存后执行：

```bash
cd /opt/vola
bash deploy/tencent/pull-and-deploy.sh
```

再验证：

```bash
curl -fsS http://127.0.0.1:18080/api/health
curl -fsS https://driver.sunningfun.cn/api/health
```

## 11. 宝塔验收清单

```text
[x] DNS A 记录已指向腾讯云公网 IP
[x] 腾讯云防火墙开放 80 和 443
[x] 腾讯云防火墙未开放 18080
[x] 宝塔已创建独立站点 driver.sunningfun.cn
[x] 站点未使用 /opt/vola 作为根目录
[x] 反向代理目标是 http://127.0.0.1:18080
[x] 未启用 proxy_cache，动态响应 no-cache
[x] nginx -t 通过
[x] HTTP 访问正常，返回 301 到 HTTPS
[x] SSL 证书启用
[x] 强制 HTTPS 已开启
[x] HTTPS 访问正常
[x] /api/health 返回 ok: true
[x] /opt/vola/config/vola.env 已设置域名
```

## 12. 不要改的东西

- 不要改 `growth.sunningfun.cn` 站点。
- 不要改已有 family-growth 或 model-pay-platform 的反向代理。
- 不要把 Vola 代理到 `3005` 或 `8100`。
- 不要开放 `18080` 给公网。
- 不要把数据库密码、JWT、Vault、ACR 密码写进宝塔备注或文档。

## 13. 参考

- 腾讯云 DNSPod A 记录文档：https://staticintl.cloudcachetci.com/doc/pdf/product/pdf/1295_76971_zh.pdf
- 腾讯云 SSL 证书部署到服务器概览：https://cloud.tencent.com/document/product/400/4143
- 宝塔反向代理文档：https://docs.bt.cn/user-guide/site/php/site-config/reverse-proxy
- 宝塔 SSL 证书部署文档：https://docs.bt.cn/10.0/getting-started/deploy-ssl
