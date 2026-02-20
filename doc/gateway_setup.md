# 文档二：Gateway 服务实施报告 (Gateway Service)

**服务名称**：gateway-service
**职责**：API 网关，统一入口，协议转换，鉴权。
**端口**：HTTP :8080

## 1. 核心功能

*   **HTTP Server**：使用 Gin 托管所有面向前端的 REST API。
*   **BFF 层**：将前端的 JSON 请求转换为内部微服务的 gRPC 请求（Protobuf）。
*   **统一鉴权**：在 Middleware 中解析 JWT Token，拦截非法请求。
*   **CORS 处理**：统一配置跨域策略。

## 2. 目录结构建议

```text
/gateway
  /middleware
    - cors.go       # Allow-Origin: *
    - jwt.go        # Header["Authorization"] -> Parse -> Context.Set("uid")
  /rpc_client
    - init.go       # gRPC Dial 连接池初始化
  /handler
    - user.go       # Login, Register, History
    - match.go      # CreateRoom, JoinRoom, ListRooms
  main.go           # Router 注册
```

## 3. API 路由定义 (Gin)

| 方法 | 路径                    | 鉴权    | 描述                   | 下游服务      |
| :--- | :---------------------- | :------ | :--------------------- | :------------ |
| POST | `/api/v1/auth/register` | No      | 用户注册               | User Service  |
| POST | `/api/v1/auth/login`    | No      | 用户登录 (返 JWT)      | User Service  |
| GET  | `/api/v1/user/history`  | **Yes** | 获取我的历史战绩       | User Service  |
| GET  | `/api/v1/match/rooms`   | **Yes** | 获取房间列表           | Match Service |
| POST | `/api/v1/match/create`  | **Yes** | 创建房间 (返 ServerIP) | Match Service |

## 4. 关键逻辑：创建房间转发

当前端请求创建房间时，Gateway 并不直接创建，而是调用 `MatchClient.CreateRoom`。

*   **Request**: JSON `{ "config": { "map_size": 2000 } }`
*   **Process**:
    1.  从 Context 获取 `uid`。
    2.  构造 gRPC Request `&pb.CreateRoomReq{ CreatorId: uid, ... }`。
    3.  调用 `MatchService`。
*   **Response**: JSON `{ "server_ip": "192.168.x.x", "port": 9003, "room_token": "..." }`。

这是一个非常棒的决定。完全可以使用 **Golang + Gin** 自己编写一个 **API Gateway** 服务。

在微服务架构中，这种模式通常被称为 **BFF (Backend for Frontend)**。这样做的好处是统一了对外的接口（RESTful API），隐藏了内部复杂的 gRPC 调用细节，同时你可以把鉴权（Auth）、跨域（CORS）、限流等逻辑全部收敛在这个 Gateway 服务中，不需要 Nginx 也能跑得很稳。

以下是为您定制的 **Gateway 服务实现方案**。

---

# 自研 Gateway 服务设计文档

## 1. 架构调整
移除 Nginx 的反向代理职能，由 `gateway-service` 接管所有 HTTP 请求。

*   **外部流量**：Web 客户端只与 `gateway-service` 进行 HTTP 交互。
*   **内部流量**：`gateway-service` 解析 HTTP 请求，通过 **gRPC** 调用后端的 `user-service` 和 `match-service`。
*   **例外**：为了保证游戏延迟最低，**游戏内的 WebSocket 连接依然建议直连 Game Server**（由 Gateway 返回目标 IP，客户端直连）。

```mermaid
graph TD
    Client[Web Client] -- HTTP/REST --> Gateway[Gateway Service (Gin)]
    
    subgraph "Internal Network (gRPC)"
        Gateway -- gRPC --> User[User Service]
        Gateway -- gRPC --> Match[Match Service]
        Match -- gRPC --> Game[Game Service]
    end
    
    Client -. WebSocket (Direct) .-> Game
    
    User -- GORM --> MySQL[(MySQL)]
    Match --> Redis[(Redis)]
    Game -- MQ --> MQ[(Message Queue)]
```

## 2. Gateway 功能职责
1.  **路由分发**：将 `/api/v1/user` 转发给 User 服务，将 `/api/v1/match` 转发给 Match 服务。
2.  **协议转换**：**HTTP (JSON) -> gRPC (Protobuf)**。前端传 JSON，Gateway 转成 Protobuf 发给微服务。
3.  **鉴权中间件**：解析 JWT Token，验证用户身份，将 UserID 注入上下文。
4.  **CORS 处理**：统一处理跨域请求。

## 3. 代码实现示例 (Golang + Gin)

假设您已经定义好了 `.proto` 文件并生成了 Go 代码。

### 3.1 项目结构
```text
/gateway
  /middleware
    - cors.go       # 跨域
    - auth.go       # JWT鉴权
  /rpc              # gRPC 客户端初始化
    - user_client.go
    - match_client.go
  /handlers         # Gin 的 Handler
    - user_handler.go
    - match_handler.go
  main.go           # 路由入口
```

### 3.2 核心代码：初始化 gRPC 客户端 (rpc/user_client.go)
我们需要在 Gateway 启动时建立与后端服务的连接。

```go
package rpc

import (
	"google.golang.org/grpc"
	"log"
	pb "mygame/proto" // 假设这是你生成的 proto 代码包
)

var UserClient pb.UserServiceClient

func InitUserClient() {
	// 连接到 user-service (地址通常从配置文件读取)
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect to user-service: %v", err)
	}
	// 这里 conn 不关闭，保持长连接（或使用连接池）
	UserClient = pb.NewUserServiceClient(conn)
}
```

### 3.3 核心代码：编写 Handler (handlers/user_handler.go)
这里演示如何将 Gin 的 HTTP 请求转换为 gRPC 请求。

```go
package handlers

import (
	"github.com/gin-gonic/gin"
	"mygame/gateway/rpc"
	pb "mygame/proto"
	"net/http"
)

// HandleLogin 处理登录请求
func HandleLogin(c *gin.Context) {
	// 1. 绑定前端传来的 JSON
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid params"})
		return
	}

	// 2. 调用 gRPC 服务 (HTTP -> gRPC)
	// rpc.UserClient 是在启动时初始化的全局变量
	grpcResp, err := rpc.UserClient.Login(c, &pb.LoginRequest{
		Username: req.Username,
		Password: req.Password,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Login failed"})
		return
	}

	// 3. 将 gRPC 响应转回 JSON 返回给前端
	c.JSON(http.StatusOK, gin.H{
		"token": grpcResp.Token,
		"uid":   grpcResp.UserId,
	})
}
```

### 3.4 核心代码：路由入口 (main.go)

```go
package main

import (
	"github.com/gin-gonic/gin"
	"mygame/gateway/handlers"
	"mygame/gateway/middleware"
	"mygame/gateway/rpc"
)

func main() {
	// 1. 初始化内部 gRPC 连接
	rpc.InitUserClient()
	rpc.InitMatchClient()

	r := gin.Default()

	// 2. 全局中间件
	r.Use(middleware.Cors())

	// 3. 路由组
	api := r.Group("/api")
	{
		// 用户相关
		user := api.Group("/user")
		{
			user.POST("/login", handlers.HandleLogin)
			user.POST("/register", handlers.HandleRegister)
			// 需要鉴权的接口
			user.GET("/history", middleware.Auth(), handlers.HandleGetHistory)
		}

		// 比赛相关
		match := api.Group("/match")
		match.Use(middleware.Auth()) // 整个组都需要鉴权
		{
			match.POST("/create", handlers.HandleCreateRoom) // 返回 server_ip, port
			match.GET("/list", handlers.HandleListRooms)
		}
	}

	// 4. 启动 Gateway 服务
	r.Run(":8080") // 比如运行在 8080 端口
}
```

## 4. 关键交互流程细节

### 4.1 登录流程
1.  **Web**: POST `http://gateway:8080/api/user/login` (JSON: user/pass)
2.  **Gateway**:
    *   接收 JSON。
    *   调用 `UserClient.Login(ctx, &pb.LoginReq{...})`。
3.  **User Service**:
    *   查 MySQL，校验密码。
    *   生成 JWT Token。
    *   返回 `&pb.LoginResp{Token: "xyz..."}`。
4.  **Gateway**: 返回 JSON `{ "token": "xyz..." }` 给 Web。

### 4.2 开始游戏流程 (重点)
1.  **Web**: POST `http://gateway:8080/api/match/create` (带 Token)。
2.  **Gateway**:
    *   Middleware 解析 Token 拿到 `UserID`。
    *   调用 `MatchClient.CreateRoom(ctx, &pb.CreateRoomReq{Uid: ...})`。
3.  **Match Service**:
    *   分配房间 ID。
    *   调度分配一台 Game Server (例如 `192.168.1.50:9001`)。
    *   返回 `&pb.RoomInfo{ Ip: "192.168.1.50", Port: 9001, Ticket: "sec-key" }`。
4.  **Gateway**: 转发 JSON 给 Web。
5.  **Web**: **直接连接 WebSocket** -> `ws://192.168.1.50:9001?ticket=sec-key`。
    *   *注意：这里 Web 拿到 IP 后直接连游戏服，不走 Gateway，避免 Gateway 成为高频游戏数据的瓶颈。*

## 5. 对您的建议

1.  **Proto 文件管理**：建议创建一个独立的 git 仓库或目录存放 `.proto` 文件，然后编写一个脚本同时生成 server 端（Go gRPC）和 client 端（Go gRPC Client）的代码，供各个服务引用。
2.  **Error 处理**：微服务间的 Error 传递比较麻烦。建议在 gRPC 返回中使用 `status` 包返回标准错误码，Gateway 解析后映射为 HTTP 状态码（如 `codes.Unauthenticated` -> HTTP 401）。
3.  **服务发现**：在本地开发时，Gateway 代码里写死 `localhost:50051` 没问题。如果上容器/K8s，建议使用环境变量传递服务地址，或者引入简单的服务发现（Consul/Etcd，不过前期用环境变量足够了）。

这个方案完全去除了 Nginx 的配置复杂度，让整个后端栈纯粹为 Go 语言，非常适合您的需求。您觉得这个代码结构清晰吗？
