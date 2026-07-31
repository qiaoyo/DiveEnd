# DiveEnd 论文检索源与 Agent 工具评估

更新时间：2026-07-31

## 结论

DiveEnd 不应再寻找一个替代 Semantic Scholar 的单一来源。更稳定的方案是按职责组合多个官方数据源：

1. `arXiv`：开放预印本和 PDF 主来源。
2. `OpenAlex`：跨学科召回、引用图、机构、开放获取位置和影响力数据。
3. `OpenReview`：ICLR、NeurIPS workshop、CoRL 等最新投稿、评审、回复、决定和 PDF。
4. `DBLP`：计算机领域的精确题名、作者、会议/期刊、年份和 DOI 校验。
5. `Hugging Face Daily Papers`：LLM、agent、robotics、world model 的近期趋势信号。
6. `Crossref`、`CORE`、`Europe PMC`：分别用于 DOI 元数据补全、开放全文补充和生医交叉领域补充。

推荐优先接入 `OpenAlex + OpenReview + DBLP`，并把 Hugging Face Daily Papers 做成独立的“趋势”入口。Semantic Scholar 恢复后仍可作为额外来源，而不是重新成为单点依赖。

## 真实请求结果

测试主题包括：

- `vision language action robotics`
- `world model robotics`
- `LLM agent benchmark`
- `embodied agent`

| 来源 | 认证 | 2026-07-31 实测 | 适合的角色 | 主要限制 |
| --- | --- | --- | --- | --- |
| OpenReview API v2 | 公共数据无需登录 | `200`；返回 VLA、world model、LLM agent benchmark 和 embodied agent 论文，以及摘要、评审、venue 状态和 PDF | 最新 ML/robotics 投稿与评审 | 会混入 rejected submission 和重复版本，必须按 forum/paperhash 去重并明确状态 |
| DBLP Search API | 无需 key | `200`；VLA 查询前五条均为高度相关的 IEEE RA-L 或机器人论文 | CS 元数据精确检索与 venue/DOI 校验 | 没有摘要、引用图和稳定 PDF 字段 |
| Hugging Face Daily Papers | 无需 key | `200`；最近 100 篇中命中 TurboVLA、PhiZero、agent benchmark、world model 等大量强相关新论文 | AI 趋势、热度和每日发现 | 公开端点缺少稳定契约，不应作为唯一检索源 |
| Europe PMC | 无需 key | `200`；返回 robotics/VLA 交叉论文 | 生医、神经、医疗机器人补充 | 对纯 CS、agent、world model 覆盖较弱 |
| OpenAlex | 免费 key，匿名额度很低 | 匿名搜索返回 `429`；官方文档确认免费 key 每日提供 1,000 次 search、10,000 次 filter、100 次内容下载 | 最适合替代 Semantic Scholar 的全局召回与引用图 | 实际使用应先申请免费 key；内容下载需要计量 |
| Crossref | 无需 key | 当前出口返回 `429` | DOI、出版信息、撤稿/更新和期刊元数据补全 | 摘要不完整；不适合承担首要召回 |
| CORE | 可匿名，注册后额度更好 | 当前出口返回 `429` | 开放获取全文和仓储版本补充 | 限流较严格，搜索质量和 CS 新论文时效需继续评估 |
| Papers with Code legacy API | 已失效 | 请求重定向至 Hugging Face Papers HTML | 不再接入 | 旧 JSON API 不可作为生产依赖 |

## 推荐的数据源分工

### LLM 与 Agent

- 首召回：arXiv、OpenAlex。
- 最新会议与评审：OpenReview。
- 热点和社区信号：Hugging Face Daily Papers。
- 会议、期刊和 DOI 校验：DBLP、Crossref。

### Robotics 与 VLA

- 首召回：arXiv、OpenAlex、DBLP。
- CoRL/ICLR 等投稿和评审：OpenReview。
- IEEE、期刊元数据：DBLP、Crossref。
- 开放全文：arXiv、OpenAlex locations、CORE。

### World Model

- 最新发现：arXiv、OpenReview、Hugging Face Daily Papers。
- 引用关系和作者/机构追踪：OpenAlex。
- 正式出版版本：DBLP、Crossref。

## 接入设计

### 统一召回

每个改写 query 并行请求来源，每个来源保留独立超时、限流和熔断状态：

```text
OpenAlex  8
arXiv     8
OpenReview 6
DBLP      5
HF Trends 仅用于趋势入口
```

合并后按以下顺序去重：

1. DOI。
2. arXiv ID。
3. OpenReview forum ID。
4. 规范化标题、年份和第一作者。

### 排序信号

- 查询与题名/摘要的匹配度。
- OpenReview venue、decision 和 review confidence。
- OpenAlex citation count、referenced/citing works 和 topic。
- DBLP venue 精确度。
- Hugging Face upvotes 只作为趋势信号，不能替代学术质量判断。
- 发表时间衰减。

### 可信度要求

- UI 必须显示来源和来源状态。
- OpenReview 必须区分 submitted、accepted、rejected、withdrawn。
- 不能把 citation count、upvote 和 review rating 混成同一种指标。
- 缺少摘要时不得生成伪摘要；可在 PDF 获取后再做模型抽取。
- 一个来源失败时保留其他来源结果，并展示可行动的诊断。

## Skill、Plugin 与 MCP 评估

### 推荐

#### Google DeepMind `literature-search-openalex` skill

- 来源：`google-deepmind/science-skills`，Apache-2.0。
- 2026-07-31：约 2.6k GitHub stars，skills.sh 约 1.6k installs。
- 支持 works、authors、institutions、topics、DOI、引用、开放 PDF 和聚合查询。
- CLI 内置限流、重试、字段裁剪和 API key 脱敏。
- 同时兼容 Codex、Claude Code 和 Agent Skills 标准。
- 需要先配置 `OPENALEX_API_KEY` 才适合稳定使用。

安装命令：

```bash
npx skills add google-deepmind/science-skills@literature-search-openalex -g -y
```

这个 skill 适合 agent 的临时研究和验证。DiveEnd 产品内仍应直接实现 OpenAlex 客户端，避免产品运行依赖 agent 环境。

#### OpenAI Zotero plugin

- 本机已有官方 plugin 缓存，但当前未启用。
- 实测 Zotero profile 存在，local API `127.0.0.1:23119` 未开启，Zotero 未运行。
- 能搜索本地文库、导出 BibTeX、插入 citation key、读取已索引全文和导入记录。
- 它不是网络论文检索器，适合作为 DiveEnd 的文库导出/引用互操作层。

### 谨慎使用

- OpenReview 官方 `openreview-mcp` 主要帮助 LLM 查询 `openreview-py` 方法签名和测试示例，并不是论文搜索 MCP。产品检索应直接使用 OpenReview API v2。
- `Academix`、`paperclip`、`openalex-mcp-server` 等社区 MCP 功能方向合理，但当前约 4-29 stars，维护面和回归保障不足，不适合成为 DiveEnd 核心依赖。
- `systematic-literature-review`、`academic-paper` 等 skill 可用于研究流程提示，但不会改善底层数据质量；不能替代官方 API 客户端。
- Claude Code 当前没有配置 MCP；已安装的 `gopls-lsp` 和 `superpowers` plugin 均处于 disabled，与论文检索无关。

### 不推荐

- Google Scholar 抓取：没有面向论文搜索的官方 Scholar API，依赖非官方抓取会带来验证码、封禁和条款风险。
- 继续使用 Papers with Code 旧 API：当前已重定向到 Hugging Face Papers 页面。
- 将通用 Web Search 当作论文元数据真源：可以补技术报告、项目主页和博客，但 DOI、venue、作者和版本必须回到学术来源校验。

## 外部账号

### 首要：OpenAlex

在 <https://openalex.org/settings/api> 创建免费 key。官方当前免费额度为：

- 1,000 次 search/天。
- 10,000 次 list/filter/天。
- 100 次内容下载/天。
- 单实体 DOI/ID 查询不限量。

收到 key 后将其放入本地忽略配置，不要粘贴到聊天或提交到 Git。

### 可选：CORE

如果后续实测证明开放全文覆盖有增益，再到 <https://core.ac.uk/services/api> 注册 key。CORE 不应阻塞第一轮 OpenAlex/OpenReview/DBLP 接入。

## 建议的下一项开发

1. 实现 OpenReview 与 DBLP 客户端，不需要等待账号。
2. 项目所有者创建 OpenAlex 免费 key。
3. 实现 OpenAlex 客户端、引用图和开放获取位置。
4. 增加 Hugging Face Daily Papers 趋势入口，并为非正式 API 保留开关和快速禁用能力。
5. 用固定的 LLM、agent、robotics、VLA、world model 查询集建立来源质量回归测试。
6. 再评估 CORE 是否对 PDF 获取率有显著提升。

## 参考资料

- OpenAlex authentication and pricing: <https://developers.openalex.org/guides/authentication>
- OpenAlex API: <https://developers.openalex.org/api-reference/introduction>
- OpenReview API v2: <https://docs.openreview.net/reference/api-v2/openapi-definition>
- DBLP search API: <https://dblp.org/faq/How+to+use+the+dblp+search+API.html>
- Crossref REST API: <https://www.crossref.org/documentation/retrieve-metadata/rest-api/>
- Europe PMC REST API: <https://www.ebi.ac.uk/europepmc/webservices/rest/search>
- CORE API: <https://core.ac.uk/services/api>
- Google DeepMind science skills: <https://github.com/google-deepmind/science-skills>
