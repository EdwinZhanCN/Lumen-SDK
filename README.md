# Lumen Gateway

Lumen Gateway 是 Lumen-SDK 的图形化网关客户端，以系统托盘（Menu Bar）应用形式运行。它在单进程中内置了服务发现、连接池、负载均衡和 HTTP REST API，并通过无边框 Webview 提供直观的节点监控看板。

---

## 核心功能

- **系统托盘集成**：支持 macOS 菜单栏与 Windows 任务栏托盘。
- **轻量级面板**：点击托盘图标弹出无边框 Webview，实时展示：
  - 核心指标（QPS、平均延迟、在线节点数、错误率）
  - 已发现节点的 CPU/GPU 负载与可用推理能力（OCR、语义向量等）
  - 活跃的任务类型分布

---

## 开发与构建

### 前提条件
- Go >= 1.25.0
- Node.js (建议 v18+)
- Wails v3 CLI (`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`)

### 1. 开发模式
启动本地热重载调试：
```bash
cd cmd/lumen-gateway
~/go/bin/wails3 dev
```

### 2. 编译与打包

在项目根目录下，使用 `Makefile` 进行构建：

- **macOS (Universal Bundle)**:
  生成 macOS 架构通用的 `Lumen Gateway.app`：
  ```bash
  make build-gateway-mac
  ```
  可执行文件及 app 包将输出至 `cmd/lumen-gateway/bin/`。

- **Windows (amd64)**:
  交叉编译 Windows 版本的 `Lumen Gateway.exe`：
  ```bash
  make build-gateway-win
  ```

- **分发包生成**:
  一键生成 macOS 的 `.zip` 发布包与 Windows 的 `.exe` 执行文件：
  ```bash
  make release-gateway
  ```
  产物将输出至根目录下的 `dist/` 目录。

---

## 分发与安装

macOS 用户可以通过 Homebrew 安装 Lumen Gateway：

1. 使用 Homebrew 安装：
   ```bash
   brew install --cask https://raw.githubusercontent.com/EdwinZhanCN/Lumen-SDK/main/lumen-gateway.rb
   ```
2. 首次启动前在终端运行以下命令以解除 macOS 安全隔离限制：
   ```bash
   xattr -d com.apple.quarantine "/Applications/Lumen Gateway.app"
   ```

Windows 用户可以从 [GitHub Releases](https://github.com/EdwinZhanCN/Lumen-SDK/releases) 下载最新的 `.exe` 安装包。
