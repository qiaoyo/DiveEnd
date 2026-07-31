# Go 结构治理

更新时间：2026-07-31

## 目标

DiveEnd 的 Go 后端曾把 Wails 入口、业务流程、外部客户端、数据库和测试全部放在仓库根目录的同一个 `package main` 中。这个结构让早期迭代很快，但会让 agent 无法从目录判断职责，也会让一个功能 PR 同时触碰大量无关文件。

当前治理采用两步策略：先隔离桌面入口，再按真实依赖逐个形成领域包。第一步已经完成；第二步不通过复制代码或临时别名制造“假分层”，而是以可测试的输入输出边界为准。

## 当前真实结构

```text
main.go                         仅负责 Wails 进程入口、资源嵌入和依赖组合
internal/
  app/                          当前应用包；Wails facade + 尚未拆开的后端工作流
    app.go                      App 状态、Wails 方法和跨领域编排
    clients.go                  LLM、检索客户端和 provider-neutral 辅助逻辑
    database.go                 SQLite 连接、迁移和核心 repository
    deepstart*.go               DeepStart 工作流及任务状态
    deepread*.go                DeepRead 工作流、PDF 状态和证据
    screening*.go               Screening 工作流、抽取和决策树
    sync*.go / baidu_pcs.go     百度同步、冲突和 PCS 客户端
    folders*.go / paper_*.go    文库、论文资产和下载
    *_test.go                    与当前未拆边界匹配的回归测试
  domain/                       跨层 DTO、配置和 Wails 数据契约
  contracts/                    LLM、检索、查询改写、DeepRead 服务端口
  platform/                     HTTP 限制、JSON 解码、脱敏等无业务平台能力
```

根目录现在只有 `main.go`。`internal/app` 仍然有较多文件，这是有意保留的过渡状态：现有工作流大量共享 `App`、`DB` 的私有字段和生命周期状态，先移出根目录可以立即建立清晰的 agent 入口，同时不改变线上行为。

## 领域目标结构

```text
internal/
  app/             Wails facade、依赖组合、跨领域 workflow orchestration
  domain/          跨层 DTO 和稳定数据契约
  contracts/       服务端口，不依赖具体 provider
  search/          OpenAlex、arXiv、OpenReview、DBLP、Semantic Scholar、排序合并
  llm/             强弱模型客户端、预算、请求协议和证据约束
  storage/         SQLite schema、migration、repository 和 restore
  deepread/        PDF 解析缓存、阅读状态、证据和阅读 AI 编排
  screening/       PDF 批量筛选、抽取、决策树和导入编排
  library/         文件夹、论文资产、PDF 下载与本地文件操作
  sync/            百度云适配、manifest、冲突与恢复
  platform/        文件安全、HTTP 限制、脱敏和 PDF service 进程管理
```

这些目录只有在包拥有独立的依赖方向和测试边界后才建立。目录名本身不是完成标志；当前未落地的目标包不会放复制实现或空壳代码。

## 依赖规则

1. `internal/domain` 只依赖标准库，不依赖 Wails、SQLite、HTTP client 或业务包。
2. `internal/contracts` 可以依赖 `internal/domain`，不能依赖 `internal/app` 或具体 provider。
3. 领域包可以依赖 `domain`、`contracts` 和更底层的 platform/storage 接口，但不能依赖 `internal/app` 的私有实现。
4. `internal/app` 负责 Wails 导出方法、生命周期、依赖注入和跨领域编排；新的 provider 逻辑不继续堆进 `app.go`。
5. provider-specific code 留在对应领域包内，通过接口注入 HTTP、数据库和 LLM 依赖，保证独立测试和独立 PR。
6. 领域之间通过 DTO 或接口传递数据，不直接读取另一个领域的私有字段。
7. 根 `main` 不包含业务逻辑，只允许出现 Wails 选项、嵌入资源和 `internal/app` 的组合调用。

## 已完成与待拆分

| 边界 | 现状 | 下一步独立 PR |
| --- | --- | --- |
| Wails 进程入口 | 已完成，根目录只保留 `main.go` | 保持稳定 |
| domain/contracts/platform | 已完成基础包和兼容适配 | 清理迁移完成后的兼容别名 |
| search | 仍在 `internal/app` | 先迁纯检索模型、source adapter 和 ranking |
| storage | 仍在 `internal/app` | 先抽 repository 接口，再迁 SQLite 实现 |
| llm | 仍与检索辅助逻辑同在 `clients.go` | 拆 provider client、budget 和 prompt/normalizer |
| deepread / screening | 仍依赖 `App` 和 `DB` 私有状态 | 先提取 service 输入输出，再迁工作流 |
| library / sync | 仍共享本地文件和数据库事务 | 先确定 managed asset 与 sync port |

## 功能 PR 归属

| 改动类型 | 当前落点 | 目标落点 | 必须同时更新 |
| --- | --- | --- | --- |
| 新检索源、重试、合并、排序 | `internal/app/clients.go`, `search_sources.go` | `internal/search` | source fixture、检索状态和 `PROJECT_MAP.md` |
| DeepRead 解析、证据、问答 | `internal/app/deepread*.go` | `internal/deepread` | reader tests、前端类型和用户可见错误 |
| Screening 抽取、决策树、导入 | `internal/app/screening*.go` | `internal/screening` | cancellation、事务和 workflow tests |
| 文件夹、论文 PDF、下载 | `internal/app/folders*.go`, `paper_*.go` | `internal/library` | path/security tests |
| 百度同步、冲突、恢复 | `internal/app/sync*.go`, `baidu_pcs.go` | `internal/sync` | mock PCS、sync progress tests |
| SQLite schema/repository | `internal/app/database*.go` | `internal/storage` | migration and restore tests |
| LLM provider、token budget | `internal/app/clients.go`, `llm_*.go` | `internal/llm` | provider mock、budget tests |
| Wails 路由、事件和生命周期 | `internal/app` + 根 `main` | `internal/app` + 根 `main` | frontend binding、browser/package smoke |

## 迁移顺序

1. 已完成：把所有 backend implementation 和 tests 从根 `package main` 移到 `internal/app`，根入口只做 Wails composition。
2. 已完成：共享 domain contracts、workflow request/response types、service ports 和 platform hardening helpers。
3. 下一步：把纯检索模型、source adapters、retry 和 ranking helpers 迁入 `internal/search`，保留 `internal/app` 的小型 facade。
4. 再下一步：抽出 `internal/storage` repository ports，迁移 SQLite repository 和 migration；禁止领域包直接访问 `DB.conn`。
5. 随后拆出 `internal/llm`，消除 `clients.go` 中 LLM 与搜索实现的混合。
6. 最后按 service 输入输出迁移 DeepRead、Screening、Library、Sync，并逐步删除 app 内部兼容别名。

每一步都必须保持：`go test ./...`、`go vet ./...`、Wails binding 生成、前端测试和对应领域回归测试通过。结构迁移不应顺带修改用户可见行为。

## Agent 工作规则

接手 Go 功能时，先读本文件和 [PROJECT_MAP.md](PROJECT_MAP.md)，再按“当前落点”查找实现、按“目标落点”设计新代码。新功能不要默认写入根目录，也不要把尚未完成的领域迁移伪装成已完成。一个 PR 尽量只包含一个领域的实现、测试和文档更新。
