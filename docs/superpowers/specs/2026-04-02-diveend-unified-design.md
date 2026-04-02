# DiveEnd - Unified Design Specification

> **Document Version**: v1.0  
> **Date**: 2026-04-02  
> **Status**: Approved for Implementation

---

## 1. Executive Summary

DiveEnd is an immersive paper collection and AI translation reading tool designed for deep academic research. It unifies three core workflows:

1. **DeepStart** - Domain exploration and paper discovery
2. **DeepRead** - AI-assisted immersive reading with translation
3. **Paper Screening Pipeline** - Batch PDF processing with interactive filtering

### Target Users
- Academic researchers exploring new domains
- Graduate students conducting literature reviews
- Engineers tracking latest developments in specific fields

### Key Differentiators
- **Immersive single-page design** - No tab switching, all actions in one view
- **Three-column layout** - Configuration | Content | Paper List
- **Seamless DeepStart → DeepRead** workflow
- **Intelligent paper screening** with LLM-powered decision trees

---

## 2. Architecture Overview

### 2.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           DiveEnd - Unified Architecture                     │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────────────┐     ┌──────────────────┐     ┌─────────────────────────┐ │
│  │   Frontend    │────▶│    Go Backend    │────▶│   Python Microservice   │ │
│  │  React/TS    │     │  (Wails v2)      │     │   (PDF + LLM Pipeline)  │ │
│  └──────────────┘     └──────────────────┘     └─────────────────────────┘ │
│         │                      │                           │               │
│         ▼                      ▼                           ▼               │
│   ┌──────────┐          ┌──────────┐              ┌──────────────┐         │
│   │ 3-Column │          │ SQLite   │              │  Marker PDF  │         │
│   │ Layout   │          │ Database │              │  Parser      │         │
│   └──────────┘          └──────────┘              └──────────────┘         │
│                                                          │                  │
│                                                          ▼                  │
│                                                   ┌──────────────┐         │
│                                                   │  LLM Clients │         │
│                                                   │  - Weak      │         │
│                                                   │  - Strong    │         │
│                                                   └──────────────┘         │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 Technology Stack

| Layer | Technology | Rationale |
|-------|------------|-----------|
| **Frontend** | React 18 + TypeScript | Type safety, component ecosystem |
| **Styling** | Tailwind CSS | Utility-first, rapid prototyping |
| **Backend Framework** | Go + Wails v2 | Native performance, single binary deployment |
| **Database** | SQLite | Zero-config, single-file, perfect for local-first |
| **PDF Parsing** | Marker (Python) | Best-in-class academic PDF parsing |
| **LLM Integration** | OpenAI + Anthropic APIs | Weak/Strong tiered approach |
| **Cloud Sync** | Baidu Cloud | Specified requirement |

### 2.3 Data Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                             Data Flow Diagram                               │
└─────────────────────────────────────────────────────────────────────────────┘

DEEPSTART FLOW:
===============

User Query ──▶ DeepStart Module ──▶ Web Search (Google Scholar/ArXiv)
                                           │
                                           ▼
                                    Preliminary Results
                                           │
                                           ▼
                                    AI Classification ──▶ Category Tree
                                                               │
                                                               ▼
                                    ┌──────────────────────────────────────┐
                                    │  User Selection (Multi-round)        │
                                    │  - Category filtering                │
                                    │  - Keyword refinement                │
                                    │  - Date/source filtering             │
                                    └──────────────────────────────────────┘
                                                               │
                                                               ▼
                                    Confirmed Papers ──▶ Import to Local DB
                                                               │
                                                               ▼
                                    ┌──────────────────────────────────────┐
                                    │  Add to:                             │
                                    │  - Custom folders                    │
                                    │  - Reading lists                     │
                                    │  - "To Read" queue                   │
                                    └──────────────────────────────────────┘

DEEPREAD FLOW:
==============

User Selects Paper ──▶ DeepRead Module
                              │
                              ├──▶ Load from Database
                              │           │
                              │           ▼
                              │    ┌───────────────┐
                              │    │ PDF Available?│
                              │    └───────┬───────┘
                              │            │
                              │      ┌─────┴─────┐
                              │      ▼           ▼
                              │    [Yes]       [No]
                              │      │           │
                              │      ▼           ▼
                              │  Display      Trigger
                              │  PDF Preview  PDF Download
                              │               (if URL available)
                              │
                              ▼
                    User Clicks "DeepRead"
                              │
                              ▼
                    ┌─────────────────────┐
                    │  1. PDF → Text Extraction
                    │     (via PDF Service)
                    │
                    │  2. Full Text Analysis
                    │     (via Strong LLM)
                    │
                    │  3. Structured Output:
                    │     - Full translation
                    │     - Section summaries
                    │     - Key findings
                    │     - Critical analysis
                    │
                    │  4. Highlight Mapping
                    │     (align sections ↔ translation)
                    └─────────────────────┘
                              │
                              ▼
                    Display Split View
                    ┌─────────────────────────────────┐
                    │  Left: Original    │  Right: CN   │
                    │  (Highlighted)     │  (Translation)│
                    │                    │              │
                    │  [Abstract]        │  [摘要]      │
                    │  [Introduction]    │  [引言]      │
                    │  [Method]           │  [方法]      │
                    │  ...                │  ...         │
                    └─────────────────────────────────┘
                              │
                              ▼
                    Interactive Features:
                    - Click section → Jump to position
                    - Select text → Add note/ask AI
                    - "Explain this" → Inline AI help
                    - Export translation (PDF/Markdown)

PAPER SCREENING PIPELINE FLOW:
==============================

Batch PDFs ──▶ Stage 1: EXTRACT ──▶ Stage 2: STORE ──▶ Stage 3: SCREEN
                  │                    │                    │
                  ▼                    ▼                    ▼
            ┌───────────┐        ┌───────────┐        ┌───────────┐
            │  Parallel │        │  SQLite   │        │  Decision │
            │  PDF Parse│        │  Storage  │        │   Tree    │
            │  (Marker) │        │           │        │  (Strong  │
            ├───────────┤        ├───────────┤        │   LLM)    │
            │ Weak LLM  │        │  papers   │        ├───────────┤
            │ Structure │        │  metrics  │        │  User     │
            │  Extract  │        │ baselines │        │ Choices   │
            └───────────┘        └───────────┘        └─────┬─────┘
                                                          │
                                                          ▼
                                                    ┌───────────┐
                                                    │  Final    │
                                                    │ Selection │
                                                    │  (Status: │
                                                    │  selected)│
                                                    └───────────┘

STAGE 3: INTERACTIVE SCREENING DETAIL:
=======================================

User initiates screening ──▶ Load all unscreened papers from DB
                                     │
                                     ▼
                         ┌───────────────────────┐
                         │  Strong LLM Analysis  │
                         │  - Read all papers    │
                         │  - Identify dimensions│
                         │  - Generate branches  │
                         └───────────┬───────────┘
                                     │
                                     ▼
                         ┌───────────────────────┐
                         │   Decision Node 1     │
                         │   (e.g., Research Area)│
                         └───────────┬───────────┘
                                     │
                    ┌────────────────┼────────────────┐
                    ▼                ▼                ▼
               Option A          Option B         Option C
               (5 papers)       (8 papers)       (3 papers)
                    │                │                │
                    └────────────────┴────────────────┘
                                     │
                                     ▼
                              User Selection
                              (e.g., A + B)
                                     │
                                     ▼
                         ┌───────────────────────┐
                         │   Decision Node 2     │
                         │  (e.g., Method Type) │
                         │   on filtered set     │
                         │      (13 papers)      │
                         └───────────────────────┘
                                     │
                   ... (continue until ≤ threshold papers)
                                     │
                                     ▼
                         ┌───────────────────────┐
                         │   Final Confirmation  │
                         │   (≤ 5 papers)         │
                         │   [Select All]         │
                         │   [Review Individually]│
                         │   [Apply Tags]          │
                         └───────────┬───────────┘
                                     │
                                     ▼
                         ┌───────────────────────┐
                         │   Save to Database    │
                         │   - Update status to  │
                         │     'selected'         │
                         │   - Record full path   │
                         │   - Apply user tags    │
                         └───────────────────────┘

SCREENING DECISION TREE SCHEMA:
===============================

interface DecisionNode {
  node_id: string;
  node_type: "branch" | "leaf" | "confirm";
  message: string;           // Display to user
  dimension: string;          // e.g., "研究方向", "方法类别"
  
  options: Array<{
    key: string;              // "A", "B", etc.
    label: string;            // Display text
    paper_ids: number[];      // Which papers this option covers
    count: number;
  }>;
  
  allow_multi_select: boolean;
  allow_skip: boolean;
  allow_free_input: boolean;
  
  remaining_paper_ids: number[];
  path_so_far: PathHistoryItem[];
}

interface PathHistoryItem {
  dimension: string;
  choice: string | string[];  // Selected key(s) or free text
  timestamp: string;
}
```

这个详细的数据流设计是否准确反映了你的需求？特别关注点：

1. **DeepStart** 的多轮 AI 引导流程是否符合你预期的"发现"体验？
2. **DeepRead** 的分屏翻译视图是否满足"沉浸式阅读"的目标？
3. **Paper Screening** 的三阶段 pipeline 是否足够清晰，特别是 Stage 3 的决策树交互？

确认无误后，我将生成设计规格文档并提交到 Git，然后启动实施计划流程。