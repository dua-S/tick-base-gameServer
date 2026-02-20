# 项目技术设计文档 (TDD) v2.0
**项目名称**：BeamArena (代号)
**版本**：2.0

## 1. 技术架构选型 (Technical Stack)

| 组件 | 选型方案 | 说明 |
| :--- | :--- | :--- |
| **前端框架** | **React** | 负责登录、大厅、战绩、HUD（血条/列表）等 UI 界面。 |
| **游戏渲染** | **Pixi.js** | 负责游戏区域 `<canvas>` 的高性能 2D 渲染（地图、角色、光柱）。 |
| **服务端语言** | **Golang** | 高并发游戏逻辑处理。 |
| **通信协议** | **WebSocket + Protobuf** | 游戏局内实时通信，使用 Protobuf 二进制压缩减少带宽。 |
| **API 协议** | **gRPC / HTTP** | 服务间通讯用 gRPC，客户端与 Web 服务器交互用 HTTP。 |
| **数据库** | **MySQL** | 持久化存储（用户、战绩）。 |
| **缓存/Session** | **Redis** | 存取 Session、热点房间数据、分布式锁。 |
| **异步处理** | **Message Queue (MQ)** | (RabbitMQ/Kafka) 游戏服务器将战绩数据推入 MQ，由 Worker 异步写入 DB，避免阻塞游戏 Loop。 |

---

## 2. 核心业务流程 (Core Logic)

### 2.1 游戏流程状态机
1.  **Waiting**: 房间创建，玩家加入。
2.  **Playing**: 游戏开始，所有玩家状态为 `ALIVE`。
3.  **Battle**:
    *   玩家移动、蓄力、攻击。
    *   玩家 HP <= 0 -> 状态变更为 `DEAD` -> 触发 **死亡逻辑**。
    *   检测存活人数 -> 若 `AliveCount == 1` -> 触发 **结算逻辑**。
4.  **Finished**: 展示结算画面（赢家信息），5-10秒后返回大厅。

### 2.2 死亡与观战逻辑 (Death & Spectator)
*   **触发**：当 `Player.HP <= 0`。
*   **状态变更**：
    *   服务端：标记该 Connection/Player 为 `SPECTATOR`。
    *   服务端：将该玩家的关注点坐标（ViewCenter）重置为 `(MapWidth/2, MapHeight/2)`。
*   **客户端表现**：
    *   隐藏自己的角色 Sprite。
    *   UI 显示“你已死亡，进入观战模式”。
    *   **操作变更**：WASD 不再控制角色物理移动，而是发送 `SpectateMove` 包（或复用 Move 包但服务端处理不同），仅改变服务端的 `ViewCenter` 坐标。
*   **视野同步 (AOI)**：服务端根据死者新的 `ViewCenter` 计算九宫格/视野范围，下发该区域内其他存活玩家的状态。

### 2.3 胜负判定 (Win Condition)
*   **检测时机**：每次有玩家死亡时。
*   **判定**：`CurrentPlayers > 1` 且 `AlivePlayers == 1`。
*   **处理**：
    1.  锁定游戏状态，停止所有输入处理。
    2.  广播 `GameEnd` 包，包含 `WinnerID`。
    3.  服务端组装 `MatchRecord`（包含所有玩家K/D/A、时长、赢家），序列化后 Publish 到 MQ。
    4.  MQ Consumer 收到消息，写入 MySQL `match_history` 表。

---

## 3. 通信协议定义 (Protobuf Draft)

为了实现二进制压缩，我们将定义 `.proto` 文件。

```protobuf
syntax = "proto3";
package game;

// 消息类型枚举
enum PacketType {
  HEARTBEAT = 0;
  INPUT = 1;         // 客户端输入
  STATE_UPDATE = 2;  // 服务端状态同步 (64Hz)
  GAME_EVENT = 3;    // 事件 (死亡, 游戏结束)
  JOIN_ROOM = 4;
}

// 客户端 -> 服务端：输入包
message C2SInput {
  int64 timestamp = 1; // 客户端时间戳 (for Sub-tick)
  // 操作类型
  oneof action {
    MoveCmd move = 2;
    ChargeCmd charge = 3; // 开始/结束蓄力
  }
}

message MoveCmd {
  float dx = 1; // -1.0 to 1.0 (X轴方向)
  float dy = 2; // -1.0 to 1.0 (Y轴方向)
  // 如果是死者，这里的 dx/dy 控制摄像机
}

message ChargeCmd {
  bool is_charging = 1; // true=按下, false=松开(发射)
  float angle = 2;      // 瞄准角度 (可选，如果光柱只能上下左右则不需要)
  int32 direction = 3;  // 0:Up, 1:Down, 2:Left, 3:Right
}

// 服务端 -> 客户端：世界快照 (Snapshot)
message S2CStateUpdate {
  int64 tick_id = 1;
  int64 server_time = 2;
  repeated PlayerState players = 3;
  repeated BeamEffect beams = 4; // 当前这一帧存在的特效
}

message PlayerState {
  int32 id = 1;
  float x = 2;
  float y = 3;
  int32 hp = 4;
  int32 max_hp = 5;
  bool is_dead = 6;
  int32 charge_level = 7; // 蓄力程度 (0-100)，用于其他玩家显示特效
  // 注意：如果是接收者自己，蓄力条由本地 Input 预测渲染，服务端数据用于校准
}

message BeamEffect {
  float x = 1;
  float y = 2;
  int32 direction = 3;
  float length = 4;
  float width = 5;
  int32 duration_ms = 6; // 特效持续时间
}

// 游戏事件
message GameEvent {
  enum EventType {
    PLAYER_DEATH = 0;
    GAME_OVER = 1;
  }
  EventType type = 1;
  string message = 2;
  int32 target_id = 3; // 谁死了，或者谁赢了
  // 结算数据
  optional GameResult result = 4;
}

message GameResult {
  int32 winner_id = 1;
  string winner_name = 2;
  // 可以在这里加一个简略的战绩表
}
```

---

## 4. 客户端实现架构 (Client - React + Pixi)

### 4.1 目录结构
```text
/client
  /src
    /api          # HTTP 请求 (Axios)
    /components   # React UI 组件 (Login, Lobby, HistoryTable)
    /game         # 游戏核心逻辑 (非 React 部分)
      /network    # WebSocket Manager, Protobuf Handler
      /engine     # Pixi Application 管理
      /entities   # Player, Beam 类
      /systems    # InputSystem, PredictionSystem
    /pages        # 页面路由 (LoginPage, LobbyPage, GamePage)
    /proto        # 编译后的 proto js/ts 文件
```

### 4.2 界面布局与渲染分离
*   **UI 层 (React DOM)**：覆盖在 Canvas 之上 (`z-index: 10`)。
    *   **左侧 (ChargeBar)**：订阅 React State `selfChargeProgress`。
        *   *优化*：由于蓄力是高频变化，建议直接操作 DOM 或使用 `useRef` 修改 CSS height，避免 React 重绘开销。
    *   **下方 (InfoPanel)**：通过 WebSocket 收到 `STATE_UPDATE` 后，低频更新（如每秒2次或仅数值变化时）React State 来渲染列表。
    *   **结算弹窗**：`GAME_OVER` 事件触发一个全屏的 Modal 组件。
*   **游戏层 (Pixi Canvas)**：
    *   使用 `requestAnimationFrame` 驱动 Pixi 的 Ticker。
    *   **View Logic**:
        *   IF `AmIAlive`: `Camera.position = Lerp(Camera.position, MyPlayer.position)`
        *   IF `AmIDead`: `Camera.position += InputVector * SpectatorSpeed` (由服务端确认位置或客户端预测移动)。

---

## 5. 数据库与 MQ 设计

### 5.1 MQ 消息格式 (JSON)
Topic: `game_results`
```json
{
  "match_id": "uuid-v4",
  "start_time": 1700000000,
  "end_time": 1700000600,
  "winner_id": 1001,
  "room_config": { "map_size": 2000, "max_players": 10 },
  "players": [
    { "uid": 1001, "kills": 3, "rank": 1 },
    { "uid": 1002, "kills": 0, "rank": 2 }
  ]
}
```

### 5.2 数据库表结构 (History)
**Table: `game_records`**
*   `id` (PK, BigInt)
*   `user_id` (FK, index) - 用于快速查询“我的战绩”
*   `match_id` (String) - 同一场比赛多条记录共用一个 ID
*   `is_winner` (Boolean)
*   `kills` (Int)
*   `rank` (Int)
*   `played_at` (Timestamp)

**API: 获取历史战绩**
*   `GET /api/history?page=1&limit=10`
*   Server 查询 DB `WHERE user_id = current_user ORDER BY played_at DESC`。

---

## 6. 关键算法与优化

### 6.1 Sub-tick 排序与执行
服务端在每一帧 (Tick N) 开始时：
1.  收集 `Tick N-1` 到 `Tick N` 之间收到的所有 Input 包。
2.  按照包内的 `timestamp` 对操作进行排序。
3.  **回放执行**：
    *   依次应用操作。
    *   若发生攻击判定，根据 Input 的 `timestamp` 查找历史位置快照（History Buffer）进行碰撞检测。
4.  更新世界状态。

### 6.2 蓄力条同步
*   **客户端**：按下按键立即开始本地计时，蓄力条上涨（预测）。
*   **服务端**：收到 `StartCharge` 包，记录 `ServerStartTime`。
*   **释放时**：
    *   客户端：发送 `Release`。
    *   服务端：计算 `Duration = Now - ServerStartTime`。
    *   *校验*：如果 `Duration` 与客户端声称的差距过大（超过 RTT 抖动范围），则判定为作弊，以服务端时间为准。

---

## 7. 开发路线建议 (Roadmap)

1.  **Phase 1: 基础框架**
    *   搭建 Golang gRPC/WebSocket 基础服务。
    *   React 登录 + 大厅 UI。
    *   Pixi.js 画出方块移动。
2.  **Phase 2: 网络同步 (Hard)**
    *   实现 Protobuf 编解码。
    *   实现 64-tick 循环与状态广播。
    *   客户端平滑插值移动。
3.  **Phase 3: 核心玩法**
    *   实现蓄力、光柱生成、碰撞检测。
    *   实现 HP 扣除与 HUD 更新。
4.  **Phase 4: 游戏流程与优化**
    *   实现死亡视角切换。
    *   实现最后一人胜出判定。
    *   接入 RabbitMQ 与 战绩数据库。
    *   Sub-tick 精准度调优。

---