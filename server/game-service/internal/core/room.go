package core

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	pb "mygame/proto"
	"mygame/server/game-service/internal/mq"
)

// Tick system constants
const (
	TickRate           = 64                      // Hz
	TickDuration       = time.Second / TickRate  // 15.625ms
	DeltaTime          = 1.0 / float64(TickRate) // 0.015625 seconds
	DelayCompensation  = 2                       // 固定2 tick延迟补偿
	AcceptableLagTicks = 2                       // 可接受的最大延迟tick数
)

type Room struct {
	ID         string
	Players    map[int64]*Player
	Beams      []*Beam // 暂存当前帧存在的光柱
	Broadcast  chan *pb.GamePacket
	Register   chan *Player
	Unregister chan int64

	// 状态控制
	Mutex           sync.RWMutex
	Ticker          *time.Ticker
	StopChan        chan bool
	IsRunning       bool
	IsInWaitingMode bool // true = 等待中, false = 游戏中
	MapSize         float64

	// Tick系统
	CurrentTick int64

	// 房主信息
	HostUID int64 // 房主UID

	LastActiveTime int64
	CreatedAt      int64
}

type Beam struct {
	ID             string
	OwnerID        int64
	StartX, StartY float64
	EndX, EndY     float64
	Width          float64
	ExpiresAt      int64
}

func NewRoom(id string) *Room {
	now := time.Now().Unix()
	return &Room{
		ID:             id,
		Players:        make(map[int64]*Player),
		Broadcast:      make(chan *pb.GamePacket),
		Register:       make(chan *Player),
		Unregister:     make(chan int64),
		StopChan:       make(chan bool),
		MapSize:        2000.0,
		LastActiveTime: now,
		CreatedAt:      now,
	}
}

func (r *Room) Run() {
	r.IsRunning = true
	r.CurrentTick = 1
	r.Ticker = time.NewTicker(TickDuration)
	defer r.Ticker.Stop()

	for {
		select {
		case <-r.StopChan:
			return

		case p := <-r.Register:
			r.Mutex.Lock()
			r.Players[p.UID] = p
			p.X = 100 + float64(time.Now().UnixNano()%1000)
			p.Y = 100 + float64(time.Now().UnixNano()%1000)
			// 第一个加入的玩家设为房主
			if r.HostUID == 0 {
				r.HostUID = p.UID
			}
			r.LastActiveTime = time.Now().Unix()
			r.Mutex.Unlock()
			fmt.Printf("Player %d joined room %s\n", p.UID, r.ID)

		case uid := <-r.Unregister:
			r.Mutex.Lock()
			delete(r.Players, uid)
			r.LastActiveTime = time.Now().Unix()

			// 如果房主离开，转移房主给另一个玩家
			if uid == r.HostUID && len(r.Players) > 0 {
				for newHostUID := range r.Players {
					r.HostUID = newHostUID
					fmt.Printf("Host transferred from %d to %d in room %s\n", uid, newHostUID, r.ID)
					break
				}
			}

			if len(r.Players) == 0 {
				r.IsRunning = false
				r.HostUID = 0
				r.Mutex.Unlock()
				r.StopChan <- true
				go RemoveRoom(r.ID)
			} else {
				r.Mutex.Unlock()
			}

		case <-r.Ticker.C:
			r.GameLoop()
		}
	}
}

// --- 核心 Tick 逻辑 ---
func (r *Room) GameLoop() {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()

	r.CurrentTick++

	// 1. 处理输入 (带延迟补偿)
	r.ProcessInputs()

	// 2. 清理过期的光柱特效
	now := time.Now().UnixMilli()
	activeBeams := make([]*Beam, 0)
	for _, b := range r.Beams {
		if b.ExpiresAt > now {
			activeBeams = append(activeBeams, b)
		}
	}
	r.Beams = activeBeams

	// 3. 检查胜利条件
	r.CheckWinCondition()

	// 4. 发送快照 (Snapshot)
	r.BroadcastSnapshot()
}

func (r *Room) ProcessInputs() {
	execTick := r.CurrentTick - DelayCompensation

	for _, p := range r.Players {
		if len(p.InputQueue) == 0 {
			continue
		}

		validInputs := make([]*pb.C2SInput, 0)
		for _, input := range p.InputQueue {
			targetTick := input.TargetTick

			// 输入验证：拒绝来自未来或过期的输入
			if targetTick == 0 {
				targetTick = input.Timestamp / int64(TickDuration)
			}

			tickDiff := targetTick - execTick

			// 过期输入：比执行tick早太多
			if tickDiff < -AcceptableLagTicks {
				continue
			}

			// 未来输入：比执行tick晚太多
			if tickDiff > AcceptableLagTicks {
				continue
			}

			validInputs = append(validInputs, input)
		}

		// 按target_tick排序
		sort.Slice(validInputs, func(i, j int) bool {
			return validInputs[i].TargetTick < validInputs[j].TargetTick
		})

		for _, input := range validInputs {
			p.TargetTick = input.TargetTick

			// 死者只能移动视角
			if !p.IsDead && input.Move != nil {
				// 使用DeltaTime计算移动
				p.X += float64(input.Move.Dx) * p.Speed * DeltaTime
				p.Y += float64(input.Move.Dy) * p.Speed * DeltaTime

				// 边界限制
				if p.X < 0 {
					p.X = 0
				}
				if p.Y < 0 {
					p.Y = 0
				}
				if p.X > r.MapSize {
					p.X = r.MapSize
				}
				if p.Y > r.MapSize {
					p.Y = r.MapSize
				}
			}

			// 蓄力与攻击处理
			if !p.IsDead && input.Charge != nil {
				if input.Charge.IsCharging {
					if !p.IsCharging {
						p.IsCharging = true
						p.ChargeStartTs = time.Now().UnixMilli()
						p.FacingAngle = input.Charge.Angle
					}
				} else {
					// 松开按键：发射
					if p.IsCharging {
						duration := time.Now().UnixMilli() - p.ChargeStartTs
						r.FireBeam(p, duration)
						p.IsCharging = false
					}
				}
			}
		}

		if len(validInputs) > 0 {
			p.LastProcessedTick = validInputs[len(validInputs)-1].TargetTick
		}

		// 清空队列
		p.InputQueue = p.InputQueue[:0]
	}
}

var direction = [4][2]int{{1, 0}, {0, 1}, {-1, 0}, {0, -1}}

func (r *Room) FireBeam(owner *Player, duration int64) {
	// 蓄力越久，伤害越高，宽度越宽
	damage := int32(duration / 10) // 简单公式: 100ms = 10伤害
	if damage > 50 {
		damage = 50
	}
	if damage < 5 {
		damage = 5
	}

	width := 20.0 + float64(duration)/20.0
	length := 800.0 // 射程

	// 计算终点
	endX := owner.X + length*float64(direction[owner.FacingAngle][0])
	endY := owner.Y + length*float64(direction[owner.FacingAngle][1])

	// 生成特效数据广播给客户端
	r.Beams = append(r.Beams, &Beam{
		ID:      fmt.Sprintf("%d-%d", owner.UID, time.Now().UnixNano()),
		OwnerID: owner.UID,
		StartX:  owner.X, StartY: owner.Y,
		EndX: endX, EndY: endY,
		Width:     width,
		ExpiresAt: time.Now().UnixMilli() + 300, // 300ms 特效存活
	})

	// 碰撞检测 (射线/矩形检测)
	// 简化：检测每个玩家是否在矩形内
	for _, target := range r.Players {
		if target.UID == owner.UID || target.IsDead {
			continue
		}
		if IsHit(owner.X, owner.Y, endX, endY, width, target.X, target.Y, 20) {
			target.HP -= damage
			if target.HP <= 0 {
				target.HP = 0
				target.IsDead = true
				r.BroadcastEvent(pb.GameEvent_PLAYER_DEATH, target.UID, "wasted")
			}
		}
	}
}

// 简单的点到线段距离碰撞检测
func IsHit(x1, y1, x2, y2, w, px, py, pr float64) bool {
	half := w / 2

	iseuqual := func(a, b float64) bool {
		return math.Abs(a-b) < 1e-6
	}

	// Axis-aligned optimizations (vertical or horizontal segment)
	if iseuqual(x1, x2) {
		minY := math.Min(y1, y2)
		maxY := math.Max(y1, y2)
		minX := x1 - half
		maxX := x1 + half

		closestX := math.Max(minX, math.Min(px, maxX))
		closestY := math.Max(minY, math.Min(py, maxY))
		dx := px - closestX
		dy := py - closestY
		return dx*dx+dy*dy <= pr*pr
	}

	if iseuqual(y1, y2) {
		minX := math.Min(x1, x2)
		maxX := math.Max(x1, x2)
		minY := y1 - half
		maxY := y1 + half

		closestX := math.Max(minX, math.Min(px, maxX))
		closestY := math.Max(minY, math.Min(py, maxY))
		dx := px - closestX
		dy := py - closestY
		return dx*dx+dy*dy <= pr*pr
	}

	return true
}

func (r *Room) CheckWinCondition() {
	aliveCount := 0
	var lastSurvivor *Player

	for _, p := range r.Players {
		if !p.IsDead {
			aliveCount++
			lastSurvivor = p
		}
	}

	// 至少要有2个人开始游戏才算，否则单人测试不结束
	if len(r.Players) > 1 && aliveCount <= 1 && r.IsRunning {
		winnerID := int64(-1)
		if lastSurvivor != nil {
			winnerID = lastSurvivor.UID
		}

		// 广播结束
		r.BroadcastEvent(pb.GameEvent_GAME_OVER, winnerID, "Game Over")
		r.IsRunning = false

		// 发送战绩到 MQ
		go mq.PublishGameResult(map[string]interface{}{
			"match_id":  r.ID,
			"winner":    winnerID,
			"timestamp": time.Now().Unix(),
		})

		// 5秒后关闭房间
		go func() {
			time.Sleep(5 * time.Second)
			r.StopChan <- true
		}()
	}
}

func (r *Room) BroadcastSnapshot() {
	snapshot := &pb.S2CSnapshot{
		ServerTime: time.Now().UnixMilli(),
		Tick:       r.CurrentTick,
		Players:    make([]*pb.PlayerState, 0),
		Beams:      make([]*pb.BeamState, 0),
	}

	// 序列化玩家
	for _, p := range r.Players {
		snapshot.Players = append(snapshot.Players, &pb.PlayerState{
			Uid:        p.UID,
			X:          float32(p.X),
			Y:          float32(p.Y),
			Hp:         p.HP,
			MaxHp:      p.MaxHP,
			IsDead:     p.IsDead,
			IsCharging: p.IsCharging,
			Username:   p.Username,
		})
	}
	// 序列化光柱
	for _, b := range r.Beams {
		snapshot.Beams = append(snapshot.Beams, &pb.BeamState{
			Id:     b.ID,
			StartX: float32(b.StartX), StartY: float32(b.StartY),
			EndX: float32(b.EndX), EndY: float32(b.EndY),
			Width:       float32(b.Width),
			RemainingMs: int32(b.ExpiresAt - time.Now().UnixMilli()),
		})
	}

	// 打包发送
	packet := &pb.GamePacket{
		Payload: &pb.GamePacket_Snapshot{Snapshot: snapshot},
	}

	// 这里简单全量广播，实际应做 AOI 过滤
	for _, p := range r.Players {
		p.Conn.Send(packet)
	}
}

func (r *Room) BroadcastEvent(evtType pb.GameEvent_EventType, targetID int64, msg string) {
	pkt := &pb.GamePacket{
		Payload: &pb.GamePacket_Event{
			Event: &pb.GameEvent{
				Type:      evtType,
				TargetUid: targetID,
				Message:   msg,
			},
		},
	}
	for _, p := range r.Players {
		p.Conn.Send(pkt)
	}
}
