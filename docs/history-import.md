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

## 写入 NoteFoundry 的顺序

当前项目已有完成导入所需的 REST 接口，但尚未提供批量导入命令。安全的应用顺序如下：

1. 对当前实例执行一次真实加密备份，并验证备份产物可读。
2. 完整校验 `manifest.json`、全部 Markdown 路径、Attachment 大小和 SHA-256；任何校验失败都应在写入前停止。
3. 通过 `POST /api/v1/spaces` 创建 private 的 `历史文档` Knowledge Space。
4. 按父级优先顺序调用 `POST /api/v1/spaces/{spaceId}/directories` 创建 Directory，并记录“清单路径到 Directory ID”的映射。
5. 调用 `POST /api/v1/notes` 创建 Learning Note 草稿，并记录“文档 key 到 Note ID”的映射。不得自动 publish。
6. 对每个附件调用 `POST /api/v1/notes/{noteId}/attachments`，校验响应中的大小和 SHA-256，并记录“暂存 asset key 到 Attachment ID”的映射。
7. 将 Markdown 中的 `asset:<key>` 替换为 `attachment:<uuid>`，再通过 `PATCH /api/v1/notes/{noteId}` 保存最终 canonical Markdown。
8. 核对 Knowledge Space、Directory、Learning Note 和 Attachment 数量，抽查 Markdown 渲染，并验证中英文搜索。
9. 导入完成后再执行一次加密备份。

## 批量导入命令的安全要求

后续实现自动应用命令时，应满足以下条件后才能对实例写入：

- Owner session 从环境变量或受限文件读取，不出现在命令行参数、日志或清单中；
- 默认只做 preflight，写入必须使用显式 `--apply`；
- 目标 Knowledge Space 已存在时默认拒绝，除非提供经过校验的续传状态；
- 每完成一个对象就原子写入本地续传映射，重试不得重复创建笔记或附件；
- 任一 Attachment 的响应校验失败时停止，不保存包含无效 `attachment:<uuid>` 的 Markdown；
- 中断后优先支持续传。当前 API 没有删除整个 Knowledge Space 的事务式回滚能力，因此不能假装全量写入是一个原子操作；
- 导入器只创建 private 草稿，不发布内容，也不改变已有 Knowledge Space 的 visibility。

在问题报告未审核、缺失附件策略未确认之前，不应把整理包自动写入运行时数据库。
