# offgame - 2D在线竞技游戏服务端
**offgame** 是一款基于浏览器的2D多人在线竞技游戏服务端实现，采用微服务架构与高性能游戏循环，为前端（React + Pixi.js）提供低延迟、高精度的对战体验。本项目仅包含服务端代码，涵盖用户认证、房间管理、游戏逻辑（64Hz tick + 固定延迟补偿）、实时WebSocket通信及异步战绩存储。

## ✨ 核心特性

- **微服务架构**：Gateway（API网关）、User（用户/战绩）、Match（房间调度）、Game（核心玩法）完全解耦，支持独立部署与横向扩展。
- **高性能游戏循环**：基于Golang的64Hz固定tick循环，配合**固定2 tick延迟补偿**，确保所有玩家输入在服务端权威执行，兼顾公平性与低延迟体验。
- **延迟补偿机制**：客户端发送的输入附带目标tick，服务端延迟2 tick后统一处理，拒绝过期或超前输入，有效对抗网络波动。
- **实时通信**：WebSocket + Protobuf 二进制协议，大幅降低带宽占用；游戏内视野管理采用AOI九宫格算法，只推送玩家感兴趣的区域。
- **异步持久化**：每局游戏结束后，结果通过RabbitMQ异步写入MySQL，避免阻塞游戏主循环。
- **蓄力战斗系统**：支持蓄力光柱攻击，服务端进行旋转矩形碰撞检测（OBB），保证判定的权威性。
- **观战模式**：死亡后自动切换为观战视角，可自由移动摄像机并实时接收存活玩家的状态更新。

## 🧱 技术栈

| 组件             | 技术选型                     | 说明                               |
| ---------------- | ---------------------------- | ---------------------------------- |
| 编程语言         | Golang 1.20+                 | 所有微服务均使用Go开发             |
| Web框架          | Gin                          | Gateway服务HTTP路由                |
| RPC框架          | gRPC + Protobuf              | 服务间高效通信                     |
| 实时通信         | WebSocket (Gorilla)          | 游戏客户端直连Game服务             |
| 数据库           | MySQL 8.0 + GORM             | 存储用户与战绩数据                 |
| 缓存             | Redis 6+                     | 会话管理、房间列表、分布式锁       |
| 消息队列         | RabbitMQ                     | 解耦战绩写入，提升Game服务吞吐量   |
| 协议序列化       | Protobuf                     | 二进制格式，降低网络开销           |
| 服务发现         | 静态配置/环境变量（后续可扩展）| 前期简化，直接配置地址             |

## 🏗️ 系统架构

```
┌─────────┐      HTTP/JSON       ┌─────────────┐
│  Client │ ───────────────────► │   Gateway   │
│  (Web)  │      (REST API)      │   :8080     │
└─────────┘                        └──────┬──────┘
      │                                     │
      │ WebSocket (Protobuf)                │ gRPC
      ▼                                     ▼
┌─────────┐                          ┌─────────────┐
│  Game   │ ◄─────────────────────── │    Match    │
│ Cluster │   gRPC (房间管理)          │   :9002     │
│ :9003+  │                          └─────────────┘
└────┬────┘                                   │
     │                                         │ Redis
     │ Publish (战绩)                           ▼
     ▼                                    ┌─────────────┐
┌─────────┐     RabbitMQ     ┌─────────┐ │    Redis    │
│   MQ    │ ◄─────────────── │  User   │ │   Session   │
└─────────┘  game_results     │ :9001   │ │   Rooms     │
                              └────┬────┘ └─────────────┘
                                   │
                                   │ GORM
                                   ▼
                              ┌─────────────┐
                              │   MySQL     │
                              │  users      │
                              │  histories  │
                              └─────────────┘
```

### 服务职责

- **Gateway**：统一API入口，处理前端HTTP请求，进行JWT鉴权，将请求通过gRPC转发至后端微服务，并返回JSON响应。游戏内WebSocket连接直连Game服务，避免网关成为瓶颈。
- **User**：用户注册/登录、JWT签发、战绩查询；同时作为RabbitMQ消费者，异步将游戏结果写入MySQL。
- **Match**：房间管理（创建、列表、更新状态），调度Game服务实例（返回IP:Port给客户端），所有房间信息存储在Redis中。
- **Game**：核心游戏服务器，每个房间独立运行64Hz tick循环，处理玩家输入、物理模拟、碰撞检测、胜负判定，并通过WebSocket广播状态更新。游戏结束后将结果发布到MQ。

## 🚀 快速开始

### 环境要求

- Go 1.20+
- MySQL 8.0
- Redis 6+
- RabbitMQ
- （可选）Protobuf编译器 `protoc`（用于生成代码）

### 配置文件

所有服务均支持通过环境变量或配置文件（如 `config.yaml`）进行配置。

### 运行步骤

1. **克隆仓库**
   ```bash
   git clone https://github.com/dua-S/tick-base-gameServer.git
   ```

2. **安装依赖**
   ```bash
   go mod download
   ```

3. **启动基础服务**（MySQL, Redis, RabbitMQ）确保已安装并运行。

4. **编译并运行各服务**（建议使用Makefile或分别运行）

   ```bash
   # 终端1: User Service
   cd server/user-service
   go run .

   # 终端2: Match Service
   cd server/match-service
   go run .

   # 终端3: Gateway Service
   cd server/gateway
   go run .

   # 终端4: Game Service (可启动多个实例)
   cd server/game-service
   go run .
   ```

## ⚙️ 核心设计原理

### 64Hz Tick与延迟补偿

游戏服务采用固定频率循环（64Hz，每15.625ms一帧），每个tick处理输入、更新物理、碰撞检测并广播状态。为确保公平性，实现**固定2 tick延迟补偿**：

- 客户端发送输入时附带目标执行tick（通常为服务端当前tick + 2）。
- 服务端收到输入后存入队列，在每个tick处理时，只执行目标tick恰好等于 `当前tick - 2` 的输入。
- 超出此窗口（±2 tick）的输入直接丢弃，有效防止作弊及网络抖动。
- 物理计算使用固定 `DeltaTime`，保证不同客户端推测的一致性。

### 异步战绩存储

- 游戏结束后，Game服务组装 `GameResult` 并序列化为JSON，发布至RabbitMQ的 `game_results` 队列。
- User服务作为消费者监听队列，将数据解析后写入MySQL `match_histories` 表，避免游戏循环被IO阻塞。

## 📜 Protobuf 编译指南

当修改了 `api/*.proto` 文件后，需要重新生成Go代码：

```bash
# 安装 protoc 及 protoc-gen-go
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 进入项目根目录执行
gen_proto.sh
```

`gen_proto.sh` 命令会扫描 `api/` 下的所有 `.proto` 文件，并在 `pkg/proto` 中生成对应的 `.pb.go` 和 `_grpc.pb.go` 文件。
