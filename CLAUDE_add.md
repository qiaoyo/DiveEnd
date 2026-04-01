# Paper Screening Pipeline — 技术设计文档

> **文档性质**：核心功能参考设计，供 LLM Agent (Claude Code) 实现  
> **版本**：v0.1 Draft

---

## 1. 系统概览

### 1.1 目标

给定一组论文 PDF，通过三阶段 pipeline 完成：提取 → 存储/展示 → 交互式筛选，最终输出经用户确认的论文候选列表。

### 1.2 架构总览

```
┌─────────────────────────────────────────────────────────────┐
│                      User Interface (Web)                   │
│         表格视图  │  筛选交互树  │  最终列表管理              │
└──────┬──────────────────┬──────────────────┬────────────────┘
       │                  │                  │
       ▼                  ▼                  ▼
┌──────────┐    ┌──────────────┐    ┌────────────────┐
│ Stage 1  │    │   Stage 2    │    │    Stage 3     │
│ Extract  │───▶│ Store/Merge  │───▶│ Interactive    │
│ (并行)    │    │ (DB + View)  │    │ Screening      │
└──────────┘    └──────────────┘    └────────────────┘
  PDF解析          SQLite/            强LLM +
  + 弱LLM          本地DB             决策树交互
```

---

## 2. Stage 1: 并行提取

### 2.1 职责

对每篇论文 PDF 独立执行：PDF 解析 → 关键段落抽取 → 弱 LLM 结构化总结。

### 2.2 处理流程

```
PDF File
  │
  ▼
┌─────────────────────────┐
│ PDF Parser               │
│ 提取:                    │
│  - 全文文本(分section)    │
│  - 表格(原始)            │
│  - 首页元信息            │
└────────┬────────────────┘
         │
         ▼
┌─────────────────────────┐
│ Section Router (规则)     │
│ 识别并截取:              │
│  - Abstract              │
│  - Introduction          │
│  - Method/Approach       │
│  - Experiments/Results   │
│  - Conclusion            │
│  - References (首页脚注) │
└────────┬────────────────┘
         │ 拼接为 condensed_text（预计占全文 30-40%）
         ▼
┌─────────────────────────┐
│ 弱LLM (structured output)│
│ Input: condensed_text    │
│ Output: ExtractionSchema │
└─────────────────────────┘
```

### 2.3 输出 Schema

```jsonc
// ExtractionSchema — 每篇论文一个对象
{
  "title": "string",
  "affiliations": ["string"],        // 作者单位列表，去重
  "github_url": "string | null",     // 从正文/脚注/abstract中提取
  "problem": "string",               // ≤2句，解决什么问题
  "method": "string",                // ≤3句，核心方法描述

  "metrics": [
    // 每个指标一个对象，直接从实验表格提取
    {
      "metric_name": "string",       // e.g. "Success Rate", "RMSE"
      "dataset_or_task": "string",   // e.g. "HumanoidBench-Walk"
      "ours_value": "string",        // 本文方法的值
      "unit": "string | null"        // e.g. "%", "m/s"
    }
  ],

  "baselines": [
    // 每个指标下的 baseline 对比，与 metrics 按 metric_name 关联
    {
      "metric_name": "string",
      "comparisons": [
        {
          "method_name": "string",   // baseline 名称
          "value": "string"
        }
      ]
    }
  ],

  "relevance_tags": ["string"],      // 从预定义枚举中选取，见 2.4

  // 元数据（管线自动填充，非LLM输出）
  "_source_pdf": "string",           // 文件路径
  "_extracted_at": "ISO8601",
  "_model_used": "string",           // e.g. "gpt-4o-mini"
  "_token_usage": { "input": 0, "output": 0 }
}
```

### 2.4 relevance_tags 枚举定义

tags 应在项目配置文件中定义，用户可自行扩展。初始集合示例：

```python
RELEVANCE_TAGS = [
    "sim-to-real",
    "locomotion",
    "manipulation",
    "reinforcement-learning",
    "imitation-learning",
    "diffusion-policy",
    "transformer-architecture",
    "sciml-pinn",
    "usd-related",
    "dataset",
    "benchmark",
    "other"
]
```

### 2.5 并行策略

| 参数 | 值 | 说明 |
|------|----|------|
| 并发度 | `max_workers` 可配，默认 8 | 受 LLM API rate limit 约束 |
| 执行方式 | `asyncio.gather` 或 `concurrent.futures.ThreadPoolExecutor` | 视 PDF 解析库的异步支持决定 |
| 失败处理 | 单篇失败不阻塞整体，记录 error 到 DB，标记 `status=failed` | — |
| 重试 | 最多 2 次，指数退避 | 针对 LLM API 调用 |

### 2.6 PDF 解析工具选型（待调研）

需评估的开源方案方向：

| 类别 | 候选 | 关注点 |
|------|------|--------|
| PDF → 文本+表格 | Marker, Docling, PyMuPDF, Nougat | 表格提取质量、学术PDF适配 |
| 学术论文专用解析 | GROBID, ScienceParse, S2ORC parser | 自动分 section、提取引用 |
| 已有 Agent Skill | 搜索 LangChain/LlamaIndex 社区的 paper reading tools | 减少重复造轮 |

> **Action Item**: 实现前先调研上述工具，选定一个 PDF 解析方案。优先考虑能自动识别 section boundary + 表格的方案。

---

## 3. Stage 2: 存储与展示

### 3.1 职责

将 Stage 1 的结构化输出写入本地数据库，提供前端表格视图。

### 3.2 数据库设计

使用 **SQLite**（单文件、零部署、适合本地工具链）。

```sql
-- 论文主表
CREATE TABLE papers (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    title         TEXT NOT NULL,
    affiliations  TEXT,            -- JSON array
    github_url    TEXT,
    problem       TEXT,
    method        TEXT,
    relevance_tags TEXT,           -- JSON array
    source_pdf    TEXT,
    extracted_at  TEXT,
    model_used    TEXT,
    token_input   INTEGER,
    token_output  INTEGER,
    status        TEXT DEFAULT 'extracted'  
                  -- enum: 'extracted' | 'failed' | 'selected' | 'rejected'
);

-- 指标表（一对多）
CREATE TABLE metrics (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    paper_id      INTEGER REFERENCES papers(id),
    metric_name   TEXT,
    dataset_or_task TEXT,
    ours_value    TEXT,
    unit          TEXT
);

-- Baseline 对比表（一对多，关联到 metric）
CREATE TABLE baselines (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    paper_id      INTEGER REFERENCES papers(id),
    metric_name   TEXT,           -- 与 metrics.metric_name 关联
    method_name   TEXT,
    value         TEXT
);

-- 筛选会话记录
CREATE TABLE screening_sessions (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at    TEXT,
    criteria_log  TEXT,           -- JSON: 用户在交互树中的完整选择路径
    result_paper_ids TEXT         -- JSON array of paper IDs
);
```

### 3.3 数据写入流程

```python
# 伪代码
def store_extraction(db: sqlite3.Connection, result: ExtractionSchema):
    paper_id = db.execute(
        "INSERT INTO papers (title, affiliations, ...) VALUES (?, ?, ...)",
        (result.title, json.dumps(result.affiliations), ...)
    ).lastrowid

    for m in result.metrics:
        db.execute(
            "INSERT INTO metrics (paper_id, metric_name, ...) VALUES (?, ?, ...)",
            (paper_id, m.metric_name, ...)
        )

    for b in result.baselines:
        for comp in b.comparisons:
            db.execute(
                "INSERT INTO baselines (paper_id, metric_name, method_name, value) ...",
                (paper_id, b.metric_name, comp.method_name, comp.value)
            )
```

### 3.4 前端展示方案

> **细节留空**，此处仅定义接口契约。

**要求**：

- 以表格形式展示 `papers` 表全部字段
- 支持按 `relevance_tags`、`status` 筛选
- `metrics` 和 `baselines` 在行内可展开查看
- 展示方案可以是：Web UI（如 Streamlit/Gradio）、TUI、或导出到飞书多维表格

**数据接口**：

```python
# 前端读取数据的统一接口
def get_all_papers(db) -> list[dict]:
    """返回所有论文，metrics 和 baselines 嵌套在每个 paper 对象内"""

def get_papers_by_tags(db, tags: list[str]) -> list[dict]:
    """按 tag 过滤"""

def update_paper_status(db, paper_id: int, status: str):
    """更新论文状态"""
```

---

## 4. Stage 3: 交互式筛选

### 4.1 职责

强 LLM 读取数据库中的结构化数据，生成**决策树**供用户逐步筛选，最终将筛选结果写回数据库。

### 4.2 交互模型：决策树

核心思想：LLM 不直接给出最终推荐，而是生成一棵**筛选维度树**，用户在每一层做选择（多选/单选/跳过/自由输入），逐步收窄候选集。

```
交互流程:

Round 1: LLM 分析全部论文，生成第一层分支
┌──────────────────────────────────────────────┐
│ 我将这 25 篇论文按研究方向分为以下类别：      │
│                                              │
│ [A] Sim-to-Real Transfer (8篇)               │
│ [B] Policy Learning - Diffusion (6篇)        │
│ [C] Policy Learning - RL (5篇)               │
│ [D] Benchmark / Dataset (4篇)                │
│ [E] Other (2篇)                              │
│                                              │
│ 请选择感兴趣的类别(多选/跳过/自由输入):       │
└──────────────────────────────────────────────┘
User: A, B

Round 2: LLM 在候选集(14篇)上生成下一层分支
┌──────────────────────────────────────────────┐
│ 在 Sim-to-Real 和 Diffusion Policy 的 14 篇中，│
│ 按任务领域可分为：                            │
│                                              │
│ [A] Locomotion (6篇)                         │
│ [B] Manipulation (5篇)                       │
│ [C] Multi-task / General (3篇)               │
│                                              │
│ 或者您可以输入其他筛选维度。                  │
└──────────────────────────────────────────────┘
User: 只看在真实机器人上验证过的

Round 3: LLM 根据自由输入条件过滤
┌──────────────────────────────────────────────┐
│ 在 14 篇中，有 6 篇包含真实机器人实验：       │
│                                              │
│ 1. Paper A - Humanoid Walk (Success: 87%)    │
│ 2. Paper B - Dexterous Grasp (SR: 92%)       │
│ ...                                          │
│                                              │
│ [A] 全部保留                                  │
│ [B] 逐篇确认                                 │
│ [C] 补充筛选条件                              │
└──────────────────────────────────────────────┘
User: A

→ 写入 DB: 6篇论文 status='selected'
→ 记录完整选择路径到 screening_sessions
```

### 4.3 决策树节点 Schema

```jsonc
// LLM 每轮输出的结构
{
  "node_type": "branch",              // "branch" | "leaf" | "confirm"
  "message": "string",                // 展示给用户的说明文本
  "dimension": "string",              // 当前分支的筛选维度名 e.g. "研究方向"
  "options": [
    {
      "key": "A",                     // 选项标识
      "label": "string",             // 选项文字
      "paper_ids": [1, 3, 7, ...],   // 该选项覆盖的论文ID
      "count": 8
    }
  ],
  "allow_multi_select": true,
  "allow_skip": true,                 // 跳过=不在此维度过滤
  "allow_free_input": true,           // 允许用户输入自定义条件
  "remaining_paper_ids": [1,2,3,...], // 当前候选集
  "path_so_far": [                    // 到达此节点的选择历史
    {"dimension": "研究方向", "choice": ["A","B"]},
    {"dimension": "自由输入", "choice": "只看真实机器人验证"}
  ]
}
```

### 4.4 用户输入 Schema

```jsonc
{
  "action": "select" | "skip" | "free_input" | "confirm_all" | "done",
  "selected_keys": ["A", "B"],       // action=select 时
  "free_text": "string"              // action=free_input 时
}
```

### 4.5 交互循环实现

```python
# 伪代码
def screening_loop(db, llm_client):
    papers = get_all_papers(db)  # 从DB读取全部已提取论文
    candidate_ids = [p["id"] for p in papers]
    path = []

    while True:
        # 1. LLM 生成当前决策节点
        node = llm_client.generate_decision_node(
            papers=papers,
            candidate_ids=candidate_ids,
            path_so_far=path,
            system_prompt=SCREENING_SYSTEM_PROMPT
        )

        # 2. 展示给用户，获取输入
        user_input = present_and_get_input(node)

        # 3. 处理用户输入
        if user_input.action == "done":
            break
        elif user_input.action == "skip":
            continue  # 不收窄候选集
        elif user_input.action == "select":
            selected_paper_ids = merge_selected_options(node, user_input.selected_keys)
            candidate_ids = selected_paper_ids
            path.append({"dimension": node.dimension, "choice": user_input.selected_keys})
        elif user_input.action == "free_input":
            # LLM 根据自由文本过滤候选集
            candidate_ids = llm_filter_by_free_text(
                llm_client, papers, candidate_ids, user_input.free_text
            )
            path.append({"dimension": "自由输入", "choice": user_input.free_text})
        elif user_input.action == "confirm_all":
            break

    # 4. 写回DB
    save_screening_result(db, candidate_ids, path)
    return candidate_ids
```

### 4.6 强 LLM 的 System Prompt 要点

```
你是论文筛选助手。你的任务是帮助用户从 {N} 篇论文中逐步筛选出感兴趣的子集。

规则：
1. 每轮你必须输出一个 JSON 格式的决策节点（schema 见下方）
2. 分支维度应基于论文数据中的实际差异，优先选择区分度最高的维度
3. 维度选择优先级：研究方向 > 任务类型 > 方法类别 > 性能水平 > 发表单位
4. 当候选集 ≤ 5 篇时，切换到 node_type="confirm"，逐篇列出供用户最终确认
5. 每个选项必须附带 paper_ids，确保可追溯
6. 不要替用户做决定，只提供结构化选项
```

---

## 5. 项目结构

```
paper-screener/
├── config/
│   ├── default.yaml          # 全局配置：LLM endpoints, concurrency, tags枚举
│   └── prompts/
│       ├── extraction.txt    # Stage 1 的 system prompt
│       └── screening.txt     # Stage 3 的 system prompt
├── src/
│   ├── stage1_extract/
│   │   ├── pdf_parser.py     # PDF 解析封装（适配选定的开源工具）
│   │   ├── section_router.py # 识别/截取关键 section
│   │   ├── llm_extractor.py  # 弱LLM 结构化提取
│   │   └── parallel.py       # 并行调度
│   ├── stage2_store/
│   │   ├── db.py             # SQLite schema 初始化 + CRUD
│   │   ├── models.py         # Pydantic models (ExtractionSchema 等)
│   │   └── export.py         # 导出为 JSON / Markdown table
│   ├── stage3_screen/
│   │   ├── decision_tree.py  # 决策树节点生成逻辑
│   │   ├── interaction.py    # 用户交互循环
│   │   └── llm_judge.py      # 强LLM 调用封装
│   ├── ui/                   # 前端展示（方案待定）
│   │   └── ...
│   └── main.py               # CLI 入口
├── data/
│   ├── papers/               # 存放 PDF 文件
│   └── screener.db           # SQLite 数据库文件
├── tests/
├── pyproject.toml
└── README.md
```

---

## 6. 配置文件结构

```yaml
# config/default.yaml

extraction:
  pdf_parser: "marker"                # 可选: marker | docling | grobid
  llm:
    provider: "openai"                # openai | anthropic | local
    model: "gpt-4o-mini"
    temperature: 0.0
    max_output_tokens: 2000
  concurrency:
    max_workers: 8
    retry_max: 2
    retry_backoff: 2.0                # 指数退避基数(秒)
  section_keywords:                   # section 识别关键词
    abstract: ["abstract"]
    introduction: ["introduction", "1. introduction"]
    method: ["method", "approach", "our method", "proposed method"]
    experiments: ["experiment", "results", "evaluation"]
    conclusion: ["conclusion", "discussion"]

screening:
  llm:
    provider: "anthropic"
    model: "claude-sonnet-4-20250514"
    temperature: 0.3
    max_output_tokens: 4000
  auto_confirm_threshold: 5           # 候选集 ≤ 此值时自动切换到逐篇确认

database:
  path: "data/screener.db"

tags:
  - "sim-to-real"
  - "locomotion"
  - "manipulation"
  # ... 用户自行扩展
```

---

## 7. 关键接口定义

### 7.1 Stage 1 对外接口

```python
async def extract_papers(
    pdf_paths: list[str],
    config: ExtractionConfig
) -> list[ExtractionResult]:
    """
    批量提取论文信息。
    返回: 每篇论文的 ExtractionSchema + 元数据，失败的论文包含 error 字段。
    """
```

### 7.2 Stage 2 对外接口

```python
def init_db(db_path: str) -> sqlite3.Connection:
def store_results(db, results: list[ExtractionResult]) -> list[int]:  # 返回 paper_ids
def get_all_papers(db) -> list[dict]:
def get_papers_by_ids(db, ids: list[int]) -> list[dict]:
def update_paper_status(db, paper_id: int, status: str):
def save_screening_session(db, paper_ids: list[int], criteria_log: list[dict]):
```

### 7.3 Stage 3 对外接口

```python
def run_screening(
    db: sqlite3.Connection,
    llm_client: LLMClient,
    presenter: UIPresenter          # 抽象层：CLI / Web / TUI
) -> list[int]:
    """
    启动交互式筛选循环。
    返回: 最终选中的 paper_ids。
    """
```

---

## 8. 待决事项

| # | 事项 | 优先级 | 备注 |
|---|------|--------|------|
| 1 | PDF 解析工具最终选型 | P0 | 需评估 Marker / Docling / GROBID 在学术论文上的表格提取质量 |
| 2 | 前端方案选型 | P1 | Streamlit / Gradio / 纯 CLI / 飞书多维表格 |
| 3 | 是否支持非 PDF 输入 (arXiv URL 自动下载) | P1 | 可后续扩展 |
| 4 | 强 LLM 的 function calling 支持情况 | P0 | 决策树输出需要严格 JSON，需确认选用模型的 structured output 能力 |
| 5 | 筛选结果的下游对接 | P2 | 如: 自动生成精读 prompt / 导出到 Zotero |


