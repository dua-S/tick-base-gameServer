package core

import (
	"sync"
	"time"
)

var (
	Rooms = make(map[string]*Room)
	mu    sync.RWMutex
)

func init() {
	go StartCleanupTask()
}

func StartCleanupTask() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		CleanupEmptyRooms()
	}
}

func CleanupEmptyRooms() {
	mu.Lock()
	defer mu.Unlock()

	now := time.Now().Unix()
	for id, room := range Rooms {
		room.Mutex.RLock()
		playerCount := len(room.Players)
		lastActive := room.LastActiveTime
		room.Mutex.RUnlock()

		if playerCount == 0 && (now-lastActive) > 60 {
			room.StopChan <- true
			delete(Rooms, id)
		}
	}
}

func GetRoom(roomID string) *Room {
	mu.RLock()
	defer mu.RUnlock()
	return Rooms[roomID]
}

func CreateRoom(roomID string) *Room {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := Rooms[roomID]; ok {
		return Rooms[roomID]
	}
	room := NewRoom(roomID)
	Rooms[roomID] = room
	go room.Run()
	return room
}

func RegisterPlayerToken(token string, uid int64) {
	mu.Lock()
	defer mu.Unlock()
	//PlayerTokens[uid] = token
}

func StartRoom(roomID string) {
	mu.RLock()
	room := Rooms[roomID]
	mu.RUnlock()

	if room != nil {
		room.Mutex.Lock()
		room.IsRunning = true
		room.Mutex.Unlock()
	}
}

func RemoveRoom(roomID string) {
	mu.Lock()
	defer mu.Unlock()
	delete(Rooms, roomID)
}
