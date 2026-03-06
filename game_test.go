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
	"net"
	"strings"
	"testing"
	"time"
)

func buildGameResponse(key uint16) []byte {
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
	binary.BigEndian.PutUint16(sendBuffer[1:3], key)
	return sendBuffer
}

func startGameListener(t *testing.T, addr string, handler func(c *net.UDPConn, readBuffer []byte, n int, addr *net.UDPAddr)) *net.UDPConn {
	t.Helper()
	s, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		t.Fatal(err)
	}
	c, err := net.ListenUDP("udp4", s)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		readBuffer := make([]byte, 64)
		err := c.SetDeadline(time.Now().Add(1 * time.Second))
		if err != nil {
			t.Error(err)
			return
		}
		n, raddr, err := c.ReadFromUDP(readBuffer)
		if err != nil {
			t.Error(err)
			return
		}
		handler(c, readBuffer, n, raddr)
	}()
	return c
}

func TestGameServer(t *testing.T) {
	game := NewGameServer("127.0.0.1:28999")

	c := startGameListener(t, "127.0.0.1:28999", func(c *net.UDPConn, readBuffer []byte, n int, addr *net.UDPAddr) {
		if n != 8 {
			t.Errorf("Wrong query length, %d", n)
			return
		}
		expected := []byte{0x62, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		expected[1] = readBuffer[1]
		expected[2] = readBuffer[2]

		if !bytes.Equal(expected, readBuffer[0:n]) {
			t.Error(readBuffer)
			return
		}

		key := binary.BigEndian.Uint16(readBuffer[1:3])
		sendBuffer := buildGameResponse(key)
		sent, err := c.WriteToUDP(sendBuffer, addr)
		if err != nil {
			t.Error(err)
			return
		}
		if sent != len(sendBuffer) {
			t.Error("Sent did not match sendBuffer length")
			return
		}
	})
	defer func() {
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	err := game.Query(0, "")
	if err != nil {
		t.Fatal(err)
	}

	if game.Game() != "Tribes" {
		t.Errorf("game.Game(): %s != Tribes", game.Game())
	}
	if game.Version() != "1.30" {
		t.Errorf("game.Version(): %s != 1.30", game.Version())
	}
	if game.Name() != "My Gameserver" {
		t.Errorf("game.Name(): %s != My Gameserver", game.Name())
	}
	if game.Info() != "My Info" {
		t.Errorf("game.Info(): %s != My Info", game.Info())
	}
	if game.Mod() != "rpg base" {
		t.Errorf("game.Mod(): %s != rpg base", game.Mod())
	}
	if game.ServerType() != "tribesrp" {
		t.Errorf("game.ServerType(): %s != tribesrp", game.ServerType())
	}
	if game.Mission() != "worlds_rpg" {
		t.Errorf("game.Mission(): %s != worlds_rpg", game.Mission())
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
	if game.CPUSpeed() != 3500 {
		t.Errorf("game.CPUSpeed(): %d != 3500", game.CPUSpeed())
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
	if game.PlayerScoreHeader() == "" {
		t.Error("game.PlayerScoreHeader() is empty")
	}
	if game.Ping() <= 0 {
		t.Error("game.Ping() should be > 0")
	}
	if game.QueryTime().IsZero() {
		t.Error("game.QueryTime() should not be zero")
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

	if players[0].Ping != 0x1c {
		t.Errorf("players[0].Ping: %d != %d", players[0].Ping, 0x1c)
	}
	if players[0].PL != 0x1 {
		t.Errorf("players[0].PL: %d != %d", players[0].PL, 0x1)
	}
	if players[0].Team != 0x0 {
		t.Errorf("players[0].Team: %d != %d", players[0].Team, 0x0)
	}
}

func TestGameServerMinPacketSize(t *testing.T) {
	game := NewGameServer("127.0.0.1:28999")

	if game.MinPacketSize() != DefaultMinPacketSize {
		t.Errorf("default MinPacketSize: %d != %d", game.MinPacketSize(), DefaultMinPacketSize)
	}

	game.SetMinPacketSize(16)
	if game.MinPacketSize() != 16 {
		t.Errorf("MinPacketSize after set: %d != 16", game.MinPacketSize())
	}

	c := startGameListener(t, "127.0.0.1:28999", func(c *net.UDPConn, readBuffer []byte, n int, addr *net.UDPAddr) {
		if n != 16 {
			t.Errorf("Wrong query length with custom padding, got %d want 16", n)
			return
		}
		// Verify padding bytes are null
		for i := 3; i < n; i++ {
			if readBuffer[i] != 0x00 {
				t.Errorf("padding byte %d is %#v, expected 0x00", i, readBuffer[i])
				return
			}
		}

		key := binary.BigEndian.Uint16(readBuffer[1:3])
		sendBuffer := buildGameResponse(key)
		if _, err := c.WriteToUDP(sendBuffer, addr); err != nil {
			t.Error(err)
		}
	})
	defer func() {
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	err := game.Query(0, "")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGameServerMinPacketSizeZero(t *testing.T) {
	game := NewGameServer("127.0.0.1:28999")
	game.SetMinPacketSize(0)

	c := startGameListener(t, "127.0.0.1:28999", func(c *net.UDPConn, readBuffer []byte, n int, addr *net.UDPAddr) {
		if n != 3 {
			t.Errorf("Wrong query length with no padding, got %d want 3", n)
			return
		}

		key := binary.BigEndian.Uint16(readBuffer[1:3])
		sendBuffer := buildGameResponse(key)
		if _, err := c.WriteToUDP(sendBuffer, addr); err != nil {
			t.Error(err)
		}
	})
	defer func() {
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	err := game.Query(0, "")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGameServerInvalidAddress(t *testing.T) {
	game := NewGameServer("not_a_valid_address")
	err := game.Query(1*time.Second, "")
	if err == nil {
		t.Fatal("expected error for invalid address")
	}
}

func TestGameServerInvalidLocalAddress(t *testing.T) {
	game := NewGameServer("127.0.0.1:28999")
	err := game.Query(1*time.Second, "not_valid")
	if err == nil {
		t.Fatal("expected error for invalid local address")
	}
}

func TestGameServerTimeout(t *testing.T) {
	game := NewGameServer("127.0.0.1:28999")

	s, err := net.ResolveUDPAddr("udp4", "127.0.0.1:28999")
	if err != nil {
		t.Fatal(err)
	}
	c, err := net.ListenUDP("udp4", s)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	// Don't respond — let it timeout

	err = game.Query(100*time.Millisecond, "")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestGameServerReplyTooShort(t *testing.T) {
	game := NewGameServer("127.0.0.1:28999")

	c := startGameListener(t, "127.0.0.1:28999", func(c *net.UDPConn, readBuffer []byte, n int, addr *net.UDPAddr) {
		// Send a reply that's too short (< 20 bytes)
		sendBuffer := []byte{0x63, 0x00, 0x00, 0x62, 0x00}
		sendBuffer[1] = readBuffer[1]
		sendBuffer[2] = readBuffer[2]
		if _, err := c.WriteToUDP(sendBuffer, addr); err != nil {
			t.Error(err)
		}
	})
	defer func() {
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	err := game.Query(1*time.Second, "")
	if err == nil {
		t.Fatal("expected error for too-short reply")
	}
	if !strings.Contains(err.Error(), "too short") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGameServerBadResponseByte(t *testing.T) {
	game := NewGameServer("127.0.0.1:28999")

	c := startGameListener(t, "127.0.0.1:28999", func(c *net.UDPConn, readBuffer []byte, n int, addr *net.UDPAddr) {
		// Send reply with wrong first byte (not 0x63)
		sendBuffer := make([]byte, 20)
		sendBuffer[0] = 0xFF
		sendBuffer[1] = readBuffer[1]
		sendBuffer[2] = readBuffer[2]
		if _, err := c.WriteToUDP(sendBuffer, addr); err != nil {
			t.Error(err)
		}
	})
	defer func() {
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	err := game.Query(1*time.Second, "")
	if err == nil {
		t.Fatal("expected error for bad response byte")
	}
	if !strings.Contains(err.Error(), "0x63") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGameServerKeyMismatch(t *testing.T) {
	game := NewGameServer("127.0.0.1:28999")

	c := startGameListener(t, "127.0.0.1:28999", func(c *net.UDPConn, readBuffer []byte, n int, addr *net.UDPAddr) {
		sendBuffer := make([]byte, 20)
		sendBuffer[0] = 0x63
		// Use wrong key
		sendBuffer[1] = readBuffer[1] ^ 0xFF
		sendBuffer[2] = readBuffer[2] ^ 0xFF
		sendBuffer[3] = 0x62
		if _, err := c.WriteToUDP(sendBuffer, addr); err != nil {
			t.Error(err)
		}
	})
	defer func() {
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	err := game.Query(1*time.Second, "")
	if err == nil {
		t.Fatal("expected error for key mismatch")
	}
	if !strings.Contains(err.Error(), "Key mismatch") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGameServerBadByte3(t *testing.T) {
	game := NewGameServer("127.0.0.1:28999")

	c := startGameListener(t, "127.0.0.1:28999", func(c *net.UDPConn, readBuffer []byte, n int, addr *net.UDPAddr) {
		sendBuffer := make([]byte, 20)
		sendBuffer[0] = 0x63
		sendBuffer[1] = readBuffer[1]
		sendBuffer[2] = readBuffer[2]
		sendBuffer[3] = 0xFF // Wrong byte 3 (should be 0x62)
		if _, err := c.WriteToUDP(sendBuffer, addr); err != nil {
			t.Error(err)
		}
	})
	defer func() {
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	err := game.Query(1*time.Second, "")
	if err == nil {
		t.Fatal("expected error for bad byte 3")
	}
	if !strings.Contains(err.Error(), "0x62") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGameServerLeftoverBytes(t *testing.T) {
	game := NewGameServer("127.0.0.1:28999")

	c := startGameListener(t, "127.0.0.1:28999", func(c *net.UDPConn, readBuffer []byte, n int, addr *net.UDPAddr) {
		key := binary.BigEndian.Uint16(readBuffer[1:3])
		sendBuffer := buildGameResponse(key)
		// Append extra bytes
		sendBuffer = append(sendBuffer, 0xFF, 0xFF)
		if _, err := c.WriteToUDP(sendBuffer, addr); err != nil {
			t.Error(err)
		}
	})
	defer func() {
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	err := game.Query(1*time.Second, "")
	if err == nil {
		t.Fatal("expected error for leftover bytes")
	}
	if !strings.Contains(err.Error(), "left over bytes") {
		t.Errorf("unexpected error: %v", err)
	}
}
