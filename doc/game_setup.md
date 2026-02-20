# 文档五：Game 服务实施报告 (Game Service)

**服务名称**：game-service
**职责**：游戏核心逻辑，WebSocket 接入。
**端口**：HTTP/WS :9003 (支持多实例部署)

## 1. 核心架构：64-tick 延迟补偿主循环

使用 Golang 的 `time.Ticker` 实现固定频率循环，配合固定2 tick延迟补偿确保公平性。

```go
const (
    TickRate          = 64
    DelayCompensation = 2  // 固定2 tick延迟
)

func (r *Room) GameLoop() {
    currentTick := r.CurrentTick
    executionTick := currentTick - DelayCompensation  // 延迟执行
    
    // 1. 处理延迟补偿后的输入
    r.ProcessInputs(executionTick)
    
    // 2. 物理更新 (使用DeltaTime)
    r.UpdatePhysics()
    
    // 3. 游戏规则判定
    r.CheckRules()
    
    // 4. 广播快照
    r.BroadcastSnapshot()
    
    r.CurrentTick++
}
```

## 2. 延迟补偿机制详解

### 2.1 输入处理流程
1. 客户端发送输入时附带`target_tick`（客户端当前tick + 1）
2. 服务端收到输入后存入InputQueue
3. 每个tick只处理`target_tick == executionTick`的输入
4. 丢弃超过`AcceptableLagTicks(2)`的过期输入

### 2.2 输入验证
```go
func (r *Room) ValidateInput(input *pb.C2SInput) bool {
    diff := input.TargetTick - r.CurrentTick
    // 接受范围: [-2, 0] tick
    return diff >= -2 && diff <= 0
}
```

### 2.3 物理计算
所有移动使用固定`DeltaTime(0.015625s)`计算，确保一致性：
```go
p.X += input.Move.Dx * p.Speed * DeltaTime
p.Y += input.Move.Dy * p.Speed * DeltaTime
```

## 3. 输入缓冲与排序

*   **输入缓冲与排序**：
    *   客户端发送输入时附带`target_tick`，服务端按tick对齐处理
    *   使用固定2 tick延迟补偿，确保所有客户端输入在相同tick执行
    *   每个tick只处理`target_tick == executionTick`的输入，丢弃过期输入
*   **蓄力光柱 (Beam)**：
    *   **Start**: 记录 `ChargeStartTime`。
    *   **Release**: 计算 `Damage = Base + (Now - StartTime) * Rate`。
    *   **Collision**: 将光柱视为旋转矩形 (OBB)，检测与其他玩家的圆形/矩形碰撞盒是否相交。
*   **死亡与观战**：
    *   当 `HP <= 0`：设置 `State = DEAD`，将该连接标记为 `Spectator`。
    *   观战移动：处理死者的移动包时，不再改变物理坐标，只改变 `CameraCenter`，用于计算 AOI（发送哪里的视野给在这个死者）。
*   **视野管理 (AOI)**：
    *   简单的实现：将地图网格化。向玩家发送其所在格子及周围8格的数据。
*   **胜负结算**：
    *   当 `ActivePlayers == 1`：游戏结束。
    *   组装 `GameResult` 结构体，序列化为 JSON。
    *   调用 MQ Producer，Publish 到 `game_result_queue`。

## 4. 通信协议

*   **WebSocket Path**: `/ws?room_id=...&token=...`
*   **Payload**: Protobuf 二进制流 (定义见 V2.0 设计文档)。

---