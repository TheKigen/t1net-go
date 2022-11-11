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

func (g *GameServer) Ping() time.Duration {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.ping
}

func (g *GameServer) Name() string {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.name
}

func (g *GameServer) Game() string {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.game
}

func (g *GameServer) Version() string {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.version
}

func (g *GameServer) Dedicated() bool {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.dedicated
}

func (g *GameServer) Password() bool {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.password
}

func (g *GameServer) NumPlayers() uint8 {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.numPlayers
}

func (g *GameServer) MaxPlayers() uint8 {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.maxPlayers
}

func (g *GameServer) CPUSpeed() uint16 {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.cpuSpeed
}

func (g *GameServer) Mod() string {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.mod
}

func (g *GameServer) ServerType() string {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.serverType
}

func (g *GameServer) Mission() string {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.mission
}

func (g *GameServer) Info() string {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.info
}

func (g *GameServer) NumTeams() uint8 {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.numTeams
}

func (g *GameServer) TeamScoreHeader() string {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.teamScoreHeader
}

func (g *GameServer) PlayerScoreHeader() string {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.playerScoreHeader
}

func (g *GameServer) Teams() []Team {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.teams
}

func (g *GameServer) Players() []Player {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.players
}

func (g *GameServer) Query() error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	s, err := net.ResolveUDPAddr("udp4", g.address)
	if err != nil {
		return err
	}

	g.ip = s.IP
	g.port = s.Port
	g.numTeams = 0
	g.numPlayers = 0
	g.maxPlayers = 0
	g.teams = nil
	g.players = nil

	c, err := net.DialUDP("udp4", nil, s)
	if err != nil {
		return err
	}

	defer func(c *net.UDPConn) {
		err := c.Close()
		if err != nil {
			panic(err)
		}
	}(c)

	key := uint16(rand.Uint32())
	sendBuffer := []byte{0x62, 0x00, 0x00}

	binary.BigEndian.PutUint16(sendBuffer[1:], key)

	g.queryTime = time.Now()
	_, err = c.Write(sendBuffer)
	if err != nil {
		return err
	}

	readBuffer := make([]byte, 2048)
	err = c.SetDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		return err
	}
	n, addr, err := c.ReadFromUDP(readBuffer)
	if err != nil {
		return err
	}

	g.ping = time.Since(g.queryTime)

	if !addr.IP.Equal(s.IP) || addr.Port != s.Port {
		return fmt.Errorf("t1net.GameServer.Query: Reply address mismatch: %s != %s", s.String(), addr.String())
	}

	if n < 20 {
		return fmt.Errorf("t1net.GameServer.Query: Reply packet length too short: %d < 20", n)
	}

	reader := bytes.NewReader(readBuffer[0:n])
	b, err := reader.ReadByte()
	if err != nil {
		return err
	}
	if b != 0x63 {
		return fmt.Errorf("t1net.GameServer.Query: Reply byte 0: %#v != 0x63", b)
	}

	var readKey uint16
	err = binary.Read(reader, binary.BigEndian, &readKey)
	if err != nil {
		return err
	}
	if key != readKey {
		return fmt.Errorf("t1net.GameServer.Query: Key mismatch: %d : %d", readKey, key)
	}

	b, err = reader.ReadByte()
	if err != nil {
		return err
	}
	if b != 0x62 {
		return fmt.Errorf("t1net.GameServer.Query: Reply byte 3: %#v != 0x62", b)
	}

	g.game, err = ReadPascalString(reader)
	if err != nil {
		return err
	}

	g.version, err = ReadPascalString(reader)
	if err != nil {
		return err
	}

	g.name, err = ReadPascalString(reader)
	if err != nil {
		return err
	}

	b, err = reader.ReadByte()
	if err != nil {
		return err
	}
	g.dedicated = b == 1

	b, err = reader.ReadByte()
	if err != nil {
		return err
	}
	g.password = b == 1

	b, err = reader.ReadByte()
	if err != nil {
		return err
	}
	g.numPlayers = b

	b, err = reader.ReadByte()
	if err != nil {
		return err
	}
	g.maxPlayers = b

	err = binary.Read(reader, binary.LittleEndian, &g.cpuSpeed)
	if err != nil {
		return err
	}

	g.mod, err = ReadPascalString(reader)
	if err != nil {
		return err
	}

	g.serverType, err = ReadPascalString(reader)
	if err != nil {
		return err
	}

	g.mission, err = ReadPascalString(reader)
	if err != nil {
		return err
	}

	g.info, err = ReadPascalString(reader)
	if err != nil {
		return err
	}

	b, err = reader.ReadByte()
	if err != nil {
		return err
	}
	g.numTeams = b

	g.teamScoreHeader, err = ReadPascalString(reader)
	if err != nil {
		return err
	}

	g.playerScoreHeader, err = ReadPascalString(reader)
	if err != nil {
		return err
	}

	g.teams = nil

	for i := uint8(0); i < g.numTeams; i++ {
		teamName, err := ReadPascalString(reader)
		if err != nil {
			return err
		}

		teamScore, err := ReadPascalString(reader)
		if err != nil {
			return err
		}

		g.teams = append(g.teams, Team{Name: teamName, Score: teamScore})
	}

	for i := uint8(0); i < g.numPlayers; i++ {
		ping, err := reader.ReadByte()
		if err != nil {
			return err
		}

		pl, err := reader.ReadByte()
		if err != nil {
			return err
		}

		team, err := reader.ReadByte()
		if err != nil {
			return err
		}

		playerName, err := ReadPascalString(reader)
		if err != nil {
			return err
		}

		playerScore, err := ReadPascalString(reader)
		if err != nil {
			return err
		}

		g.players = append(g.players, Player{Ping: ping, PL: pl, Team: team, Name: playerName, Score: playerScore})
	}

	if reader.Len() != 0 {
		return fmt.Errorf("t1net.GameServer.Query: %d left over bytes", reader.Len())
	}

	return nil
}

func NewGameServer(address string) *GameServer {
	return &GameServer{address: address}
}
