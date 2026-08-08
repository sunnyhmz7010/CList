# CList 完整实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: 使用 `superpowers:subagent-driven-development`（推荐）或 `superpowers:executing-plans` 执行本计划。所有步骤使用 checkbox 跟踪，并在每个任务结束时运行对应验证。

**Goal:** 在纯 Docker、零环境变量启动的前提下，分期交付 CList 单租户网盘：本地/官方 Telegram/自建 Telegram Streaming 三种存储、匿名游客与管理员权限、可恢复分片上传、逻辑目录、相册预览、固定回收站、跨存储迁移和 REST API。

**Architecture:** 使用 Go 模块化单体承载 HTTP API、后台任务和存储适配器，SQLite WAL 保存唯一元数据；React + TypeScript + Vite 构建前端并通过 `go:embed` 嵌入 Go 二进制。Telegram 文件始终以频道消息为物理对象，文件夹、相册状态、回收站和公开链接只由 SQLite 维护。

**Tech Stack:** Go 1.26.5、`modernc.org/sqlite` v1.56.0、`github.com/go-chi/chi/v5` v5.3.1、React 19.2.8、React Router DOM 7.18.2、TanStack Query 5.101.4、Vite 8.2.1、TypeScript 7.0.2、Vitest 4.1.10、Playwright 1.62.1、`docx-preview` 0.4.0、`pdfjs-dist` 6.2.108、Docker `node:24-alpine`/Go 多阶段构建。

## Global Constraints

- 纯 Docker；默认命令不需要任何环境变量：`docker run -d --name clist --restart unless-stopped -p 8080:8080 -v clist_data:/data ghcr.io/sunnyhmz7010/clist:latest`。
- SQLite 是唯一必需元数据数据库，启动时启用 `journal_mode=WAL`、外键约束、忙等待和合理同步级别；不使用 Redis、PostgreSQL、Nginx 或 Cloudflare。
- 不实现注册、用户列表、用户组、配额、计费、OIDC、自动负载均衡、多实例共享 SQLite、回收站自动清理。
- 所有大文件先进入 `/data/chunks` 分片目录，支持乱序、重复、缺失查询、断点恢复、重启恢复和 SHA-256 校验，不把完整文件一次性读入内存。
- 官方 Telegram 与 Local 尽量提供 `Range`；自建 `telegram-bot-api-file-streaming` 只声明完整顺序流，前端必须提示不支持断点续传。
- Telegram 频道消息不携带 CList 文件夹；`folder_id`、相册继承、回收站和公开 ID 均由 SQLite 管理。
- 删除固定先软删除进入回收站；仅手动彻底删除才调用 Telegram `deleteMessage` 或删除本地对象。
- 公开 ID 为 `crypto/rand` 生成的 128 位 URL 安全随机值，改名、移动、恢复和迁移不得改变直链。
- 日志、URL 和持久化内容不得泄露 Bot Token、API Token、恢复密钥、Cookie 或密码；所有后端模板输出必须转义。
- 每个代码任务遵循 TDD：先写失败测试，验证失败，再写最小实现，验证通过后使用中文 Commit Message 提交。
- 开始 Phase 1 前先使用 `superpowers:using-git-worktrees` 创建独立工作区；未经用户明确同意，不在 `main` 直接执行较大实现。

## 文件结构与职责

以下路径是计划中的完整边界；任务只创建或修改列出的文件，后续任务沿用已定义的接口。

```text
go.mod                         Go 模块与依赖
cmd/clist/main.go              进程入口、嵌入前端、优雅退出
internal/config/config.go      默认配置、数据目录和端点校验
internal/crypto/secrets.go     主密钥、哈希、加密配置、随机 ID
internal/db/db.go              SQLite 连接、WAL 参数、事务辅助
internal/db/migrations/*.sql   按序数据库迁移
internal/db/repository/*.go    settings、文件、目录、会话、任务仓储
internal/auth/*.go              管理员、游客、恢复密钥、Token 鉴权
internal/storage/storage.go    统一存储适配器契约与能力模型
internal/storage/local/*.go    本地对象存储与 Range
internal/storage/telegram/*.go 官方 Bot API 适配器
internal/storage/streaming/*.go 自建 Streaming API 适配器
internal/files/*.go             文件、目录、相册可见性应用服务
internal/uploads/*.go           分片任务、合并、校验和恢复
internal/jobs/*.go              租约、重试、心跳和启动恢复
internal/trash/*.go             回收站批次、恢复和彻底删除
internal/migration/*.go         跨存储迁移与 cleanup_pending
internal/preview/*.go           预览授权、DOCX/PDF/TXT 和缓存
internal/webhook/*.go           Telegram Webhook 校验、幂等入库、回链
internal/api/*.go               chi 路由、DTO、错误映射和 HTTP 处理器
internal/health/*.go            live/ready、诊断和备份命令
web/package.json                前端依赖和脚本
web/vite.config.ts              Vite 构建配置
web/src/main.tsx                React 入口
web/src/app/*.tsx               路由、布局、主题
web/src/api/*.ts                类型安全 API 客户端
web/src/features/*              上传、文件、目录、相册、回收站、迁移
web/src/components/*            通用组件、预览器、错误边界
web/tests/*.spec.ts             Playwright 关键路径
Dockerfile                      前后端多阶段镜像
docker-compose.yml              本地零环境变量示例
README.md                       安装、首次初始化和能力差异
```

---

## Phase 1：工程基础与本地网盘 MVP

### Task 1: 初始化 Go/React/Docker 工程骨架

**Files:**
- Create: `go.mod`, `cmd/clist/main.go`, `web/package.json`, `web/tsconfig.json`, `web/vite.config.ts`, `web/src/main.tsx`, `Dockerfile`, `docker-compose.yml`
- Test: `cmd/clist/main_test.go`, `web/src/main.test.tsx`

**Interfaces:**
- Produces `main()`、`web` 的 `build`/`test` 脚本，以及后续任务使用的 `/app`、`/data` 容器布局。

- [ ] **Step 1: 写入口失败测试**

```go
func TestDefaultDataDir(t *testing.T) {
    if got := defaultDataDir(); got != "/data" {
        t.Fatalf("defaultDataDir() = %q", got)
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./cmd/clist -run TestDefaultDataDir -count=1
```

预期：因 `defaultDataDir` 未定义而失败。

- [ ] **Step 3: 写最小骨架实现**

```go
package main

import (
    "embed"
    "log"
    "net/http"
)

//go:embed web-dist/*
var frontend embed.FS

func defaultDataDir() string { return "/data" }

func main() {
    log.Fatal(http.ListenAndServe(":8080", http.FileServer(http.FS(frontend))))
}
```

同步添加 `web/package.json` 的 `build`、`test` 脚本和 Vite React 入口；Dockerfile 先构建前端，再构建 Go 二进制并复制到 `gcr.io/distroless/static-debian12`。

- [ ] **Step 4: 验证通过**

```bash
go test ./cmd/clist -run TestDefaultDataDir -count=1
npm --prefix web install
npm --prefix web run build
docker build -t clist:plan-check .
```

- [ ] **Step 5: 提交**

```bash
git add go.mod cmd/clist web Dockerfile docker-compose.yml
git commit -m "工程：初始化 CList 单体与前端骨架"
```

### Task 2: 配置、主密钥与 SQLite WAL 启动

**Files:**
- Create: `internal/config/config.go`, `internal/crypto/secrets.go`, `internal/db/db.go`, `internal/db/db_test.go`
- Modify: `cmd/clist/main.go`

**Interfaces:**
- `config.Load(dataDir string) (config.Config, error)`；`db.Open(ctx, path string) (*sql.DB, error)`；`crypto.MasterKey.LoadOrCreate(path string) ([]byte, error)`。

- [ ] **Step 1: 写 WAL 与零环境变量失败测试**

```go
func TestOpenEnablesWALAndForeignKeys(t *testing.T) {
    db, err := Open(context.Background(), filepath.Join(t.TempDir(), "clist.db"))
    if err != nil { t.Fatal(err) }
    var journal, foreignKeys string
    if err := db.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil { t.Fatal(err) }
    if err := db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil { t.Fatal(err) }
    if journal != "wal" || foreignKeys != "1" { t.Fatalf("journal=%s fk=%s", journal, foreignKeys) }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/db -run TestOpenEnablesWALAndForeignKeys -count=1
```

预期：`Open` 未实现而失败。

- [ ] **Step 3: 实现启动配置**

```go
func Open(ctx context.Context, path string) (*sql.DB, error) {
    db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
    if err != nil { return nil, err }
    for _, stmt := range []string{
        "PRAGMA journal_mode=WAL",
        "PRAGMA foreign_keys=ON",
        "PRAGMA synchronous=NORMAL",
    } { if _, err = db.ExecContext(ctx, stmt); err != nil { db.Close(); return nil, err } }
    return db, nil
}
```

`config.Load` 固定默认 `DataDir=/data`、`ListenAddr=:8080`，只接受显式 CLI 覆盖；`MasterKey.LoadOrCreate` 使用 `0600` 文件权限和 32 字节随机值。启动时创建 `/data/files`、`/data/chunks`、`/data/migrations`、`/data/cache/previews`、`/data/secrets`，数据库为空时建立指向 `/data/files` 的默认 Local 存储档案。

- [ ] **Step 4: 验证通过**

```bash
go test ./internal/db ./internal/config ./internal/crypto -count=1
```

- [ ] **Step 5: 提交**

```bash
git add internal/config internal/crypto internal/db cmd/clist/main.go
git commit -m "基础设施：启用 SQLite WAL 与数据目录配置"
```

### Task 3: 数据库迁移与核心仓储

**Files:**
- Create: `internal/db/migrations/001_core.sql`, `internal/db/migrations/002_jobs.sql`, `internal/db/migrations/003_auth.sql`, `internal/db/migrate.go`, `internal/db/repository/files.go`, `internal/db/repository/folders.go`, `internal/db/repository/settings.go`, `internal/db/repository/idempotency.go`, `internal/db/migrate_test.go`

**Interfaces:**
- `migrate.Apply(ctx, db) error`；`repository.FileRepo.Create/List/Get/UpdateState`；`repository.FolderRepo.Create/List/Move`；`repository.IdempotencyRepo.Get/Reserve/Complete`；领域枚举 `FileState=uploading|active|trashed|purged`、`Visibility=inherit|visible|hidden`。

- [ ] **Step 1: 写迁移失败测试**

```go
func TestApplyCreatesCoreTables(t *testing.T) {
    db := testDB(t)
    if err := Apply(context.Background(), db); err != nil { t.Fatal(err) }
    for _, name := range []string{"settings", "storage_profiles", "guest_vaults", "folders", "files", "file_secrets", "trash_batches", "trash_items", "jobs", "upload_sessions", "upload_chunks", "api_tokens", "sessions", "audit_logs", "telegram_messages", "idempotency_keys"} {
        var got string
        if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&got); err != nil { t.Fatalf("missing %s: %v", name, err) }
    }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/db -run TestApplyCreatesCoreTables -count=1
```

- [ ] **Step 3: 实现迁移和仓储**

```sql
CREATE TABLE IF NOT EXISTS files (
  public_id TEXT PRIMARY KEY, folder_id TEXT, owner_vault_id TEXT,
  storage_profile_id TEXT NOT NULL, storage_key TEXT, file_name TEXT NOT NULL,
  mime_type TEXT NOT NULL, size INTEGER NOT NULL, sha256 TEXT NOT NULL,
  gallery_visibility TEXT NOT NULL DEFAULT 'inherit', state TEXT NOT NULL,
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_files_state_created ON files(state, created_at);
```

三个初始迁移一次性建立规格中的全部实体以及 `upload_sessions`、`upload_chunks`、`trash_items`、`telegram_messages`、`idempotency_keys` 辅助表；迁移器按文件名排序，在事务中写入 `schema_migrations`。仓储全部使用参数绑定，列表使用 `public_id` 游标和 `LIMIT ?`；后续任务不得回改已执行迁移，只消费这里定义的表和列。

- [ ] **Step 4: 验证通过**

```bash
go test ./internal/db/... -count=1
```

- [ ] **Step 5: 提交**

```bash
git add internal/db
git commit -m "数据库：建立核心表与文件目录仓储"
```

### Task 4: 管理员首次初始化与会话鉴权

**Files:**
- Create: `internal/auth/password.go`, `internal/auth/admin.go`, `internal/auth/session.go`, `internal/auth/auth_test.go`, `internal/api/auth_handlers.go`
- Modify: `cmd/clist/main.go`

**Interfaces:**
- `AdminService.Initialize(ctx, account, password) error`、`Login(ctx, account, password) (Session, error)`、`ChangePassword(ctx, oldPassword, newPassword) error`；`Authenticator.RequireAdmin(http.Handler) http.Handler`；会话 Cookie 名称 `clist_admin`。

- [ ] **Step 1: 写密码与初始化失败测试**

```go
func TestInitializeIsSingleUseAndLoginUsesConstantTimeCheck(t *testing.T) {
    svc := newAuthService(t)
    if err := svc.Initialize(ctx, "admin", "correct horse battery"); err != nil { t.Fatal(err) }
    if err := svc.Initialize(ctx, "other", "another"); !errors.Is(err, ErrAlreadyInitialized) { t.Fatalf("got %v", err) }
    if _, err := svc.Login(ctx, "admin", "wrong"); !errors.Is(err, ErrUnauthorized) { t.Fatalf("got %v", err) }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/auth -run TestInitializeIsSingleUseAndLoginUsesConstantTimeCheck -count=1
```

- [ ] **Step 3: 实现 Argon2id 密码哈希、事务初始化和 HttpOnly/SameSite 会话**

```go
func (s *Service) Initialize(ctx context.Context, account, password string) error {
    hash, err := hashPassword(password); if err != nil { return err }
    return s.db.WithTx(ctx, func(tx *sql.Tx) error {
        var n int; if err := tx.QueryRow("SELECT COUNT(*) FROM settings WHERE key='admin.account'").Scan(&n); err != nil { return err }
        if n != 0 { return ErrAlreadyInitialized }
        _, err = tx.ExecContext(ctx, "INSERT INTO settings(key,value) VALUES('admin.account',?),('admin.password_hash',?)", account, hash)
        return err
    })
}
```

加入失败限速、修改密码后注销全部旧会话和 CSRF 双提交 Token；API 错误统一返回 `code/message/request_id/retriable`。

- [ ] **Step 4: 验证通过**

```bash
go test ./internal/auth ./internal/api -run 'Auth|Login|Initialize' -count=1
```

- [ ] **Step 5: 提交**

```bash
git add internal/auth internal/api/auth_handlers.go cmd/clist/main.go
git commit -m "认证：加入管理员初始化与安全会话"
```

### Task 5: 存储契约与 Local 适配器

**Files:**
- Create: `internal/storage/storage.go`, `internal/storage/local/local.go`, `internal/storage/local/local_test.go`, `internal/storage/registry.go`

**Interfaces:**
- `storage.Backend`：`Validate(ctx) error`、`Capabilities() Capabilities`、`HealthCheck(ctx) error`、`Put(ctx, io.Reader, ObjectMeta) (Object, error)`、`Open(ctx, key string, r *ByteRange) (Reader, error)`、`Delete(ctx, key string) error`。
- `Capabilities{Range, Head, Streaming bool}`；`Object{Key, Size, SHA256 string}`；`Reader{io.ReadCloser, Size int64, ContentRange string, Partial bool}`。

- [ ] **Step 1: 写 Local 原子写入和路径边界失败测试**

```go
func TestLocalPutUsesStableKeyAndRejectsEscape(t *testing.T) {
    root := t.TempDir(); b := New(root)
    obj, err := b.Put(ctx, strings.NewReader("hello"), storage.ObjectMeta{Key:"objects/a"})
    if err != nil || obj.Key != "objects/a" { t.Fatalf("obj=%+v err=%v", obj, err) }
    if _, err := b.Open(ctx, "../secret", nil); !errors.Is(err, ErrPathEscape) { t.Fatalf("got %v", err) }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/storage/local -run TestLocalPutUsesStableKeyAndRejectsEscape -count=1
```

- [ ] **Step 3: 实现 fsync + 原子 rename、符号链接解析和单区间 Range**

```go
func (b *Backend) Put(ctx context.Context, r io.Reader, meta storage.ObjectMeta) (storage.Object, error) {
    final, err := b.safePath(meta.Key); if err != nil { return storage.Object{}, err }
    if err := os.MkdirAll(filepath.Dir(final), 0o750); err != nil { return storage.Object{}, err }
    tmp, err := os.CreateTemp(filepath.Dir(final), ".upload-*"); if err != nil { return storage.Object{}, err }
    defer os.Remove(tmp.Name()); h := sha256.New()
    n, err := io.Copy(io.MultiWriter(tmp, h), r); if err != nil { tmp.Close(); return storage.Object{}, err }
    if err = tmp.Sync(); err == nil { err = tmp.Close() }; if err != nil { return storage.Object{}, err }
    if err = os.Rename(tmp.Name(), final); err != nil { return storage.Object{}, err }
    return storage.Object{Key:meta.Key, Size:n, SHA256:hex.EncodeToString(h.Sum(nil))}, nil
}
```

- [ ] **Step 4: 验证通过**

```bash
go test ./internal/storage/... -count=1
```

- [ ] **Step 5: 提交**

```bash
git add internal/storage
git commit -m "存储：定义适配器契约并实现本地后端"
```

### Task 6: 文件与逻辑目录应用服务及列表 API

**Files:**
- Create: `internal/files/service.go`, `internal/files/service_test.go`, `internal/api/file_handlers.go`, `internal/api/folder_handlers.go`, `internal/api/router.go`

**Interfaces:**
- `FileService.CreateIndex(input CreateFileInput) (File, error)`、`List(ctx, Actor, Cursor, Limit) (Page[File], error)`、`Rename`、`Move`；`FolderService.Create/List/Move/Delete`。
- 路由：`GET /api/v1/files`、`GET /api/v1/files/{id}`、`PATCH/DELETE /api/v1/files/{id}`、`GET/POST /api/v1/folders`、`PATCH/DELETE /api/v1/folders/{id}`。

- [ ] **Step 1: 写公开 ID 稳定和逻辑移动失败测试**

```go
func TestMoveDoesNotChangePublicID(t *testing.T) {
    f := seedFile(t, "a-folder")
    before := f.PublicID
    if err := service.Move(ctx, f.PublicID, "b-folder", actor); err != nil { t.Fatal(err) }
    got, _ := service.Get(ctx, before, actor)
    if got.PublicID != before || got.FolderID != "b-folder" { t.Fatalf("got %+v", got) }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/files -run TestMoveDoesNotChangePublicID -count=1
```

- [ ] **Step 3: 实现服务和 DTO**

```go
type CreateFileInput struct { FileName, MIMEType string; Size int64; SHA256, FolderID, OwnerVaultID, StorageProfileID, StorageKey string }
func (s *Service) Move(ctx context.Context, publicID, folderID string, actor auth.Actor) error {
    f, err := s.repo.Get(ctx, publicID); if err != nil { return err }
    if err = s.authz.CanManage(actor, f); err != nil { return err }
    return s.repo.Move(ctx, publicID, folderID)
}
```

路由层只做 JSON 解码、鉴权、服务调用和统一错误映射；所有文件名在 HTML/JSON 外发前按上下文转义。

- [ ] **Step 4: 验证通过**

```bash
go test ./internal/files ./internal/api -count=1
```

- [ ] **Step 5: 提交**

```bash
git add internal/files internal/api
git commit -m "文件管理：加入逻辑目录与稳定链接列表接口"
```

### Task 7: 可恢复分片上传、合并校验与本地下载

**Files:**
- Create: `internal/uploads/service.go`, `internal/uploads/service_test.go`, `internal/api/upload_handlers.go`, `internal/api/download_handlers.go`
- Modify: `internal/api/router.go`, `internal/jobs/*`

**Interfaces:**
- `UploadService.Init(ctx, InitInput) (Upload, error)`、`PutChunk(ctx, uploadID string, index int, r io.Reader, sha256 string) error`、`MissingChunks(ctx, uploadID) ([]int, error)`、`Complete(ctx, uploadID string) (File, error)`、`Abort(ctx, uploadID string) error`。
- 路由完整覆盖 `POST /api/v1/uploads`、`PUT /api/v1/uploads/{id}/chunks/{index}`、`GET /api/v1/uploads/{id}`、`POST /api/v1/uploads/{id}/complete`、`DELETE /api/v1/uploads/{id}`；初始化和完成使用 `Idempotency-Key`。
- `GET /f/{public_id}/{filename}` 支持 `HEAD` 和单区间 `Range`，状态为 `trashed/purged` 返回 `410`，并输出 `Content-Length`、`Content-Range`、`Accept-Ranges` 和 `X-CList-Storage-Capabilities`。

- [ ] **Step 1: 写乱序、重复、缺失和哈希失败测试**

```go
func TestChunksAreResumableAndCompleteChecksSHA256(t *testing.T) {
    u := newUpload(t, 2, "aGVsbG8gd29ybGQ=")
    if err := svc.PutChunk(ctx, u.ID, 1, strings.NewReader("world"), sha256sum("world")); err != nil { t.Fatal(err) }
    if err := svc.PutChunk(ctx, u.ID, 0, strings.NewReader("hello "), sha256sum("hello ")); err != nil { t.Fatal(err) }
    if err := svc.PutChunk(ctx, u.ID, 0, strings.NewReader("hello "), sha256sum("hello ")); err != nil { t.Fatal(err) }
    if _, err := svc.Complete(ctx, u.ID); err != nil { t.Fatal(err) }
    missing, _ := svc.MissingChunks(ctx, u.ID); if len(missing) != 0 { t.Fatalf("missing=%v", missing) }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/uploads -run TestChunksAreResumableAndCompleteChecksSHA256 -count=1
```

- [ ] **Step 3: 实现分片落盘和流式合并**

```go
func (s *Service) PutChunk(ctx context.Context, id string, index int, r io.Reader, want string) error {
    path := s.chunkPath(id, index); tmp := path + ".tmp"
    f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640); if err != nil { return err }
    h := sha256.New(); if _, err = io.Copy(io.MultiWriter(f, h), io.LimitReader(r, s.maxChunk)); err != nil { f.Close(); return err }
    if err = f.Sync(); err == nil { err = f.Close() }; if err != nil { return err }
    if hex.EncodeToString(h.Sum(nil)) != want { os.Remove(tmp); return ErrChunkHashMismatch }
    return os.Rename(tmp, path)
}
```

合并前查询缺失索引，按序把分片流送入 `storage.Backend.Put`，事务写入文件索引并完成幂等键后删除分片；重复初始化或完成请求返回同一结果。取消上传删除其临时分片和任务记录；启动恢复由 `jobs.RecoverExpired` 重新排队。

- [ ] **Step 4: 验证通过**

```bash
go test ./internal/uploads ./internal/jobs ./internal/api -count=1
```

- [ ] **Step 5: 提交**

```bash
git add internal/uploads internal/jobs internal/api
git commit -m "上传：实现可恢复分片与本地 Range 下载"
```

### Task 8: Phase 1 前端文件管理和 Docker 验收门

**Files:**
- Create: `web/src/app/App.tsx`, `web/src/app/router.tsx`, `web/src/api/client.ts`, `web/src/features/files/FileList.tsx`, `web/src/features/files/FolderTree.tsx`, `web/src/features/uploads/UploadQueue.tsx`, `web/src/styles.css`, `web/tests/local-mvp.spec.ts`

**Interfaces:**
- `apiClient<T>(path, init): Promise<T>`；上传队列使用 `UploadQueueItem{uploadId,file,chunkSize,missing,state}`；页面路由 `/`、`/setup`、`/admin`。

- [ ] **Step 1: 写组件失败测试**

```tsx
it('shows local file list and upload progress', () => {
  render(<FileList files={[{publicId:'p1', fileName:'a.txt', size:5}]} />)
  expect(screen.getByText('a.txt')).toBeInTheDocument()
  expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '0')
})
```

- [ ] **Step 2: 验证失败**

```bash
npm --prefix web test -- --run src/features/files/FileList.test.tsx
```

- [ ] **Step 3: 实现明亮效率布局、主题切换和分片队列**

```tsx
export function FileList({files}: {files: FileSummary[]}) {
  return <ul>{files.map(f => <li key={f.publicId}><a href={`/f/${f.publicId}/${encodeURIComponent(f.fileName)}`}>{f.fileName}</a><progress max="100" value="0" /></li>)}</ul>
}
```

使用 TanStack Query 管理列表，`UploadQueue` 并发 3 个分片、先调用缺失查询再补传，页面显示后端 `Capabilities`。

- [ ] **Step 4: 验证通过并执行 Phase 1 门**

```bash
npm --prefix web test -- --run
npm --prefix web run build
docker compose up -d --build
curl -fsS http://localhost:8080/health/live
```

预期：首次访问出现管理员初始化页；本地文件可上传、列表、改名、移动、下载，容器重启后索引不丢失。

- [ ] **Step 5: 提交**

```bash
git add web
git commit -m "前端：交付本地网盘 MVP 文件管理界面"
```

---

## Phase 2：游客访问与恢复

### Task 9: 首页、相册和单文件密码作用域

**Files:**
- Create: `internal/auth/guest.go`, `internal/auth/file_password.go`, `internal/auth/guest_test.go`, `internal/api/guest_handlers.go`, `internal/api/file_access_handlers.go`
- Modify: `internal/auth/session.go`, `internal/api/router.go`

**Interfaces:**
- `GuestService.SetHomePassword/SetGalleryPassword/ClearPassword`；`FilePasswordService.Set/Clear/Verify`；`GuestAuthenticator.RequireScope(scope Scope) middleware`；作用域常量 `home`、`gallery`、`file:{public_id}`。文件密码路由为 `PUT/DELETE /api/v1/files/{id}/password` 与 `POST /api/v1/files/{id}/access`。

- [ ] **Step 1: 写作用域隔离失败测试**

```go
func TestHomeSessionCannotEnterGallery(t *testing.T) {
    home := loginGuest(t, ScopeHome, "pw")
    if err := authz.RequireScope(home, ScopeGallery); !errors.Is(err, ErrScopeRequired) { t.Fatalf("got %v", err) }
    if err := authz.RequireFile(home, "public-1"); !errors.Is(err, ErrScopeRequired) { t.Fatalf("home session bypassed file password") }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/auth -run TestHomeSessionCannotEnterGallery -count=1
```

- [ ] **Step 3: 实现密码设置、限速和独立 HttpOnly Cookie**

```go
type Scope string
const ( ScopeHome Scope = "home"; ScopeGallery Scope = "gallery" )
```

首页默认无密码；管理员开启后仅验证 `/api/v1/guest/home/session`，相册另行验证 `/api/v1/guest/gallery/session`。管理员或对应游客恢复密钥可以设置/清除单文件密码；验证成功只签发该 `public_id` 的短期 HttpOnly 会话，文件直链和预览在发送响应体前检查该作用域。

- [ ] **Step 4: 验证通过**

```bash
go test ./internal/auth ./internal/api -run 'Guest|Scope|Password' -count=1
```

- [ ] **Step 5: 提交**

```bash
git add internal/auth internal/api
git commit -m "游客：加入首页与相册独立访问密码"
```

### Task 10: 匿名恢复密钥与游客上传记录

**Files:**
- Create: `internal/auth/vault.go`, `internal/auth/vault_test.go`, `internal/api/vault_handlers.go`
- Modify: `internal/files/service.go`

**Interfaces:**
- `VaultService.Create() (GuestVault, plaintextKey string, error)`；`Recover(ctx, plaintextKey) (GuestVault, error)`；`Revoke`；`GET /api/v1/vault/files`。

- [ ] **Step 1: 写高熵密钥只存哈希失败测试**

```go
func TestVaultStoresHashAndRecoversAcrossDevices(t *testing.T) {
    vault, key, err := svc.Create(); if err != nil { t.Fatal(err) }
    if vault.KeyHash == key || len(key) < 32 { t.Fatalf("weak or plaintext key") }
    got, err := svc.Recover(ctx, key); if err != nil || got.ID != vault.ID { t.Fatalf("got=%+v err=%v", got, err) }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/auth -run TestVaultStoresHashAndRecoversAcrossDevices -count=1
```

- [ ] **Step 3: 实现随机密钥、哈希、浏览器保存和权限绑定**

```go
func (s *VaultService) Create() (GuestVault, string, error) {
    raw, err := randomURLSafe(32); if err != nil { return GuestVault{}, "", err }
    hash := sha256.Sum256([]byte(raw))
    v := GuestVault{ID: randomID(), KeyHash: hex.EncodeToString(hash[:])}
    return v, raw, s.repo.Insert(v)
}
```

游客创建文件时写入 `owner_vault_id`；恢复成功后签发仅能访问自己文件的会话，撤销后旧会话立即失效。

- [ ] **Step 4: 验证通过**

```bash
go test ./internal/auth ./internal/files ./internal/api -run 'Vault|Owner' -count=1
```

- [ ] **Step 5: 提交**

```bash
git add internal/auth internal/files internal/api
git commit -m "游客记录：加入匿名恢复密钥与跨设备恢复"
```

### Task 11: 游客分片上传界面与 Phase 2 验收门

**Files:**
- Modify: `web/src/features/uploads/UploadQueue.tsx`, `web/src/features/files/FileList.tsx`, `web/src/app/App.tsx`
- Create: `web/src/features/auth/GuestGate.tsx`, `web/src/features/auth/GuestRecovery.tsx`, `web/src/features/settings/AccessSettings.tsx`, `web/src/features/files/FileAccessDialog.tsx`, `web/tests/guest-recovery.spec.ts`

**Interfaces:**
- `localStorage['clist_guest_recovery_key']`；`UploadQueue` 在初始化时调用 `GET /api/v1/uploads/{id}`，将 `missingChunks` 重新排队。

- [ ] **Step 1: 写恢复密钥展示失败测试**

```tsx
it('offers copy and download for a newly created recovery key', async () => {
  render(<GuestRecovery keyValue="k" />)
  expect(screen.getByText('请立即保存恢复密钥')).toBeInTheDocument()
  expect(screen.getByRole('button', {name:'复制'})).toBeInTheDocument()
})
```

- [ ] **Step 2: 验证失败**

```bash
npm --prefix web test -- --run src/features/auth/GuestRecovery.test.tsx
```

- [ ] **Step 3: 实现恢复流程和断点上传状态机**

```ts
export async function resumeUpload(id: string) {
  const state = await apiClient<UploadState>(`/api/v1/uploads/${id}`)
  for (const index of state.missingChunks) await uploadChunk(id, index)
  return apiClient(`/api/v1/uploads/${id}/complete`, {method:'POST'})
}
```

游客首次上传后弹出高熵恢复密钥；恢复页允许粘贴密钥并刷新“我的上传”，支持撤销并轮换密钥且不把密钥放进 URL。管理员访问设置页可分别启停首页密码与相册密码，管理员或文件所属游客可通过 `FileAccessDialog` 设置单文件密码。

- [ ] **Step 4: 验证通过并执行 Phase 2 门**

```bash
npm --prefix web test -- --run
npx playwright test web/tests/guest-recovery.spec.ts
```

预期：首页密码与相册密码互不替代；游客可上传、刷新、跨浏览器恢复记录并管理自己的文件。

- [ ] **Step 5: 提交**

```bash
git add web
git commit -m "游客前端：支持恢复密钥与断点上传"
```

---

## Phase 3：Telegram 存储与 Webhook

### Task 12: 存储档案配置加密与注册表

**Files:**
- Create: `internal/storage/profile.go`, `internal/storage/profile_test.go`, `internal/api/storage_handlers.go`
- Modify: `internal/storage/registry.go`, `internal/crypto/secrets.go`

**Interfaces:**
- `ProfileService.Create/Update/List/SetDefault`；`Registry.Resolve(profileID) storage.Backend`；配置 JSON 使用 AES-256-GCM 主密钥加密。

- [ ] **Step 1: 写配置不落明文失败测试**

```go
func TestProfileConfigIsEncryptedAtRest(t *testing.T) {
    svc := newProfileService(t, bytes.Repeat([]byte("x"), 32))
    if err := svc.Create(ctx, ProfileInput{Type:"telegram_official", Config:map[string]string{"bot_token":"secret"}}); err != nil { t.Fatal(err) }
    raw := readColumn(t, "encrypted_config"); if bytes.Contains(raw, []byte("secret")) { t.Fatal("plaintext secret persisted") }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/storage -run TestProfileConfigIsEncryptedAtRest -count=1
```

- [ ] **Step 3: 实现加密配置与默认档案选择**

```go
func EncryptConfig(key []byte, value []byte) ([]byte, error) {
    block, err := aes.NewCipher(key); if err != nil { return nil, err }
    gcm, err := cipher.NewGCM(block); if err != nil { return nil, err }
    nonce := make([]byte, gcm.NonceSize()); if _, err = rand.Read(nonce); err != nil { return nil, err }
    return gcm.Seal(nonce, nonce, value, nil), nil
}
```

新增档案类型校验：`local`、`telegram_official`、`telegram_streaming`；自建 API 地址仅允许管理员提交的 `http/https` 绝对 URL。Local 档案只接受已挂载的绝对目录，规范化路径、解析符号链接，并复用 `local.safePath` 验证最终对象位于配置根目录内。

- [ ] **Step 4: 验证通过**

```bash
go test ./internal/storage ./internal/api -run 'Profile|Registry' -count=1
```

- [ ] **Step 5: 提交**

```bash
git add internal/storage internal/crypto internal/api
git commit -m "存储配置：加入加密档案与后端注册表"
```

### Task 13: 官方 Telegram Bot API 适配器

**Files:**
- Create: `internal/storage/telegram/official.go`, `internal/storage/telegram/official_test.go`, `internal/storage/telegram/client.go`

**Interfaces:**
- `OfficialBackend` 实现 `storage.Backend`；`sendDocument` 返回 `chat_id/message_id/file_id/file_unique_id`；`Open` 先 `getFile`，再代理文件 URL，并在上游支持时转发单区间 Range。

- [ ] **Step 1: 写模拟 Bot API 契约失败测试**

```go
func TestOfficialPutSendsToConfiguredChannel(t *testing.T) {
    srv := newBotAPIMock(t)
    b := New(telegram.Config{BaseURL:srv.URL, BotToken:"token", ChannelID:"-1001"})
    obj, err := b.Put(ctx, strings.NewReader("data"), storage.ObjectMeta{FileName:"a.txt", MIMEType:"text/plain"})
    if err != nil || obj.Telegram.MessageID == 0 || srv.LastChatID != "-1001" { t.Fatalf("obj=%+v chat=%s err=%v", obj, srv.LastChatID, err) }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/storage/telegram -run TestOfficialPutSendsToConfiguredChannel -count=1
```

- [ ] **Step 3: 实现 multipart 上传、getFile、Range 和删除**

```go
type TelegramRef struct { ChatID, FileID, FileUniqueID string; MessageID int64; Size int64 }
func (b *OfficialBackend) Delete(ctx context.Context, key string) error {
    ref, err := decodeTelegramKey(key); if err != nil { return err }
    return b.client.DeleteMessage(ctx, ref.ChatID, ref.MessageID)
}
```

未知超时由上层任务标记 `uncertain`，适配器不自行重发。上传前调用对应 Bot API 能力和文件大小限制校验；超过当前档案允许大小时在发送频道消息前返回确定性错误。

- [ ] **Step 4: 验证通过**

```bash
go test ./internal/storage/telegram -count=1
```

- [ ] **Step 5: 提交**

```bash
git add internal/storage/telegram
git commit -m "Telegram：接入官方 Bot API 存储与 Range 下载"
```

### Task 14: 自建 Telegram Streaming 适配器与能力提示

**Files:**
- Create: `internal/storage/streaming/streaming.go`, `internal/storage/streaming/streaming_test.go`
- Modify: `internal/storage/profile.go`, `internal/storage/storage.go`

**Interfaces:**
- `StreamingBackend` 实现 `storage.Backend`；流式 URL 格式 `/stream/file/bot<TOKEN>/<URL_ENCODED_FILE_ID>`；`Capabilities()` 固定返回 `Range=false, Head=false, Streaming=true`。

- [ ] **Step 1: 写不支持续传失败测试**

```go
func TestStreamingCapabilitiesRejectRange(t *testing.T) {
    b := New(Config{BaseURL:"http://bot-api", BotToken:"t"})
    if b.Capabilities().Range || b.Capabilities().Head { t.Fatal("streaming backend advertised unsupported capability") }
    if _, err := b.Open(ctx, "file-id", &storage.ByteRange{Start:1, End:2}); !errors.Is(err, storage.ErrRangeUnsupported) { t.Fatalf("got %v", err) }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/storage/streaming -run TestStreamingCapabilitiesRejectRange -count=1
```

- [ ] **Step 3: 实现完整顺序 GET 和可信文件大小头**

```go
func (b *Backend) streamURL(fileID string) string {
    return strings.TrimRight(b.baseURL, "/") + "/stream/file/bot" + url.PathEscape(b.token) + "/" + url.PathEscape(fileID)
}
```

请求只在服务端发出，设置 `X-Telegram-File-Size`，拒绝客户端传入同名敏感头；删除调用自建 API 的 `deleteMessage` 兼容端点。

- [ ] **Step 4: 验证通过**

```bash
go test ./internal/storage/streaming ./internal/storage -count=1
```

- [ ] **Step 5: 提交**

```bash
git add internal/storage/streaming internal/storage
git commit -m "Telegram：接入自建流式 API 并声明能力差异"
```

### Task 15: 存储编排、任务状态和不确定发送结果

**Files:**
- Create: `internal/jobs/worker.go`, `internal/jobs/worker_test.go`, `internal/storage/orchestrator.go`, `internal/storage/orchestrator_test.go`
- Modify: `internal/uploads/service.go`

**Interfaces:**
- `JobStore.Enqueue/Lease/Heartbeat/Finish/RecoverExpired`；`Orchestrator.Put(profileID, stream, meta)`；`Orchestrator.ResolveUncertain(jobID, action, telegramRef)`，其中 `action=bind|retry|fail`；任务状态 `queued|running|retry_wait|succeeded|failed|cleanup_pending|uncertain`。

- [ ] **Step 1: 写未知结果不重试失败测试**

```go
func TestTimeoutAfterTelegramSendMarksUncertain(t *testing.T) {
    backend := fakeBackend{Err: context.DeadlineExceeded, Sent:true}
    result := orchestrator.Put(ctx, "tg", strings.NewReader("x"), meta)
    if result.State != jobs.Uncertain || result.Attempts != 1 { t.Fatalf("got %+v", result) }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/jobs ./internal/storage -run 'Uncertain|Lease' -count=1
```

- [ ] **Step 3: 实现租约心跳、指数退避和能力路由**

```go
func backoff(attempt int, seed time.Duration) time.Duration {
    max := seed << min(attempt, 6); return max/2 + time.Duration(rand.Int63n(int64(max/2)))
}
```

启动 worker 先 `RecoverExpired`，每个任务持有租约并定时心跳；可恢复网络错误进入 `retry_wait`，发送结果未知进入 `uncertain`。管理员可绑定已存在的频道消息、明确重新上传或标记失败；只有明确选择重试才再次发送。

- [ ] **Step 4: 验证通过**

```bash
go test ./internal/jobs ./internal/storage ./internal/uploads -count=1
```

- [ ] **Step 5: 提交**

```bash
git add internal/jobs internal/storage internal/uploads
git commit -m "任务：加入租约恢复与 Telegram 不确定状态"
```

### Task 16: Telegram Webhook 幂等入库与稳定回链

**Files:**
- Create: `internal/webhook/telegram.go`, `internal/webhook/telegram_test.go`, `internal/api/webhook_handlers.go`
- Modify: `internal/api/router.go`, `internal/storage/telegram/client.go`

**Interfaces:**
- `WebhookHandler.ServeHTTP` 校验 `X-Telegram-Bot-Api-Secret-Token`；`POST /webhooks/telegram/{profileSecret}`；`telegram_messages(chat_id,message_id)` 唯一键；回链格式 `/f/{public_id}/{filename}`。

- [ ] **Step 1: 写重复 Webhook 只入库一次失败测试**

```go
func TestWebhookIsIdempotentAndRepliesStableLink(t *testing.T) {
    payload := channelPostPayload("-1001", 42, "file-1", "a.png")
    first := postWebhook(t, payload); second := postWebhook(t, payload)
    if first.StatusCode != 200 || second.StatusCode != 200 || countFiles(t) != 1 { t.Fatal("webhook not idempotent") }
    if got := mockBotReplies(t)[0].Text; !strings.Contains(got, "/f/") { t.Fatal("missing stable link") }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/webhook -run TestWebhookIsIdempotentAndRepliesStableLink -count=1
```

- [ ] **Step 3: 实现 Secret/频道校验、媒体解析和回链**

```go
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Telegram-Bot-Api-Secret-Token")), h.secret) != 1 { http.Error(w, "forbidden", http.StatusForbidden); return }
    var u Update; if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&u); err != nil { http.Error(w, "bad request", 400); return }
    if err := h.service.IngestChannelPost(r.Context(), u); err != nil { h.errors.Write(w, err); return }; w.WriteHeader(http.StatusOK)
}
```

以 `chat_id + message_id` 做幂等键，频道直接上传归管理员；Webhook 路径只含随机公开密钥，不含 Bot Token。

- [ ] **Step 4: 验证通过**

```bash
go test ./internal/webhook ./internal/api -count=1
```

- [ ] **Step 5: 提交**

```bash
git add internal/webhook internal/api internal/storage/telegram
git commit -m "Webhook：实现 Telegram 频道幂等入库与直链回链"
```

### Task 17: Telegram 管理界面与 Phase 3 验收门

**Files:**
- Create: `web/src/features/settings/StorageProfiles.tsx`, `web/src/features/settings/WebhookSettings.tsx`, `web/src/features/jobs/UncertainJobDialog.tsx`, `web/src/components/CapabilityBadge.tsx`, `web/tests/telegram-capabilities.spec.ts`
- Modify: `web/src/api/client.ts`, `web/src/features/uploads/UploadQueue.tsx`

**Interfaces:**
- 前端类型 `StorageProfile{ id,type,name,capabilities,enabled,isDefault }`；上传响应包含 `capabilities.range/head/streaming`。

- [ ] **Step 1: 写能力提示失败测试**

```tsx
it('warns that streaming backend cannot resume', () => {
  render(<CapabilityBadge capabilities={{range:false,head:false,streaming:true}} />)
  expect(screen.getByText('当前后端不支持断点续传')).toBeInTheDocument()
})
```

- [ ] **Step 2: 验证失败**

```bash
npm --prefix web test -- --run src/components/CapabilityBadge.test.tsx
```

- [ ] **Step 3: 实现档案表单、Webhook Secret 展示和上传提示**

```tsx
export function CapabilityBadge({capabilities}: Props) {
  return capabilities.range ? <span>支持断点续传</span> : <span>当前后端不支持断点续传</span>
}
```

Bot Token 只允许写入，读取接口返回掩码；表单保存后刷新档案列表，不能把密钥拼到 URL 或日志。`UncertainJobDialog` 提供“绑定频道消息 / 重新上传 / 标记失败”三个明确动作，默认不自动重试。

- [ ] **Step 4: 验证通过并执行 Phase 3 门**

```bash
npm --prefix web test -- --run
npx playwright test web/tests/telegram-capabilities.spec.ts
```

预期：官方 Telegram 可上传/下载并尽量 Range，自建流式后端可完整下载且明确显示“不支持断点续传”；重复 Webhook 不产生重复文件。

- [ ] **Step 5: 提交**

```bash
git add web
git commit -m "Telegram 前端：加入存储档案与能力提示"
```

---

## Phase 4：相册与在线预览

### Task 18: 相册可见性继承与筛选 API

**Files:**
- Create: `internal/gallery/service.go`, `internal/gallery/service_test.go`, `internal/api/gallery_handlers.go`
- Modify: `internal/files/service.go`, `internal/db/repository/files.go`, `internal/api/router.go`

**Interfaces:**
- `GalleryService.ResolveVisibility(fileID) bool`：文件 `visible/hidden` 优先，`inherit` 递归查父目录，最终使用默认值 `visible`；`GalleryService.SetEnabled` 控制全站相册开关；`GET /api/v1/gallery?cursor=&type=&folder=`。

- [ ] **Step 1: 写三态继承失败测试**

```go
func TestFileVisibilityOverridesFolder(t *testing.T) {
    folder := seedFolder(t, VisibilityHidden); file := seedFileIn(t, folder, VisibilityVisible)
    if ok, _ := gallery.ResolveVisibility(ctx, file.PublicID); !ok { t.Fatal("file override ignored") }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/gallery -run TestFileVisibilityOverridesFolder -count=1
```

- [ ] **Step 3: 实现继承解析、游标分页和类型/名称/时间筛选**

```go
func (s *Service) ResolveVisibility(ctx context.Context, id string) (bool, error) {
    f, err := s.files.Get(ctx, id); if err != nil { return false, err }
    if f.GalleryVisibility == VisibilityVisible { return true, nil }
    if f.GalleryVisibility == VisibilityHidden { return false, nil }
    return s.folders.EffectiveVisibility(ctx, f.FolderID)
}
```

相册只列 `active` 且可见文件，管理员会话自动绕过相册密码，游客必须持有 `gallery` 作用域会话；管理员和对应游客恢复密钥都可以修改自己有权管理的文件/文件夹三态值。

- [ ] **Step 4: 验证通过**

```bash
go test ./internal/gallery ./internal/api -count=1
```

- [ ] **Step 5: 提交**

```bash
git add internal/gallery internal/files internal/db/repository internal/api
git commit -m "相册：实现文件夹继承与可见性筛选"
```

### Task 19: 预览授权、缓存和安全解析

**Files:**
- Create: `internal/preview/service.go`, `internal/preview/service_test.go`, `internal/api/preview_handlers.go`, `internal/preview/docx_limits.go`

**Interfaces:**
- `PreviewService.Open(ctx, publicID, actor) (Preview, error)`；`Preview{Kind, MIME, Body io.ReadCloser, Size int64}`；`GET /p/{public_id}` 与 `GET /api/v1/files/{id}/preview`。

- [ ] **Step 1: 写类型分派、删除文件和 DOCX 限制失败测试**

```go
func TestPreviewRejectsTrashedAndUnsafeDocx(t *testing.T) {
    f := seedTrashed(t, "a.pdf")
    if _, err := svc.Open(ctx, f.PublicID, admin); !errors.Is(err, ErrGone) { t.Fatalf("got %v", err) }
    if err := ValidateDocx(zipBombFixture()); !errors.Is(err, ErrArchiveLimit) { t.Fatalf("got %v", err) }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/preview -run TestPreviewRejectsTrashedAndUnsafeDocx -count=1
```

- [ ] **Step 3: 实现预览类型、安全头和可再生缓存**

```go
func kindFor(mime, name string) Kind {
    switch { case strings.HasPrefix(mime, "image/"): return KindImage; case strings.HasPrefix(mime, "video/"): return KindVideo; case strings.HasPrefix(mime, "audio/"): return KindAudio; case mime == "application/pdf": return KindPDF; case strings.HasSuffix(strings.ToLower(name), ".docx"): return KindDOCX; case mime == "text/plain": return KindText; default: return KindDownload }
}
```

TXT 限制 2 MiB 并以纯文本返回；DOCX 解压限制文件数、展开总大小和压缩比，浏览器端只读渲染；响应设置 CSP、`X-Content-Type-Options: nosniff` 和安全 `Content-Disposition`。

- [ ] **Step 4: 验证通过**

```bash
go test ./internal/preview ./internal/api -count=1
```

- [ ] **Step 5: 提交**

```bash
git add internal/preview internal/api
git commit -m "预览：支持媒体文档安全在线预览"
```

### Task 20: 相册与预览前端及 Phase 4 验收门

**Files:**
- Create: `web/src/features/gallery/Gallery.tsx`, `web/src/components/Previewer.tsx`, `web/src/components/previews/DocxPreview.tsx`, `web/src/components/previews/PdfPreview.tsx`, `web/src/components/previews/TextPreview.tsx`, `web/tests/gallery-preview.spec.ts`
- Modify: `web/src/app/router.tsx`, `web/src/api/client.ts`, `web/src/styles.css`

**Interfaces:**
- `Previewer` 根据 `Preview.kind` 分派 `image|video|audio|pdf|docx|text|download`；前端不对 DOCX 使用 `innerHTML` 注入外部脚本。

- [ ] **Step 1: 写预览分派失败测试**

```tsx
it.each([['image','img'],['video','video'],['audio','audio'],['text','pre']])('renders %s', (kind, tag) => {
  const {container} = render(<Previewer preview={{kind, url:'/p/1'}} />)
  expect(container.querySelector(tag)).toBeTruthy()
})
```

- [ ] **Step 2: 验证失败**

```bash
npm --prefix web test -- --run src/components/Previewer.test.tsx
```

- [ ] **Step 3: 实现响应式相册、继承开关和五类预览**

```tsx
export function Previewer({preview}: Props) {
  if (preview.kind === 'image') return <img src={preview.url} alt="" />
  if (preview.kind === 'video') return <video controls src={preview.url} />
  if (preview.kind === 'audio') return <audio controls src={preview.url} />
  if (preview.kind === 'text') return <pre>{preview.text}</pre>
  return <a href={preview.url}>下载文件</a>
}
```

相册提供文件夹、类型、名称、时间筛选；文件和文件夹管理页提供 `inherit/visible/hidden` 三态选择。

- [ ] **Step 4: 验证通过并执行 Phase 4 门**

```bash
npm --prefix web test -- --run
npx playwright test web/tests/gallery-preview.spec.ts --project=chromium
npx playwright test web/tests/gallery-preview.spec.ts --project=mobile
```

预期：图片、视频、音频、PDF、DOCX、TXT 可预览；相册密码独立生效；移动端和桌面端均能筛选和打开稳定链接。

- [ ] **Step 5: 提交**

```bash
git add web
git commit -m "相册前端：交付响应式媒体与文档预览"
```

---

## Phase 5：回收站与跨存储迁移

### Task 21: 固定回收站批次、软删除和恢复

**Files:**
- Create: `internal/trash/service.go`, `internal/trash/service_test.go`, `internal/api/trash_handlers.go`
- Modify: `internal/files/service.go`, `internal/api/router.go`

**Interfaces:**
- `TrashService.DeleteFile/DeleteFolder`、`List`、`RestoreBatch/RestoreItem`；`DELETE /api/v1/files/{id}`、`DELETE /api/v1/folders/{id}`、`GET /api/v1/trash`、`POST /api/v1/trash/{id}/restore`。

- [ ] **Step 1: 写软删除不触碰物理对象失败测试**

```go
func TestDeleteMovesToTrashWithoutBackendDelete(t *testing.T) {
    f := seedActiveFile(t); backend := fakeBackend{}
    if err := trash.DeleteFile(ctx, f.PublicID, actor); err != nil { t.Fatal(err) }
    if backend.DeleteCalls != 0 || fileState(t, f.PublicID) != "trashed" { t.Fatalf("delete was physical") }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/trash -run TestDeleteMovesToTrashWithoutBackendDelete -count=1
```

- [ ] **Step 3: 实现批次快照、410 直链和冲突恢复**

```go
func (s *Service) DeleteFile(ctx context.Context, id string, actor auth.Actor) error {
    return s.db.WithTx(ctx, func(tx *sql.Tx) error {
        if err := s.authz.Check(tx, id, actor); err != nil { return err }
        batch := newBatch("file", id); if err := s.repo.InsertBatch(tx, batch); err != nil { return err }
        return s.repo.MarkTrashed(tx, id, batch.ID)
    })
}
```

目录删除把目录及后代文件纳入同一批次；恢复优先原目录，名称冲突返回 `409 conflict`，由客户端选择新目录或重命名；恢复成功只改元数据，不改变公开 ID 或物理对象。

- [ ] **Step 4: 验证通过**

```bash
go test ./internal/trash ./internal/files ./internal/api -count=1
```

- [ ] **Step 5: 提交**

```bash
git add internal/trash internal/files internal/api
git commit -m "回收站：实现固定软删除与批次恢复"
```

### Task 22: 手动彻底删除与物理对象幂等清理

**Files:**
- Create: `internal/trash/purge.go`, `internal/trash/purge_test.go`
- Modify: `internal/jobs/worker.go`, `internal/db/repository/files.go`

**Interfaces:**
- `PurgeService.PurgeBatch/PurgeFile`；物理删除成功后状态为 `purged`，清空存储键、Telegram 标识、文件密码和游客归属，保留公开 ID 与最小审计字段。

- [ ] **Step 1: 写 Local/TG 删除和失败重试失败测试**

```go
func TestPurgeIsIdempotentAndRetainsFailedItems(t *testing.T) {
    b := fakeBackend{DeleteErr: errors.New("offline")}; id := seedTrashedWithBackend(t, b)
    if err := purge.PurgeFile(ctx, id, admin); err == nil { t.Fatal("expected failure") }
    if fileState(t, id) != "trashed" || lastError(t, id) == "" { t.Fatal("failure was lost") }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/trash -run TestPurgeIsIdempotentAndRetainsFailedItems -count=1
```

- [ ] **Step 3: 实现物理删除顺序和 `cleanup_pending`**

```go
func (s *PurgeService) PurgeFile(ctx context.Context, id string, actor auth.Actor) error {
    f, err := s.repo.Get(ctx, id); if err != nil { return err }
    if err = s.authz.CanManage(actor, f); err != nil { return err }
    if err = s.backends[f.StorageProfileID].Delete(ctx, f.StorageKey); err != nil { return s.repo.RecordPurgeError(ctx, id, err) }
    return s.repo.MarkPurgedAndClearSecrets(ctx, id)
}
```

源对象不存在按幂等成功；目标切换后源清理失败的迁移对象进入 `cleanup_pending`，不回滚目标绑定。

- [ ] **Step 4: 验证通过**

```bash
go test ./internal/trash ./internal/jobs -count=1
```

- [ ] **Step 5: 提交**

```bash
git add internal/trash internal/jobs internal/db/repository
git commit -m "回收站：加入手动彻底删除与幂等清理"
```

### Task 23: 跨存储迁移状态机

**Files:**
- Create: `internal/migration/service.go`, `internal/migration/service_test.go`, `internal/api/migration_handlers.go`
- Modify: `internal/storage/orchestrator.go`, `internal/api/router.go`

**Interfaces:**
- `MigrationService.Start(ctx, MigrationInput) (Job, error)`、`Run(ctx, jobID) error`；`POST /api/v1/migrations`、`GET /api/v1/jobs/{id}`；创建迁移使用 `Idempotency-Key`，迁移暂存目录 `/data/migrations`。

- [ ] **Step 1: 写目标校验失败保留源、切换后清理失败失败测试**

```go
func TestMigrationKeepsSourceWhenTargetHashMismatch(t *testing.T) {
    src, dst := fakeBackend{}, fakeBackend{Corrupt:true}; id := seedFileOn(t, src)
    err := migration.Run(ctx, startJob(id, dst)); if err == nil { t.Fatal("expected mismatch") }
    if currentBackend(t, id) != src.Name { t.Fatal("source was lost") }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/migration -run TestMigrationKeepsSourceWhenTargetHashMismatch -count=1
```

- [ ] **Step 3: 实现流式复制、双 SHA-256 校验、事务切换和 cleanup_pending**

```go
func (s *Service) Run(ctx context.Context, id string) error {
    job, f, err := s.load(ctx, id); if err != nil { return err }
    tmp := filepath.Join(s.dir, id+".part"); if err = copyAndHash(ctx, s.src(f), tmp, f.SHA256); err != nil { return s.fail(job, err) }
    obj, err := s.dst.Put(ctx, mustOpen(tmp), storage.ObjectMeta{Key:f.StorageKey, FileName:f.FileName, MIMEType:f.MIMEType}); if err != nil { return s.fail(job, err) }
    if obj.SHA256 != f.SHA256 { return s.fail(job, ErrHashMismatch) }
    if err = s.repo.SwitchStorage(ctx, f.PublicID, job.TargetProfileID, obj.Key); err != nil { return s.fail(job, err) }
    if err = s.srcBackend.Delete(ctx, f.StorageKey); err != nil { return s.repo.MarkCleanupPending(ctx, job.ID, err) }
    return s.succeed(job.ID)
}
```

- [ ] **Step 4: 验证通过**

```bash
go test ./internal/migration ./internal/jobs ./internal/storage -count=1
```

- [ ] **Step 5: 提交**

```bash
git add internal/migration internal/jobs internal/storage internal/api
git commit -m "迁移：实现跨存储校验切换与清理待重试"
```

### Task 24: 回收站/迁移前端与 Phase 5 验收门

**Files:**
- Create: `web/src/features/trash/TrashPage.tsx`, `web/src/features/migration/MigrationDialog.tsx`, `web/tests/trash-migration.spec.ts`
- Modify: `web/src/features/files/FileList.tsx`, `web/src/api/client.ts`, `web/src/app/router.tsx`

**Interfaces:**
- `TrashPage` 调用 `GET /api/v1/trash`、`POST .../restore`、`DELETE ...`；`MigrationDialog` 提交 `MigrationInput` 并轮询 `/api/v1/jobs/{id}`。

- [ ] **Step 1: 写回收站 UI 失败测试**

```tsx
it('requires explicit permanent delete', () => {
  render(<TrashPage items={[{id:'b1',name:'a.txt'}]} />)
  expect(screen.getByRole('button',{name:'彻底删除'})).toBeInTheDocument()
  expect(screen.getByText('彻底删除后无法恢复')).toBeInTheDocument()
})
```

- [ ] **Step 2: 验证失败**

```bash
npm --prefix web test -- --run src/features/trash/TrashPage.test.tsx
```

- [ ] **Step 3: 实现恢复、永久删除确认和迁移进度**

```ts
export async function pollJob(id: string, signal: AbortSignal) {
  while (!signal.aborted) { const job = await apiClient<Job>(`/api/v1/jobs/${id}`); if (['succeeded','failed','cleanup_pending'].includes(job.state)) return job; await new Promise(r => setTimeout(r, 1000)) }
  throw new DOMException('aborted', 'AbortError')
}
```

永久删除按钮必须二次确认；恢复冲突弹窗要求选择新目录或新名称；迁移失败保留源文件并在界面显示可重试状态。

- [ ] **Step 4: 验证通过并执行 Phase 5 门**

```bash
npm --prefix web test -- --run
npx playwright test web/tests/trash-migration.spec.ts
```

预期：删除阶段直链返回 410、Telegram 消息未删除；彻底删除才调用物理删除；迁移校验失败不损坏源文件。

- [ ] **Step 5: 提交**

```bash
git add web
git commit -m "管理前端：交付回收站与跨存储迁移"
```

---

## Phase 6：REST API、运维与最终验收

### Task 25: API Token 与 curl/PicGo/ShareX 兼容上传

**Files:**
- Create: `internal/auth/api_token.go`, `internal/auth/api_token_test.go`, `internal/api/token_handlers.go`, `internal/api/compat_handlers.go`, `web/src/features/settings/ApiTokens.tsx`, `web/src/features/settings/ApiTokens.test.tsx`
- Modify: `internal/api/router.go`

**Interfaces:**
- `TokenService.Create(scopes []string, expiresAt *time.Time) (plaintext string, Token, error)`、`Revoke`、`Authenticate`；`Authorization: Bearer <token>`；兼容 `POST /api/v1/upload` 返回 `{url, public_id, delete_url}`。

- [ ] **Step 1: 写 Token 只存哈希和 scope 限制失败测试**

```go
func TestTokenCannotDeleteWithoutDeleteScope(t *testing.T) {
    plain, _, _ := tokens.Create([]string{"upload"}, nil)
    actor, _ := tokens.Authenticate(ctx, plain)
    if err := authz.RequireScope(actor, "delete"); !errors.Is(err, ErrForbidden) { t.Fatalf("got %v", err) }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/auth -run TestTokenCannotDeleteWithoutDeleteScope -count=1
```

- [ ] **Step 3: 实现 Token 哈希、过期和兼容响应**

```go
func (s *TokenService) Authenticate(ctx context.Context, plain string) (Actor, error) {
    sum := sha256.Sum256([]byte(plain)); tok, err := s.repo.FindActive(ctx, hex.EncodeToString(sum[:]))
    if err != nil { return Actor{}, ErrUnauthorized }; return Actor{Kind:ActorToken, Scopes:parseScopes(tok.Scopes)}, nil
}
```

Token 明文只在创建响应出现一次，管理页立即提供复制/下载并提示无法再次查看；PicGo/ShareX 使用 multipart `file` 字段，统一返回稳定直链和可选删除 URL。

- [ ] **Step 4: 验证通过**

```bash
go test ./internal/auth ./internal/api -run 'Token|Compat|Bearer' -count=1
npm --prefix web test -- --run src/features/settings/ApiTokens.test.tsx
```

- [ ] **Step 5: 提交**

```bash
git add internal/auth internal/api web/src/features/settings
git commit -m "API：加入 Token 权限与客户端兼容上传"
```

### Task 26: 健康检查、诊断、备份和安全加固

**Files:**
- Create: `internal/health/health.go`, `internal/health/health_test.go`, `internal/audit/service.go`, `internal/audit/service_test.go`, `internal/api/health_handlers.go`, `internal/api/middleware.go`, `cmd/clist/backup.go`, `cmd/clist/restore.go`, `web/src/features/settings/Diagnostics.tsx`
- Modify: `cmd/clist/main.go`, `Dockerfile`, `README.md`

**Interfaces:**
- `GET /health/live`、`GET /health/ready`；管理员诊断 `GET /api/v1/admin/diagnostics`；`audit.Service.Record(ctx, action, targetID, actor, result)`；CLI `clist backup --data-dir /data --output /backup.db`、`clist restore --data-dir /data --input /backup.db`。

- [ ] **Step 1: 写就绪检查和路径穿越失败测试**

```go
func TestReadyFailsWhenMasterKeyMissing(t *testing.T) {
    h := New(HealthDeps{DB: healthyDB(), MasterKeyPath: filepath.Join(t.TempDir(), "missing")})
    if got := h.Ready(ctx); got.OK { t.Fatal("ready despite missing key") }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/health ./internal/api -run 'Ready|Path|Security' -count=1
```

- [ ] **Step 3: 实现诊断、在线备份、脱敏日志和安全中间件**

```go
func (h *Handler) Live(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, map[string]any{"ok":true}) }
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) { result := h.health.Ready(r.Context()); writeJSON(w, result.Status(), result) }
```

中间件生成 `request_id`、限制请求体、设置 CSP/`nosniff`、校验 CSRF；日志过滤 `token|secret|password|cookie` 键和值。审计服务只记录关键管理、删除、恢复、迁移和 Token 动作，不保存配置正文或敏感凭据。诊断页展示存储连通性、目录权限、磁盘空间和未完成任务。备份使用 SQLite Online Backup API，同时复制 `/data/secrets/master.key` 到用户指定的备份目录并拒绝覆盖非空目标。

- [ ] **Step 4: 验证通过**

```bash
go test ./internal/health ./internal/api ./cmd/clist -count=1
go run ./cmd/clist backup --data-dir "$(pwd)/.tmp-data" --output "$(pwd)/.tmp-backup.db"
go run ./cmd/clist restore --data-dir "$(pwd)/.tmp-restore" --input "$(pwd)/.tmp-backup.db"
```

- [ ] **Step 5: 提交**

```bash
git add internal/health internal/audit internal/api cmd/clist web/src/features/settings/Diagnostics.tsx Dockerfile README.md
git commit -m "运维：加入健康检查、诊断与 SQLite 备份"
```

### Task 27: 全链路测试、Docker 验收和发布文档

**Required sub-skill:** 修改 `README.md` 和仓库基础设施前加载 `github-repo-infrastructure`。

**Files:**
- Create: `internal/contract/storage_contract_test.go`, `internal/e2e/test_server.go`, `web/tests/full-flow.spec.ts`, `.github/workflows/ci.yml`
- Modify: `README.md`, `docker-compose.yml`

**Interfaces:**
- 存储契约测试对 Local、Official Mock、Streaming Mock 统一执行 `Validate/Capabilities/Put/Open/Delete`；Playwright 覆盖游客上传、恢复、相册预览、管理员回收站和迁移。

- [ ] **Step 1: 写跨适配器契约失败测试**

```go
func TestBackendContract(t *testing.T) {
    for _, tc := range []struct{name string; new func(t *testing.T) storage.Backend}{
        {"local", newLocalBackend}, {"official-mock", newOfficialMock}, {"streaming-mock", newStreamingMock},
    } { t.Run(tc.name, func(t *testing.T) { runBackendContract(t, tc.new(t)) }) }
}
```

- [ ] **Step 2: 验证失败**

```bash
go test ./internal/contract -run TestBackendContract -count=1
```

- [ ] **Step 3: 补齐集成测试、CI 和 README 部署说明**

```yaml
name: ci
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: {go-version: '1.26.5'}
      - run: go test ./...
      - uses: actions/setup-node@v4
        with: {node-version: '24'}
      - run: npm --prefix web ci && npm --prefix web test -- --run && npm --prefix web run build
      - run: docker build -t clist:ci .
```

README 明确零环境变量 Docker 命令、首次管理员初始化、数据卷目录、三种后端能力差异、恢复密钥责任、回收站不可逆操作、备份主密钥和 Telegram 受控联调方式。

- [ ] **Step 4: 验证通过并执行最终 Docker 门**

```bash
go test ./... -count=1
npm --prefix web test -- --run
npx playwright test web/tests/full-flow.spec.ts --project=chromium --project=mobile
docker compose down -v
docker compose up -d --build
curl -fsS http://localhost:8080/health/live
curl -fsS http://localhost:8080/health/ready
```

验收必须证明：无环境变量一条命令启动；容器重启后索引/任务恢复；1 GiB 分片上传内存不随文件线性增长；Local/官方支持续传、自建流式后端明确提示不支持；改名/移动/恢复/迁移不改公开链接；回收站阶段不删 Telegram 消息，彻底删除才删；迁移校验失败源文件仍可用。

- [ ] **Step 5: 提交最终文档与 CI**

```bash
git add .github/workflows/ci.yml README.md docker-compose.yml internal/contract web/tests
git commit -m "验收：补齐全链路测试与 Docker 发布文档"
```

---

## 计划自审记录

- 规格覆盖：六期任务覆盖认证、游客恢复、三种存储、Webhook、分片上传、能力差异、目录/相册、五类预览、固定回收站、迁移、Token、健康检查、备份、安全和 Docker 验收；未把负载均衡、Cloudflare 或自建 API 的 Range 扩展混入首版。
- 占位符扫描：未发现不可执行的占位性描述；每个代码步骤都给出接口、代码片段或可直接执行命令。
- 类型一致性：`storage.Backend`、`Capabilities`、`FileState`、`Visibility`、`Job` 状态和 REST 路径在前后任务中保持同名；后续执行者不得擅自改名，若需变更先更新本计划与相关测试。
- 每期验收门均要求先通过本期测试，再进入下一期；真实 Telegram 凭据只允许本地/受控环境联调，不进入 CI。
