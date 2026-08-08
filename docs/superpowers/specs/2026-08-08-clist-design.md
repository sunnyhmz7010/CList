# CList 网盘系统设计规格

日期：2026-08-08
状态：已完成需求澄清，待进入实现计划

## 1. 产品定位

CList 是一个纯 Docker 部署的单租户文件网盘与图床系统。它以 Telegram 频道消息作为一种文件存储后端，同时支持官方 Telegram Bot API、自建 Telegram Bot API（接入 `telegram-bot-api-file-streaming`）和本地存储。

系统不提供注册、用户列表、用户组或配额体系，只区分：

- 一个管理员账号与密码；
- 未登录游客；
- 游客用于找回上传记录的匿名恢复密钥。

首版强调一条 Docker 命令启动、分片上传、可恢复后台任务、逻辑目录、相册预览、回收站和 REST API。负载均衡、Cloudflare 部署和自建流式后端的 `Range` 扩展不阻塞首版，分别作为后续范围。

## 2. 目标与非目标

### 2.1 目标

- Go 模块化单体，编译为包含 React + TypeScript 前端的单二进制。
- SQLite WAL 作为唯一必需的元数据数据库，不依赖 Redis、PostgreSQL 或 Nginx。
- 动态配置多个存储档案：官方 Telegram、自建 Telegram 流式 API、本地目录。
- 默认存储可由管理员选择；首版不自动负载均衡。
- 所有大文件先分片写入本地临时目录，支持缺失分片查询、乱序上传、重复提交、断点恢复和 SHA-256 校验。
- Telegram Webhook 接收频道文件并自动回复 CList 稳定直链。
- 文件夹、相册显示状态、回收站和跨存储迁移均由 SQLite 元数据统一管理。
- 文件公开 ID 稳定，改名、移动、恢复和迁移不改变原链接。
- 游客可上传、查看自己的上传记录并在不同设备恢复；管理员可管理全部内容。
- 相册支持图片、视频、音频、PDF、DOCX、TXT 等文件的在线预览。
- 提供 REST API Token，兼容 `curl`、PicGo、ShareX 等客户端。

### 2.2 非目标

- 首版不支持 Cloudflare Pages、Workers 或 Cloudflare KV/R2。
- 首版不实现自动负载均衡、节点权重调度或多实例共享 SQLite。
- 首版不提供多用户注册、用户组、配额、计费和 OIDC。
- 首版不修改 `telegram-bot-api-file-streaming`；其 `Range`、断点续传和 `HEAD` 支持为独立后续项目。
- 回收站不自动清理，只允许手动彻底删除。
- 不把 Telegram 频道中的消息组织成真实文件夹；文件夹仅存在于 CList 元数据中。

## 3. 总体架构

```text
浏览器 / REST 客户端
        ↓
Go HTTP 服务 + 嵌入式 React 前端
        ↓
应用服务层
├─ 管理员、游客和文件密码认证
├─ 文件、逻辑目录和相册
├─ 上传、下载、迁移和预览任务
├─ 回收站与恢复
└─ REST API Token
        ↓
基础设施层
├─ SQLite WAL
├─ 本地临时分片目录
├─ Local Storage Adapter
├─ Telegram Official Adapter
└─ Telegram Streaming Adapter
```

采用模块化单体而不是微服务：API、后台任务和存储适配器运行在同一 Go 进程内，任务状态写入 SQLite。这样可保持一条 Docker 命令启动，同时为未来拆分 worker 保留清晰边界。

HTTP 层只负责路由、请求解析、鉴权、响应和错误映射；应用服务层实现文件、目录、相册、回收站、迁移和任务状态机；基础设施层负责 SQLite、文件系统和外部存储协议。

## 4. Docker 运行形态

默认启动不需要任何环境变量：

```bash
docker run -d \
  --name clist \
  --restart unless-stopped \
  -p 8080:8080 \
  -v clist_data:/data \
  ghcr.io/sunnyhmz7010/clist:latest
```

数据卷目录建议如下：

```text
/data/
├─ clist.db              SQLite 主数据库
├─ clist.db-wal          WAL 日志
├─ clist.db-shm          SQLite 共享内存文件
├─ files/                默认本地存储根目录
├─ chunks/               上传临时分片
├─ migrations/           迁移暂存文件
├─ cache/previews/       可再生成的预览与缩略图
└─ secrets/master.key    存储配置加密主密钥
```

管理员可在后台增加其他已经挂载到容器内的本地绝对路径。CList 必须规范化路径、解析符号链接并验证最终路径位于管理员配置的根目录内。

SQLite 启动时启用 `journal_mode=WAL`、外键约束、忙等待和合理同步级别。数据库文件不支持在 SMB/NFS 等网络共享盘上由多个实例共同写入。

## 5. 数据模型

SQLite 只保存索引、权限、配置和任务状态，不保存 Telegram 或本地文件正文。

核心实体：

| 实体 | 关键字段 | 说明 |
| --- | --- | --- |
| `settings` | `key`, `value` | 管理员、游客/相册密码哈希及站点设置 |
| `storage_profiles` | `id`, `type`, `name`, `encrypted_config`, `enabled`, `is_default` | 存储档案及加密配置 |
| `guest_vaults` | `id`, `key_hash`, `created_at`, `revoked_at` | 匿名游客恢复密钥 |
| `folders` | `id`, `parent_id`, `name`, `owner_vault_id`, `gallery_visibility` | 逻辑目录树 |
| `files` | `public_id`, `folder_id`, `owner_vault_id`, `storage_profile_id`, `storage_key`, `file_name`, `mime_type`, `size`, `sha256`, `gallery_visibility`, `state` | 文件索引与权限归属 |
| `file_secrets` | `file_id`, `password_hash`, `salt` | 单文件访问密码 |
| `trash_batches` | `id`, `root_type`, `root_id`, `deleted_at` | 删除批次及恢复信息 |
| `jobs` | `id`, `kind`, `state`, `progress`, `attempts`, `lease_until`, `last_error` | 上传合并、迁移、预览和彻底删除任务 |
| `api_tokens` | `id`, `token_hash`, `scopes`, `expires_at`, `last_used_at` | REST API Token |
| `sessions` | `id`, `kind`, `expires_at` | 管理员、首页游客和相册游客会话 |
| `audit_logs` | `id`, `action`, `target_public_id`, `actor_kind`, `created_at`, `result` | 不包含敏感配置的最小操作审计 |

文件状态为：`uploading`、`active`、`trashed`、`purged`。公开 ID 使用由 `crypto/rand` 生成的 128 位 URL 安全随机值，文件名只用于展示，不作为物理定位依据。

Telegram 文件记录至少包含 `chat_id`、`message_id`、`file_id`、`file_unique_id` 和文件大小。Telegram 频道消息不会携带 CList 文件夹信息。

## 6. 认证与权限

### 6.1 管理员

首启没有管理员凭据时，后台提供未初始化设置页。第一次保存账号密码使用 SQLite 事务完成初始化；初始化后后台必须登录。系统不创建用户表，也不开放注册。修改管理员密码会注销全部旧会话。

管理员会话使用 HttpOnly Cookie、SameSite、CSRF 防护、失败限速和恒定时间密码比较。密码使用慢哈希保存。

### 6.2 游客首页与相册

- 首页默认公开；管理员可设置全站游客密码，设置后游客必须验证才能继续访问和上传。
- 相册有独立开关和独立密码，管理员会话自动通过。
- 首页密码和相册密码生成不同作用域的匿名会话，不互相替代。
- 文件公开直链不受首页或相册密码影响。

### 6.3 游客上传记录

浏览器首次上传时生成高熵恢复密钥。服务端只保存密钥哈希，文件和目录关联到匿名 `guest_vault`。当前浏览器保存密钥，其他设备可通过恢复密钥导入全部上传记录。持有密钥者可以重命名、移动、删除、迁移和修改相册显示状态。密钥丢失无法找回，但管理员仍可管理文件；旧密钥可被撤销并替换。

### 6.4 API Token

API Token 仅由管理员创建，存储哈希并支持 `upload`、`read`、`delete`、`manage` 权限及过期时间。Token、恢复密钥和 Bot Token 不出现在 URL 或日志中。

## 7. 存储适配器

所有后端实现统一接口：

```text
Validate
Capabilities
HealthCheck
Put(stream, metadata)
Open(key, range)
Delete(key)
```

### 7.1 官方 Telegram

使用官方 Bot API 地址，把文件以消息附件发送到配置频道，保存返回的 Telegram 标识。下载通过 `getFile` 获取路径，再代理文件内容；上游支持时透传 `Range`。按官方接口返回和存储档案能力执行大小校验。

### 7.2 自建 Telegram Streaming

使用自建 Bot API 地址发送到频道，下载调用：

```text
GET /stream/file/bot<TOKEN>/<URL_ENCODED_FILE_ID>
```

CList 只在服务端调用该地址，并发送可信的 `X-Telegram-File-Size`。当前适配器声明支持完整顺序流，不支持 `Range`、`HEAD` 和断点续传；前端据此显示能力提示。自建端点的 `Range` 扩展另行设计，不作为 CList 首版依赖。

### 7.3 Local

对象使用稳定内部键写入管理员授权的本地根目录。写入使用临时文件、`fsync` 和原子重命名；读取支持 `HEAD`、单区间 `Range` 和断点续传。

## 8. 上传、下载和 Webhook

上传任务流程：

```text
初始化任务
  → 浏览器并发上传分片
  → 临时目录落盘
  → 校验分片、总大小、SHA-256
  → 流式写入目标存储
  → 事务写入文件索引
  → 清理临时分片
```

任务支持乱序、重复提交、缺失分片查询、断点恢复和容器重启恢复。完整文件不会一次性加载到内存。

下载行为：

- Local 和官方 Telegram 尽量支持 `Range`。
- 自建流式 API 从零开始完整转发；客户端中断后从头重试，并显示“不支持断点续传”。
- 所有响应先检查文件状态、文件密码和权限；响应开始后发生上游错误则中断连接，客户端必须校验 `Content-Length`。

Telegram Webhook：

- 每个 Telegram 存储档案可启用独立 Webhook。
- 校验 `X-Telegram-Bot-Api-Secret-Token` 和配置频道。
- 解析 `message`/`channel_post`，提取文件媒体和 Telegram 标识。
- 以 `chat_id + message_id` 幂等入库，生成稳定 CList 直链并调用 `sendMessage` 回复。
- 频道直接上传文件默认归管理员，不进入游客上传记录。

对于 Telegram 发送请求超时但结果未知的情况，任务标记为 `uncertain`，避免自动重试造成重复频道消息；管理员可检查、绑定或重新上传。

## 9. 文件夹、相册和预览

文件夹永远是 SQLite 逻辑目录。移动 Telegram 文件只更新 `folder_id`，不重新上传频道消息；本地对象路径也不随逻辑目录变化。

相册为全站展示页面，不单独建立多个相册实体。默认展示允许显示的全部文件，支持文件夹、类型、名称和时间筛选。文件与文件夹的显示状态使用 `inherit`、`visible`、`hidden` 三态，文件设置优先于文件夹设置，文件夹设置优先于全局默认。

预览支持：

- 图片：缩放、旋转、缩略图；
- 视频和音频：HTML5 播放器；
- PDF：浏览器或 PDF.js；
- TXT：受限大小的纯文本；
- DOCX：浏览器端只读解析，CSP 隔离，不执行宏、脚本和外部资源；
- 其他类型：元信息和下载入口。

缩略图与预览缓存位于 `/data/cache/previews`，可删除并重新生成，不影响原文件。

## 10. 回收站与迁移

删除操作固定进入回收站，不提供关闭选项：

- 删除文件或目录只修改 SQLite 状态。
- Telegram 消息、本地文件和公开对象暂不删除。
- 直链返回 `410 Gone`，正常列表、相册和游客记录隐藏。
- 目录删除作为批次处理，支持整体恢复或逐项处理。
- 恢复优先回到原目录；出现冲突时提示用户选择，不自动覆盖。
- 回收站只允许手动彻底删除。

彻底删除时，Telegram 调用 `deleteMessage`，Local 删除实际对象；对象已不存在视为幂等成功。实际对象删除成功后，文件行转为 `purged`，清除存储键、Telegram 标识、文件密码和游客归属，仅保留公开 ID、清理时间及最小审计信息以支持幂等请求。失败项继续保留在回收站并显示错误，允许重试。

跨存储迁移由管理员或对应游客恢复密钥发起：读取源文件到迁移暂存目录，校验 SHA-256，写入目标并回读校验，事务切换文件存储绑定，最后永久清理原对象。目标失败时原文件继续可用；切换成功而源清理失败时标记 `cleanup_pending`，文件使用目标存储并等待手动重试。

## 11. API 契约

业务 API 使用 `/api/v1`：

```text
POST   /api/v1/uploads
PUT    /api/v1/uploads/{id}/chunks/{index}
GET    /api/v1/uploads/{id}
POST   /api/v1/uploads/{id}/complete
DELETE /api/v1/uploads/{id}

GET    /api/v1/files
GET    /api/v1/files/{id}
PATCH  /api/v1/files/{id}
DELETE /api/v1/files/{id}

GET    /api/v1/folders
POST   /api/v1/folders
PATCH  /api/v1/folders/{id}
DELETE /api/v1/folders/{id}

GET    /api/v1/trash
POST   /api/v1/trash/{id}/restore
DELETE /api/v1/trash/{id}

POST   /api/v1/migrations
GET    /api/v1/jobs/{id}
GET    /api/v1/gallery
POST   /api/v1/vault/recover
POST   /api/v1/upload
```

文件直链为 `/f/{public_id}/{filename}`，预览页为 `/p/{public_id}`。列表和相册使用游标分页；创建、完成和迁移支持 `Idempotency-Key`。错误统一返回 `code`、`message`、`request_id` 和 `retriable`。下载响应暴露 `Content-Length`、`Content-Range`、`Accept-Ranges` 和后端能力标识。

Telegram Webhook 使用每个档案独立的随机路径和 Secret Token，不把 Token 放进 URL。

## 12. 任务可靠性、安全与运维

任务状态包括 `queued`、`running`、`retry_wait`、`succeeded`、`failed`、`cleanup_pending` 和 `uncertain`。任务使用 SQLite 租约和心跳；启动时回收过期租约并恢复未完成任务。可恢复网络错误使用指数退避和随机抖动，达到上限后保留现场并允许手动重试。

安全要求：

- 日志脱敏 Bot Token、API Token、恢复密钥、Cookie 和密码；
- 管理员、游客密码、相册密码和恢复接口限速；
- 本地路径检查绝对路径、符号链接和根目录边界；
- 文件响应设置安全 MIME、`Content-Disposition`、CSP 和 `X-Content-Type-Options`；
- TXT 按纯文本输出；DOCX 解压限制数量、展开大小和压缩比；
- 自建 Bot API 地址只允许管理员配置；
- 破坏性操作必须服务端鉴权。

运维接口：

- `/health/live`：进程存活；
- `/health/ready`：SQLite、数据目录和主密钥可用；
- 管理后台系统诊断：存储连通性、目录权限、磁盘空间和未完成任务；
- 结构化日志包含 `request_id`、任务 ID 和存储档案 ID，不包含敏感值；
- 提供 SQLite 在线备份/恢复命令；
- `/data/secrets/master.key` 与数据库必须同时备份。

## 13. 测试与验收

后端单元和集成测试覆盖目录树、相册继承、权限、回收站、迁移状态机、SQLite WAL、任务恢复、分片乱序/重复/缺失、SHA-256 不符、路径穿越、符号链接、原子写入和错误重试。

存储适配器使用本地模拟 HTTP 服务执行统一契约测试，覆盖官方 Telegram、自建流式 API、本地存储、限流、断流、重复 Webhook 和不确定发送结果。真实 Telegram 联调只在本地或受控环境执行，不在 CI 保存凭据。

前端测试覆盖上传队列、能力提示、目录操作、回收站和恢复密钥；Playwright 覆盖游客上传、记录恢复、相册预览、管理员管理和迁移流程，桌面与移动视口都运行关键路径。

Docker 验收标准：

- 无环境变量的一条命令可启动；
- 数据卷重启后配置、任务和索引完整；
- 1 GiB 本地分片上传时内存不会随文件大小线性增长；
- Local/官方 Telegram 支持断点续传，自建流式 API 明确提示不支持；
- 改名、移动、恢复和迁移不改变公开链接；
- 回收站阶段不删除 Telegram 消息，彻底删除才调用 `deleteMessage`；
- 迁移目标校验失败时源文件继续可用。

## 14. 实施边界与后续阶段

首个实现计划只覆盖上述纯 Docker 单体版本。以下能力单独排期，不进入首版阻塞链：

- `telegram-bot-api-file-streaming` 的 `Range`、`HEAD` 和断点续传扩展；
- 多节点负载均衡和自动故障切换；
- Cloudflare Pages/Workers 运行时；
- 多用户注册、用户组、配额、计费和 OIDC；
- 回收站自动清理；
- 多进程/多实例分布式任务队列。

## 15. 参考项目与取舍

- [K-Vault](https://github.com/katelya77/K-Vault)：参考双模产品体验、Telegram Webhook 回链、签名直链、元数据可选写入和多存储适配器；CList 不复制其 Cloudflare/Docker 两套后端。
- [CloudFlare-ImgBed](https://github.com/MarSeventh/CloudFlare-ImgBed)：参考游客/管理员双会话、相册、在线预览、目录索引和批量管理；CList 将 Telegram 文件夹处理统一为 SQLite 逻辑目录，避免对 Telegram 消息执行伪移动。
- [imgli](https://github.com/yixian-huang/imgli)：参考单二进制、自托管运维、存储适配器契约、健康检查和迁移任务；CList 使用 Go 单体并按自身匿名游客模型重新实现。
- [telegram-bot-api-file-streaming](https://github.com/sunnyhmz7010/telegram-bot-api-file-streaming)：接入其基于 `file_id` 的完整顺序流；当前版本不支持 `Range`、`HEAD` 和断点续传，因此 CList 首版显式声明该能力差异。

所有实现代码将重新编写，参考项目仅用于产品特性和边界分析。
