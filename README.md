# NoteFoundry

一个面向个人学习的知识管理项目。第一阶段先把学习笔记的记录、组织、搜索、发布和备份做可靠，再逐步加入学习资料采集、RAG、Agent、记忆与上下文管理能力。

## 当前状态

项目已进入 v0.1 增量开发阶段。当前后端骨架位于 `backend/`，首个切片建立了可运行的 Go API 健康检查和 Knowledge Space 核心领域规则。

## 本地开发

需要 Go 1.24 或更高版本。

```bash
cd backend
go test ./...
go run ./cmd/api
```

API 默认监听 `http://localhost:8080`，健康检查地址为 `GET /healthz`。

## 版本路线

- **v0.1：知识管理核心** — 知识空间、目录、Markdown 笔记、草稿与发布、修订、标签、链接与反向链接、全文搜索、公开阅读、回收站、附件和自动备份。
- **v0.2：学习资料** — 网页/PDF/手工资料采集、资料收件箱、异步提取、去重和引用。
- **v0.3：AI 学习能力** — RAG、Agent 修改建议、记忆与上下文实验、学习专题和自动信息采集。

## 技术方向

- Frontend: Next.js + TypeScript
- Backend: Go modular monolith
- Database: PostgreSQL
- API: REST + OpenAPI
- Deployment: Docker Compose + Caddy

## 开始开发前

请按顺序阅读：

1. [AGENTS.md](./AGENTS.md)
2. [CONTEXT.md](./CONTEXT.md)
3. [docs/product-scope.md](./docs/product-scope.md)
4. [docs/architecture.md](./docs/architecture.md)
5. [docs/adr](./docs/adr)

产品与架构设计的简要汇总见 [docs/design-summary.md](./docs/design-summary.md)。

## v0.1 验收主线

部署系统并真实用于学习 Hermes Agent：建立 `AI Agent/Hermes Agent` 目录结构，完成并发布至少五篇学习笔记，验证中英文搜索、稳定链接与反向链接、公开/私有隔离、删除恢复及完整备份恢复。
