# Vola 本地 Markdown 知识索引设计

## 背景

用户期望的不是普通文件扫描，而是接近 Karpathy “LLM Wiki” 的个人知识库工作流：

- 原始资料保持不变，作为可信来源。
- LLM 把原材料整理成结构化 Markdown wiki。
- wiki 包含摘要、概念页、反向链接、概念分类和内容间链接。
- `index.md` 负责内容目录，`log.md` 负责时间线记录。
- 后续问答、整理、lint 结果可以继续写回 wiki，让知识库持续变丰富。

参考资料：Karpathy 的公开 gist《LLM Wiki》。

## 当前实现

Vola 已先做本地结构化索引层，入口在“项目文档”页：

- 新增 `POST /api/local/library/index`。
- 扫描本地 Markdown，并跳过疑似敏感文件。
- 解析 Markdown 标题结构。
- 解析普通 Markdown 链接和 `[[WikiLink]]`。
- 解析 frontmatter/inline tags。
- 生成项目、概念、链接关系、反向链接和索引树。
- 前端展示“项目 / 概念 / 链接关系”三类树。
- 右侧展示当前文档摘要、标题结构、概念、链接出去、反向链接。
- 可复制真实文件路径给 Codex。
- 可复制“编译提示词”，交给 Codex/LLM 在 `.vola/index/` 下生成结构化 Markdown。

这一步不自动改写用户原始文件，也不自动写 `.vola/index/`。写入生成内容应由用户确认后执行。

## 下一阶段

下一阶段才是真正的 LLM Wiki 编译器：

- 用户选择项目或文档集合。
- Vola 生成编译计划。
- Codex/LLM 读取源文件。
- 在 `.vola/index/` 下生成：
  - `README.md`
  - `index.md`
  - `log.md`
  - `concepts.md`
  - `backlinks.md`
  - `orphans.md`
  - 独立概念页
- 每次 ingest/query/lint 都追加到 `log.md`。
- Lint 检查孤立页、缺链接、重复概念、过时结论和矛盾说法。

## 产品边界

Vola 的核心仍然是简洁地管理 Skill、MCP、团队共享资料和项目 Markdown。知识索引不应该变成另一个复杂编辑器，它应该服务于一个动作：让用户在使用 Codex 时更快拿到正确上下文。
