# 文档三：User 服务实施报告 (User Service)

**服务名称**：user-service
**职责**：用户数据管理，战绩归档。
**端口**：gRPC :9001
**数据库**：MySQL (Table: `users`, `match_histories`)

## 1. 核心功能

*   **身份认证**：way）。
*   **战绩查询**：提供 gRPC 接口供 Gateway 查询某用户的历史记录。
*   **战绩归档 (Consumer)**：
    *   启动一个 Goroutine 监听 RabbitMQ 的 `game_result_queue` 队列。
    *   收到消息后，使用 GORM 将数据解析并批量插入 `match_histories` 表。

## 2. 数据库设计 (GORM Model)

```go
身份认证：

type User struct {
    ID        uint   `gorm:"primarykey"`
    Username  string `gorm:"type:varchar(32);uniqueIndex"`
    Password  string `gorm:"type:varchar(100)"` // Hash
    CreatedAt time.Time
}

type MatchHistory struct {
    ID        uint      `gorm:"primarykey"`
    UserID    uint      `gorm:"index"` // 外键，索引用于快速查询
    MatchID   string    `gorm:"type:varchar(64)"`
    IsWinner  bool
    Kills     int
    Rank      int       // 排名
    PlayedAt  time.Time
}
```

## 3. gRPC 接口定义 (Proto)

*   `rpc Register(RegisterReq) returns (RegisterResp)`
*   `rpc Login(LoginReq) returns (LoginResp)`
*   `rpc GetHistory(GetHistoryReq) returns (GetHistoryResp)`
*   `rpc GetUserInfo(UserInfoReq) returns (UserInfoResp)`