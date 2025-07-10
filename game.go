/*
   Copyright 2022 Max Krivanek

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package t1net

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

type Team struct {
	Name  string
	Score string
}

type Player struct {
	Name  string
	Team  uint8
	Score string
	Ping  uint8
	PL    uint8
}

type GameServer struct {
	mutex             sync.RWMutex
	address           string
	ip                net.IP
	port              int
	ping              time.Duration
	queryTime         time.Time
	name              string
	game              string
	version           string
	dedicated         bool
	password          bool
	numPlayers        uint8
	maxPlayers        uint8
	cpuSpeed          uint16
	mod               string
	serverType        string
	mission           string
	info              string
	numTeams          uint8
	teamScoreHeader   string
	playerScoreHeader string
	teams             []Team
	players           []Player
}

func (g *GameServer) Ping() (ping time.Duration) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.ping
}

func (g *GameServer) QueryTime() (queryTime time.Time) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.queryTime
}

func (g *GameServer) Name() (name string) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.name
}

func (g *GameServer) Game() (game string) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.game
}

func (g *GameServer) Version() (version string) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.version
}

func (g *GameServer) Dedicated() (dedicated bool) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.dedicated
}

func (g *GameServer) Password() (password bool) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.password
}

func (g *GameServer) NumPlayers() (numPlayers uint8) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.numPlayers
}

func (g *GameServer) MaxPlayers() (maxPlayers uint8) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.maxPlayers
}

func (g *GameServer) CPUSpeed() (cpuSpeed uint16) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.cpuSpeed
}

func (g *GameServer) Mod() (mod string) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.mod
}

func (g *GameServer) ServerType() (serverType string) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.serverType
}

func (g *GameServer) Mission() (mission string) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.mission
}

func (g *GameServer) Info() (info string) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.info
}

func (g *GameServer) NumTeams() (numTeams uint8) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.numTeams
}

func (g *GameServer) TeamScoreHeader() (teamScoreHeader string) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.teamScoreHeader
}

func (g *GameServer) PlayerScoreHeader() (playerScoreHeader string) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.playerScoreHeader
}

func (g *GameServer) Teams() (teams []Team) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	teams = make([]Team, len(g.teams))
	copy(teams, g.teams)
	return
}

func (g *GameServer) Players() (players []Player) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	players = make([]Player, len(g.players))
	copy(players, g.players)
	return
}

func (g *GameServer) Query(timeout time.Duration, localAddress string) (err error) {
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	var localAddr *net.UDPAddr

	if len(localAddress) != 0 {
		localAddr, err = net.ResolveUDPAddr("udp4", localAddress)
		if err != nil {
			return
		}
	}

	remoteAddr, err := net.ResolveUDPAddr("udp4", g.address)
	if err != nil {
		return
	}

	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.ip = remoteAddr.IP
	g.port = remoteAddr.Port
	g.numTeams = 0
	g.numPlayers = 0
	g.maxPlayers = 0
	g.teams = nil
	g.players = nil

	c, err := net.DialUDP("udp4", localAddr, remoteAddr)
	if err != nil {
		return
	}

	defer func(c *net.UDPConn) {
		err := c.Close()
		if err != nil {
			fmt.Printf("t1net.GameServer.Query: Error closing UDP connection: %v\n", err)
		}
	}(c)

	key := uint16(rand.Uint32())
	// 0x62 = GameSpy query request, next two bytes are key
	sendBuffer := []byte{0x62, 0x00, 0x00}

	binary.BigEndian.PutUint16(sendBuffer[1:], key)

	g.queryTime = time.Now()
	_, err = c.Write(sendBuffer)
	if err != nil {
		return
	}

	readBuffer := make([]byte, 2048)
	err = c.SetDeadline(time.Now().Add(timeout))
	if err != nil {
		return
	}
	n, addr, err := c.ReadFromUDP(readBuffer)
	if err != nil {
		return
	}

	g.ping = time.Since(g.queryTime)

	if !addr.IP.Equal(remoteAddr.IP) || addr.Port != remoteAddr.Port {
		return fmt.Errorf("t1net.GameServer.Query: Reply address mismatch: %s != %s", remoteAddr.String(), addr.String())
	}

	if n < 20 {
		return fmt.Errorf("t1net.GameServer.Query: Reply packet length too short: %d < 20", n)
	}

	reader := bytes.NewReader(readBuffer[0:n])
	b, err := reader.ReadByte()
	if err != nil {
		return
	}
	// 0x63 = GameSpy query response
	if b != 0x63 {
		return fmt.Errorf("t1net.GameServer.Query: Reply byte 0: %#v != 0x63", b)
	}

	var readKey uint16
	err = binary.Read(reader, binary.BigEndian, &readKey)
	if err != nil {
		return
	}
	if key != readKey {
		return fmt.Errorf("t1net.GameServer.Query: Key mismatch: %d : %d", readKey, key)
	}

	b, err = reader.ReadByte()
	if err != nil {
		return
	}
	// 0x62 = The request we sent, in this case the GameSpy Query request
	if b != 0x62 {
		return fmt.Errorf("t1net.GameServer.Query: Reply byte 3: %#v != 0x62", b)
	}

	g.game, err = ReadPascalString(reader)
	if err != nil {
		return
	}

	g.version, err = ReadPascalString(reader)
	if err != nil {
		return
	}

	g.name, err = ReadPascalString(reader)
	if err != nil {
		return
	}

	b, err = reader.ReadByte()
	if err != nil {
		return
	}
	g.dedicated = b == 1

	b, err = reader.ReadByte()
	if err != nil {
		return
	}
	g.password = b == 1

	b, err = reader.ReadByte()
	if err != nil {
		return
	}
	g.numPlayers = b

	b, err = reader.ReadByte()
	if err != nil {
		return
	}
	g.maxPlayers = b

	err = binary.Read(reader, binary.LittleEndian, &g.cpuSpeed)
	if err != nil {
		return
	}

	g.mod, err = ReadPascalString(reader)
	if err != nil {
		return
	}

	g.serverType, err = ReadPascalString(reader)
	if err != nil {
		return
	}

	g.mission, err = ReadPascalString(reader)
	if err != nil {
		return
	}

	g.info, err = ReadPascalString(reader)
	if err != nil {
		return
	}

	b, err = reader.ReadByte()
	if err != nil {
		return
	}
	g.numTeams = b

	g.teamScoreHeader, err = ReadPascalString(reader)
	if err != nil {
		return
	}

	g.playerScoreHeader, err = ReadPascalString(reader)
	if err != nil {
		return
	}

	var teamName, teamScore string
	for i := uint8(0); i < g.numTeams; i++ {
		teamName, err = ReadPascalString(reader)
		if err != nil {
			return
		}

		teamScore, err = ReadPascalString(reader)
		if err != nil {
			return
		}

		g.teams = append(g.teams, Team{Name: teamName, Score: teamScore})
	}

	var ping, pl, team byte
	var playerName, playerScore string
	for i := uint8(0); i < g.numPlayers; i++ {
		ping, err = reader.ReadByte()
		if err != nil {
			return
		}

		pl, err = reader.ReadByte()
		if err != nil {
			return
		}

		team, err = reader.ReadByte()
		if err != nil {
			return
		}

		playerName, err = ReadPascalString(reader)
		if err != nil {
			return
		}

		playerScore, err = ReadPascalString(reader)
		if err != nil {
			return
		}

		g.players = append(g.players, Player{Ping: ping, PL: pl, Team: team, Name: playerName, Score: playerScore})
	}

	if reader.Len() != 0 {
		return fmt.Errorf("t1net.GameServer.Query: %d left over bytes", reader.Len())
	}

	return
}

func NewGameServer(address string) *GameServer {
	return &GameServer{address: address}
}
