// systems/network.go — Multiplayer networking for AIEngine.
// Provides UDP-based game synchronization (positions, events) and
// TCP-based lobby/connection setup. Supports dedicated server (headless),
// peer-to-peer, and client-server modes.
// Configurable via config.aie: SERVER and CONNECT directives.

package systems

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"net"
	"sync"
	"time"
)

// --- Packet Protocol ---

// PacketType identifies the kind of network message.
type PacketType uint8

const (
	PktConnect     PacketType = 1  // client → server: join request
	PktAccept      PacketType = 2  // server → client: join accepted
	PktDisconnect  PacketType = 3  // either direction: leaving
	PktPlayerState PacketType = 10 // client → server: position/angle update
	PktWorldState  PacketType = 11 // server → client: all player states
	PktEvent       PacketType = 20 // either direction: game event (shoot, death, trigger)
	PktChat        PacketType = 30 // either direction: chat message
	PktPing        PacketType = 40 // either direction: latency check
	PktPong        PacketType = 41 // response to ping
)

// Packet header size: 1 byte type + 2 bytes payload length.
const PacketHeaderSize = 3
const MaxPacketSize = 1400 // safe UDP size below MTU

// NetworkPacket represents a serialized game message.
type NetworkPacket struct {
	Type    PacketType
	Payload []byte
}

// Serialize converts a packet to bytes for sending.
func (p *NetworkPacket) Serialize() []byte {
	buf := make([]byte, PacketHeaderSize+len(p.Payload))
	buf[0] = byte(p.Type)
	binary.BigEndian.PutUint16(buf[1:3], uint16(len(p.Payload)))
	copy(buf[3:], p.Payload)
	return buf
}

// DeserializePacket parses bytes into a NetworkPacket.
func DeserializePacket(data []byte) (*NetworkPacket, error) {
	if len(data) < PacketHeaderSize {
		return nil, fmt.Errorf("packet too short: %d bytes", len(data))
	}

	pktType := PacketType(data[0])
	payloadLen := binary.BigEndian.Uint16(data[1:3])

	if int(payloadLen)+PacketHeaderSize > len(data) {
		return nil, fmt.Errorf("payload length %d exceeds data length %d", payloadLen, len(data)-PacketHeaderSize)
	}

	return &NetworkPacket{
		Type:    pktType,
		Payload: data[3 : 3+payloadLen],
	}, nil
}

// --- Player Network State ---

// PlayerNetState is the data synced for each player.
type PlayerNetState struct {
	ID       uint32
	X, Y, Z  float64
	Angle    float64
	HP       float32
	Alive    bool
	Weapon   string
	SeqNum   uint32 // sequence number for ordering
}

// SerializePlayerState packs player state into bytes.
func SerializePlayerState(ps *PlayerNetState) []byte {
	buf := make([]byte, 4+8*4+4+1+32+4) // approx
	offset := 0
	binary.BigEndian.PutUint32(buf[offset:], ps.ID)
	offset += 4
	binary.BigEndian.PutUint64(buf[offset:], mathFloat64ToBits(ps.X))
	offset += 8
	binary.BigEndian.PutUint64(buf[offset:], mathFloat64ToBits(ps.Y))
	offset += 8
	binary.BigEndian.PutUint64(buf[offset:], mathFloat64ToBits(ps.Z))
	offset += 8
	binary.BigEndian.PutUint64(buf[offset:], mathFloat64ToBits(ps.Angle))
	offset += 8
	binary.BigEndian.PutUint32(buf[offset:], mathFloat32ToBits(ps.HP))
	offset += 4
	if ps.Alive {
		buf[offset] = 1
	} else {
		buf[offset] = 0
	}
	offset++
	// Weapon name (padded to 32 bytes).
	copy(buf[offset:offset+32], []byte(ps.Weapon))
	offset += 32
	binary.BigEndian.PutUint32(buf[offset:], ps.SeqNum)

	return buf[:offset+4]
}

// DeserializePlayerState unpacks bytes into a PlayerNetState.
func DeserializePlayerState(data []byte) *PlayerNetState {
	if len(data) < 4 {
		return nil
	}
	ps := &PlayerNetState{}
	offset := 0
	ps.ID = binary.BigEndian.Uint32(data[offset:])
	offset += 4
	ps.X = mathFloat64FromBits(binary.BigEndian.Uint64(data[offset:]))
	offset += 8
	ps.Y = mathFloat64FromBits(binary.BigEndian.Uint64(data[offset:]))
	offset += 8
	ps.Z = mathFloat64FromBits(binary.BigEndian.Uint64(data[offset:]))
	offset += 8
	ps.Angle = mathFloat64FromBits(binary.BigEndian.Uint64(data[offset:]))
	offset += 8
	ps.HP = mathFloat32FromBits(binary.BigEndian.Uint32(data[offset:]))
	offset += 4
	ps.Alive = data[offset] == 1
	offset++
	ps.Weapon = string(data[offset : offset+32])
	offset += 32
	if offset+4 <= len(data) {
		ps.SeqNum = binary.BigEndian.Uint32(data[offset:])
	}
	return ps
}

// --- GameServer ---

// GameServer handles incoming connections and game state synchronization.
type GameServer struct {
	mu sync.RWMutex

	Port       int
	MaxPlayers int
	Mode       string // "coop", "deathmatch", etc.

	udpConn  *net.UDPConn
	tcpListener *net.TCPListener

	Players map[uint32]*RemotePlayer
	nextID  uint32

	Running bool
	quit    chan struct{}
}

// RemotePlayer represents a connected client on the server.
type RemotePlayer struct {
	ID         uint32
	Addr       *net.UDPAddr
	State      PlayerNetState
	LastUpdate time.Time
}

// NewGameServer creates a game server on the given port.
func NewGameServer(port int) *GameServer {
	return &GameServer{
		Port:       port,
		MaxPlayers: 4,
		Mode:       "coop",
		Players:    make(map[uint32]*RemotePlayer),
		quit:       make(chan struct{}),
	}
}

// Start begins listening for client connections.
func (gs *GameServer) Start() error {
	addr := &net.UDPAddr{
		Port: gs.Port,
		IP:   net.ParseIP("0.0.0.0"),
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("UDP listen on :%d: %w", gs.Port, err)
	}
	gs.udpConn = conn
	gs.Running = true

	log.Printf("[Server] Listening on UDP :%d (mode: %s, max players: %d)",
		gs.Port, gs.Mode, gs.MaxPlayers)

	go gs.listenLoop()
	return nil
}

// listenLoop receives and dispatches incoming UDP packets.
func (gs *GameServer) listenLoop() {
	buf := make([]byte, MaxPacketSize)

	for gs.Running {
		gs.udpConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

		n, remoteAddr, err := gs.udpConn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			if gs.Running {
				log.Printf("[Server] Read error: %v", err)
			}
			continue
		}

		pkt, err := DeserializePacket(buf[:n])
		if err != nil {
			log.Printf("[Server] Bad packet from %v: %v", remoteAddr, err)
			continue
		}

		gs.handlePacket(pkt, remoteAddr)
	}
}

// handlePacket processes a single game packet.
func (gs *GameServer) handlePacket(pkt *NetworkPacket, addr *net.UDPAddr) {
	switch pkt.Type {
	case PktConnect:
		gs.handleConnect(addr)

	case PktDisconnect:
		gs.handleDisconnect(addr)

	case PktPlayerState:
		gs.handlePlayerState(pkt, addr)

	case PktEvent:
		gs.broadcastEvent(pkt, addr)

	case PktChat:
		gs.broadcastChat(pkt, addr)

	case PktPing:
		gs.sendPong(addr)
	}
}

// handleConnect adds a new player.
func (gs *GameServer) handleConnect(addr *net.UDPAddr) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if len(gs.Players) >= gs.MaxPlayers {
		log.Printf("[Server] Connection from %v rejected: server full", addr)
		return
	}

	// Check if already connected.
	for _, p := range gs.Players {
		if p.Addr.String() == addr.String() {
			return
		}
	}

	gs.nextID++
	player := &RemotePlayer{
		ID:         gs.nextID,
		Addr:       addr,
		LastUpdate: time.Now(),
		State: PlayerNetState{
			ID:    gs.nextID,
			Alive: true,
			HP:    100,
		},
	}
	gs.Players[gs.nextID] = player

	// Send accept.
	acceptPayload := make([]byte, 8)
	binary.BigEndian.PutUint32(acceptPayload[0:4], gs.nextID)
	binary.BigEndian.PutUint32(acceptPayload[4:8], uint32(len(gs.Players)))
	acceptPkt := &NetworkPacket{Type: PktAccept, Payload: acceptPayload}
	gs.udpConn.WriteToUDP(acceptPkt.Serialize(), addr)

	log.Printf("[Server] Player %d connected from %v (total: %d)", gs.nextID, addr, len(gs.Players))
}

// handleDisconnect removes a player.
func (gs *GameServer) handleDisconnect(addr *net.UDPAddr) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	for id, p := range gs.Players {
		if p.Addr.String() == addr.String() {
			log.Printf("[Server] Player %d disconnected", id)
			delete(gs.Players, id)
			return
		}
	}
}

// handlePlayerState updates a player's game state and broadcasts to others.
func (gs *GameServer) handlePlayerState(pkt *NetworkPacket, addr *net.UDPAddr) {
	ps := DeserializePlayerState(pkt.Payload)
	if ps == nil {
		return
	}

	gs.mu.Lock()
	player, ok := gs.Players[ps.ID]
	if !ok {
		gs.mu.Unlock()
		return
	}
	player.State = *ps
	player.LastUpdate = time.Now()

	// Broadcast world state to all other players.
	gs.broadcastWorldStateLocked(addr)
	gs.mu.Unlock()
}

// broadcastWorldStateLocked sends all player states. Must hold mu.
func (gs *GameServer) broadcastWorldStateLocked(excludeAddr *net.UDPAddr) {
	for _, p := range gs.Players {
		if p.Addr.String() == excludeAddr.String() {
			continue
		}
		payload := SerializePlayerState(&p.State)
		worldPkt := &NetworkPacket{Type: PktWorldState, Payload: payload}
		gs.udpConn.WriteToUDP(worldPkt.Serialize(), p.Addr)
	}
}

// broadcastEvent sends an event to all connected players except sender.
func (gs *GameServer) broadcastEvent(pkt *NetworkPacket, senderAddr *net.UDPAddr) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	for _, p := range gs.Players {
		if p.Addr.String() == senderAddr.String() {
			continue
		}
		gs.udpConn.WriteToUDP(pkt.Serialize(), p.Addr)
	}
}

// broadcastChat relays a chat message.
func (gs *GameServer) broadcastChat(pkt *NetworkPacket, senderAddr *net.UDPAddr) {
	gs.broadcastEvent(pkt, senderAddr)
}

// sendPong responds to a ping with a pong.
func (gs *GameServer) sendPong(addr *net.UDPAddr) {
	pong := &NetworkPacket{Type: PktPong, Payload: nil}
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	gs.udpConn.WriteToUDP(pong.Serialize(), addr)
}

// Stop shuts down the server.
func (gs *GameServer) Stop() {
	gs.Running = false
	close(gs.quit)
	if gs.udpConn != nil {
		gs.udpConn.Close()
	}
	if gs.tcpListener != nil {
		gs.tcpListener.Close()
	}
	log.Println("[Server] Stopped")
}

// --- GameClient ---

// GameClient connects to a game server and syncs local player state.
type GameClient struct {
	mu sync.RWMutex

	ServerAddr  string
	ServerPort  int
	PlayerID    uint32
	LocalPlayer PlayerNetState

	udpConn     *net.UDPConn
	serverAddr  *net.UDPAddr
	RemotePlayers map[uint32]*PlayerNetState

	Connected bool
	Running   bool
	quit      chan struct{}

	// Callbacks.
	OnWorldState  func(players []PlayerNetState)
	OnEvent       func(eventType string, data []byte)
	OnChat        func(playerID uint32, message string)
	OnDisconnected func()
}

// NewGameClient creates a game client.
func NewGameClient(serverAddr string, port int) *GameClient {
	return &GameClient{
		ServerAddr:    serverAddr,
		ServerPort:    port,
		RemotePlayers: make(map[uint32]*PlayerNetState),
		quit:          make(chan struct{}),
	}
}

// Connect joins a game server.
func (gc *GameClient) Connect() error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", gc.ServerAddr, gc.ServerPort))
	if err != nil {
		return fmt.Errorf("resolve server: %w", err)
	}
	gc.serverAddr = addr

	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 0})
	if err != nil {
		return fmt.Errorf("UDP listen: %w", err)
	}
	gc.udpConn = conn

	// Send connect packet.
	connectPkt := &NetworkPacket{Type: PktConnect, Payload: nil}
	_, err = gc.udpConn.WriteToUDP(connectPkt.Serialize(), gc.serverAddr)
	if err != nil {
		return fmt.Errorf("send connect: %w", err)
	}

	gc.Running = true
	go gc.listenLoop()

	log.Printf("[Client] Connecting to %s:%d...", gc.ServerAddr, gc.ServerPort)
	return nil
}

// listenLoop receives packets from the server.
func (gc *GameClient) listenLoop() {
	buf := make([]byte, MaxPacketSize)

	for gc.Running {
		gc.udpConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

		n, _, err := gc.udpConn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			if gc.Running {
				log.Printf("[Client] Read error: %v", err)
			}
			continue
		}

		pkt, err := DeserializePacket(buf[:n])
		if err != nil {
			continue
		}

		gc.handlePacket(pkt)
	}
}

// handlePacket processes a server packet.
func (gc *GameClient) handlePacket(pkt *NetworkPacket) {
	switch pkt.Type {
	case PktAccept:
		if len(pkt.Payload) >= 4 {
			gc.mu.Lock()
			gc.PlayerID = binary.BigEndian.Uint32(pkt.Payload[0:4])
			gc.Connected = true
			gc.mu.Unlock()
			log.Printf("[Client] Connected! Player ID: %d", gc.PlayerID)
		}

	case PktWorldState:
		ps := DeserializePlayerState(pkt.Payload)
		if ps != nil {
			gc.mu.Lock()
			if ps.ID != gc.PlayerID {
				gc.RemotePlayers[ps.ID] = ps
			}
			gc.mu.Unlock()

			if gc.OnWorldState != nil {
				gc.mu.RLock()
				players := make([]PlayerNetState, 0, len(gc.RemotePlayers))
				for _, rp := range gc.RemotePlayers {
					players = append(players, *rp)
				}
				gc.mu.RUnlock()
				gc.OnWorldState(players)
			}
		}

	case PktEvent:
		if gc.OnEvent != nil {
			eventType := "custom"
			if len(pkt.Payload) > 0 {
				eventType = string(pkt.Payload)
			}
			gc.OnEvent(eventType, pkt.Payload)
		}

	case PktChat:
		if gc.OnChat != nil && len(pkt.Payload) >= 4 {
			playerID := binary.BigEndian.Uint32(pkt.Payload[0:4])
			msg := string(pkt.Payload[4:])
			gc.OnChat(playerID, msg)
		}

	case PktPong:
		// Latency measurement: could compute RTT.
	}
}

// SendPlayerState sends the local player's current state to the server.
func (gc *GameClient) SendPlayerState() {
	if !gc.Connected {
		return
	}

	gc.mu.RLock()
	state := gc.LocalPlayer
	state.ID = gc.PlayerID
	state.SeqNum++
	gc.mu.RUnlock()

	payload := SerializePlayerState(&state)
	pkt := &NetworkPacket{Type: PktPlayerState, Payload: payload}
	gc.udpConn.WriteToUDP(pkt.Serialize(), gc.serverAddr)
}

// SendEvent sends a game event to the server.
func (gc *GameClient) SendEvent(eventType string, data []byte) {
	if !gc.Connected {
		return
	}
	payload := append([]byte(eventType), data...)
	pkt := &NetworkPacket{Type: PktEvent, Payload: payload}
	gc.udpConn.WriteToUDP(pkt.Serialize(), gc.serverAddr)
}

// SendChat sends a chat message.
func (gc *GameClient) SendChat(message string) {
	if !gc.Connected {
		return
	}
	payload := make([]byte, 4+len(message))
	binary.BigEndian.PutUint32(payload[0:4], gc.PlayerID)
	copy(payload[4:], []byte(message))
	pkt := &NetworkPacket{Type: PktChat, Payload: payload}
	gc.udpConn.WriteToUDP(pkt.Serialize(), gc.serverAddr)
}

// Disconnect gracefully leaves the server.
func (gc *GameClient) Disconnect() {
	if gc.Connected {
		pkt := &NetworkPacket{Type: PktDisconnect, Payload: nil}
		gc.udpConn.WriteToUDP(pkt.Serialize(), gc.serverAddr)
	}
	gc.Running = false
	gc.Connected = false
	close(gc.quit)
	if gc.udpConn != nil {
		gc.udpConn.Close()
	}
	if gc.OnDisconnected != nil {
		gc.OnDisconnected()
	}
	log.Println("[Client] Disconnected")
}

// UpdateLocalPlayer updates the client's player state.
func (gc *GameClient) UpdateLocalPlayer(x, y, z, angle, hp float64, alive bool, weapon string) {
	gc.mu.Lock()
	gc.LocalPlayer = PlayerNetState{
		X:      x,
		Y:      y,
		Z:      z,
		Angle:  angle,
		HP:     float32(hp),
		Alive:  alive,
		Weapon: weapon,
	}
	gc.mu.Unlock()
}

// --- Float bit conversion helpers ---

func mathFloat64ToBits(f float64) uint64 {
	return math.Float64bits(f)
}

func mathFloat64FromBits(b uint64) float64 {
	return math.Float64frombits(b)
}

func mathFloat32ToBits(f float32) uint32 {
	return math.Float32bits(f)
}

func mathFloat32FromBits(b uint32) float32 {
	return math.Float32frombits(b)
}
