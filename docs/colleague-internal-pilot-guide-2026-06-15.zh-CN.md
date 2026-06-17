# Vola 内部试用说明

更新时间：2026-06-15 14:20 CST

## 结论

可以发给同事做内部试用。

适合范围：

- 团队内部同事自助注册账号。
- 每个账号默认 100MB 云端资料额度。
- 同事自行连接 GitHub Backup，并创建或授权自己的私有备份仓库。
- 试用 Vola 的个人资料库、技能资料、团队资料和 GitHub Backup 流程。

不建议现在当作完全公开发布：

- 24 小时自动备份观察还没完成一次到期验证。
- 运行时文件对象存储仍是 `OBJECT_STORAGE_BACKEND=db`，COS 当前用于外部备份，不是运行时对象存储。
- 公开注册已经打开，但当前主要靠默认 100MB 限制单账号资料容量，没有完整账号数量、防垃圾注册和频率治理策略。

## 发给同事的入口

网址：

```text
https://driver.sunningfun.cn/signup
```

建议发送文案：

```text
这是 Vola 的内部试用入口：

https://driver.sunningfun.cn/signup

你可以自己注册账号。注册后建议先完成两件事：

1. 进入系统后看一下个人资料库和技能资料入口。
2. 如果要使用 GitHub Backup，按页面提示连接 GitHub，并创建一个私有备份仓库。

当前每个账号默认 100MB 云端资料额度。这个版本先用于内部试用，如果遇到注册、登录、GitHub 授权或同步问题，直接把截图和操作时间发给我。
```

## 同事使用步骤

1. 打开 `https://driver.sunningfun.cn/signup`。
2. 填写邮箱、密码、账户名和显示名称。
3. 注册后进入 Vola。
4. 需要备份时，进入 GitHub Backup 页面。
5. 点击连接 GitHub。
6. 推荐创建或授权一个私有备份仓库。
7. 点击立即同步，确认页面显示同步成功。

## 已验证状态

- 生产 `/api/health` 正常，`storage=postgres`。
- `/api/config` 显示 `public_registration_enabled=true`。
- 注册页显示邮箱、密码、账户名、显示名称和“创建账号”按钮。
- 临时账号真实注册成功，`POST /api/auth/register` 返回 HTTP 201。
- 新账号可以访问 `/api/auth/me` 和 `/api/tree/`。
- 临时测试账号已删除。
- 新账号继承默认额度：`USER_STORAGE_QUOTA_BYTES=100MB`。
- GitHub App 已启用，slug 为 `vola-backup`。
- 旧 Vault secret 已用生产 dump 和原生产 `VAULT_MASTER_KEY` 做过脱敏复验。

## 已知边界

- GitHub Backup 是用户级配置；每个同事要自己授权 GitHub。
- `/api/ops/status` 是当前用户状态，不等同于实例级状态。
- 实例级健康要看 `/api/health` 和实例级运维状态。
- 自动备份 24 小时触发还在观察中，不能写成已完成。
- 100MB 是每个账号的 Vola 用户资料额度，不是 COS bucket 总容量。

## 运维观察清单

试用开始后的前 24 到 48 小时建议观察：

- 新同事能否完成注册和登录。
- GitHub 授权是否顺畅。
- 私有备份仓库是否能创建或复用。
- 手动同步是否成功。
- 到 24 小时后，自动备份是否产生新的成功记录。
- 是否出现异常注册、重复账号或容量超限反馈。

## 回滚公开注册

如果需要暂停同事自助注册，把生产 env 中：

```text
VOLA_ENABLE_PUBLIC_REGISTRATION=1
```

改回：

```text
VOLA_ENABLE_PUBLIC_REGISTRATION=0
```

然后只重启 `neudrive-server`：

```bash
ssh family-growth-tencent '
  cd /opt/neudrive/deploy/tencent &&
  docker compose -p neudrive --env-file /opt/neudrive/config/neudrive.env -f docker-compose.yml up -d --no-deps server
'
```
