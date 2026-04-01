# DiveEnd

沉浸式论文收集与 AI 翻译阅读工具，当前实现基于 Wails + React + Go。

## 当前已实现

- 三栏桌面布局：左侧设置、中间 DeepStart / DeepRead、右侧论文库
- 本机配置文件持久化：LLM、主题、数据目录等设置保存在用户配置目录
- SQLite 文库与迁移：自动创建默认 `Inbox` 文件夹，兼容旧 `papers` 表升级
- 论文搜索与导入：支持 Semantic Scholar + arXiv 聚合搜索，并导入指定文件夹
- DeepRead 手动章节翻译：保存章节原文、中文翻译与摘要历史

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
