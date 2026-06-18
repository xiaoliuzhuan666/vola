# Vola App Icon 设计记录

更新日期：2026-06-18

## 当前方案

Vola 当前主 icon 采用浅色圆角底、中层蓝色圆角核心、白色 `V` 标识的结构。

设计目标：

- 简单清爽，主形状在 128px 和 Dock 小尺寸下仍清晰。
- 保留轻立体感，但减少复杂细节。
- 配色从原来的青绿蓝改为更明亮的蓝紫色系。
- 接近现代工具类 app 的视觉气质：白底、亮蓝主色、高对比前景符号。

当前预览源文件：

- `web/public/vola-app-icon.png`
- `web/public/vola-mark.svg`
- `web/public/favicon.svg`

## 视觉结构

图标由三层组成：

1. 外层底座
   - 近白色到浅蓝白的圆角矩形。
   - 用很浅的描边保留 macOS app icon 的边界感。

2. 中层核心块
   - 大圆角矩形。
   - 渐变从左上亮青蓝过渡到右下蓝紫。
   - 顶部有一条半透明高光，保留立体感。

3. 前景标识
   - 白色粗线 `V`。
   - 底部保留一条淡蓝紫弧线，增加亲和感，但不影响小尺寸识别。

## 配色

外层底座：

| 用途 | 色值 |
| --- | --- |
| 背景高光 | `#FFFFFF` |
| 背景中段 | `#F4F8FF` |
| 背景暗部 | `#EAF1FF` |
| 外框描边 | `#D7E4FF` |

中层核心：

| 用途 | 色值 |
| --- | --- |
| 左上高亮蓝 | `#5DDCFF` |
| 主蓝 | `#2F8DFF` |
| 右下蓝紫 | `#5C5BFF` |

前景和光泽：

| 用途 | 色值 |
| --- | --- |
| 主 `V` | `#FFFFFF` |
| 顶部高光 | `#FFFFFF`，透明度约 `0.7 -> 0` |
| 底部弧线起点 | `#CBF6FF` |
| 底部弧线终点 | `#D7D6FF` |
| 投影色 | `#29529B`，透明度约 `0.18` |

## 参考方向

参考过的方向：

- Apple Human Interface Guidelines 的 App Icons 规范：前景要清晰，背景服务主体，图标在不同尺寸和外观下都要能识别。
- Microsoft Windows app icon design / Fluent iconography：高对比环境下不能只依赖复杂渐变，主体轮廓要明确。
- 现代工具类 app icon 趋势：单一核心符号、柔和阴影、明亮渐变、少细节。

落到 Vola 上的判断：

- 保留现在的白底和立体结构。
- 主色明显偏蓝，避免继续偏青绿色。
- 不增加更多符号、数据点或复杂纹理。
- `V` 保持白色粗线，不做彩色或细线版本。

## 资源生成链路

源头资源：

```bash
web/public/vola-mark.svg
web/public/favicon.svg
web/public/vola-app-icon.png
```

桌面图标输出：

```bash
src-tauri/icons/
desktop/icons/
```

重新生成 1024 PNG：

```bash
sips -s format png -z 1024 1024 web/public/vola-mark.svg --out web/public/vola-app-icon.png
```

重新生成 Tauri 图标：

```bash
cd web
npm run tauri -- icon public/vola-app-icon.png --output ../src-tauri/icons --ios-color '#f4f8ff'
npm run tauri -- icon public/vola-app-icon.png --output ../desktop/icons --ios-color '#f4f8ff'
```

## 打包注意事项

项目里目前有两条桌面打包路线：

| 路线 | 图标目录 | 产物示例 |
| --- | --- | --- |
| `src-tauri` | `src-tauri/icons` | `src-tauri/target/release/bundle/dmg/vola_0.1.6_aarch64.dmg` |
| `desktop` | `desktop/icons` | `desktop/target/release/bundle/dmg/Vola_0.1.0_aarch64.dmg` |

两条路线都需要重新生成图标，否则其中一条仍可能使用旧图标。

另外，`web/dist` 和 `internal/web/dist` 是构建产物。如果只改 `web/public`，但没有重新执行前端 build，打包时仍可能带入旧 Web favicon 或旧 `vola-app-icon.png`。

建议打包前执行：

```bash
cd desktop
npm run build
```

或者：

```bash
cd src-tauri
../web/node_modules/.bin/tauri build --bundles app dmg --no-sign
```

正式发版不能使用 `--no-sign`，需要完整签名流程。

## 旧包误判记录

2026-06-18 排查时发现，桌面上存在旧发布包：

```bash
/Users/zhongmoshu/Desktop/macos-x86_64-vola_0.1.11_x64.dmg
```

用户截图打开的是这份旧包，里面的 `vola.app` 仍是旧 icon。后来已生成新版图标的同名包，并把旧包保留为：

```bash
/Users/zhongmoshu/Desktop/macos-x86_64-vola_0.1.11_x64-old-icon.dmg
```

判断一个 DMG 是否使用新版 icon，不能只看文件名或 Finder 预览，需要挂载后检查内部资源：

```bash
hdiutil attach /path/to/vola.dmg
shasum -a 256 /Volumes/vola/.VolumeIcon.icns
shasum -a 256 /Volumes/vola/vola.app/Contents/Resources/icon.icns
```

新版 `src-tauri` icon 的参考哈希：

```text
ca5cfe07ace7a72c75261c45ce3a3cc57d48c56a1c327cda2f84a66249821b6f  src-tauri/icons/icon.icns
```

新版 Web PNG 的参考哈希：

```text
5339722b406a286351dd7e36871fa05f1b41eabfcd81a1fdce3fcfbe315fc157  web/public/vola-app-icon.png
```

## 验证命令

检查源文件：

```bash
file web/public/vola-app-icon.png src-tauri/icons/icon.icns desktop/icons/icon.icns
shasum -a 256 web/public/vola-app-icon.png web/public/vola-mark.svg web/public/favicon.svg
```

检查打包后的 `.app`：

```bash
shasum -a 256 src-tauri/target/release/bundle/macos/vola.app/Contents/Resources/icon.icns
shasum -a 256 desktop/target/release/bundle/macos/Vola.app/Contents/Resources/icon.icns
```

检查 DMG 内部：

```bash
rm -rf /tmp/vola-icon-check-mounted
mkdir -p /tmp/vola-icon-check-mounted
hdiutil attach /path/to/vola.dmg -mountpoint /tmp/vola-icon-check-mounted -nobrowse -readonly
shasum -a 256 /tmp/vola-icon-check-mounted/.VolumeIcon.icns
shasum -a 256 /tmp/vola-icon-check-mounted/vola.app/Contents/Resources/icon.icns
hdiutil detach /tmp/vola-icon-check-mounted
```

如果 Finder 仍显示旧图标，先确认打开的是哪一个 DMG，再刷新缓存：

```bash
hdiutil info
qlmanage -r cache
killall Finder
killall Dock
```
