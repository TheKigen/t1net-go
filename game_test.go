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
	"math/rand"
	"net"
	"testing"
	"time"
)

func TestGameServer(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	game := NewGameServer("127.0.0.1:28999")

	s, err := net.ResolveUDPAddr("udp4", "127.0.0.1:28999")
	if err != nil {
		t.Fatal(err)
	}
	c, err := net.ListenUDP("udp4", s)
	if err != nil {
		t.Fatal(err)
	}
	defer func(c *net.UDPConn) {
		err := c.Close()
		if err != nil {
			t.Fatal(err)
		}
	}(c)

	go func() {
		readBuffer := make([]byte, 64)
		err := c.SetDeadline(time.Now().Add(1 * time.Second))
		if err != nil {
			t.Error(err)
			return
		}
		n, addr, err := c.ReadFromUDP(readBuffer)
		if err != nil {
			t.Error(err)
			return
		}
		if n != 3 {
			t.Errorf("Wrong query length, %d", n)
			return
		}
		expected := []byte{0x62, 0x00, 0x00}
		// Key is random and must match.
		expected[1] = readBuffer[1]
		expected[2] = readBuffer[2]

		if bytes.Compare(expected, readBuffer[0:n]) != 0 {
			t.Error(readBuffer)
			return
		}

		sendBuffer := []byte{
			0x63,       // Reply
			0x00, 0x00, // Key
			0x62,                            // Unknown
			6, 'T', 'r', 'i', 'b', 'e', 's', // Game
			4, '1', '.', '3', '0', // Version
			13, 'M', 'y', ' ', 'G', 'a', 'm', 'e', 's', 'e', 'r', 'v', 'e', 'r', // Name
			0x1,       // Dedicated
			0x0,       // Password
			2,         // Num Players
			96,        // Max Players
			0xac, 0xd, // CPU Speed
			8, 'r', 'p', 'g', ' ', 'b', 'a', 's', 'e', // Mod
			8, 't', 'r', 'i', 'b', 'e', 's', 'r', 'p', // ServerType
			10, 'w', 'o', 'r', 'l', 'd', 's', '_', 'r', 'p', 'g', // Mission
			7, 'M', 'y', ' ', 'I', 'n', 'f', 'o', // Info
			0x8, // Num Teams
			0x0, // Team Score Header
			23, 'N', 'a', 'm', 'e', '\t', 'P', 'Z', 'o', 'n', 'e', '\t', 0xc2, 'L', 'V', 'L', '\t', 0xdb, 'S',
			't', 'a', 't', 'u', 's', // Player Score Header
			// Team Name / Score
			7, 'C', 'i', 't', 'i', 'z', 'e', 'n', 0x0,
			5, 'E', 'n', 'e', 'm', 'y', 0x0,
			10, 'G', 'r', 'e', 'e', 'n', 's', 'k', 'i', 'n', 's', 0x0,
			5, 'E', 'n', 'e', 'm', 'y', 0x0,
			6, 'U', 'n', 'd', 'e', 'a', 'd', 0x0,
			3, 'E', 'l', 'f', 0x0,
			8, 'M', 'i', 'n', 'o', 't', 'a', 'u', 'r', 0x0,
			4, 'U', 'b', 'e', 'r', 0x0,
			// Players
			0x1c,            // Ping
			0x1,             // PL
			0x0,             // Team
			0x2, 0x74, 0x64, // Name
			0x20, 0x74, 0x64, 0x9,
			0x4f, 0x6c, 0x64, 0x20, 0x4a, 0x61, 0x74, 0x65, 0x6e, 0x20, 0x4f, 0x75, 0x74, 0x70, 0x6f, 0x73, 0x74, 0x9,
			0x31, 0x33, 0x34, 0x9, 0x69, 0x64, 0x6c, 0x65, 0x20, 0x20, 0x20, 0xa, 0x0, 0x0, 0x7, 0x70, 0x68, 0x61, 0x6e,
			0x74, 0x6f, 0x6d, 0x20, 0x70, 0x68, 0x61, 0x6e, 0x74, 0x6f, 0x6d, 0x9, 0x4b, 0x65, 0x6c, 0x64, 0x72, 0x69,
			0x6e, 0x20, 0x54, 0x6f, 0x77, 0x6e, 0x9, 0x32, 0x9, 0x69, 0x64, 0x6c, 0x65, 0x20, 0x20, 0x20, 0x20, 0x20,
		}
		sendBuffer[1] = readBuffer[1]
		sendBuffer[2] = readBuffer[2]
		sent, err := c.WriteToUDP(sendBuffer, addr)
		if err != nil {
			t.Error(err)
			return
		}
		if sent != len(sendBuffer) {
			t.Error("Sent did not match sendBuffer length")
			return
		}
	}()

	err = game.Query()
	if err != nil {
		t.Fatal(err)
	}

	if game.Game() != "Tribes" {
		t.Errorf("game.Game(): %s != Tribes\n", game.Game())
	}
	if game.Version() != "1.30" {
		t.Errorf("game.Version(): %s != 1.30\n", game.Version())
	}
	if game.Name() != "My Gameserver" {
		t.Errorf("game.Name(): %s != My Gameserver\n", game.Game())
	}
	if game.Info() != "My Info" {
		t.Errorf("game.Info(): %s != My Info\n", game.Info())
	}
	if game.Mod() != "rpg base" {
		t.Errorf("game.Mod(): %s != rpg base\n", game.Mod())
	}
	if game.ServerType() != "tribesrp" {
		t.Errorf("game.ServerType(): %s != tribesrp\n", game.ServerType())
	}
	if game.Mission() != "worlds_rpg" {
		t.Errorf("game.Mission(): %s != worlds_rpg\n", game.Mission())
	}
	if game.NumTeams() != 8 {
		t.Errorf("game.NumTeams(): %d != 8", game.NumTeams())
	}
	if game.NumPlayers() != 2 {
		t.Errorf("game.NumPlayers(): %d != 2", game.NumPlayers())
	}
	if game.MaxPlayers() != 96 {
		t.Errorf("game.MaxPlayers(): %d != 96", game.MaxPlayers())
	}
	if game.Dedicated() != true {
		t.Error("game.Dedicated() != true")
	}
	if game.Password() != false {
		t.Error("game.Password() != false")
	}
	if game.TeamScoreHeader() != "" {
		t.Errorf("game.TeamScoreHeader(): %s != \"\"", game.TeamScoreHeader())
	}

	expectedTeams := []string{"Citizen", "Enemy", "Greenskins", "Enemy", "Undead", "Elf", "Minotaur", "Uber"}

	teams := game.Teams()

	if len(teams) != len(expectedTeams) {
		t.Errorf("game.Teams(): Length mismatch: %d != %d", len(teams), len(expectedTeams))
	}

	for i, team := range teams {
		if team.Name != expectedTeams[i] {
			t.Errorf("game.Teams(): Team name mismatch: %s != %s", team.Name, expectedTeams[i])
		}
		if team.Score != "" {
			t.Errorf("game.Teams(): Team score mismatch: %s != \"\"", team.Score)
		}
	}

	expectedPlayerNames := []string{"td", "phantom"}

	players := game.Players()

	for i, player := range players {
		if player.Name != expectedPlayerNames[i] {
			t.Errorf("game.Players(): Player name mismatch: %s != %s", player.Name, expectedPlayerNames[i])
		}
	}
}
