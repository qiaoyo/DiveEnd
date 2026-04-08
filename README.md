# DiveEnd

沉浸式论文收集与 AI 翻译阅读工具，当前实现基于 Wails + React + Go。

## 当前已实现

- 三栏桌面布局：左侧设置、中间 DeepStart / DeepRead、右侧论文库
- 本机配置文件持久化：LLM（强/弱）、主题、数据目录等设置保存在用户配置目录
- SQLite 文库与迁移：自动创建默认 `Inbox` 文件夹，兼容旧 `papers` 表升级
- 论文搜索与导入：支持 Semantic Scholar + arXiv 聚合搜索，并导入指定文件夹
- DeepRead 手动章节翻译：保存章节原文、中文翻译与摘要历史
- 配置安全性：API Key / Token 默认隐藏显示，避免首次打开即明文暴露
- 主题切换：`light/dark` 会影响全局背景与文字颜色（基础版本）
- 配置预填充：首次启动会从 `config/strong_llm.json`、`config/weak_llm.json`、`baiduyun_token.json` 读取并写入运行时配置（不强制覆盖已有配置）

## 项目状态（2026-04-08 快照）

### 已完成（本轮检查与修复）

- 配置栏改造：增加强/弱两套 LLM 配置与两条 Key 输入，密钥字段默认隐藏，可手动点亮可见
- 移除当前不需要的 `Semantic Scholar key` 强制输入：搜索方案暂不要求额外 key
- 基础主题生效：全局 `body/html` 深浅色背景与文字颜色随主题切换
- 布局稳定性：补齐多处 `min-h-0/overflow`，减少缩放与分栏时遮挡/溢出（重点覆盖 DeepStart / DeepRead / 三栏容器）
- 配置系统支持弱模型：后端 `AppConfig` 增加 `weakLLM`，并支持种子配置读取与 secrets 合并
- 测试与构建：`go test -vet=off ./...` 与 `frontend` 的 `npm run build` 可通过

### 未完成 / 已知缺口

- Wails `dev` 仍可能出现空白页（需结合 Webview 控制台/运行时脚本进一步定位）
- Screening Pipeline 仍是半成品：后端计划（`docs/superpowers/plans/2026-04-05-phase5-screening-backend.md`）未落地，前端页面多为占位与演示链路
- PDF Service 微服务链路未闭环：Marker/抽取/结构化/翻译自动化仍需补齐
- 百度网盘同步未闭环：仅保留配置入口，缺少真实同步、冲突解决与状态回传
- UI 仍有细节问题：部分页面在极窄窗口/高 DPI 下仍可能出现拥挤，需要继续做响应式收敛与面板最小宽度策略

## 开发方式

### 前端构建

```bash
cd frontend
npm run build
```

### 后端测试

```bash
go test ./...
```

### Wails 开发

```bash
wails dev
```

## 数据与配置

- 配置文件：`$XDG_CONFIG_HOME/DiveEnd/config.json` 或系统默认用户配置目录
- 数据库目录：配置中的 `dataPath`
- SQLite 文件：`<dataPath>/diveend.db`

## 后续待完善

- 百度网盘同步
- PDF 驱动阅读与解析
- 更强的 DeepStart 分类与筛选能力
- 文件夹重命名 / 删除等文库管理操作

## 文档索引

- 统一设计：`docs/superpowers/specs/2026-04-02-diveend-unified-design.md`
- 实现计划：`docs/superpowers/plans/2026-04-02-diveend-implementation-plan.md`
- Screening 后端计划：`docs/superpowers/plans/2026-04-05-phase5-screening-backend.md`
