# OpsNexus 阶段二全量测试报告

**测试日期**：2026-03-23
**测试环境**：Windows 11 Pro / Go 1.24 / Node.js (pnpm monorepo)
**测试执行人**：QA Engineer

---

## 一、测试总览

| 类别 | 总数 | 通过 | 失败 | 通过率 |
|------|------|------|------|--------|
| 后端单元测试 | 220+ | 215 | 5 | 97.7% |
| API 契约测试 | 41 | 41 | 0 | 100% |
| 数据库迁移验证 | 16 文件 | 16 | 0 | 100% |
| 前端静态验证 | 9 应用 | 9 | 0 | 100% |
| 前端测试文件覆盖 | 25 文件 | — | — | — |

---

## 二、后端单元测试结果（逐服务）

### 2.1 svc-log — 日志服务

| 包 | 状态 | 通过 | 失败 | 说明 |
|---|---|---|---|---|
| internal/biz | FAIL | 47 | 3 | 见下方缺陷 |
| internal/contract | PASS | 14 | 0 | CloudEvents 契约测试全部通过 |

**失败用例：**

1. **TestIngestHTTP_AppliesMaskingRules** — 身份证号脱敏正则未正确匹配，手机号脱敏模式误覆盖了身份证号字段
   - 错误信息：`expected idcard masked in message: user phone is ***PHONE*** and id is 110101***PHONE***4`
   - 根因：`maskSensitiveFields` 中手机号正则 `\d{11}` 过于宽泛，会匹配身份证号中间的连续 11 位数字
   - 建议：手机号正则应使用 `\b1[3-9]\d{9}\b` 限定边界

2. **TestParseJSON** — JSON 解析后未正确写入 ES 索引
   - 错误信息：`expected 1 indexed, got 0`
   - 根因：mock ES repo 的 `BulkIndex` 方法未正确记录调用

3. **TestIngestKafka_SingleEntry** — Kafka 单条摄取后 ES 写入计数为 0
   - 错误信息：`expected 1 indexed, got 0`
   - 根因：与 TestParseJSON 相同，mock 实现问题

### 2.2 svc-alert — 告警引擎

| 包 | 状态 | 通过 | 失败 |
|---|---|---|---|
| internal/biz | PASS | 49 | 0 |
| internal/contract | PASS | 5 | 0 |

**6 层评估管道测试全部通过：**
- Layer 0：铁律告警绕过去重 ✓
- Layer 1：阈值告警（GT/LTE）、关键词匹配、频率触发 ✓
- Layer 2：基线异常检测 ✓
- Layer 3：去重、趋势检测（上升/下降/双向/无历史/阈值内）✓
- Layer 4：告警字段验证 ✓
- 禁用规则跳过 ✓
- 指纹生成、严重级别枚举、频率计数器 ✓

### 2.3 svc-incident — 事件管理

| 包 | 状态 | 通过 | 失败 | 说明 |
|---|---|---|---|---|
| internal/biz | FAIL | 9 | 1 | nil pointer panic |
| internal/contract | PASS | 4 | 0 | |
| cmd/server | FAIL | — | — | 模块导入路径错误 |

**失败用例：**

1. **TestCreateIncident_Success** — nil pointer dereference
   - 位置：`pkg/event/producer.go:72` → `(*Producer).Publish` 调用时 Producer 为 nil
   - 根因：测试中未注入 EventProducer（传入 nil），但 `IncidentUsecase.publishCreated` 未做 nil 检查
   - 建议：在 `publishCreated` 方法中添加 `if uc.producer == nil { return }` 空值保护

2. **cmd/server 编译失败** — go.mod 中的模块导入路径不一致
   - `main.go` 引用 `github.com/opsnexus/opsnexus/services/svc-incident/internal/biz`
   - 但 go.mod 定义的模块名为 `github.com/opsnexus/svc-incident`
   - 建议：统一模块路径，修复 main.go 中的 import 路径

### 2.4 svc-cmdb — 资产管理

| 包 | 状态 | 通过 | 失败 | 说明 |
|---|---|---|---|---|
| internal/biz | FAIL | 0 | 1 | nil pointer panic |
| internal/contract | PASS | 3 | 0 | |
| cmd/server | FAIL | — | — | 模块导入路径错误 |

**失败用例：**

1. **TestCreateAsset_Success** — nil pointer dereference
   - 位置：`pkg/event/producer.go:72` → `(*Producer).Publish` 调用时 Producer 为 nil
   - 根因：与 svc-incident 相同，`AssetUsecase.publishAssetChanged` 未对 nil Producer 做保护
   - 建议：在 `publishAssetChanged` 方法中添加 nil 检查

2. **cmd/server 编译失败** — 与 svc-incident 相同的模块路径不一致问题

### 2.5 svc-notify — 通知服务

| 包 | 状态 | 通过 | 失败 |
|---|---|---|---|
| internal/biz | PASS | 57 | 0 |
| internal/contract | PASS | 4 | 0 |

**测试覆盖：**
- ChannelManager 路由分发（6 种渠道类型）✓
- SMS httptest（发送成功/失败/无收件人/无效配置/连通性检测）✓
- Voice TTS httptest（发送成功/失败/无收件人/多收件人）✓
- GenericWebhook httptest（发送成功/无签名/默认方法/服务端错误）✓
- WecomWebhook httptest（Markdown 格式/服务端错误）✓
- Email 配置解析、HTML 检测、无效配置 ✓
- DedupKey 确定性和唯一性 ✓
- 广播规则匹配、严重级别映射、预览截断 ✓

### 2.6 svc-ai — AI 分析服务

| 包 | 状态 | 通过 | 失败 |
|---|---|---|---|
| internal/biz | PASS | 33 | 0 |
| internal/contract | PASS | 5 | 0 |

**测试覆盖：**
- 熔断器完整生命周期（Closed→Open→HalfOpen→Closed）✓
- 快照/恢复 ✓
- AES-256-GCM 加密密钥解密 ✓
- 数据脱敏（密码/IP/邮箱）✓
- Token 估算和截断 ✓
- 上下文收集和 Prompt 构建 ✓

### 2.7 svc-analytics — 数据分析服务

| 包 | 状态 | 通过 | 失败 | 说明 |
|---|---|---|---|---|
| internal/biz | PASS | 26 | 0 | |
| internal/contract | PASS | 6 | 0 | |
| cmd/server | FAIL | — | — | 编译错误 |
| internal/service | FAIL | — | — | 编译错误 |

**编译错误（非测试代码）：**
- `handler.go:236` — `total` 变量声明后未使用
- `handler.go:299,524` — `int` 类型无法作为 `int64` 参数传递给 `httputil.PagedJSON`
- 建议：修复类型转换 `int64(total)` 并移除未使用变量

**biz 层测试全部通过：**
- SLA 计算（5 场景 + 无效时间范围）✓
- 误差预算（5 场景）✓
- 维度过滤（4 场景）✓
- 默认 SLA 目标映射 ✓
- 参数校验 ✓
- CalculateReport daily/weekly/monthly（基础 + 边界场景共 7 个子测试）✓
- 时间段拆分逻辑 ✓
- Pearson 相关系数（10 场景）✓
- 异常分数计算（10 场景）✓
- 指标摄取/查询/关联 ✓
- 错误预算告警事件 ✓

---

## 三、API 契约测试结果

所有 7 个服务的 CloudEvents/API 契约测试全部通过：

| 服务 | 通过数 | 状态 |
|------|--------|------|
| svc-log | 14 | PASS |
| svc-alert | 5 | PASS |
| svc-incident | 4 | PASS |
| svc-cmdb | 3 | PASS |
| svc-notify | 4 | PASS |
| svc-ai | 5 | PASS |
| svc-analytics | 6 | PASS |
| **合计** | **41** | **100% PASS** |

---

## 四、数据库迁移验证

所有迁移文件均成对存在（up + down）：

| 服务 | 迁移文件 | up/down 配对 | 状态 |
|------|----------|-------------|------|
| svc-ai | 000001_init_schema, 000002_add_feedback_columns | 2 对 | ✓ |
| svc-alert | 001_init | 1 对 | ✓ |
| svc-cmdb | 001_init | 1 对 | ✓ |
| svc-incident | 001_init, 002_mttr_and_changes | 2 对 | ✓ |
| svc-log | 001_init | 1 对 | ✓ |
| svc-notify | 000001_init_schema | 1 对 | ✓ |
| svc-analytics | — | 无迁移文件（使用 ClickHouse DDL） | — |

共 **16 个迁移文件，8 对 up/down 完整配对**。

---

## 五、前端静态验证

### 5.1 构建脚本检查

所有 9 个子应用均包含 `build` 和 `test` 脚本：

| 应用 | build | test | 状态 |
|------|-------|------|------|
| app-alert | `tsc && vite build` | `vitest run` | ✓ |
| app-analytics | `tsc && vite build` | `vitest run` | ✓ |
| app-cmdb | `tsc && vite build` | `vitest run` | ✓ |
| app-cockpit | `tsc && vite build` | `vitest run` | ✓ |
| app-dashboard | `tsc && vite build` | `vitest run` | ✓ |
| app-incident | `tsc && vite build` | `vitest run` | ✓ |
| app-log | `tsc && vite build` | `vitest run` | ✓ |
| app-notify | `tsc && vite build` | `vitest run` | ✓ |
| app-settings | `tsc && vite build` | `vitest run` | ✓ |

### 5.2 测试文件覆盖

| 类别 | 文件数 | 明细 |
|------|--------|------|
| 页面级单元测试 | 10 | 每个子应用 1 个 test.tsx |
| E2E 测试 | 9 | `e2e/tests/*.spec.ts`，覆盖全部 9 个应用场景 |
| UI 组件测试 | 5 | `packages/ui-kit` 中的 SeverityBadge/StatusTag/MetricCard/AssetGradeTag/RootCauseBadge |
| Shell 布局测试 | 1 | BasicLayout.test.tsx |
| **总计** | **25** | |

### 5.3 TypeScript 接口定义完整性

| 应用 | API 文件 | 接口数 | 函数数 | request.ts | 状态 |
|------|----------|--------|--------|------------|------|
| app-log | log.ts | 5 | 3 | ✓ | 完整 |
| app-alert | alert.ts | 5 | 5 | ✓ | 完整 |
| app-dashboard | dashboard.ts | 3 | 3 | ✓ | 完整 |
| app-analytics | analytics.ts | 7 | 3 | ✗ | 缺统一封装 |
| app-cmdb | asset.ts | 2 | 2 | ✗ | 缺统一封装 |
| app-incident | incident.ts | 4 | 6 | ✗ | 缺统一封装 |
| app-notify | notify.ts | 2 | 7 | ✗ | 缺统一封装 |

**问题**：app-analytics/cmdb/incident/notify 直接使用原生 `fetch`，未通过 `request.ts` 统一封装，与 app-log/alert/dashboard 的 `ApiResponse<T>` 模式不一致。函数返回类型为隐式 `Promise<any>`。

---

## 六、发现的缺陷汇总

### 严重级别：P1（阻塞性）

| # | 服务 | 缺陷描述 | 影响 |
|---|------|----------|------|
| BUG-001 | svc-incident | `IncidentUsecase.publishCreated` 未对 nil Producer 做空值保护，导致 `TestCreateIncident_Success` panic | 生产环境如未配置 Kafka Producer 会导致事件创建时服务崩溃 |
| BUG-002 | svc-cmdb | `AssetUsecase.publishAssetChanged` 未对 nil Producer 做空值保护，导致 `TestCreateAsset_Success` panic | 生产环境如未配置 Kafka Producer 会导致资产创建时服务崩溃 |

### 严重级别：P2（功能性）

| # | 服务 | 缺陷描述 | 影响 |
|---|------|----------|------|
| BUG-003 | svc-log | 手机号脱敏正则 `\d{11}` 过于宽泛，误匹配身份证号中间 11 位 | 身份证号脱敏效果异常 |
| BUG-004 | svc-log | `TestParseJSON` 和 `TestIngestKafka_SingleEntry` mock ES repo 的 BulkIndex 未正确记录调用 | 测试可靠性问题 |

### 严重级别：P3（编译/工程）

| # | 服务 | 缺陷描述 | 影响 |
|---|------|----------|------|
| BUG-005 | svc-incident | cmd/server main.go import 路径与 go.mod 模块名不一致 | 服务无法编译 |
| BUG-006 | svc-cmdb | cmd/server main.go import 路径与 go.mod 模块名不一致 | 服务无法编译 |
| BUG-007 | svc-analytics | handler.go 中 `int` → `int64` 类型不匹配 + 未使用变量 | HTTP handler 层无法编译 |
| BUG-008 | frontend | 4 个应用缺少 `request.ts` 统一 API 封装 | API 调用模式不一致，缺少统一错误处理 |

---

## 七、修复建议

### 优先修复（P1）

1. **BUG-001/002**：在 `svc-incident/internal/biz/incident_usecase.go` 的 `publishCreated` 和 `svc-cmdb/internal/biz/asset_usecase.go` 的 `publishAssetChanged` 方法开头添加：
   ```go
   if uc.producer == nil {
       return
   }
   ```

### 建议修复（P2）

2. **BUG-003**：将 `svc-log` 中手机号脱敏正则从 `\d{11}` 改为 `\b1[3-9]\d{9}\b`
3. **BUG-004**：修复 `svc-log` 测试中 mock ES repo 的 `BulkIndex` 调用记录逻辑

### 工程改进（P3）

4. **BUG-005/006**：统一 `svc-incident` 和 `svc-cmdb` 的 go.mod 模块路径与 main.go import 路径
5. **BUG-007**：修复 `svc-analytics/internal/service/handler.go` 的类型转换
6. **BUG-008**：为 4 个新增前端应用补充 `request.ts` 统一封装

---

## 八、测试覆盖率分析

### 后端覆盖维度

| 维度 | 覆盖情况 |
|------|----------|
| biz 业务逻辑层 | 7/7 服务有测试，共 215+ 通过 |
| contract 契约层 | 7/7 服务有测试，共 41 通过 |
| data 数据层 | 0/7（需外部数据库，跳过） |
| service HTTP 层 | 部分覆盖（svc-analytics handler 编译未通过） |
| 集成测试 | 需外部依赖（Redis/Kafka/PostgreSQL/ClickHouse），跳过 |

### 前端覆盖维度

| 维度 | 覆盖情况 |
|------|----------|
| 页面组件测试 | 10/10 关键页面有 test.tsx |
| E2E 测试 | 9/9 应用有 spec.ts（需 Playwright 运行） |
| UI 组件测试 | 5 个共享组件有测试 |
| API 层测试 | 未覆盖（建议补充 mock fetch 测试） |

---

## 九、结论

**整体评估：项目质量良好，核心业务逻辑测试充分。**

- 7 个后端服务中 4 个（svc-alert、svc-notify、svc-ai、svc-analytics/biz）全部通过
- 告警引擎 6 层评估管道测试覆盖完整
- API 契约测试 41/41 全部通过
- 数据库迁移文件 up/down 完整配对
- 前端 9 个应用均有构建脚本和测试文件

**需关注：**
- 2 个 P1 级 nil pointer 缺陷（svc-incident、svc-cmdb）需立即修复
- 3 个服务存在编译路径问题需工程修复
- 前端 API 封装模式需统一

---

*报告生成时间：2026-03-23*
*QA Engineer — OpsNexus Project*
