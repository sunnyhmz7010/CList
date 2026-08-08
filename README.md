<div align="center">
  <h1>CList</h1>
  <p>CList 是一个把 Telegram 与本地文件统一管理的单租户网盘系统。</p>
</div>

<p align="center">
  <a href="https://github.com/sunnyhmz7010/CList/releases"><img src="https://img.shields.io/github/v/release/sunnyhmz7010/CList?label=Release&color=3b82f6" alt="Release" /></a>
  <a href="https://github.com/sunnyhmz7010/CList/blob/main/LICENSE"><img src="https://img.shields.io/github/license/sunnyhmz7010/CList?color=10b981" alt="License" /></a>
  <a href="https://github.com/sunnyhmz7010/CList/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/sunnyhmz7010/CList/ci.yml?branch=main&label=CI" alt="CI" /></a>
</p>

---

## ✨ 为什么做这个项目

CList 面向希望用一个可备份、可恢复的单体服务管理本地文件和 Telegram 频道文件的个人或小团队。它不引入注册系统、Redis、PostgreSQL、Nginx 或环境变量配置，数据索引集中在 SQLite WAL 中。

## 🚀 核心能力

- Local、官方 Telegram Bot API 和自建 Streaming API 三种存储档案。
- 乱序、重复、缺失查询、重启恢复和 SHA-256 校验的分片上传。
- 管理员、游客恢复密钥和相册独立访问密码。
- 逻辑目录、稳定公开 ID、相册可见性继承、媒体与文档预览。
- 固定回收站、手动彻底删除、跨存储迁移和 REST API Token。

## ⚡ 快速开始

### 📋 前置要求

- 已安装 Docker，宿主机提供可持久化卷。
- 首次启动后通过浏览器完成管理员初始化。

### 📦 安装与运行

```bash
docker run -d --name clist --restart unless-stopped -p 8080:8080 -v clist_data:/data ghcr.io/sunnyhmz7010/clist:latest
```

也可以在仓库根目录运行：

```bash
docker compose up -d --build
```

## 📖 使用说明

打开 `http://localhost:8080`，初始化管理员后即可创建目录和上传文件。游客访问密码、相册密码、单文件密码分别生效；游客恢复密钥只在创建或恢复时显示，请离线保存。

上传大文件时客户端会将文件切成分片并查询缺失分片。Local 与官方 Telegram 支持 Range；自建 Streaming API 只支持完整顺序下载，页面会明确提示不支持断点续传。

删除文件或目录会先进入回收站。只有回收站中的彻底删除操作才会删除本地对象或 Telegram 消息，并且该操作不可恢复。迁移失败时源文件保持可用。

## 🧠 功能细节

SQLite 使用 WAL、外键约束和任务表保存上传、迁移、租约及不确定发送结果。Telegram 文件正文保存在频道消息，目录和相册层级只保存在 SQLite。公开文件 ID 在改名、移动、恢复和迁移后保持不变。

自建 Streaming API 的能力固定为 `Range=false`、`Head=false`、`Streaming=true`，CList 不扩展该后端的断点续传能力。配置档案使用 `/data/secrets/master.key` 加密，备份时必须同时保存主密钥。

## 🧱 技术栈

- Go 1.26：模块化单体 HTTP 服务与存储适配器。
- SQLite WAL：唯一必需的元数据数据库。
- React、TypeScript、Vite：前端并嵌入 Go 单二进制。
- Docker：零环境变量的一条命令部署。

## 🗂️ 项目结构

```
CList/
├── cmd/clist/          # 可执行入口、备份恢复和嵌入前端
├── internal/api/       # HTTP 路由、鉴权和响应
├── internal/db/        # SQLite 迁移与仓储
├── internal/storage/   # Local、Telegram 和编排器
├── internal/gallery/   # 相册可见性与筛选
├── internal/trash/     # 回收站与物理清理
├── internal/migration/ # 跨存储迁移
└── web/                # React/TypeScript 前端
```

## 👨‍💻 本地开发

### 🧰 环境

需要 Go 1.26、Node.js 24、npm 和 Docker。运行时不需要设置环境变量；本地数据建议放在临时目录。

### ⚙️ 命令

```bash
go test ./...
npm --prefix web install
npm --prefix web test -- --run
npm --prefix web run build
docker compose up -d --build
```

## 🔐 安全报告

如果发现安全问题，请不要公开披露细节。请优先参考仓库中的 [SECURITY.md](./SECURITY.md) 提交安全报告。

## 📄 许可证

本项目基于 [GPL-3.0](./LICENSE) 开源。

<div align="center">
  <sub>Built with ❤️ by Sunny</sub>
</div>
