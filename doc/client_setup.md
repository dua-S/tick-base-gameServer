# 文档六：客户端实施报告 (Client)

**项目类型**：Web SPA
**技术栈**：React, Pixi.js, Protobuf.js

## 1. 目录与架构

*   `/src/ui` (React): 处理 DOM 界面。
*   `/src/game` (Pixi): 处理 Canvas 游戏内容。

## 2. 模块细节

### 2.1 界面层 (React)

*   **登录页**：表单提交 -> Gateway `/login` -> 保存 JWT 到 LocalStorage。
*   **大厅页**：
    *   `useEffect` 轮询或手动刷新 Gateway `/rooms` 接口。
    *   点击“创建房间” -> 弹窗设置参数 -> POST `/create` -> 拿到 IP/Port -> 切换路由到 `/game`。
    *   点击“历史战绩” -> 展示表格（数据源：Gateway `/history`）。
*   **游戏内 HUD**：
    *   **左侧蓄力条**：监听 `GameEngine` 的 `chargeProgress` 事件（0.0 - 1.0），动态设置 `div.style.height`。
    *   **底部信息板**：监听 `GameEngine` 的 `stateUpdate` 事件，更新玩家列表（React Virtualized List）。
    *   **结算弹窗**：监听 `gameOver` 事件，全屏覆盖显示胜利者。

### 2.2 游建立 WebSocket 连接。

*   *   加载资源（SpriteSheet）。
    *   创建 `PIXI.Application`。
*   **输入预测 (Prediction)**：
    *   按下 `W` -> 立即移动本地 Container -> 记录输入序列号 `Seq` -> 发送给服务端。
    *   收到服务端 `Snapshot` -> 检查服务端确认的位置与预测位置偏差 -> 若偏差过大则**和解 (Reconcile)**（强行拉回服务端位置）。
*   **蓄力表现**：
    *   **自己**：按下空格 -> 本地计时器启动 -> 渲染光柱变粗/变长动画。
    *   **他人**：收到服务端 `Snapshot.ChargeLevel` -> 根据数值渲染对应状态。
*   **死亡视角**：
    *   当 `isDead = true`，停止渲染自己的 Sprite。
    *   将 `Camera` 绑定到一个虚拟的 `SpectatorPoint` 上，允许 WASD 移动这个点。

## 3. Tick同步与Target Tick计算

### 3.1 服务端方案
服务端采用 **64Hz 传统Tick + 固定2 Tick延迟补偿**：
- Tick周期：15.625ms（64Hz）
- 延迟补偿：所有输入延迟 **2 ticks（31.25ms）**执行
- 输入验证：拒绝超过±2 tick的输入

### 3.2 客户端计算流程

```typescript
class NetworkManager {
    serverTick: number = 0;
    rttTicks: number = 1;  // 默认假设1 tick延迟
    
    // 收到服务端快照时更新
    onSnapshot(snapshot: S2CSnapshot) {
        this.serverTick = snapshot.tick;
        
        // 测量RTT并转换为tick数（向上取整）
        const rttMs = this.measureRTT();
        this.rttTicks = Math.ceil(rttMs / 15.625);
    }
    
    // 发送输入时计算target_tick
    sendInput(input: InputCmd) {
        // target_tick = 服务端当前tick + 网络延迟 + 1 tick缓冲
        const targetTick = this.serverTick + this.rttTicks + 1;
        
        this.send({
            target_tick: targetTick,
            timestamp: Date.now(),
            move: input.move,
            charge: input.charge
        });
    }
}
```

### 3.3 完整同步流程

```
Tick 10: 服务端发送 Snapshot{tick: 10}
         客户端收到，更新 serverTick = 10
         
Tick 11: 客户端发送输入{target_tick: 12}
         (计算: 10 + 1 + 1 = 12，假设RTT≈1 tick)
         
Tick 12: 服务端执行 (12 - 2 = 10，即延迟2 tick后执行)
```

### 3.4 容错处理
- 如果输入被拒绝（target_tick不匹配），增加rttTicks缓冲
- 定期重新测量RTT，适应网络变化

## 4. 开发注意事项

1.  **坐标系同步**：确保服务端和客户端的地图原点一致（通常左上角为 0,0）。
2.  **二进制协议**：使用 `protobufjs` 库在前端加载 `.proto` 文件，发送前 `encode`，接收后 `decode`。
3.  **资源释放**：React 组件卸载（离开游戏页）时，必须调用 `app.destroy()` 并断开 WebSocket，防止内存泄漏。