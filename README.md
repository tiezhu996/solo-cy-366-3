# 电竞馆上机管理系统（Esports Bar）

面向电竞馆/网咖门店的一站式运营工具：机位/包厢实时状态看板、会员充值与时长包、机位预约与续费、上机时长排行榜、电竞赛事报名与战队管理。

## 快速启动（Docker Compose，推荐）

```bash
# 在项目根目录执行
docker compose up -d --build
```

启动后访问：

| 入口 | 地址 |
| --- | --- |
| 前端页面 | http://localhost:28506 |
| 后端健康检查 | http://localhost:29506/healthz |
| 后端 API | http://localhost:29506/api/v1 |

演示账号（由后端启动时自动初始化）：

| 账号 | 密码 | 角色 |
| --- | --- | --- |
| admin | admin123456 | 管理员 |
| staff | staff123456 | 店员（扫码开机） |
| member | member123456 | 会员（生成开机码） |

## 主要功能

1. **机位/包厢实时状态看板**：网格/列表展示机位实时状态（空闲/使用中/故障/预约），按区域筛选，支持 WebSocket 实时推送（`/api/v1/ws/stations`）。
2. **会员充值与时长包**：会员充值余额、购买 10 小时/30 小时/月卡，消费时优先扣除时长包余额，不足时扣余额。
3. **机位预约、动态开机码与续费**：会员预约指定机位与时段，到店后在「我的开机码」生成 5 分钟有效的 6 位动态开机码（二维码），店员在「扫码开机」扫码校验预约、机位、有效期与角色，校验通过即占用机位、创建上机记录、按预约时长扣减时长包/余额并返回上机详情；上机过程可续费延长时长。
4. **上机时长排行榜**：按日/周/月统计会员累计上机时长，支持按游戏类型（LOL/CSGO/王者荣耀）筛选。
5. **赛事报名与战队管理**：门店发布电竞赛事，玩家个人/战队报名，系统自动抽签分组，记录比赛结果与战绩。

## 技术栈

| 层 | 技术栈 |
| --- | --- |
| 前端 | Vue 3 + TypeScript |
| 后端 | Go 1.22 + Gin + GORM |
| 数据库 | MySQL 8.0 |
| 缓存/限流 | Redis 7 |
| 实时通信 | gorilla/websocket |
| 认证 | JWT + RBAC |
| 日志 | log/slog |
| 参数校验 | go-playground/validator/v10 |
| 接口文档 | 见下方 API 清单（README 全量列出） |

## 项目目录结构

```
.
├── backend/
│   ├── cmd/server/main.go          # 入口：装配依赖、启动服务
│   ├── internal/
│   │   ├── config/                 # 配置加载
│   │   ├── database/               # MySQL/Redis 连接与种子数据
│   │   ├── model/                  # 每个实体一个文件
│   │   ├── dto/                    # 每个实体一个 DTO 文件
│   │   ├── repository/             # 每个实体一个 repository 文件
│   │   ├── service/                # 每个实体一个 service 文件
│   │   ├── handler/                # 每个实体一个 handler 文件
│   │   ├── router/                 # 每个实体一个路由注册文件
│   │   ├── middleware/             # auth/rbac/audit/request_id/error_handler/rate_limit/logger
│   │   ├── constants/              # 枚举、错误码、日志模板、文案
│   │   └── util/                   # jwt、logger、formatters、app_error 等
│   ├── pkg/response/               # 统一响应封装
│   ├── migrations/001_init.sql     # 参考建表脚本
│   ├── Dockerfile
│   └── go.mod
├── frontend/
│   ├── src/api/                    # 每个实体一个 API 文件
│   ├── src/components/             # StatusBadge / EmptyState / DataTable / ConfirmDialog / QrCodeCard / BootResultPanel
│   ├── src/pages/                  # 每个模块一个页面（BootCode 我的开机码 / ScanBoot 扫码开机）
│   ├── src/stores/                 # auth / station / reservation / tournament
│   ├── src/hooks/                  # useAuth / usePagination
│   ├── src/utils/                  # request / format
│   ├── src/constants/              # 与后端对应的枚举
│   ├── Dockerfile
│   └── nginx.conf
├── database/init.sql               # MySQL 初始化脚本
├── docker-compose.yml
├── .env
├── .env.example
└── README.md
```

## 环境变量说明

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `COMPOSE_PROJECT_NAME` | Compose 项目名，决定容器名前缀 | `esportsbar` |
| `DB_NAME` | 数据库名 | `esportsbar_db` |
| `DB_USER` | 数据库用户 | `esportsbar_user` |
| `DB_PASSWORD` | 数据库密码 | `esportsbar_pwd` |
| `JWT_SECRET` | JWT 签名密钥（生产必须修改） | `change_me_to_a_long_random_string` |
| `JWT_EXPIRE_SEC` | JWT 有效期（秒） | `86400` |
| `FRONTEND_PORT` | 前端宿主机映射端口 | `28506` |
| `BACKEND_PORT` | 后端宿主机映射端口 | `29506` |
| `DB_PORT` | MySQL 宿主机映射端口 | `44005` |
| `REDIS_PORT` | Redis 宿主机映射端口 | `46305` |
| `LOG_LEVEL` | 后端日志级别（debug/info/warn/error） | `info` |

## API 清单（前缀 /api/v1，统一响应 { "code": 0, "message": "ok", "data": ... }）

### 认证

| 方法 | 路径 | 说明 | 权限 |
| --- | --- | --- | --- |
| POST | /auth/register | 会员注册 | 公开 |
| POST | /auth/login | 登录（返回 JWT） | 公开 |
| GET | /auth/profile | 当前用户信息 | 登录 |

### 用户

| 方法 | 路径 | 说明 | 权限 |
| --- | --- | --- | --- |
| GET | /users | 用户分页列表 | admin/staff |
| GET | /users/:id | 用户详情 | admin/staff |
| POST | /users | 创建用户 | admin |
| PUT | /users/:id | 更新用户 | admin |
| DELETE | /users/:id | 删除用户 | admin |

### 机位

| 方法 | 路径 | 说明 | 权限 |
| --- | --- | --- | --- |
| GET | /stations | 机位分页列表（区域/状态筛选） | 登录 |
| GET | /stations/all | 全部机位（看板） | 登录 |
| GET | /stations/:id | 机位详情 | 登录 |
| POST | /stations | 创建机位 | admin/staff |
| PUT | /stations/:id | 更新机位 | admin/staff |
| PUT | /stations/:id/status | 机位状态流转 | admin/staff |
| DELETE | /stations/:id | 删除机位 | admin |

### 充值与时长包

| 方法 | 路径 | 说明 | 权限 |
| --- | --- | --- | --- |
| POST | /recharges | 会员充值 | admin/staff |
| GET | /recharges/mine | 我的充值记录 | 登录 |
| POST | /recharges/packages | 购买时长包 | 登录 |
| GET | /recharges/packages/orders | 我的时长包订单 | 登录 |
| GET | /packages | 时长包分页列表 | 登录 |
| GET | /packages/active | 在售时长包 | 登录 |
| GET | /packages/:id | 时长包详情 | 登录 |
| POST | /packages | 创建时长包 | admin/staff |
| PUT | /packages/:id | 更新时长包 | admin/staff |
| DELETE | /packages/:id | 删除时长包 | admin |

### 预约

| 方法 | 路径 | 说明 | 权限 |
| --- | --- | --- | --- |
| GET | /reservations | 预约分页列表 | 登录 |
| POST | /reservations | 创建预约 | 登录 |
| POST | /reservations/:id/confirm | 确认预约 | admin/staff |
| POST | /reservations/:id/cancel | 取消预约 | 登录 |
| POST | /reservations/:id/checkin | 到店开机 | admin/staff |

### 上机记录

| 方法 | 路径 | 说明 | 权限 |
| --- | --- | --- | --- |
| GET | /sessions | 上机记录分页列表 | 登录 |
| GET | /sessions/rank | 上机时长排行榜（day/week/month + game_type） | 登录 |
| POST | /sessions | 开机上机 | 登录 |
| POST | /sessions/:id/renew | 续费 | 登录 |
| POST | /sessions/:id/end | 下机结算 | 登录 |

### 动态开机码（扫码开机）

会员到店后基于「已确认」预约生成动态开机码，店员扫码核销完成开机。一个服务方法 `SessionService.ChargeWithinTx` 同时被**扫码开机、续费、下机超时结算**三处复用（时长包优先、余额兜底）。

| 方法 | 路径 | 说明 | 权限 |
| --- | --- | --- | --- |
| POST | /boot-codes | 会员生成动态开机码（同预约旧码自动作废，5 分钟有效） | member |
| GET | /boot-codes/mine | 会员查询当前有效开机码（可带 `reservation_id`） | member |
| POST | /boot-codes/verify | 店员扫码校验并开机，返回上机详情（扣时长包/余额） | admin/staff |
| GET | /boot-codes | 开机码核销记录分页（可按 status/reservation_id/user_id 筛选） | admin/staff |

**核销事务（`BootCodeService.Verify`，单事务 + `SELECT ... FOR UPDATE`）校验顺序与失败码**：

| 失败场景 | 错误码 | HTTP |
| --- | --- | --- |
| 开机码不存在/非法 | `40010` CodeBootNotFound | 404 |
| 开机码超过 5 分钟有效期 / 已作废 | `40011` CodeBootExpired | 409 |
| 开机码已核销、预约已开机（重复扫码） | `40012` CodeBootUsed | 409 |
| 扫码机位与预约机位不一致、预约已取消 | `40013` CodeBootMismatch | 409 |
| 预约已过结束时间、未在到店时间窗（提前 15 分钟）内 | `40014` CodeBootWindow | 409 |
| 机位使用中 / 故障 / 已有进行中会话 | `40004`/`40005`/`40008` | 409 |
| 非会员角色的码、会员账户停用、非店员扫码（RBAC） | `40300` CodeForbidden | 403 |
| 时长包与余额均不足 | `40006` CodeInsufficient | 409（事务整体回滚，机位不被占用） |

### 赛事

| 方法 | 路径 | 说明 | 权限 |
| --- | --- | --- | --- |
| GET | /tournaments | 赛事分页列表 | 登录 |
| GET | /tournaments/:id | 赛事详情 | 登录 |
| POST | /tournaments | 创建赛事 | admin/staff |
| PUT | /tournaments/:id | 更新赛事 | admin/staff |
| DELETE | /tournaments/:id | 删除赛事 | admin |
| POST | /tournaments/:id/register | 赛事报名（solo/team） | 登录 |
| GET | /tournaments/:id/registrations | 报名列表 | 登录 |
| POST | /tournaments/:id/draw | 自动抽签分组 | admin/staff |
| GET | /tournaments/:id/matches | 比赛场次 | 登录 |
| GET | /teams/mine | 我的战队 | 登录 |
| POST | /teams | 创建战队 | 登录 |
| POST | /matches/:id/result | 提交比赛结果 | admin/staff |

### 审计与看板

| 方法 | 路径 | 说明 | 权限 |
| --- | --- | --- | --- |
| GET | /audits | 操作审计日志分页 | admin/staff |
| GET | /dashboard/summary | 看板汇总统计 | 登录 |
| GET | /ws/stations | 机位状态 WebSocket 实时推送 | 登录 |

> 复用关系标注：`/stations/all` 与看板页面复用 `StationService.ListAll`/`StationRepository.ListAll`；`/packages/active` 与购买页复用 `TimePackageService.ListActive`；`/sessions/rank` 与 `/sessions` 复用 `SessionService` 的仓储查询能力；`/reservations` 列表与 `DashboardService` 均复用 `StationRepository`；**扫码开机 `/boot-codes/verify`、`/sessions/:id/renew`、`/sessions/:id/end` 三个接口复用同一个计费方法 `SessionService.ChargeWithinTx`（时长包优先、余额兜底）**。

## curl 调用示例

```bash
# 1. 健康检查
curl -sS http://localhost:29506/healthz

# 2. 登录获取 JWT
TOKEN=$(curl -sS -X POST http://localhost:29506/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123456"}' | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['token'])")

# 3. 带 JWT 请求头访问列表
curl -sS http://localhost:29506/api/v1/stations?page=1\&page_size=5 -H "Authorization: Bearer $TOKEN"

# 4. 创建预约
curl -sS -X POST http://localhost:29506/api/v1/reservations \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"station_id":1,"start_time":"2026-08-17T10:00:00+08:00","end_time":"2026-08-17T12:00:00+08:00"}'

# 5. 购买时长包
curl -sS -X POST http://localhost:29506/api/v1/recharges/packages \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"package_id":1,"payment_method":"balance"}'

# 6. 上机时长排行榜
curl -sS "http://localhost:29506/api/v1/sessions/rank?period=week&limit=10" -H "Authorization: Bearer $TOKEN"

# 7. 会员生成动态开机码（需用 member/member123456 的会员 JWT）
curl -sS -X POST http://localhost:29506/api/v1/boot-codes \
  -H "Authorization: Bearer $MEMBER_TOKEN" -H "Content-Type: application/json" \
  -d '{"reservation_id":1}'

# 8. 店员扫码核销开机（需用 staff/staff123456 的店员 JWT）
curl -sS -X POST http://localhost:29506/api/v1/boot-codes/verify \
  -H "Authorization: Bearer $STAFF_TOKEN" -H "Content-Type: application/json" \
  -d '{"code":"123456"}'

# 9. 店员查询核销记录
curl -sS "http://localhost:29506/api/v1/boot-codes?page=1&page_size=10" -H "Authorization: Bearer $STAFF_TOKEN"
```

## 本地开发方式

后端（Go 1.22+）：

```bash
cd backend
go mod tidy
go run ./cmd/server
# 需自行准备 MySQL（默认连接 127.0.0.1:44005）与 Redis（127.0.0.1:46305）
```

后端构建与测试：

```bash
cd backend
go build ./...
go vet ./...
go test ./...
# 扫码开机闭环集成测试（httptest + 真实 Gin 路由/JWT/RBAC + 内存 SQLite，
# 每个用例自建独立数据、无落盘残留，可重复连续运行）
go test ./test/integration/ -v -count=1
```

> 集成测试 `backend/test/integration/` 覆盖：有效开机码开机（机位占用/预约 checked_in/会话留存/扣费/码核销）、重复扫码拒绝、过期码与作废旧码拒绝、扫码机位与预约机位不一致拒绝、会员越权为他人预约生成码、店员/管理员/会员/未登录四种角色的生成与核销权限矩阵、到店时间窗、余额不足事务整体回滚、时长包优先扣减、机位占用拒绝。失败信息以 `[METHOD /api/v1/...]` 前缀标明接口、以 `[DB 表/ID]` 标明落库断言，便于区分接口层还是业务/数据层问题。

前端（Node 18+）：

```bash
cd frontend
npm config set registry https://registry.npmmirror.com
npm install
npm run dev     # 开发服务器 http://localhost:28506，/api 代理到 29506
npm run build
npm test        # Vitest：开机码载荷解析、扫码页面角色守卫、二维码短码显示回归
```

## Docker 部署说明

- 端口映射：前端 `28506:80`、后端 `29506:8080`、MySQL `44005:3306`、Redis `46305:6379`。
- 数据卷：`db_data`（MySQL 数据）、`redis_data`（Redis 数据）为命名卷，删除容器不丢数据。
- 健康检查：db（mysqladmin ping）、backend（/healthz），backend 等待 db/redis healthy 后再启动。
- 常用命令：
  - 查看状态：`docker compose ps`
  - 查看日志：`docker compose logs -f backend`
  - 停止并清理：`docker compose down -v --remove-orphans`（-v 会同时删除数据卷，谨慎使用）
- 常见问题：
  - 端口冲突：修改 `.env` 中对应端口后重新 `docker compose up -d`。
  - 首次启动较慢：需拉取镜像并构建前后端。
  - 修改 JWT_SECRET 后所有旧 token 失效，需重新登录。

## 枚举出现位置清单

### 机位状态（idle / using / fault / reserved）

| 端 | 文件 |
| --- | --- |
| 后端 | `backend/internal/constants/enums.go`（定义）、`backend/internal/model/station.go`（模型默认值）、`backend/internal/dto/station_dto.go`（handler 校验 oneof）、`backend/internal/service/station_service.go`（状态机 allowedStationTransition）、`backend/internal/util/formatters.go`（StatusText）、`backend/internal/constants/error_codes.go`（CodeStationBusy/Fault）、`backend/internal/constants/log_templates.go`（station_status_change 模板）、`backend/internal/repository/station_repository.go`（筛选） |
| 前端 | `frontend/src/constants/index.ts`（STATION_STATUS/TEXT/TYPE）、`frontend/src/components/StatusBadge.vue`、`frontend/src/pages/Stations.vue`（筛选与徽标）、`frontend/src/pages/Dashboard.vue`（看板状态展示） |

### 预约状态（pending / confirmed / checked_in / completed / cancelled）

| 端 | 文件 |
| --- | --- |
| 后端 | `backend/internal/constants/enums.go`（定义）、`backend/internal/model/reservation.go`、`backend/internal/dto/reservation_dto.go`（oneof 校验）、`backend/internal/service/reservation_service.go`（状态机 Confirm/Cancel/CheckIn）、`backend/internal/util/formatters.go`（StatusText）、`backend/internal/constants/error_codes.go`（CodeReservation）、`backend/internal/constants/log_templates.go`（reservation_* 模板）、`backend/internal/repository/reservation_repository.go`（CountConflict 状态集合） |
| 前端 | `frontend/src/constants/index.ts`（RESERVATION_STATUS/TEXT/TYPE）、`frontend/src/components/StatusBadge.vue`、`frontend/src/pages/Reservations.vue`（筛选与操作按钮显隐） |

### 赛事状态（draft / open / ready / finished）

| 端 | 文件 |
| --- | --- |
| 后端 | `backend/internal/constants/enums.go`、`backend/internal/model/tournament.go`、`backend/internal/dto/tournament_dto.go`（oneof）、`backend/internal/service/tournament_service.go`（Register/DrawGroups 状态机）、`backend/internal/util/formatters.go`（StatusText）、`backend/internal/constants/error_codes.go`（CodeTournament）、`backend/internal/constants/log_templates.go`（tournament_* 模板） |
| 前端 | `frontend/src/constants/index.ts`（TOURNAMENT_STATUS/TEXT/TYPE）、`frontend/src/components/StatusBadge.vue`、`frontend/src/pages/Tournaments.vue`（按钮显隐） |

### 动态开机码状态（active / used / expired / cancelled）

| 端 | 文件 |
| --- | --- |
| 后端 | `backend/internal/constants/enums.go`（BootCode* 定义 + `BootCodeTTLMinutes`）、`backend/internal/model/boot_code.go`（模型默认值）、`backend/internal/dto/boot_code_dto.go`（oneof 校验）、`backend/internal/service/boot_code_service.go`（状态机 `checkBootCodeStatus`：active→used/expired/cancelled）、`backend/internal/util/formatters.go`（StatusText 已核销/已失效）、`backend/internal/constants/error_codes.go`（CodeBootNotFound 40010 / CodeBootExpired 40011 / CodeBootUsed 40012 / CodeBootMismatch 40013 / CodeBootWindow 40014）、`backend/internal/handler/helpers.go` 与 `backend/internal/middleware/error_handler.go`（错误码→HTTP 映射两处）、`backend/internal/constants/log_templates.go`（boot_code_generate_ok/revoke_ok/verify_ok/verify_fail/consume_ok）、`backend/internal/constants/messages.go`（MsgBootCodeGenerateOK/VerifyOK）、`backend/internal/repository/boot_code_repository.go`（状态筛选/作废/过期标记） |
| 前端 | `frontend/src/constants/index.ts`（BOOT_CODE_STATUS/TEXT/TYPE + TTL）、`frontend/src/components/StatusBadge.vue`（kind=boot）、`frontend/src/pages/BootCode.vue`（状态徽标与倒计时）、`frontend/src/pages/ScanBoot.vue`（核销记录徽标）、`frontend/src/api/bootCode.ts`（类型与请求） |

### 用户角色（admin / staff / member）

| 端 | 文件 |
| --- | --- |
| 后端 | `backend/internal/constants/roles.go`、`backend/internal/model/user.go`、`backend/internal/dto/user_dto.go`（oneof）、`backend/internal/middleware/rbac.go`（角色鉴权）、`backend/internal/router/*.go`（路由 RBAC 组合，含 `router/boot_code.go`：生成/查询限 member、核销/记录限 staff/admin）、`backend/internal/util/formatters.go`（RoleText）、`backend/internal/constants/log_templates.go`（user_* 模板） |
| 前端 | `frontend/src/constants/index.ts`（USER_ROLE/TEXT）、`frontend/src/hooks/useAuth.ts`（isAdmin/isStaff/isMember/isStaffOrAdmin）、`frontend/src/router/index.ts`（meta.roles 路由守卫）、`frontend/src/pages/Stations.vue`/`Reservations.vue`/`Recharge.vue`/`Tournaments.vue`/`BootCode.vue`/`ScanBoot.vue`/`Dashboard.vue`（按钮与入口显隐） |

### 游戏类型（lol / csgo / kog / other）与支付方式（balance / cash / wechat / alipay）

| 端 | 文件 |
| --- | --- |
| 后端 | `backend/internal/constants/enums.go`、`backend/internal/dto/session_dto.go`（oneof）、`backend/internal/dto/tournament_dto.go`（oneof）、`backend/internal/model/session.go`、`backend/internal/service/session_service.go`（defaultGameType）、`backend/internal/util/formatters.go`（GameTypeText） |
| 前端 | `frontend/src/constants/index.ts`（GAME_TYPE/TEXT、PAYMENT_METHOD/TEXT）、`frontend/src/pages/Sessions.vue`、`frontend/src/pages/Recharge.vue`、`frontend/src/pages/Tournaments.vue` |

## 设计说明

- 分层依赖严格单向：handler → service → repository → model，构造器注入，无反向引用。
- 多步写操作均放入 service 事务（`gorm.DB.Transaction`）；并发场景使用 `SELECT ... FOR UPDATE`（`repository/common.go` 的 `clauseLocking`），如余额扣减、机位状态流转、预约冲突校验、**扫码核销时对开机码/预约/机位/会员四行加行锁**，扣费失败整体回滚（机位不被占用、码保持 active）。
- 扫码开机闭环：会员 `BootCode.vue` 生成二维码（`qrcode`，5 分钟倒计时）→ 店员 `ScanBoot.vue` 摄像头扫码（`html5-qrcode`，摄像头不可用/非 HTTPS 环境时自动降级为手动输入 6 位码）→ `BootCodeService.Verify` 单事务校验码/预约/机位/有效期/角色 → 占用机位（idle/reserved→using）、预约（confirmed→checked_in）、创建 active 会话、按预约剩余时长扣时长包/余额、核销开机码（active→used 并回填 session_id/店员）、返回 `BootSessionDetail` 上机详情。
- 横切关注点：JWT + RBAC（`middleware/auth.go`、`middleware/rbac.go`、`util/jwt.go`、前端 `router/index.ts` 的 meta.roles 守卫）、操作审计（`middleware/audit.go` + `audit_logs` 表 + 审计页面）、全局错误处理与请求追踪（`middleware/request_id.go`、`middleware/error_handler.go`、`util/app_error.go`、`constants/error_codes.go`）。
- 共享组件：`StatusBadge`（新增 kind=boot）、`EmptyState`、`DataTable`、`ConfirmDialog`、`QrCodeCard`（会员码与核销结果复用）、`BootResultPanel`（上机详情结果卡）；共享 hooks/utils：`useAuth`（isMember/isStaff）、`usePagination`、`request.ts`、`format.ts`。
- 严禁合并职责到单一文件：每个实体按 model / dto / repository / service / handler / router / constants 拆分，前端按 api / stores / pages / components 拆分。
- 状态机跨多处定义（屎山耦合设计）：新增机位/预约/赛事状态需同步修改后端 constants、DTO 校验、service 状态机、formatters、错误码、日志模板与前端 constants、StatusBadge、页面按钮显隐等 ≥10 处文件。

## License

MIT
