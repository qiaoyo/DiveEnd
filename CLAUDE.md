# DiveEnd - 项目进度概览

> **项目名称**: DiveEnd - 沉浸式论文收集与AI翻译阅读工具  
> **最后更新**: 2026-04-02  
> **当前阶段**: Phase 3 - DeepRead (沉浸式阅读体验)

---

## 项目架构总览

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

---

## 开发阶段进度

| 阶段 | 名称 | 状态 | 关键交付物 |
|------|------|------|------------|
| **Phase 1** | Foundation (基础架构) | ✅ 已完成 | Config系统、LLM Client、Database基础 |
| **Phase 2** | PDF Service (PDF服务) | ✅ 已完成 | Python microservice、Marker集成、FastAPI |
| **Phase 3** | DeepRead (沉浸式阅读) | ✅ 已完成 | 分屏阅读UI、翻译视图、PDF查看器 |
| Phase 4 | DeepStart (领域探索) | ⏳ 待开始 | 搜索集成、AI引导选择、导入流程 |
| Phase 5 | Screening Pipeline (论文筛选) | ⏳ 待开始 | 三阶段Pipeline、决策树UI |
| Phase 6 | Sync & Polish (同步与优化) | ⏳ 待开始 | 百度云同步、冲突解决、测试文档 |

---

## 当前阶段详细任务 (Phase 3: DeepRead)

### 3.1 阅读界面核心组件

- [ ] **SplitScreenLayout 分屏布局组件**
  - 左侧：PDF 原文显示区
  - 右侧：AI 翻译/总结区
  - 支持拖拽调整分屏比例
  - 支持全屏切换

- [ ] **PDFViewer 组件**
  - 基于 react-pdf 实现
  - 支持页面导航、缩放
  - 支持文本选择和复制
  - 支持章节跳转

- [ ] **TranslationPanel 翻译面板**
  - 显示 AI 翻译内容
  - 支持双语对照模式
  - 支持章节导航
  - 显示关键概念高亮

### 3.2 DeepRead 功能实现

- [ ] **DeepRead 按钮与触发**
  - 在论文列表中添加 "DeepRead" 按钮
  - 点击后触发全文分析流程
  - 显示加载状态

- [ ] **全文分析 Pipeline**
  - PDF → Text 提取 (调用 PDF Service)
  - Text → LLM 分析 (调用 Strong LLM)
  - 结构化输出：翻译、总结、关键发现

- [ ] **结果展示与存储**
  - 将分析结果存储到数据库
  - 缓存翻译结果避免重复请求
  - 支持导出翻译内容

### 3.3 UI 细节优化

- [ ] **章节高亮与同步**
  - 原文章节标黄显示
  - 点击章节自动跳转
  - 滚动同步（原文 ↔ 翻译）

- [ ] **交互增强**
  - 选词快速翻译
  - 添加阅读笔记
  - 关键概念卡片

- [ ] **主题与布局**
  - 支持 Dark/Light 模式
  - 字体大小调整
  - 布局模式切换（分屏/全屏）

---

## 技术栈确认

| 层级 | 技术选择 | 状态 |
|------|----------|------|
| 前端框架 | React 18 + TypeScript | ✅ 已配置 |
| 构建工具 | Vite | ✅ 已配置 |
| UI 组件 | Radix UI + Tailwind CSS | ✅ 已配置 |
| 状态管理 | Zustand | ✅ 已配置 |
| 分屏布局 | allotment | ✅ 已配置 |
| PDF 渲染 | react-pdf | ✅ 已配置 |
| 后端框架 | Go + Wails v2 | ✅ 已配置 |
| 数据库 | SQLite (GORM) | ✅ 已配置 |
| PDF 解析 | Python + Marker | ✅ 已配置 |
| LLM API | OpenAI + Anthropic | ✅ 已配置 |

---

## 项目目录结构

```
DiveEnd/
├── CLAUDE.md                    # 本文件 - 项目进度概览
├── README.md                    # 项目说明
├── wails.json                   # Wails 配置
├── go.mod / go.sum             # Go 依赖
├── main.go                      # Go 入口
├── app.go / app_test.go        # 主应用逻辑
├── clients.go                  # LLM 客户端
├── database.go                 # 数据库操作
├── deepstart.go                # DeepStart 模块
├── config_store.go             # 配置存储
├── models.go                   # 数据模型
├── build/                      # 构建输出
├── config/                     # 配置文件目录
│   ├── weak_llm.json
│   ├── strong_llm.json
│   └── app.yaml
├── src/                        # Go 源码
│   ├── config/                # 配置管理
│   ├── database/              # 数据库
│   ├── llm/                  # LLM 客户端
│   ├── deepread/            # DeepRead (当前开发)
│   ├── deepstart/           # DeepStart
│   └── screening/          # Screening Pipeline
├── frontend/                  # React 前端
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── tailwind.config.js
│   ├── index.html
│   └── src/
│       ├── main.tsx
│       ├── App.tsx
│       ├── components/      # 通用组件
│       ├── pages/            # 页面组件
│       ├── hooks/            # 自定义 Hooks
│       ├── stores/           # Zustand stores
│       └── utils/            # 工具函数
└── services/                 # 外部服务
    └── pdf_service/         # Python PDF 服务
        ├── Dockerfile
        ├── docker-compose.yml
        ├── requirements.txt
        ├── app/               # FastAPI 应用
        ├── core/              # 核心逻辑
        └── prompts/           # LLM Prompts
```

---

## 下一步行动

### 当前任务 (Phase 3.1)

1. **创建 DeepRead 核心组件**
   - [ ] `frontend/src/pages/DeepRead.tsx` - DeepRead 主页面
   - [ ] `frontend/src/components/deepread/SplitScreenLayout.tsx` - 分屏布局
   - [ ] `frontend/src/components/deepread/PDFViewer.tsx` - PDF 查看器
   - [ ] `frontend/src/components/deepread/TranslationPanel.tsx` - 翻译面板

2. **实现 Go 后端 DeepRead 模块**
   - [ ] `src/deepread/service.go` - DeepRead 服务层
   - [ ] `src/deepread/handler.go` - 请求处理器
   - [ ] `src/deepread/models.go` - DeepRead 数据模型

3. **集成与测试**
   - [ ] 前后端 API 联调
   - [ ] PDF 加载与显示测试
   - [ ] 翻译结果展示测试

---

## 开发规范

### 代码规范

- **Go**: 使用标准 Go 格式化，遵循 Effective Go
- **TypeScript**: 使用 strict 模式，遵循 Airbnb 规范
- **Git**: 使用 Conventional Commits
  - `feat:` 新功能
  - `fix:` 修复
  - `docs:` 文档
  - `refactor:` 重构
  - `test:` 测试
  - `chore:` 构建/工具

### 提交模板

```
feat(deepread): add PDF viewer component

- Implement PDFViewer with react-pdf
- Add zoom and page navigation
- Support text selection

Closes #123
```

---

## 参考资源

### 设计文档

- `/docs/superpowers/specs/2026-04-02-diveend-unified-design.md` - 统一设计规格
- `/docs/superpowers/plans/2026-04-02-diveend-implementation-plan.md` - 实施计划
- `/services/pdf_service/README.md` - PDF 服务文档

### 外部参考

- [Wails v2 Documentation](https://wails.io/docs/)
- [Marker PDF Parser](https://github.com/VikParuchuri/marker)
- [react-pdf](https://react-pdf.org/)
- [Radix UI](https://www.radix-ui.com/)

---

> **注意**: 本文档是活文档，会随着项目进展持续更新。请确保在每次关键里程碑后更新进度状态。
