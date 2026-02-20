package core

import (
	pb "mygame/proto"
	"sync"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

type WebSocketConn struct {
	Conn *websocket.Conn
	mu   sync.Mutex
}

func (c *WebSocketConn) Send(pkt *pb.GamePacket) {
	data, err := proto.Marshal(pkt)
	if err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	// WriteMessage 是线程不安全的，所以需要加锁
	c.Conn.WriteMessage(websocket.BinaryMessage, data)
}
