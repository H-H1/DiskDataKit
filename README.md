# DiskDataKit

> 磁盘数据管理与 AI 智能分析工具

一个用 Go 写的本地磁盘管理工具。它能帮你搞清楚"磁盘空间到底被什么占了"，还能用 AI 判断哪些文件是垃圾、哪些是流氓软件，然后一键清理。

## 它能干什么

### 1. 磁盘文件可视化
- 用帕累托图（从大到小排列的柱状图）展示文件夹里各文件的占用情况
- 点击某个柱子可以往下钻取，看它里面的子文件
- 右键可以直接打开文件所在的文件夹
- 支持文件夹大小缓存，第二次打开速度更快

### 2. AI 智能分析
- 内置 AI 助手，可以对话提问，比如"D盘为什么满了"、"AppData 里的 Temp 能删吗"
- 扫描启动项后，AI 自动判断每个启动项是否可疑（流氓软件检测），可疑的置顶显示
- 扫描缓存目录后，AI 分析每个缓存属于哪个软件、能不能安全删除
- 支持多家 AI 厂商：DeepSeek、通义千问、Claude、Gemini、OpenAI 等，可自定义添加

### 3. 系统管理
- **启动项管理** - 查看和管理开机自启程序，支持启用/禁用
- **常规清理** - 扫描并清理系统临时文件、用户临时文件
- **缓存清理** - 扫描各软件产生的缓存目录，AI 辅助判断是否可删
- **系统快捷入口** - 一键打开 Windows 的"应用和功能"、"开机启动项"、"磁盘清理"、"存储感知"
- **文件夹追踪** - 关注特定文件夹，实时监控其大小变化

### 4. 其他功能
- 最近访问记录，快速回到上次看的文件夹
- 管理员权限检测，非管理员启动会提示
- 暗黑 + 暖金配色，纯前端渲染，流畅好看

## 怎么用

### 直接运行（推荐）

1. 下载对应平台的可执行文件
2. 双击运行（Windows 会弹出 UAC 提权，点"是"即可）
3. 浏览器会自动打开 `http://localhost:8080`
4. 开始使用

### 从源码编译

需要安装 [Go 1.25+](https://go.dev/dl/)。

```bash
# 克隆项目
git clone <仓库地址>
cd DiskDataKit

# Windows 编译（带图标，无控制台窗口）
make windows

# macOS 编译
make darwin        # Intel
make darwin-arm64  # Apple Silicon

# 直接运行
make run
```

编译后会生成 `DiskDataKit.exe`（Windows）或 `DiskDataKit`（macOS），双击即可运行。

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go 1.25、标准库 `net/http` |
| 前端 | 原生 HTML/CSS/JS，无框架 |
| AI | 支持 OpenAI 兼容接口、Claude、Gemini |
| 通信 | REST API + WebSocket（实时扫描进度） |
| 打包 | 前端通过 Go `embed` 嵌入，单文件运行 |

## 项目结构

```
DiskDataKit/
├── main.go                # 入口：路由注册、启动服务
├── cache.go               # 文件夹大小缓存（GZIP+Gob）
├── chat.go                # AI 对话与模型配置
├── cleanup.go             # 临时文件扫描与清理
├── scan.go                # 流氓软件扫描（AI 判断）
├── startup.go             # 开机启动项读取与管理
├── track.go               # 文件夹追踪
├── elevate_*.go           # 各平台提权（UAC/osascript/pkexec）
├── ai/                    # AI 客户端封装
│   ├── openai.go          # OpenAI 兼容接口
│   ├── claude.go          # Claude
│   ├── gemini.go          # Gemini
│   └── keypool.go         # API Key 轮询
├── web/                   # 前端资源（嵌入到二进制）
│   ├── index.html         # 主页面
│   ├── app.js             # 文件浏览与可视化
│   ├── chat.js            # AI 对话界面
│   ├── scan.js            # 流氓软件扫描
│   ├── startup.js         # 启动项管理
│   ├── cleanup.js         # 清理功能
│   ├── cache.js           # 缓存扫描
│   ├── track.js           # 文件夹追踪
│   ├── system.js          # 系统管理入口
│   └── style.css          # 全局样式
├── Makefile               # 构建脚本
└── go.mod                 # Go 模块定义
```

## 平台支持

| 平台 | 状态 | 说明 |
|---|---|---|
| Windows | 完整支持 | 自动 UAC 提权，带应用图标 |
| macOS | 完整支持 | 自动 osascript 提权 |
| Linux | 基本支持 | 自动 pkexec 提权 |

## 开发模式

需要查看详细日志时，设置环境变量 `DEV=1` 启动：

```bash
# Windows PowerShell
$env:DEV=1; .\DiskDataKit.exe

# macOS / Linux
DEV=1 ./DiskDataKit
```

开发模式下会写入日志文件到 `log/` 目录（启动日志、缓存日志）。

## AI 配置

程序内置了 DeepSeek 模型，开箱即用。如需切换或自定义：

1. 点击界面右上角的模型切换按钮
2. 选择厂商，填入你的 API Key
3. 获取可用模型列表，勾选要使用的模型
4. 保存即可

配置文件存储在用户目录：`%LOCALAPPDATA%\DiskDataKit\ai_config.json`

## 许可证

[Apache License 2.0](LICENSE)
