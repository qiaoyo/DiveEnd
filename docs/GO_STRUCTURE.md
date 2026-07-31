# Go 结构治理

更新时间：2026-07-31

## 目标

DiveEnd 的 Go 后端原来把 Wails 入口、业务流程、外部客户端、数据库和测试全部放在根目录的同一个 `package main` 中。这样短期开发很快，但会让一个功能 PR 同时触碰很多无关文件，也让 agent 很难判断某段代码属于哪个产品能力。

结构治理采用渐进迁移：每次只移动一个有清晰边界的模块，先保留行为和 Wails 方法签名，再扩大测试覆盖，最后删除根目录兼容层。当前不进行一次性的大规模重写。

## 当前边界

```text
Wails root package (main)
  app.go / app_lifecycle.go / main.go     入口、生命周期、组合和 Wails 方法
  wails_models.go                          Wails 兼容别名，真实契约在 internal/domain
  clients.go                              当前客户端实现，逐步迁移到 feature packages
  database.go / sync.go / ...             当前遗留聚合实现，按领域逐步拆分

internal/
  domain/                                 跨层数据契约，不依赖业务实现
  contracts/                              LLM、检索、阅读助手等服务端口
  platform/                               provider-neutral 安全和系统能力
```

`internal/domain`、`internal/contracts` 和 `internal/platform` 是本次治理的第一批实际代码迁移。根包中的类型别名和 platform wrappers 是有意保留的 Wails/legacy 适配层：Wails 绑定继续从 `main` 组合，非 Wails 代码可以直接依赖稳定的 internal 契约。

## 目标目录

```text
internal/
  domain/          跨层 DTO、配置、检索、阅读、筛选、同步数据模型
  contracts/       服务端口和跨模块接口
  search/          Semantic Scholar、OpenAlex、arXiv、OpenReview、DBLP 与统一排序
  deepread/        PDF 解析缓存、阅读状态、证据和阅读 AI 编排
  screening/       PDF 批量筛选、抽取、决策树和导入编排
  library/         文件夹、论文资产、PDF 下载与本地文件操作
  sync/            百度云适配、manifest、冲突与恢复
  storage/         SQLite schema、迁移和各领域 repository
  platform/        文件安全、HTTP 限制、脱敏和 PDF service 进程管理
  llm/             强弱模型客户端、预算和 provider-neutral 请求协议
```

目标目录不是要求一次建完的空壳。只有当一个目录有独立的输入输出和测试边界时才迁移代码；禁止为了减少根目录文件数量而复制实现或制造双向依赖。

## 依赖规则

1. `internal/domain` 只能依赖标准库；它不依赖 Wails、SQLite、HTTP client 或任何业务包。
2. `internal/contracts` 可以依赖 `internal/domain`，不能依赖根 `main` 或具体 provider。
3. 目标 feature package 可以依赖 `domain`、`contracts` 和更底层的 platform/storage port；不能依赖 `main`。
4. 根 `main` 负责 Wails 导出方法、生命周期、依赖注入和兼容适配；不能把新的 provider 逻辑继续添加到 `app.go`。
5. provider-specific code 必须留在对应 feature package 内，通过接口注入 HTTP、数据库和 LLM 依赖，便于单元测试和独立 PR。
6. 跨领域共享状态优先通过明确的 DTO 或接口传递，不直接读取另一个领域的私有字段。

## 功能 PR 归属

| 改动类型 | 首选目录 | 必须同时更新 |
| --- | --- | --- |
| 新检索源、重试、合并、排序 | `internal/search` | source fixture、检索状态和 `PROJECT_MAP.md` |
| DeepRead 解析、证据、问答 | `internal/deepread` | reader tests、前端类型和用户可见错误 |
| Screening 抽取、决策树、导入 | `internal/screening` | cancellation、事务和 workflow tests |
| 文件夹、论文 PDF、下载 | `internal/library` | path/security tests |
| 百度同步、冲突、恢复 | `internal/sync` | mock PCS、sync progress tests |
| SQLite schema/repository | `internal/storage` | migration and restore tests |
| LLM provider、token budget | `internal/llm` | provider mock、budget tests |
| Wails 路由、事件和生命周期 | 根 `main` | frontend binding、browser/package smoke |

## 迁移顺序

1. 已完成：共享 domain contracts 和 workflow request/response types。
2. 已完成：LLM、检索、阅读助手的 contracts ports。
3. 下一步：把纯检索模型、source adapters 和 ranking helpers 迁入 `internal/search`，根包暂时保留 `SearchClient` façade。
4. 再下一步：拆出 `internal/storage`，先迁移 repository，再迁移 SQLite migration；根包保留 composition adapter。
5. 最后迁移 DeepRead、Screening、Library、Sync 和 LLM provider，逐个删除兼容别名和根目录实现。

## Agent 工作规则

接手 Go 功能时，先读本文件和 [PROJECT_MAP.md](PROJECT_MAP.md)，再按目标目录查找代码。新功能不要默认写入根目录；如果当前模块尚未迁移，先在对应目标目录建立最小端口或提出单独结构 PR。一个 PR 尽量只包含一个领域的实现、测试和文档更新。
