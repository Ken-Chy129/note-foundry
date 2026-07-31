# 历史文档导入

历史文档迁移采用“先生成可审计整理包，再写入运行时系统”的两阶段流程。第一阶段不连接 PostgreSQL，也不调用 NoteFoundry 管理 API；第二阶段必须在 Owner 身份下执行，并在写入前完成备份。

## 生成整理包

在仓库根目录执行：

```bash
cd backend
go run ./cmd/historyimport \
  -source /path/to/history-exports \
  -output /path/to/empty-staging-directory
```

工具支持独立 Markdown、带附件的 ZIP、SiYuan 嵌套 ZIP 和语雀 `.lakebook`。它会：

- 将文档整理到一个 private 的 `历史文档` Knowledge Space 下；
- 保留嵌套 Directory，按主题补充顶层 Directory；
- 只规范 BOM、换行、布局空行、空标题和代码围栏语言，不改写正文语义；
- 保守合并重复文档，标题不同的文档只有正文完全相同时才会合并；
- 将可取得的图片写入 `attachments/`，并在暂存 Markdown 中使用 `asset:<key>`；
- 将缺失附件、空文档和网络图片下载失败记录到清单，不删除原引用。

默认只允许从 `cdn.nlark.com` 下载 HTTPS 图片。增加域名属于新的外部网络信任，应在确认后显式配置：

```bash
go run ./cmd/historyimport \
  -source /path/to/history-exports \
  -output /path/to/empty-staging-directory \
  -allowed-asset-hosts cdn.nlark.com,images.example.com
```

若不希望访问网络，可使用 `-skip-remote-assets`。网络图片将保留原 URL，并作为未解决问题进入报告。

## 整理包结构

```text
staging/
├── manifest.json
├── report.md
├── notes/
└── attachments/
```

`report.md` 用于人工检查分类和问题摘要。`manifest.json` 是后续导入的机器可读契约，包含：

- 目标 Knowledge Space 及其 visibility；
- 每篇 Learning Note 的标题、Directory、Markdown 路径和来源；
- Attachment 的暂存 key、文件路径、媒体类型和 SHA-256；
- 重复合并记录和逐项问题明细。

整理包不是运行时数据。`asset:<key>` 只是迁移期引用，不能直接成为 canonical Markdown 中的正式附件地址。

## 写入 NoteFoundry

`historyapply` 是仅在受控服务器环境中运行的批量导入命令。默认只校验整理包，不连接数据库：

```bash
go run ./cmd/historyapply -staging /path/to/staging
```

实际写入必须显式提供 `-apply`，并通过环境变量提供运行时数据库和附件目录：

```bash
DATABASE_URL=... \
ATTACHMENTS_DIR=/var/lib/notefoundry/attachments \
go run ./cmd/historyapply -staging /path/to/staging -apply
```

命令会在整理包内原子维护 `apply-state.json`。中断后使用相同命令可续传；它会校验整个整理包摘要，并从已经完成的 Knowledge Space、Directory、Learning Note 和 Attachment 写入中恢复，不重复创建。应在导入完成后将 `apply-state.json` 与 `manifest.json` 一起保存，作为后续补附件时的来源到 Note ID 映射。

## 补回已导入文档的缺失图片

已导入的 Learning Note 如果后来找回了原图，应单独制作高置信恢复包，而不是重新执行整批历史导入。恢复包只接受 `confidence: "high"` 的文档，每个映射必须包含现有 Note ID、原 Markdown 中唯一的 `assets/...` 引用、恢复文件相对路径和 SHA-256：

```json
{
  "version": 1,
  "documents": [
    {
      "key": "stable-source-key",
      "noteId": "existing-note-uuid",
      "title": "现有标题",
      "confidence": "high",
      "mappings": [
        {
          "missingReference": "assets/original.png",
          "file": "files/original.png",
          "sha256": "..."
        }
      ]
    }
  ]
}
```

默认命令只校验清单、路径、文件大小、SHA-256 和实际图片媒体类型：

```bash
go run ./cmd/historyrecover -bundle /path/to/recovery-bundle
```

实际恢复必须显式使用 `-apply`：

```bash
DATABASE_URL=... \
ATTACHMENTS_DIR=/var/lib/notefoundry/attachments \
go run ./cmd/historyrecover -bundle /path/to/recovery-bundle -apply
```

命令会先只读校验整个批次中的 Note 标题和引用，任何笔记不匹配时都在上传前停止。写入时复用 Attachment 和 Note 服务：上传文件后把精确的 `assets/...` 替换为 `attachment:<uuid>`，每篇 Note 只 autosave 一次，不触发 publish，也不修改 Published Content。`recovery-state.json` 记录每个引用对应的 Attachment ID；重试时还会通过同 Note、原文件名和 SHA-256 找回“附件已上传但状态尚未落盘”的进度。

低置信或仅凭图片顺序猜测的候选不应进入恢复包，应保留给人工逐图确认。

安全的应用顺序如下：

1. 对当前实例执行一次真实加密备份，并验证备份产物可读。
2. 完整校验 `manifest.json`、全部 Markdown 路径、Attachment 大小和 SHA-256；任何校验失败都应在写入前停止。
3. 通过 `POST /api/v1/spaces` 创建 private 的 `历史文档` Knowledge Space。
4. 按父级优先顺序调用 `POST /api/v1/spaces/{spaceId}/directories` 创建 Directory，并记录“清单路径到 Directory ID”的映射。
5. 调用 `POST /api/v1/notes` 创建 Learning Note 草稿，并记录“文档 key 到 Note ID”的映射。不得自动 publish。
6. 对每个附件调用 `POST /api/v1/notes/{noteId}/attachments`，校验响应中的大小和 SHA-256，并记录“暂存 asset key 到 Attachment ID”的映射。
7. 将 Markdown 中的 `asset:<key>` 替换为 `attachment:<uuid>`，再通过 `PATCH /api/v1/notes/{noteId}` 保存最终 canonical Markdown。
8. 核对 Knowledge Space、Directory、Learning Note 和 Attachment 数量，抽查 Markdown 渲染，并验证中英文搜索。
9. 导入完成后再执行一次加密备份。

## 批量导入命令的安全约束

命令满足以下约束后才会对实例写入：

- 直接在受控服务器环境中使用现有运行时配置，不新增网络管理接口，也不需要 Owner session；
- 默认只做 preflight，写入必须使用显式 `-apply`；
- 目标 Knowledge Space 已存在时默认拒绝，除非提供经过校验的续传状态；
- 每完成一个对象就原子写入本地续传映射，重试不得重复创建笔记或附件；
- 任一 Attachment 的响应校验失败时停止，不保存包含无效 `attachment:<uuid>` 的 Markdown；
- 中断后优先支持续传。当前 API 没有删除整个 Knowledge Space 的事务式回滚能力，因此不能假装全量写入是一个原子操作；
- 导入器只创建 private 草稿，不发布内容，也不改变已有 Knowledge Space 的 visibility。

## 公开历史草稿的发布校正

如果历史迁移完成后，Owner 已经明确把目标 Knowledge Space 改为 public，但其中的既有草稿也应作为首版公开内容发布，必须让每篇 Learning Note 走正常发布流程。不要直接更新 `published_at` 或 `published_markdown`，否则会漏掉 Note Revision、公开搜索、稳定链接和附件可见性投影。

部署镜像提供了一个默认只读的预检命令。它会列出所有位于 public Knowledge Space 且从未发布的 Learning Note：

```bash
docker compose run --rm api /usr/local/bin/notefoundry-publish-public
```

确认列表、Knowledge Space 可见性和最近备份无误后，再显式执行：

```bash
docker compose run --rm api /usr/local/bin/notefoundry-publish-public -confirm
```

命令逐篇调用正式发布领域流程，并且只选择 `published_at IS NULL` 的笔记，因此成功完成后重复执行不会再次发布已有 Published Content。这个命令是受控迁移校正工具，不会改变后续新建笔记仍需显式发布的产品规则。

在问题报告未审核、缺失附件策略未确认之前，不应把整理包自动写入运行时数据库。
