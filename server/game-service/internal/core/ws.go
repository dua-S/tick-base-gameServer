package core

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	pb "mygame/proto"
	"mygame/server/game-service/internal/dao"
)

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

const (
	wsReadDeadline  = 60 * time.Second
	wsWriteDeadline = 10 * time.Second
	wsPingPeriod    = 30 * time.Second
)

func HandleWebSocket(c *gin.Context) {
	roomID := c.Query("room_id")
	token := c.Query("token")

	if roomID == "" || token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "room_id and token required"})
		return
	}

	// 在升级连接前通过 Redis 校验 room token 是否匹配该 room_id
	if ok, err := dao.ValidateRoomToken(context.Background(), roomID, token); err != nil {
		log.Println("redis error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	} else if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid room token"})
		return
	}

	room := CreateRoom(roomID)

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Upgrade failed:", err)
		return
	}
	defer ws.Close()

	// 要求客户端提供真实的 uid（由登录/网关验证后得到）
	uidStr := c.Query("uid")
	if uidStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uid required"})
		return
	}
	uid, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid uid"})
		return
	}

	username := "Player"

	// 把 room token 与真实 uid 绑定，供后续服务内校验使用
	RegisterPlayerToken(token, uid)

	playerConn := &WebSocketConn{Conn: ws}
	player := NewPlayer(uid, username, playerConn)

	room.Register <- player

	// 发送初始状态给客户端，确认连接已建立
	initialPacket := &pb.GamePacket{
		Payload: &pb.GamePacket_Snapshot{
			Snapshot: &pb.S2CSnapshot{
				ServerTime: time.Now().UnixMilli(),
				Tick:       0,
				Players:    []*pb.PlayerState{},
			},
		},
	}
	playerConn.Send(initialPacket)
	defer func() {
		room.Unregister <- uid
	}()

	ws.SetReadDeadline(time.Now().Add(wsReadDeadline))
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(wsReadDeadline))
		return nil
	})

	pingTicker := time.NewTicker(wsPingPeriod)
	defer pingTicker.Stop()

	messageChan := make(chan []byte)
	doneChan := make(chan bool)

	go func() {
		defer close(doneChan)
		for {
			_, data, err := ws.ReadMessage()
			if err != nil {
				log.Println("Read error:", err)
				return
			}
			messageChan <- data
		}
	}()

	for {
		select {
		case <-pingTicker.C:
			ws.SetWriteDeadline(time.Now().Add(wsWriteDeadline))
			if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Println("Ping error:", err)
				return
			}

		case data := <-messageChan:
			ws.SetReadDeadline(time.Now().Add(wsReadDeadline))

			var pkt pb.GamePacket
			if err := proto.Unmarshal(data, &pkt); err != nil {
				continue
			}

			if input := pkt.GetInput(); input != nil {
				player.InputQueue = append(player.InputQueue, input)
			}

			if join := pkt.GetJoin(); join != nil {
				if join.Username != "" {
					player.Username = join.Username
				}
			}

		case <-doneChan:
			return
		}
	}
}
