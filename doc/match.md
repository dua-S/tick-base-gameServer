# 文档四：Match 服务实施报告 (Match Service)

**服务名称**：match-service
**职责**：房间管理，调度器。
**端口**：gRPC :9002
**存储**：Redis

## 1. 核心功能

*   **房间列表管理**：
    *   使用 Redis Hash 存储房间信息：`room:{id}:info` -> `{ "map": 2000, "players": 5/10, "ip": "..." }`。
    *   使用 Redis Set 存储房间列表：`rooms:available`。
*   **调度器 (Scheduler)**：
    *   维护一份可用的 Game Service 列表（可以是配置文件写死，或通过 Etcd 服务发现）。
    *   当收到 `CreateRoom` 请求时，选择一个负载最低的 Game Service 节点，返回其 IP 和 Port。

## 2. gRPC 接口定义

*   `rpc CreateRoom(CreateReq) returns (CreateResp)`
    *   逻辑：生成 UUID 作为 RoomID -> 选定 GameServer -> 存入 Redis -> 返回连接信息。
*   `rpc ListRooms(ListReq) returns (ListResp)`
    *   逻辑：扫描 Redis `rooms:available` -> 返回列表。
*   `rpc UpdateRoomStatus(UpdateReq) returns (Ack)`
    *   逻辑：供 Game Service 调用。当游戏开始或人数变化时，Game Service 通知 Match Service 更新 Redis 中的显示状态。

