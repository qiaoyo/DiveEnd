# DiveEnd 人工全量使用与验收手册

更新时间：2026-08-12

这是一份给 DiveEnd 个人研究者和开发者使用的真实验收手册。目标是完整走完“提出研究问题、检索论文、阅读论文、筛选论文、保存笔记、同步备份”的闭环，并在网络、模型、PDF、云同步和应用重启等异常条件下确认数据不会丢失、页面不会白屏、操作不会卡死。

建议第一次按本文从上到下执行。之后每次版本更新，可以执行“快速冒烟路径”和本次改动相关的专项路径。

## 0. 使用方式

每个验收项都有一个编号。测试时填写：

- `✅`：通过，实际结果符合预期。
- `❌`：失败，记录复现步骤、错误文字、发生时间和截图。
- `⚠️`：功能可用，但体验、性能、文案或结果质量有问题。
- `⏭`：由于账号、网络、文件或权限条件暂时无法测试。

反馈建议使用下面的格式：

```text
[DR-07] ❌
环境：macOS / Wails 开发窗口 / 2026-08-12 15:20
操作：输入“robot learning from human feedback”，点击开始检索
实际：Semantic Scholar 一直显示第 1 次尝试，最终只返回 OpenAlex 结果
预期：其他来源失败时保留部分结果，并明确提示来源状态
附件：截图、终端日志、相关论文标题
严重度：P1
```

严重度建议：

- `P0`：数据丢失、密钥泄露、应用无法启动、数据库被破坏。
- `P1`：核心流程无法完成、结果错误、操作卡死、恢复失败。
- `P2`：功能可用但交互不合理、错误提示不清楚、明显视觉问题。
- `P3`：文案、间距、轻微动画或非核心体验问题。

## 1. 测试前准备

### 1.1 真实验收和浏览器演示的区别

完整真实验收必须使用 Wails 桌面应用。浏览器中的 `npm run dev` 主要使用 `frontend/src/lib/backend.ts` 的 mock fallback，适合验证页面状态和无后端时的布局，不等价于真实论文检索、PDF 解析、LLM 调用或云同步。

| 环境 | 适合测试 | 不应据此宣称通过 |
| --- | --- | --- |
| `wails dev` | 真实 Go 后端、Wails 事件、真实本地文件、真实网络和 PDF 服务 | 不能替代正式打包生命周期 |
| `wails build` 后的 `DiveEnd.app` | 资源嵌入、asset server、懒加载和桌面生命周期 | 不能替代真实云账号验收 |
| `cd frontend && npm run dev` | 页面路由、mock 数据、空态、加载态、错误态、响应式布局 | 不能作为真实 API、LLM、PDF 或同步通过证据 |
| `node frontend/scripts/ui-smoke.mjs` | 自动覆盖 25 个浏览器 mock 状态 | 不会验证真实 key、PDF 和云端文件 |

### 1.2 依赖、账号和测试材料

开始真实验收前确认：

- [ ] macOS 已安装 Go 1.25.12 或同一 1.25 系列中的更高安全补丁版本。
- [ ] 已安装 Node.js、npm、Python 和 Wails CLI。
- [ ] 已执行 `cd frontend && npm install`。
- [ ] 已执行 `bash scripts/setup_pdf_service.sh` 创建 PDF 服务环境。
- [ ] 强模型和弱模型配置至少能完成一次真实请求。
- [ ] OpenAlex key 可用。
- [ ] Semantic Scholar key 可用，并且本地 seed 文件格式正确。
- [ ] 如果测试同步，Google Drive Desktop OAuth client JSON 或百度 token 文件已准备。
- [ ] 准备至少两篇真实论文 PDF，最好一篇短论文、一篇 10 页以上论文。
- [ ] 另准备一篇没有可下载 PDF 的候选，用于失败恢复测试。
- [ ] 准备两个研究问题。例如：`近两年 LLM agent 在长任务规划和工具调用可靠性上的主要方法、benchmark 与失败模式是什么？` 和 `视觉语言动作模型在真实机器人任务中的泛化和评测方法有哪些？`

### 1.3 本地凭据

以下文件只放在本机，禁止提交 Git，也不要把内容复制到反馈中：

```text
/Users/bytedance/DiveEnd/config/strong_llm.json
/Users/bytedance/DiveEnd/config/weak_llm.json
/Users/bytedance/DiveEnd/config/openalex.json
/Users/bytedance/DiveEnd/config/semantic_scholar.json
/Users/bytedance/DiveEnd/baiduyun_token.json
/Users/bytedance/DiveEnd/config/google_drive_client.json
```

模板位于 `config/*.example` 和 `baiduyun_token.json.example`。Google Drive 的 OAuth client JSON 和授权后的 refresh token 都不能进入 Git。

### 1.4 数据隔离和备份

macOS 默认用户配置通常位于：

```text
~/Library/Application Support/DiveEnd/config.json
```

默认数据目录通常位于：

```text
~/DiveEndData/
```

其中包括 SQLite 数据库、论文 PDF、解析缓存、Screening 管理文件和同步暂存目录。第一次做删除、恢复和中断同步测试前：

- [ ] 完全退出 DiveEnd。
- [ ] 备份整个数据目录到项目目录之外：

  ```bash
  ditto "$HOME/DiveEndData" "$HOME/DiveEndData-backup-before-uat"
  ```

- [ ] 如需破坏性测试，在设置页把数据路径改到新的隔离目录，例如 `~/DiveEndData-uat-20260812`，保存后重启。
- [ ] 不要直接删除生产数据目录来清空应用，先确认设置中的 `数据存储路径`。

## 2. 快速冒烟路径

### UAT-SMOKE-01 启动与首页

- [ ] 启动应用，等待初始化完成。
- [ ] 看到首页标题 `从问题到结论`，没有白屏、无限 spinner 或未处理错误。
- [ ] 顶部可以看到 `DiveEnd`、`发现`、`阅读`、`分析`，右侧可以打开 `研究记录`、`同步`、`设置`。
- [ ] 首页统计能显示论文、文件夹和研究记录数量。
- [ ] 在研究问题框输入测试问题，点击 `带着问题去发现`。
- [ ] 确认问题被带入发现页，没有丢失或额外发起重复检索。

### UAT-SMOKE-02 真实 DeepStart

- [ ] 确认目标文件夹已选择。
- [ ] 点击 `开始检索`。
- [ ] 观察来源进度、阶段、百分比和预计剩余时间。
- [ ] 等待首批结果和最终研究会话完成。
- [ ] 确认候选有标题、作者/机构、来源、年份、摘要或缺失信息说明。
- [ ] 打开一篇 `论文详情`，检查匹配理由、关键词、作者、完整摘要和外部链接。
- [ ] 选择 1 到 3 篇论文，点击 `导入选中`。
- [ ] 确认论文进入目标文件夹；下载状态可能先是排队中或下载中。

### UAT-SMOKE-03 真实 DeepRead

- [ ] 点击 `去 DeepRead` 或顶部 `阅读`。
- [ ] 在论文库中选择一篇已经下载的论文。
- [ ] 点击 `准备阅读内容`，等待 PDF 服务 ready、PDF 加载和章节解析完成。
- [ ] 点击不同章节，确认原文区域会切换。
- [ ] 在 AI 阅读助手中输入问题，点击 `基于论文回答`。
- [ ] 检查回答、takeaway、论文依据和限制说明；点击依据，确认能跳转到对应章节。
- [ ] 点击 `总结全文`，确认强模型总结完成。
- [ ] 选择章节，生成翻译与摘要，保存一条笔记。
- [ ] 切换页面后返回 DeepRead，确认论文、翻译记录和笔记仍在。

### UAT-SMOKE-04 Screening 与同步

- [ ] 点击顶部 `分析`，选择至少两篇本地 PDF。
- [ ] 等待内容提取完成，进入筛选标准。
- [ ] 选择一个或多个 AI 筛选选项，点击 `继续下一步`，直到筛选完成。
- [ ] 点击 `导入到文库`，确认论文进入文库。
- [ ] 进入 `同步`，点击 `刷新`，检查 provider 和本地数据统计。
- [ ] 点击 `检查并同步`，阅读预检中的文件数、总大小、论文 PDF 数量和数据库快照大小。
- [ ] 先点击 `暂不上传`，确认不会开始同步；再次预检后点击 `确认开始同步`。
- [ ] 观察同步进度、历史记录和最终状态。

## 3. 启动、窗口和生命周期

### UAT-BOOT-01 开发窗口

在项目根目录执行：

```bash
wails dev
```

- [ ] 终端没有 Go panic、前端编译失败或 Wails binding 错误。
- [ ] 应用窗口能打开，等待 `正在打开研究工作区` 消失。
- [ ] 关闭应用后，`wails dev` 进程能结束，或能明确看到服务被回收。
- [ ] 再次启动不会出现重复数据库迁移错误或重复启动不可用 PDF 服务。

### UAT-BOOT-02 正式打包

```bash
wails build
open build/bin/DiveEnd.app
```

- [ ] 打包成功并能打开应用。
- [ ] 顶部路由、懒加载页面和图标都正常。
- [ ] 真实 PDF 能通过 Wails asset server 加载；大文件没有明显卡死。
- [ ] 关闭应用后重新打开，配置、研究记录、论文库和笔记仍存在。

### UAT-BOOT-03 启动失败恢复

- [ ] 在空测试目录启动一次，确认没有论文时仍显示正常空态。
- [ ] 暂时把某个本地 seed 文件移到项目外再启动；确认页面能打开，缺失凭据会以可理解状态提示，而不是白屏。
- [ ] 恢复 seed 文件后重启，确认配置重新加载。
- [ ] 不要用生产数据测试损坏 SQLite；如必须测试，使用副本并先保存备份。

## 4. 首页与一级导航

### UAT-HOME-01 首页入口

- [ ] 点击左上角 `DiveEnd`，返回首页，不是无反应或回到发现页。
- [ ] 点击 `发现`，进入研究问题输入页。
- [ ] 点击 `阅读`，进入阅读工作台。
- [ ] 点击 `分析`，进入 Screening。
- [ ] 点击右侧 `研究记录`、`同步`、`设置` 图标，页面和 active 状态正确；在同一图标上再次点击会关闭工具页并返回打开前的页面。
- [ ] 点击最右侧主题图标，确认可在浅色/深色之间切换，刷新后主题仍保持。
- [ ] 切换页面时没有旧页面覆盖新页面，也没有明显布局跳动。

### UAT-HOME-02 首页问题接力

- [ ] 输入空白问题，点击 `打开发现`，确认进入发现页且不会发起空 query。
- [ ] 输入有内容的问题，点击 `带着问题去发现`，确认发现页文本框保留问题。
- [ ] 使用 `Ctrl/Cmd + Enter` 提交，结果与点击按钮相同。
- [ ] 点击三个示例研究问题，确认文本框被替换为对应问题。
- [ ] 点击最近研究，确认进入对应 session，而不是创建新研究。
- [ ] 点击 `查看全部研究记录`，确认进入历史页。
- [ ] 在空数据目录验证“还没有研究记录”“当前目录暂无论文”等空态文案。

## 5. 设置与凭据

设置页分为 `软件`、`凭据`、`云同步` 三个 tab。保存配置前后观察底部状态文案。

### UAT-SET-01 强模型和弱模型

进入 `设置 -> 软件`：

- [ ] 检查强模型和弱模型的 Provider 类型、Provider ID、显示名称、Base URL、Wire API、模型名。
- [ ] 在 OpenAI-compatible 和 Anthropic 之间切换，确认字段随 provider 变化且不会白屏。
- [ ] 修改 Base URL 和模型名，点击 `保存配置`，确认显示 `配置已保存`。
- [ ] 如果修改数据路径，确认提示“将在重启后生效”，重启后验证实际路径改变。
- [ ] 验证 `OpenAI Bearer Auth` 和 `禁用响应存储` 开关可切换、保存并重启后保留。
- [ ] 设置每日 LLM token 上限，观察今日已用和剩余额度。
- [ ] 在隔离目录把额度设小，触发请求直到达到上限，确认新请求在发送前停止。

### UAT-SET-02 凭据输入和脱敏

进入 `设置 -> 凭据`：

- [ ] 强模型和弱模型都能输入 key、显示/隐藏 key、清除 key。
- [ ] 已配置时留空保存，确认不会意外清空原有本地密钥；要清除时使用明确的清除按钮。
- [ ] 保存后重新进入设置，只显示“已配置”或脱敏状态，不显示完整 key。
- [ ] 使用无效 key 执行请求，确认错误中没有回显 key、Authorization header 或完整 URL query。
- [ ] 恢复正确 key 后重新测试。

### UAT-SET-03 搜索源实际状态

当前代码包含 OpenAlex、Semantic Scholar、arXiv、OpenReview 和 DBLP。搜索源 key 主要通过本地 seed 文件加载，当前设置页不会逐项展示所有搜索源开关。

- [ ] 查看 `config/app.yaml` 的 `search` 配置。
- [ ] 注意当前 checkout 中 `enable_semantic_scholar` 可能仍是 `false`，而部分项目文档描述为默认启用；这是配置漂移风险，不能只凭文档判定 Semantic Scholar 已启用。
- [ ] 运行一次 DeepStart，观察来源进度中是否出现 `Semantic Scholar`。
- [ ] 如果没有出现，记录 `[SET-SEARCH-01]`，不要把其他来源有结果当作 Semantic Scholar 通过。
- [ ] 修改配置前记录原值；真实 key 只能留在本地，不能写进 Git。

### UAT-SET-04 深色模式和数据路径

- [ ] 切换深色模式，检查首页、发现、会话详情、阅读、分析、同步、设置的背景、文本、按钮和弹窗可读。
- [ ] 重启应用，确认主题保留。
- [ ] 把数据路径改到全新的测试目录，保存并重启。
- [ ] 在新目录创建一个文件夹和一条研究记录，退出后确认数据仍在新目录。
- [ ] 恢复原数据路径并重启，确认生产数据仍可见。

## 6. DeepStart 发现页

### UAT-DS-01 输入与提交

- [ ] 空输入点击 `开始检索`，确认被阻止并显示“请输入要研究的问题、方法或领域”。
- [ ] 输入中文问题并提交。
- [ ] 输入英文问题并提交。
- [ ] 输入前后带空格的问题，确认后端使用 trim 后的 query。
- [ ] 使用 `Ctrl/Cmd + Enter` 提交。
- [ ] 连续快速点击按钮，确认只创建一个 session。
- [ ] 检查 `保存到目标文件夹` 初始为空；确认可以先点击 `新建文件夹` 创建并自动选中，也可以明确选择已有文件夹。
- [ ] 未选择目标文件夹时，确认 `开始检索` 保持禁用，不会把结果悄悄保存到上一次使用的目录。

### UAT-DS-02 真实来源进度

执行一个真实问题并观察全程：

- [ ] 看到 `检索候选`、`补全元信息`、`获取全文`、`解析论文`、`提取结构`、`生成研究地图`、`保存工作区` 等阶段时，文字和进度条一致。
- [ ] 来源列表出现实际启用的 OpenAlex、Semantic Scholar、arXiv、OpenReview、DBLP。
- [ ] 成功来源显示返回篇数；成功但没有命中时显示 `无匹配`。
- [ ] 失败来源显示本来源暂不可用或重试次数，不能阻塞其他来源。
- [ ] 进度从 0 到接近 100 后稳定完成，不一直停在 99%。
- [ ] 首批候选可用后，界面能显示结果；后台补全继续时页面仍可操作。
- [ ] 结果不足时显示空态和建议，不显示伪造论文。

### UAT-DS-03 停止和异常

- [ ] 在任务刚开始时点击 `停止`，确认不会重复发起取消。
- [ ] session ID 建立后点击 `停止`，确认进度进入 `正在停止`，最终显示已停止。
- [ ] 停止后确认没有把半完成结果错误标成完整分析。
- [ ] 断开网络后开始检索，确认来源独立失败、错误可读、页面仍能操作。
- [ ] 恢复网络后重试，确认可重新开始任务。
- [ ] 让 Semantic Scholar 触发 429 或暂时不可用，确认退避重试且不拖垮其他来源。
- [ ] 检查错误文案中没有 API key、Authorization、refresh token 或敏感 query 参数。

## 7. DeepStart 会话详情

### UAT-SESSION-01 会话恢复和顶部操作

- [ ] 从最近研究或历史页打开一个 session。
- [ ] 刷新窗口或重新启动应用后直接打开同一个 session，确认会话能重新加载。
- [ ] 点击 `返回历史`，确认返回研究记录。
- [ ] 在 `入库到` 下拉框切换目标文件夹，确认状态保存。
- [ ] 点击 `新建文件夹`，输入 `Robotics/VLA/Benchmarks` 这样的路径，确认多级目录被创建并成为可选目标。
- [ ] 尝试空路径、重复路径和非法路径，确认有提示且不会创建异常目录。

### UAT-SESSION-02 重搜、回退和补充检索

- [ ] 在顶部 query 输入新的检索问题，按 Enter，确认候选池被新结果替换，原 session 仍可追踪。
- [ ] 点击 `重搜`，确认按钮进入处理中且不能重复点击。
- [ ] 重搜过程中点击 `停止`，确认原会话不会被空结果覆盖。
- [ ] 点击 `回退上一轮`，确认候选池和会话消息回到上一轮状态。
- [ ] 在研究助理输入“只保留真实机器人实验和公开 benchmark 的论文”，点击 `发送`。
- [ ] 点击 AI 返回的建议 query，确认它只填入输入框，不会未经确认自动发请求。
- [ ] 将 `每源` 数量设为 5、20、100，分别执行 `发起补充检索`。
- [ ] 输入 0、负数、超过 100、非数字，失焦后确认被限制在合法范围。
- [ ] 补充检索后确认新结果与旧结果去重、当前池数量更新、消息记录保留。
- [ ] 缩窄或补充检索时切换页面再回来，确认不会出现重复消息或旧请求覆盖新状态。
- [ ] 收起和展开 `研究助理`，确认输入内容和已有消息不丢失。

### UAT-SESSION-03 研究概览和结果质量

- [ ] 检查研究概览中的原始问题、英文检索词、重写命中，以及“来源返回、合并重复版本后、当前候选池”三段数量。
- [ ] 检查当前候选池数量和后台处理状态。
- [ ] 检查研究方向名称、摘要、为什么归入该方向以及每个方向的论文数。
- [ ] 点击方向的 `全选`，确认只选择该方向的论文。
- [ ] 点击方向的 `清空`，确认只清除该方向的选择。
- [ ] 逐篇点击 `选择` / `已选择`，确认选择状态持久化。
- [ ] 检查论文卡片的标题、venue、年份、引用数、来源、机构、匹配理由、关键词和摘要。
- [ ] 遇到缺失摘要、作者、年份、机构或 PDF 的论文，确认使用明确的缺失文案而不是空白或 `undefined`。

### UAT-SESSION-04 论文详情抽屉

- [ ] 点击论文卡片，确认打开右侧 `论文详情` 抽屉，背景遮罩可点击关闭。
- [ ] 点击关闭按钮，确认抽屉关闭且滚动位置基本保持。
- [ ] 在详情中确认检索匹配、命中词、机构/学校、关键词、作者列表和完整摘要。
- [ ] 点击 `加入选中`，确认卡片和底部数量同步更新。
- [ ] 点击 `打开链接`，确认外部链接在新窗口打开且没有被应用拦截。
- [ ] 点击 `复制摘要`，确认剪贴板内容与摘要一致；在不支持剪贴板的环境确认有提示。
- [ ] 关闭抽屉后快速打开另一篇，确认内容没有串台。

### UAT-SESSION-05 本地存储速览与下载队列

- [ ] 查看本地存储速览中的文件夹、论文卡片数量、排队中、已下载和失败数量。
- [ ] 导入论文后观察 `queued -> downloading -> downloaded` 的状态变化。
- [ ] 对没有可下载 PDF 的论文观察失败状态和错误文案。
- [ ] 完成任务后等待数秒，确认不会无限制地每 3 秒刷新或重复请求。
- [ ] 任务后台处理中返回历史再回来，确认状态继续更新且不会重置对话输入。

### UAT-SESSION-06 批量入库

- [ ] 不选择论文时确认 `导入选中` 禁用，并显示“请先选择至少一篇论文”。
- [ ] 没有目标文件夹时确认显示目标文件夹提示。
- [ ] 选择一篇论文入库，确认反馈中有新增数量。
- [ ] 再次导入同一篇，确认重复论文被跳过，不产生重复文库项。
- [ ] 同时选择多篇，确认批量导入成功，PDF 下载在后台进行。
- [ ] 导入完成后点击 `去 DeepRead`、`去历史`、`回首页`，确认三个跳转都正确。
- [ ] 导入过程中快速点击按钮，确认不会重复创建论文或重复下载。

## 8. 研究记录

进入 `研究记录`：

- [ ] 没有 session 时检查空态、`新建检索` 和 `前往发现`。
- [ ] 有 session 时检查标题、根问题、完成/处理中状态和更新时间。
- [ ] 点击每条记录，确认进入对应 session，而不是最后一次 session。
- [ ] 完成中的 session 重新进入，确认可以继续观察后台进度。
- [ ] 点击 `新建检索`，确认不会携带旧 session 的临时输入。
- [ ] 在多个 session 之间切换，确认论文候选、消息和选择状态不串台。

## 9. DeepRead 阅读工作台

### UAT-READ-01 文库选择与加载

- [ ] 在没有选论文时打开 `阅读`，确认显示“从论文库选择一篇论文开始阅读”。
- [ ] 点击 `论文库` / `收起论文库`，确认侧边文库可显示和隐藏。
- [ ] 选择不同文件夹，确认论文列表更新，不会被上一个文件夹的响应覆盖。
- [ ] 在当前目录搜索论文标题和作者，确认过滤结果正确。
- [ ] 点击论文，确认标题、PDF 状态和 AI 区域都切换到该论文。
- [ ] 快速切换两篇论文，确认不会把第一篇的章节、摘要、PDF 或 AI 回答显示到第二篇。

### UAT-READ-02 PDF 下载失败修复

分别对 queued、downloading、downloaded、failed 状态测试：

- [ ] `downloaded`：点击 `准备阅读内容`，确认加载 PDF 和章节。
- [ ] `failed`：点击 `重试`，确认状态刷新且按钮不会重复提交。
- [ ] 没有可下载 PDF URL：确认自动展开 `手动链接`，不会只显示技术栈错误。
- [ ] 输入可访问的 `http/https` PDF URL，点击 `使用该链接重试`，确认下载后状态更新。
- [ ] 输入空链接、非 http/https 链接或网页 HTML URL，确认阻止或显示明确错误。
- [ ] 点击 `本地PDF`，在桌面文件选择器中选择真实 PDF，确认附件后可以准备阅读。
- [ ] 点击 `重新下载当前文件夹未完成 PDF`，确认批量队列和最终数量正确；没有待处理文件时显示明确提示。
- [ ] 点击 `打开网页`，确认论文网页在外部浏览器打开。

### UAT-READ-03 章节、页码和缩放

- [ ] 章节列表有内容时点击每个章节，确认正文和待翻译文本同步变化。
- [ ] 切换 `目录结构` 与 `页面缩略图`。
- [ ] 在缩略图中点击第 1 页、中间页、最后一页，确认 PDF 跳页。
- [ ] 点击 `上一页`，在第 1 页确认按钮禁用。
- [ ] 点击 `下一页`，在最后一页确认按钮禁用。
- [ ] 点击缩小、重置、放大，确认缩放范围稳定，不造成页面横向失控。
- [ ] 在 900 px 和 720 px 窗口下检查章节栏、原文区、AI 区没有互相遮挡。
- [ ] 在无法解析 PDF 时确认 `重试解析` 和 `准备阅读内容` 可以再次触发，而不是一直停在失败态。

### UAT-READ-04 AI 辅助阅读

- [ ] 未准备章节时点击 `基于论文回答`，确认按钮禁用或提示先准备阅读内容。
- [ ] 输入空问题，确认不会发起请求。
- [ ] 输入“这篇论文的核心假设是什么？实验是否真正支持结论？”，点击回答。
- [ ] 检查按钮 loading、回答、takeaway、证据章节和 limitations。
- [ ] 点击一条证据，确认左侧章节切到对应章节。
- [ ] 点击 `总结全文`，确认走完整论文总结路径，而不是把问题框内容当作总结问题。
- [ ] AI 请求运行时点击 `停止`，确认请求被取消，界面恢复可用，取消不会被错误显示成失败。
- [ ] 请求运行时切换论文，确认旧请求结果不会覆盖新论文。
- [ ] 故意断网或使用无效模型 key，确认显示友好错误并保留已有阅读内容。
- [ ] 检查回答中的证据确实来自当前解析文本，不接受明显幻觉或不存在的 section ID；发现时记录为 P1。

### UAT-READ-05 翻译、摘要和笔记

- [ ] 选择章节，检查待翻译文本自动填充。
- [ ] 手工修改待翻译文本，点击 `生成翻译与摘要`。
- [ ] 检查译文、摘要和翻译历史。
- [ ] 弱模型不可用时确认错误提示包含检查 key、base URL、网络或缩短文本的建议。
- [ ] 输入空笔记并点击 `保存笔记`，确认被阻止。
- [ ] 输入短笔记和多段长笔记，点击保存，确认显示在笔记记录中。
- [ ] 切换章节后保存另一条笔记，确认 section 标签正确。
- [ ] 重启应用后重新打开论文，确认翻译记录和笔记仍存在。
- [ ] 在翻译或保存笔记过程中重复点击，确认只创建一条记录。

### UAT-READ-06 阅读主题与生命周期

- [ ] 点击阅读页主题切换，确认全局主题变化。
- [ ] 从 DeepRead 切换到其他页面再回来，确认主题、论文和章节保持。
- [ ] 在 PDF、AI 和笔记区域连续滚动较长时间，确认 CPU/内存没有持续异常增长或页面冻结。
- [ ] 关闭应用时如果 AI 请求正在运行，确认应用可以退出，重启后没有残留“处理中”假状态。

## 10. Screening 批量筛选

Screening 的真实路径是：`导入 PDF -> 内容提取 -> AI 标准筛选 -> 结果入库`。

### UAT-SCREEN-01 导入 PDF

- [ ] 点击顶部 `分析`，确认四步状态条为 `导入 / 内容提取 / 标准筛选 / 结果入库`。
- [ ] 使用桌面原生文件选择器选择一个 PDF。
- [ ] 一次选择多个 PDF，检查文件名列表和数量。
- [ ] 将多个真实 PDF 拖进虚线区域，确认拖拽高亮、松开后开始处理。
- [ ] 拖入非 PDF、空目录或空文件选择，确认提示清晰且不会创建空会话。
- [ ] 导入过程中连续点击选择按钮、重复拖入，确认只创建一个 session。
- [ ] 浏览器开发模式下如没有 native picker，可使用 `使用演示样本`；此项只验证 mock 页面，不代表真实 PDF 通过。

### UAT-SCREEN-02 内容提取

- [ ] 看到总数、已处理数、当前文件和进度条。
- [ ] 等待 PDF 服务自动启动并进入 ready。
- [ ] 观察每个文件从 extracting 到 extracted 的状态。
- [ ] 点击 `停止任务`，确认提取可取消且状态为已取消，不被误报成普通失败。
- [ ] 取消后点击 `重新提取`，确认可以从当前会话重试。
- [ ] 只要部分文件已完成，确认可以点击 `基于已完成论文继续筛选`。
- [ ] PDF 服务不可用时，确认错误提示明确指向 PDF 服务、模型配置或网络，而不是白屏。

### UAT-SCREEN-03 AI 决策树

- [ ] 进入 `标准筛选` 后检查当前判断、维度、选项和每个选项的论文数量。
- [ ] 单选或多选选项，确认选中视觉状态和已选择数量更新。
- [ ] 点击 `继续下一步`，确认按钮进入处理中并禁用重复提交。
- [ ] 检查路径历史，确认每一步的维度和选择保留。
- [ ] 在每一轮测试停止任务，确认 session 可以恢复。
- [ ] 让模型返回完整节点，确认进入 `结果入库`。
- [ ] 在异常模型响应下，确认界面保留当前会话，不把候选论文全部删除。

### UAT-SCREEN-04 结果与入库

- [ ] 检查总论文数和保留论文数。
- [ ] 检查保留论文标题、作者和状态。
- [ ] 点击 `导入到文库`，确认导入成功，文库论文数增加。
- [ ] 再次进入同一已完成 session，确认显示已入库，不重复导入。
- [ ] 点击 `开始新的筛选`，确认状态、文件列表、错误和旧结果都被清理，但历史 session 不被删除。

### UAT-SCREEN-05 Screening 会话恢复

- [ ] 在上传后关闭应用，重新进入分析页，确认最近筛选列表出现该 session。
- [ ] 在提取过程中关闭应用，重新打开并点击最近 session，确认进入提取状态，可继续或重试。
- [ ] 在决策树中关闭应用，重新打开 session，确认当前节点和路径历史恢复。
- [ ] 在已入库状态恢复，确认不会自动重新调用模型。

## 11. 论文库与文件夹管理

### 11.1 当前实际可达入口

DeepRead 自带论文库，DeepStart 会话详情也能选择目标文件夹、新建多级文件夹和查看本地存储。仓库中的 `frontend/src/components/paperlist/PaperListPanel.tsx` 保留了更完整的文件夹/论文管理组件，但当前 `AppLayout` 没有挂载它，因此不能把它的按钮当作当前产品的真实用户入口；它目前主要由组件测试覆盖。发现该组件无法从界面打开时，不要重复测试或误报为路由故障，应记录为“功能未挂载”。

### UAT-LIB-01 当前可达功能

- [ ] 在 DeepStart 详情页切换入库目标文件夹。
- [ ] 通过 `新建文件夹` 创建单级和多级路径。
- [ ] 在本地存储速览查看目录和下载统计。
- [ ] 在 DeepRead 文件夹目录切换目录。
- [ ] 在 DeepRead 论文卡片集中搜索论文。
- [ ] 在 DeepRead 中选择、重试、手动 URL 重试、本地 PDF 附件和批量重试。
- [ ] 删除非系统文件夹时确认弹窗显示影响范围；确认删除后当前目录自动回到系统目录。

### UAT-LIB-02 组件级开发回归

仅当你作为开发者需要验证未挂载的 `PaperListPanel` 时执行：

- [ ] 新建文件夹、取消新建、空名称校验。
- [ ] 重命名文件夹、取消重命名、系统文件夹不可重命名。
- [ ] 移动文件夹，确认不能移动到自身或子孙目录。
- [ ] 删除文件夹，确认二次确认、子文件夹统计和当前目录回退。
- [ ] 论文搜索、单选、全选当前结果、批量移动、单篇移动。
- [ ] 删除论文后当前选择和论文列表同步刷新。
- [ ] 运行：

  ```bash
  cd frontend
  npm test -- --run src/components/paperlist/PaperListPanel.test.tsx
  ```

## 12. Google Drive 和百度云同步

同步是有破坏性风险的功能。第一次真实同步请使用测试数据目录，确认云端文件正确后再同步生产库。

Google Drive 的申请和授权步骤见 [`GOOGLE_DRIVE_SYNC_SETUP.md`](GOOGLE_DRIVE_SYNC_SETUP.md)。

### UAT-SYNC-01 Google Drive 授权

进入 `设置 -> 云同步`：

- [ ] OAuth 客户端 JSON 路径指向实际的 Desktop app JSON。
- [ ] 状态显示客户端文件已找到。
- [ ] 点击 `连接 Google Drive`。
- [ ] 浏览器使用加入 Test users 的同一个 Google 账号完成授权。
- [ ] 返回应用后状态显示已授权，token 只保存在本机配置目录。
- [ ] 勾选 `启用 Google Drive 主同步`，确认 fallback 选项符合预期。
- [ ] 点击 `保存配置`，重启应用后确认授权状态仍可读。
- [ ] 点击 `移除本机授权`，确认本机授权失效但不删除云端历史备份。
- [ ] 再次授权，确认可以恢复。

### UAT-SYNC-02 同步预检

进入 `同步`：

- [ ] 点击 `刷新`，检查本地论文总数、待处理文件、冲突数、失败数和 provider。
- [ ] provider 未启用时确认 `检查并同步` 禁用或给出配置提示。
- [ ] 点击 `检查并同步`，确认只打开预检，不立即上传。
- [ ] 检查预检中的 provider、文件数、总大小、论文 PDF 数量、PDF 总大小、数据库快照大小、本地数据路径、远端目录和文件列表。
- [ ] 文件超过 20 个时确认界面提示只展示前 20 个，但不会只同步前 20 个。
- [ ] 点击 `关闭` 或 `暂不上传`，确认没有同步进度和远端新增。
- [ ] 重新预检并点击 `确认开始同步`。

### UAT-SYNC-03 同步进度和历史

- [ ] 观察当前文件、完成数/总数、进度条和状态。
- [ ] 同步期间重复点击 `刷新`、`检查并同步`，确认不会创建并发同步。
- [ ] 同步完成后刷新，检查最后同步时间和总同步成功数量。
- [ ] 在 `同步记录` 中查看上传/下载记录、文件名、类型、时间和错误信息。
- [ ] 关闭应用后重新打开，确认同步状态不会永远停在进行中。
- [ ] Google Drive 上传失败发生在开始前且 fallback 开启时，确认整个本次运行切换到百度云，而不是把部分文件拆到两个 provider。
- [ ] 上传已经开始后不要人为切换 provider；验证它会完成或明确失败，避免产生半个快照。

### UAT-SYNC-04 百度云 fallback 和 token

- [ ] 确认 `baiduyun_token.json` 包含 access token、refresh token、client id 和 client secret，且文件未提交 Git。
- [ ] 在设置页启用百度网盘同步。
- [ ] 当前 provider 为百度云时，在同步页点击 `更新凭证`。
- [ ] 检查刷新结果中是否有 access token、refresh token、client id、client secret 的配置状态；不要记录 token 内容。
- [ ] 执行一次预检和同步，确认真实百度云上传、列表、下载/清理路径可用。
- [ ] 令 access token 过期或使用旧 token，点击更新凭证，确认 refresh token 自动换取新 token 并安全保存。
- [ ] fallback 测试结束后清理测试云端目录，避免污染长期备份。

### UAT-SYNC-05 自动同步设置

- [ ] `启动时同步` 开关保存后重启应用，确认启动行为符合设置。
- [ ] `退出前同步` 开关打开后退出应用，确认有同步提示或执行退出前同步。
- [ ] `定时同步变更` 打开，检查间隔 5、15、30、60、180、720、1440 分钟选项。
- [ ] 自动同步关闭时确认间隔选择不可用或不会独立启动任务。
- [ ] 冲突策略分别测试 `保留较新`、`手动处理`、`保留本地`、`使用云端`。
- [ ] 设置保存失败时，确认 UI 回滚到旧设置。

### UAT-SYNC-06 冲突解决

在测试副本中制造同一个文件的本地/云端差异后：

- [ ] 刷新同步页，进入 `冲突处理`。
- [ ] 检查冲突文件名、文件类型、更新侧、大小、时间和本地/云端路径。
- [ ] 点击 `保留本地版本`，确认冲突消失或显示已解决，历史记录更新。
- [ ] 再制造一次冲突，点击 `使用云端版本`，确认本地数据按预期更新。
- [ ] 检查被覆盖版本是否保留到 `.sync-conflicts`，不要未经确认删除。

### UAT-SYNC-07 数据库恢复

只有在已备份测试数据后执行：

- [ ] 让同步流程发现云端数据库恢复候选。
- [ ] 确认页面显示远端来源、暂存状态和本地备份路径。
- [ ] 点击 `暂不恢复`，确认暂存状态取消且当前本地数据库不变。
- [ ] 再次出现恢复候选后点击 `立即应用并刷新`。
- [ ] 确认系统先备份本地数据库，再应用云端数据库，并刷新页面。
- [ ] 重启应用，检查论文、研究记录、设置和同步记录是否来自预期版本。
- [ ] 如果应用恢复失败，不要继续写入生产数据；保存错误、备份路径和当前数据目录，优先恢复本地备份。

## 13. 异常、边界和长时间使用

### UAT-EDGE-01 空数据

- [ ] 空论文库打开 DeepRead。
- [ ] 空研究记录打开历史页。
- [ ] 空文件夹打开论文列表。
- [ ] 空 PDF 选择提交 Screening。
- [ ] 空问题提交 DeepStart。
- [ ] 没有可下载 PDF 的候选进入 DeepRead。
- [ ] 没有冲突时打开冲突 tab。
- [ ] 没有同步历史时打开同步记录。

### UAT-EDGE-02 网络和来源异常

- [ ] 在请求前断网。
- [ ] 在检索过程中断网。
- [ ] 恢复网络后重试。
- [ ] 只让一个来源失败，确认其他来源仍返回。
- [ ] 让全部来源失败，确认得到明确错误且应用仍能继续导航。
- [ ] 测试 429、408、5xx 的退避和最终错误。
- [ ] 观察长时间等待时页面是否能取消，不要强制杀进程作为唯一恢复手段。

### UAT-EDGE-03 并发与重复操作

- [ ] 连续双击开始检索。
- [ ] 连续点击重搜、补充检索、导入、同步、保存笔记、继续筛选。
- [ ] 快速切换多个会话页面或路由。
- [ ] 任务运行时切换文件夹、论文和页面。
- [ ] 在同步进度刷新时手动刷新。
- [ ] 在 PDF 下载轮询时切换论文。
- [ ] 预期：按钮有 loading/disabled 状态；只保留一个有效请求；旧响应不能覆盖新选择；页面不会白屏。

### UAT-EDGE-04 长文本与大列表

- [ ] 首页输入超长问题。
- [ ] DeepStart 输入超长筛选偏好和长 query。
- [ ] DeepRead 对长论文执行总结和问答。
- [ ] 保存多条长笔记。
- [ ] 文库中导入 20 篇以上论文，检查滚动和搜索。
- [ ] 同步 20 个以上文件，检查预检列表截断说明和实际总数。
- [ ] 观察长时间滚动、反复切换和多次请求后的内存和响应速度。

### UAT-EDGE-05 应用中断与恢复

- [ ] DeepStart 检索时关闭应用，再打开并进入 session。
- [ ] DeepStart 后台下载时关闭应用，再打开检查状态。
- [ ] DeepRead AI 请求时关闭应用，再打开检查是否能继续阅读。
- [ ] Screening 抽取时关闭应用，再恢复 session。
- [ ] Screening 决策树中关闭应用，再恢复节点和路径。
- [ ] 同步中关闭应用，重启后检查是否显示明确的部分失败/可重试状态。
- [ ] 每个场景都检查是否有重复论文、半条笔记、损坏数据库或永远 processing 的任务。

## 14. 开发者操作手册

### DEV-01 接手项目的阅读顺序

每次自己或让 agent 接手代码，按以下顺序：

1. `README.md`
2. `AGENTS.md`
3. `docs/PROJECT_MAP.md`
4. `docs/GO_STRUCTURE.md`
5. `docs/PAPER_SEARCH_SOURCE_EVALUATION.md`
6. `docs/GOOGLE_DRIVE_SYNC_SETUP.md`
7. `docs/USER_ACCEPTANCE_MANUAL.md`
8. 与当前功能对应的 `internal/app`、`frontend/src` 和测试文件

### DEV-02 日常开发命令

```bash
cd frontend
npm install
npm test -- --run
npm run build
cd ..

go test ./...
go vet ./...
go build ./...

services/pdf_service/.venv/bin/python -m pytest services/pdf_service/tests

bash scripts/secret_scan.sh
git diff --check

# 单独运行；关闭开发窗口后再执行后面的构建命令
wails dev
wails build
```

如果 `services/pdf_service/.venv/bin/python` 不存在，先执行：

```bash
bash scripts/setup_pdf_service.sh
```

不要因为系统 `python` 命令不存在就跳过 PDF 测试，使用虚拟环境中的 Python。

### DEV-03 浏览器 mock 烟测

```bash
node frontend/scripts/ui-smoke.mjs
UI_SMOKE_WIDTH=720 UI_SMOKE_HEIGHT=1000 node frontend/scripts/ui-smoke.mjs
```

烟测覆盖首页、发现、DeepStart、DeepRead、Screening、Sync 等 25 个状态；窄窗口额外检查页面级横向溢出。报告和截图写入系统临时目录。烟测通过只能证明 mock 页面状态和路由可用，不能替代真实用户验收。

### DEV-04 真实 E2E 测试

这些测试会消耗 API、LLM 或云端额度，只在确认 key、网络和测试数据后执行：

```bash
DIVEEND_REAL_SEARCH_E2E=1 go test ./internal/app -run TestRealDeepStartRetrievalE2E -v
DIVEEND_REAL_LLM_E2E=1 go test ./internal/app -run TestRealStrongAndWeakLLMBudgetE2E -v
DIVEEND_REAL_DEEPREAD_E2E=1 go test ./internal/app -run 'TestRealDeepRead(Grounding|FastQuestion)E2E' -v
```

真实 PDF 解析/抽取必须提供测试 PDF：

```bash
DIVEEND_REAL_PDF_EXTRACTION_E2E=1 \
  DIVEEND_REAL_PDF_PATH="$HOME/path/to/test-paper.pdf" \
  go test ./internal/app -run TestRealPDFExtractionBudgetE2E -v
```

真实百度同步前确认使用测试数据目录：

```bash
DIVEEND_REAL_BAIDU_E2E=1 go test ./internal/app -run TestRealBaiduSyncE2E -v
DIVEEND_REAL_BAIDU_E2E=1 go test ./internal/app -run TestRealBaiduSyncCurrentDataE2E -v
```

当前 Google Drive 真实验收主要通过桌面设置页、浏览器授权、同步预检和实际同步完成；Google Drive provider 的 mock/单元测试不等于真实 OAuth E2E。

### DEV-05 后端和前端代码定位

| 任务 | 首先查看 |
| --- | --- |
| 初始化、配置、数据库 | `internal/app/app.go`、`config_store.go`、`database.go` |
| 搜索源、重试、去重、排序 | `internal/app/clients.go`、`search_sources.go`、`deepstart_pipeline.go` |
| DeepStart 会话和后台任务 | `deepstart.go`、`deepstart_task.go`、`deepstart_background.go` |
| DeepRead、PDF、AI 证据 | `deepread.go`、`deepread_task.go`、`pdf_service_client.go` |
| Screening | `screening_workflow.go`、`screening_task.go` |
| 文库和文件夹 | `folders_api.go`、`paper_import_assets.go`、`local_file_actions.go` |
| 同步、Google Drive、百度 fallback | `sync.go`、`sync_api.go`、`google_drive.go`、`sync_provider.go`、`baidu_pcs.go` |
| Wails 前端桥接和 mock | `frontend/src/lib/backend.ts` |
| 路由和全局错误 | `frontend/src/components/layout/Router.tsx`、`AppLayout.tsx` |
| 主页、发现、会话 | `frontend/src/components/home/`、`deepstart/` |
| 阅读 | `frontend/src/components/deepread/DeepReadPanel.tsx` |
| 分析 | `frontend/src/pages/Screening.tsx` |
| 同步和设置 | `frontend/src/pages/Sync.tsx`、`frontend/src/components/settings/SettingsPanel.tsx` |

### DEV-06 诊断日志和安全检查

开发窗口中观察启动终端和 Go 日志；前端错误应出现在顶部错误条或页面内 `role="alert"` 区域。反馈时保留：

- 操作编号和时间。
- 当前页面、当前 session ID 或 paper ID；不要提供 key。
- 是 Wails、浏览器 mock 还是打包应用。
- 与操作相邻的日志行，先删掉 URL query 中的 token 和个人路径。
- 截图和复现次数。

提交或推送前必须执行：

```bash
bash scripts/secret_scan.sh
git status --short --ignored
git diff --check
```

必须保持 ignored 的内容包括 API key、OAuth client JSON、Google/Baidu token、LLM 配置、`frontend/node_modules`、`frontend/dist`、`build/bin`、Python 虚拟环境、测试缓存和本地数据目录。历史密钥审计：

```bash
bash scripts/history_secret_audit.sh
```

该审计在历史尚未重写前可能失败；不要未经明确授权执行破坏性 history rewrite。

### DEV-07 SQLite 只读检查

确认数据路径后，可以只读检查数据库：

```bash
DATA_PATH="$HOME/DiveEndData"
sqlite3 "$DATA_PATH/diveend.db" '.tables'
sqlite3 "$DATA_PATH/diveend.db" 'select count(*) from papers;'
sqlite3 "$DATA_PATH/diveend.db" 'select count(*) from deepstart_sessions;'
sqlite3 "$DATA_PATH/diveend.db" 'select count(*) from screening_sessions;'
```

如果系统没有 `sqlite3`，不要为了诊断直接修改数据库；使用应用界面或 Go 测试。不要在应用运行时对生产数据库执行写操作。

### DEV-08 变更后的最小回归

- 修改路由/布局：运行 Router、相关页面测试、前端 build 和桌面烟测。
- 修改 DeepStart：运行 `SessionDetailPanel.test.tsx`、搜索/DeepStart Go tests 和对应烟测。
- 修改 DeepRead：运行 `DeepReadPanel.test.tsx`、PDF/DeepRead Go tests 和真实 PDF 测试。
- 修改 Screening：运行 `Screening.test.tsx`、screening Go tests 和真实 PDF/LLM 测试。
- 修改同步：运行 `Sync.test.tsx`、Google Drive/Baidu/sync Go tests 和预检 UI 测试。
- 修改配置/凭据：运行 `config_store_test.go`、secret scan 和重启持久化测试。

发布前完整基线：

```bash
go test ./...
go vet ./...
go build ./...
go test -race ./...
cd frontend
npm test -- --run
npm run build
cd ..
services/pdf_service/.venv/bin/python -m pytest services/pdf_service/tests
bash scripts/secret_scan.sh
git diff --check
wails build
node frontend/scripts/ui-smoke.mjs
UI_SMOKE_WIDTH=720 UI_SMOKE_HEIGHT=1000 node frontend/scripts/ui-smoke.mjs
```
