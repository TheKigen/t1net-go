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
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// dynamicMockConn captures the sent packet and builds a response dynamically.
// Needed because query keys are random — the handler sees what was sent and echoes the key.
type dynamicMockConn struct {
	addr       net.Addr
	handler    func(sent []byte) []byte
	sentPacket []byte
	responded  bool
	closed     bool
}

func (d *dynamicMockConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	d.sentPacket = make([]byte, len(b))
	copy(d.sentPacket, b)
	return len(b), nil
}

func (d *dynamicMockConn) ReadFrom(b []byte) (int, net.Addr, error) {
	if d.responded {
		return 0, nil, &net.OpError{Op: "read", Err: fmt.Errorf("timeout")}
	}
	d.responded = true
	resp := d.handler(d.sentPacket)
	n := copy(b, resp)
	return n, d.addr, nil
}

func (d *dynamicMockConn) SetDeadline(t time.Time) error { return nil }
func (d *dynamicMockConn) Close() error                  { d.closed = true; return nil }

// dynamicMultiMockConn returns multiple response packets (for MasterQuery).
type dynamicMultiMockConn struct {
	addr       net.Addr
	handler    func(sent []byte) [][]byte
	sentPacket []byte
	responses  [][]byte
	readIndex  int
	closed     bool
}

func (d *dynamicMultiMockConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	d.sentPacket = make([]byte, len(b))
	copy(d.sentPacket, b)
	d.responses = d.handler(d.sentPacket)
	return len(b), nil
}

func (d *dynamicMultiMockConn) ReadFrom(b []byte) (int, net.Addr, error) {
	if d.readIndex >= len(d.responses) {
		return 0, nil, &net.OpError{Op: "read", Err: fmt.Errorf("timeout")}
	}
	resp := d.responses[d.readIndex]
	d.readIndex++
	n := copy(b, resp)
	return n, d.addr, nil
}

func (d *dynamicMultiMockConn) SetDeadline(t time.Time) error { return nil }
func (d *dynamicMultiMockConn) Close() error                  { d.closed = true; return nil }

// mockOpts returns a *QueryOptions with a mock dial function that returns the given conn.
func mockOpts(mock packetConn) *QueryOptions {
	return &QueryOptions{
		dialFunc: func(localAddr *net.UDPAddr) (packetConn, error) {
			return mock, nil
		},
	}
}

func TestApplyDefaults(t *testing.T) {
	t.Parallel()

	d := applyDefaults(nil)
	if d.Timeout != 5*time.Second {
		t.Errorf("nil opts timeout: got %v, want 5s", d.Timeout)
	}
	if d.MinPacketSize != DefaultMinPacketSize {
		t.Errorf("nil opts MinPacketSize: got %d, want %d", d.MinPacketSize, DefaultMinPacketSize)
	}
	if d.LocalAddress != "" {
		t.Errorf("nil opts LocalAddress: got %q, want empty", d.LocalAddress)
	}

	d = applyDefaults(&QueryOptions{})
	if d.Timeout != 5*time.Second {
		t.Errorf("zero opts timeout: got %v, want 5s", d.Timeout)
	}

	d = applyDefaults(&QueryOptions{Timeout: 2 * time.Second, MinPacketSize: 16, LocalAddress: "0.0.0.0:0"})
	if d.Timeout != 2*time.Second {
		t.Errorf("explicit timeout: got %v, want 2s", d.Timeout)
	}
	if d.MinPacketSize != 16 {
		t.Errorf("explicit MinPacketSize: got %d, want 16", d.MinPacketSize)
	}
	if d.LocalAddress != "0.0.0.0:0" {
		t.Errorf("explicit LocalAddress: got %q, want 0.0.0.0:0", d.LocalAddress)
	}
}

func TestParseScoreEntries(t *testing.T) {
	t.Parallel()

	result := &GameResult{
		ScoreEntries: []string{
			"00",                                                   // empty team marker
			"01Team Name\t\xa6Count\t\xd6Score",                   // team header (skipped)
			"00Blood Eagle\t  1\t  1",                              // team data
			"00Diamond Sword\t  1\t  1",                            // team data
			"00",                                                   // end of teams marker
			"11Player Name\toTeam\t\xa6Score\t\xcfPing\t\xefPL",   // player header (skipped)
			"10stefano\tBlood Eagle\t  7\t123\t0",                  // player data
			"10Noodles\tDiamond Sword\t  7\t39\t0",                 // player data
		},
	}

	parseScoreEntries(result)

	if result.NumTeams != 2 {
		t.Errorf("NumTeams: got %d, want 2", result.NumTeams)
	}
	if len(result.Teams) != 2 {
		t.Fatalf("Teams length: got %d, want 2", len(result.Teams))
	}
	if result.Teams[0].Name != "Blood Eagle" {
		t.Errorf("Teams[0].Name: got %q, want %q", result.Teams[0].Name, "Blood Eagle")
	}
	if result.Teams[0].Score != "1" {
		t.Errorf("Teams[0].Score: got %q, want %q", result.Teams[0].Score, "1")
	}
	if result.Teams[1].Name != "Diamond Sword" {
		t.Errorf("Teams[1].Name: got %q, want %q", result.Teams[1].Name, "Diamond Sword")
	}

	if len(result.Players) != 2 {
		t.Fatalf("Players length: got %d, want 2", len(result.Players))
	}
	if result.Players[0].Name != "stefano" {
		t.Errorf("Players[0].Name: got %q, want %q", result.Players[0].Name, "stefano")
	}
	if result.Players[0].Score != "7" {
		t.Errorf("Players[0].Score: got %q, want %q", result.Players[0].Score, "7")
	}
	if result.Players[1].Name != "Noodles" {
		t.Errorf("Players[1].Name: got %q, want %q", result.Players[1].Name, "Noodles")
	}
}

func TestParseScoreEntriesEmpty(t *testing.T) {
	t.Parallel()

	result := &GameResult{}
	parseScoreEntries(result)

	if result.NumTeams != 0 {
		t.Errorf("NumTeams: got %d, want 0", result.NumTeams)
	}
	if len(result.Teams) != 0 {
		t.Errorf("Teams should be nil, got %d", len(result.Teams))
	}
	if len(result.Players) != 0 {
		t.Errorf("Players should be nil, got %d", len(result.Players))
	}
}

func buildGameSpyResponse(key uint16) []byte {
	sendBuffer := []byte{
		0x63,       // Reply
		0x00, 0x00, // Key
		0x62,                            // Echo request type
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
		7, 'C', 'i', 't', 'i', 'z', 'e', 'n', 0x0,
		5, 'E', 'n', 'e', 'm', 'y', 0x0,
		10, 'G', 'r', 'e', 'e', 'n', 's', 'k', 'i', 'n', 's', 0x0,
		5, 'E', 'n', 'e', 'm', 'y', 0x0,
		6, 'U', 'n', 'd', 'e', 'a', 'd', 0x0,
		3, 'E', 'l', 'f', 0x0,
		8, 'M', 'i', 'n', 'o', 't', 'a', 'u', 'r', 0x0,
		4, 'U', 'b', 'e', 'r', 0x0,
		0x1c, 0x1, 0x0,
		0x2, 0x74, 0x64,
		0x20, 0x74, 0x64, 0x9,
		0x4f, 0x6c, 0x64, 0x20, 0x4a, 0x61, 0x74, 0x65, 0x6e, 0x20, 0x4f, 0x75, 0x74, 0x70, 0x6f, 0x73, 0x74, 0x9,
		0x31, 0x33, 0x34, 0x9, 0x69, 0x64, 0x6c, 0x65, 0x20, 0x20, 0x20, 0xa, 0x0, 0x0, 0x7, 0x70, 0x68, 0x61, 0x6e,
		0x74, 0x6f, 0x6d, 0x20, 0x70, 0x68, 0x61, 0x6e, 0x74, 0x6f, 0x6d, 0x9, 0x4b, 0x65, 0x6c, 0x64, 0x72, 0x69,
		0x6e, 0x20, 0x54, 0x6f, 0x77, 0x6e, 0x9, 0x32, 0x9, 0x69, 0x64, 0x6c, 0x65, 0x20, 0x20, 0x20, 0x20, 0x20,
	}
	binary.LittleEndian.PutUint16(sendBuffer[1:3], key)
	return sendBuffer
}

func TestGameSpyQuery(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			key := binary.LittleEndian.Uint16(sent[1:3])
			return buildGameSpyResponse(key)
		},
	}

	result, err := GameSpyQuery("127.0.0.1:28000", mockOpts(mock))
	if err != nil {
		t.Fatal(err)
	}
	if result.Game != "Tribes" {
		t.Errorf("Game: got %q, want %q", result.Game, "Tribes")
	}
	if result.Version != "1.30" {
		t.Errorf("Version: got %q, want %q", result.Version, "1.30")
	}
	if result.Name != "My Gameserver" {
		t.Errorf("Name: got %q, want %q", result.Name, "My Gameserver")
	}
	if result.Dedicated != true {
		t.Error("Dedicated: got false, want true")
	}
	if result.Password != false {
		t.Error("Password: got true, want false")
	}
	if result.NumPlayers != 2 {
		t.Errorf("NumPlayers: got %d, want 2", result.NumPlayers)
	}
	if result.MaxPlayers != 96 {
		t.Errorf("MaxPlayers: got %d, want 96", result.MaxPlayers)
	}
	if result.CPUSpeed != 3500 {
		t.Errorf("CPUSpeed: got %d, want 3500", result.CPUSpeed)
	}
	if result.Mod != "rpg base" {
		t.Errorf("Mod: got %q, want %q", result.Mod, "rpg base")
	}
	if result.ServerType != "tribesrp" {
		t.Errorf("ServerType: got %q, want %q", result.ServerType, "tribesrp")
	}
	if result.Mission != "worlds_rpg" {
		t.Errorf("Mission: got %q, want %q", result.Mission, "worlds_rpg")
	}
	if result.Info != "My Info" {
		t.Errorf("Info: got %q, want %q", result.Info, "My Info")
	}
	if result.NumTeams != 8 {
		t.Errorf("NumTeams: got %d, want 8", result.NumTeams)
	}
	if len(result.Teams) != 8 {
		t.Fatalf("Teams length: got %d, want 8", len(result.Teams))
	}
	if result.Teams[0].Name != "Citizen" {
		t.Errorf("Teams[0].Name: got %q, want %q", result.Teams[0].Name, "Citizen")
	}
	if len(result.Players) != 2 {
		t.Fatalf("Players length: got %d, want 2", len(result.Players))
	}
	if result.Players[0].Name != "td" {
		t.Errorf("Players[0].Name: got %q, want %q", result.Players[0].Name, "td")
	}
	if result.Players[0].Ping != 0x1c {
		t.Errorf("Players[0].Ping: got %d, want %d", result.Players[0].Ping, 0x1c)
	}
}

func TestGameSpyQueryKeyMismatch(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			key := binary.LittleEndian.Uint16(sent[1:3])
			return buildGameSpyResponse(key ^ 0xFFFF)
		},
	}
	_, err := GameSpyQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for key mismatch")
	}
	if !strings.Contains(err.Error(), "key mismatch") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGameSpyQueryReplyTooShort(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr:    remoteAddr,
		handler: func(sent []byte) []byte { return []byte{0x63, 0x00, 0x00, 0x62, 0x00} },
	}
	_, err := GameSpyQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for too-short reply")
	}
	if !strings.Contains(err.Error(), "too short") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGameSpyQueryBadResponseByte(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			resp := make([]byte, 20)
			resp[0] = 0xFF
			return resp
		},
	}
	_, err := GameSpyQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for bad response byte")
	}
	if !strings.Contains(err.Error(), "0x63") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGameSpyQueryBadByte3(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			resp := make([]byte, 20)
			resp[0] = 0x63
			binary.LittleEndian.PutUint16(resp[1:3], binary.LittleEndian.Uint16(sent[1:3]))
			resp[3] = 0xFF // wrong byte 3
			return resp
		},
	}
	_, err := GameSpyQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for bad byte 3")
	}
	if !strings.Contains(err.Error(), "0x62") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGameSpyQueryLeftoverBytes(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			key := binary.LittleEndian.Uint16(sent[1:3])
			resp := buildGameSpyResponse(key)
			return append(resp, 0xFF, 0xFF)
		},
	}
	_, err := GameSpyQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for leftover bytes")
	}
	if !strings.Contains(err.Error(), "left over bytes") {
		t.Errorf("unexpected error: %v", err)
	}
}

func buildPingResponse(key uint16, name string, numPlayers, maxPlayers byte) []byte {
	resp := make([]byte, 40)
	resp[0] = 0x10
	resp[1] = 0x04
	resp[2] = 0xFF
	resp[3] = 0xf0
	binary.LittleEndian.PutUint16(resp[4:6], key)
	resp[6] = maxPlayers
	resp[7] = numPlayers
	binary.LittleEndian.PutUint16(resp[8:10], 1)
	copy(resp[10:], []byte(name))
	return resp
}

func TestPingQuery(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			key := binary.LittleEndian.Uint16(sent[4:6])
			return buildPingResponse(key, "Test Server", 5, 32)
		},
	}
	result, err := PingQuery("127.0.0.1:28000", mockOpts(mock))
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "Test Server" {
		t.Errorf("Name: got %q, want %q", result.Name, "Test Server")
	}
	if result.NumPlayers != 5 {
		t.Errorf("NumPlayers: got %d, want 5", result.NumPlayers)
	}
	if result.MaxPlayers != 32 {
		t.Errorf("MaxPlayers: got %d, want 32", result.MaxPlayers)
	}
	if result.PacketVersion != 1 {
		t.Errorf("PacketVersion: got %d, want 1", result.PacketVersion)
	}
}

func TestPingQueryVerifyByte(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			key := binary.LittleEndian.Uint16(sent[4:6])
			resp := buildPingResponse(key, "Test", 0, 0)
			resp[3] = 0x00
			return resp
		},
	}
	_, err := PingQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for wrong verify byte")
	}
	if !strings.Contains(err.Error(), "0xf0") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPingQueryKeyMismatch(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			key := binary.LittleEndian.Uint16(sent[4:6])
			return buildPingResponse(key^0xFFFF, "Test", 0, 0)
		},
	}
	_, err := PingQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for key mismatch")
	}
	if !strings.Contains(err.Error(), "key mismatch") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPingQueryReplyTooShort(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr:    remoteAddr,
		handler: func(sent []byte) []byte { return []byte{0x10, 0x04, 0xFF, 0xf0, 0x00} },
	}
	_, err := PingQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for too-short reply")
	}
	if !strings.Contains(err.Error(), "too short") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPingQueryBadByte0(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			key := binary.LittleEndian.Uint16(sent[4:6])
			resp := buildPingResponse(key, "Test", 0, 0)
			resp[0] = 0xFF // wrong byte 0
			return resp
		},
	}
	_, err := PingQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for bad byte 0")
	}
	if !strings.Contains(err.Error(), "0x10") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPingQueryBadByte1(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			key := binary.LittleEndian.Uint16(sent[4:6])
			resp := buildPingResponse(key, "Test", 0, 0)
			resp[1] = 0xFF // wrong byte 1
			return resp
		},
	}
	_, err := PingQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for bad byte 1")
	}
	if !strings.Contains(err.Error(), "0x04") {
		t.Errorf("unexpected error: %v", err)
	}
}

func buildGameInfoResponse(key uint16, payload []byte) []byte {
	header := make([]byte, 8)
	header[0] = 0x10
	header[1] = 0x08
	header[2] = 0x01
	header[3] = 0x01
	binary.LittleEndian.PutUint16(header[4:6], key)
	resp := make([]byte, len(header)+len(payload))
	copy(resp, header)
	copy(resp[len(header):], payload)
	return resp
}

func TestGameInfoQuery(t *testing.T) {
	gameInfoPayload := []byte{
		0x01, 0x00, 0x0d, 0x56, 0xc6, 0xbb, 0x6f, 0x09, 0xc4, 0x52, 0x14, 0x2f,
		0xac, 0xbe, 0xc1, 0x4d, 0xdf, 0xa4, 0xb7, 0x02, 0x90, 0x09, 0x5d, 0xc9,
		0x14, 0x92, 0xf0, 0x07, 0x00, 0x80, 0x25, 0x00, 0x00, 0x24, 0xd8, 0x65,
		0x7b, 0x03, 0x43, 0x54, 0x46, 0x11, 0xfc, 0xf3, 0x03, 0x2b, 0xfa, 0xd8,
		0x0b, 0xa6, 0x58, 0xed, 0x2b, 0xf8, 0x25, 0xfa, 0xc5, 0x14, 0xab, 0x3d,
	}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			key := binary.LittleEndian.Uint16(sent[4:6])
			return buildGameInfoResponse(key, gameInfoPayload)
		},
	}
	result, err := GameInfoQuery("127.0.0.1:28000", mockOpts(mock))
	if err != nil {
		t.Fatal(err)
	}
	if result.PacketVersion != 1 {
		t.Errorf("PacketVersion: got %d, want 1", result.PacketVersion)
	}
	if result.Game != "Tribes" {
		t.Errorf("Game: got %q, want %q", result.Game, "Tribes")
	}
	if result.Name != "Test Server" {
		t.Errorf("Name: got %q, want %q", result.Name, "Test Server")
	}
	if result.NumPlayers != 5 {
		t.Errorf("NumPlayers: got %d, want 5", result.NumPlayers)
	}
	if result.MaxPlayers != 32 {
		t.Errorf("MaxPlayers: got %d, want 32", result.MaxPlayers)
	}
	if result.Mission != "Raindance" {
		t.Errorf("Mission: got %q, want %q", result.Mission, "Raindance")
	}
	if result.Dedicated != true {
		t.Error("Dedicated: got false, want true")
	}
	if result.CPUSpeed != 2400 {
		t.Errorf("CPUSpeed: got %d, want 2400", result.CPUSpeed)
	}
	if result.Mod != "base" {
		t.Errorf("Mod: got %q, want %q", result.Mod, "base")
	}
	if result.ServerType != "CTF" {
		t.Errorf("ServerType: got %q, want %q", result.ServerType, "CTF")
	}
	if result.Info != "Welcome!" {
		t.Errorf("Info: got %q, want %q", result.Info, "Welcome!")
	}
	if result.TeamScoreHeader != "Score" {
		t.Errorf("TeamScoreHeader: got %q, want %q", result.TeamScoreHeader, "Score")
	}
	if result.PlayerScoreHeader != "Name\tScore" {
		t.Errorf("PlayerScoreHeader: got %q, want %q", result.PlayerScoreHeader, "Name\tScore")
	}
}

func TestGameInfoQueryReplyTooShort(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr:    remoteAddr,
		handler: func(sent []byte) []byte { return []byte{0x10, 0x08, 0x01, 0x01, 0x00} },
	}
	_, err := GameInfoQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for too-short reply")
	}
	if !strings.Contains(err.Error(), "too short") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGameInfoQueryBadByte0(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			resp := make([]byte, 16)
			resp[0] = 0xFF // wrong byte 0
			resp[1] = 0x08
			return resp
		},
	}
	_, err := GameInfoQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for bad byte 0")
	}
	if !strings.Contains(err.Error(), "0x10") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGameInfoQueryBadByte1(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			resp := make([]byte, 16)
			resp[0] = 0x10
			resp[1] = 0xFF // wrong byte 1
			return resp
		},
	}
	_, err := GameInfoQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for bad byte 1")
	}
	if !strings.Contains(err.Error(), "0x08") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGameInfoQueryKeyMismatch(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			key := binary.LittleEndian.Uint16(sent[4:6])
			return buildGameInfoResponse(key^0xFFFF, []byte{0x01, 0x00})
		},
	}
	_, err := GameInfoQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for key mismatch")
	}
	if !strings.Contains(err.Error(), "key mismatch") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGameInfoQueryWithScores(t *testing.T) {
	gameInfoPayloadWithScores := []byte{
		0x01, 0x00, 0x0d, 0x56, 0xc6, 0xbb, 0x6f, 0x09, 0xc4, 0x52, 0x14, 0x33,
		0x4c, 0xb1, 0xda, 0xdf, 0xf4, 0x4d, 0x7a, 0x13, 0x80, 0xd8, 0xd0, 0xa5,
		0x63, 0xbc, 0x61, 0xcd, 0x06, 0x6f, 0x01, 0x01, 0xb8, 0x0b, 0x00, 0x00,
		0x0f, 0xda, 0xcc, 0x1b, 0xbc, 0x19, 0x18, 0xa2, 0x32, 0x5a, 0xe0, 0xef,
		0xc7, 0xd0, 0x09, 0xab, 0x4f, 0x34, 0xc5, 0x6a, 0x7f, 0x61, 0x6f, 0x64,
		0xed, 0x6d, 0x8a, 0xd5, 0xbe, 0x41, 0x9c, 0xc0, 0x6a, 0x62, 0x83, 0x38,
		0x81, 0xd5, 0xf6, 0x01,
	}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			key := binary.LittleEndian.Uint16(sent[4:6])
			return buildGameInfoResponse(key, gameInfoPayloadWithScores)
		},
	}
	result, err := GameInfoQuery("127.0.0.1:28000", mockOpts(mock))
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "Score Server" {
		t.Errorf("Name: got %q, want %q", result.Name, "Score Server")
	}
	if result.Password != true {
		t.Error("Password: got false, want true")
	}
	if result.CPUSpeed != 3000 {
		t.Errorf("CPUSpeed: got %d, want 3000", result.CPUSpeed)
	}
	if len(result.ScoreEntries) != 2 {
		t.Fatalf("ScoreEntries length: got %d, want 2", len(result.ScoreEntries))
	}
	if result.ScoreEntries[0] != "Entry1" {
		t.Errorf("ScoreEntries[0]: got %q, want %q", result.ScoreEntries[0], "Entry1")
	}
	if result.ScoreEntries[1] != "Entry2" {
		t.Errorf("ScoreEntries[1]: got %q, want %q", result.ScoreEntries[1], "Entry2")
	}
}

func buildMasterResponse1(key uint16) []byte {
	sendBuffer := []byte{
		0x10, 0x6, 1, 2, 0x00, 0x00, 0x0, 0x66,
		13, 'T', 'r', 'i', 'b', 'e', 's', ' ', 'M', 'a', 's', 't', 'e', 'r',
		9, 'T', 'e', 's', 't', ' ', 'M', 'O', 'T', 'D',
		0, 42, // Server Count
		0x6, 0x43, 0xde, 0x8a, 0x2e, 0x67, 0x6d,
		0x6, 0x18, 0x24, 0xaf, 0x99, 0x61, 0x6d,
		0x6, 0x2d, 0x22, 0xf, 0x5a, 0x61, 0x6d,
		0x6, 0x6b, 0x5, 0xc3, 0xcd, 0x61, 0x6d,
		0x6, 0x6b, 0xad, 0xa7, 0x7c, 0x61, 0x6d,
		0x6, 0x6b, 0xad, 0xa7, 0x6d, 0x61, 0x6d,
		0x6, 0xae, 0x32, 0xa7, 0xa, 0x64, 0x6d,
		0x6, 0x2d, 0x4f, 0x89, 0x6d, 0x61, 0x6d,
		0x6, 0xad, 0x1a, 0xf8, 0x72, 0x61, 0x6d,
		0x6, 0xcf, 0x94, 0xd, 0x84, 0x66, 0x6d,
		0x6, 0x88, 0x24, 0x5b, 0xe, 0x61, 0x6d,
		0x6, 0xd8, 0x80, 0x96, 0xd0, 0x61, 0x6d,
		0x6, 0x6b, 0xad, 0xa7, 0x6d, 0x62, 0x6d,
		0x6, 0xae, 0x37, 0x58, 0xbe, 0x61, 0x6d,
		0x6, 0xae, 0x32, 0xa7, 0xa, 0x61, 0x6d,
		0x6, 0x49, 0x5a, 0x18, 0xc3, 0x61, 0x6d,
		0x6, 0x2d, 0x22, 0xf, 0x5a, 0x63, 0x6d,
		0x6, 0xae, 0x32, 0xa7, 0xa, 0x63, 0x6d,
		0x6, 0x8b, 0x63, 0xfd, 0x23, 0x61, 0x6d,
		0x6, 0xae, 0x32, 0xa7, 0xa, 0x62, 0x6d,
		0x6, 0x12, 0xda, 0x1e, 0x7, 0x61, 0x6d,
		0x6, 0x90, 0xca, 0x36, 0x93, 0x65, 0x6d,
		0x6, 0x6b, 0xad, 0xa7, 0x71, 0xc5, 0x6d,
		0x6, 0x2d, 0x3f, 0x41, 0xf6, 0x65, 0x6d,
		0x6, 0x2d, 0x22, 0xf, 0x5a, 0x62, 0x6d,
		0x6, 0xc, 0xea, 0x96, 0xd6, 0x61, 0x6d,
		0x6, 0xae, 0x32, 0xa7, 0xa, 0xc2, 0x6d,
		0x6, 0x6b, 0xad, 0xa7, 0x71, 0xc6, 0x6d,
		0x6, 0x9f, 0x2, 0x2e, 0x79, 0x61, 0x6d,
		0x6, 0xae, 0x37, 0x58, 0xbe, 0x66, 0x6d,
		0x6, 0xae, 0x37, 0x58, 0xbe, 0xbb, 0xa1,
		0x6, 0x4b, 0x83, 0xaf, 0x5c, 0x61, 0x6d,
		0x6, 0x4a, 0x33, 0x1, 0x7e, 0x61, 0x6d,
		0x6, 0xae, 0x37, 0x58, 0xbe, 0x7b, 0x94,
		0x6, 0xae, 0x37, 0x58, 0xbe, 0xcf, 0x74,
		0x6, 0xae, 0x37, 0x58, 0xbe, 0xed, 0x3,
		0x6, 0xae, 0x37, 0x58, 0xbe, 0x65, 0x6d,
		0x6, 0xae, 0x37, 0x58, 0xbe, 0x68, 0x6d,
		0x6, 0xae, 0x37, 0x58, 0xbe, 0x6b, 0x6d,
		0x6, 0x4b, 0x83, 0xaf, 0x5c, 0x62, 0x6d,
		0x6, 0xae, 0x37, 0x58, 0xbe, 0xef, 0x3,
		0x6, 0xae, 0x37, 0x58, 0xbe, 0xee, 0x3,
	}
	binary.LittleEndian.PutUint16(sendBuffer[4:6], key)
	return sendBuffer
}

func buildMasterResponse2(key uint16) []byte {
	sendBuffer := []byte{
		0x10, 0x6, 2, 2, 0x00, 0x00, 0x0, 0x66,
		13, 'T', 'r', 'i', 'b', 'e', 's', ' ', 'M', 'a', 's', 't', 'e', 'r',
		9, 'T', 'e', 's', 't', ' ', 'M', 'O', 'T', 'D',
		0, 2, // Server Count
		6, 12, 13, 14, 15, 97, 109,
		6, 22, 23, 24, 25, 97, 109,
	}
	binary.LittleEndian.PutUint16(sendBuffer[4:6], key)
	return sendBuffer
}

func TestMasterQuery(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	multiMock := &dynamicMultiMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) [][]byte {
			key := binary.LittleEndian.Uint16(sent[4:6])
			return [][]byte{buildMasterResponse1(key), buildMasterResponse2(key)}
		},
	}

	result, err := MasterQuery("127.0.0.1:28000", mockOpts(multiMock))
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "Tribes Master" {
		t.Errorf("Name: got %q, want %q", result.Name, "Tribes Master")
	}
	if result.MOTD != "Test MOTD" {
		t.Errorf("MOTD: got %q, want %q", result.MOTD, "Test MOTD")
	}
	if result.ServerCount != 44 {
		t.Errorf("ServerCount: got %d, want 44", result.ServerCount)
	}
	if len(result.Servers) != 44 {
		t.Fatalf("Servers length: got %d, want 44", len(result.Servers))
	}
	expectedServers := []string{
		"67.222.138.46:28007", "24.36.175.153:28001", "45.34.15.90:28001",
		"107.5.195.205:28001", "107.173.167.124:28001", "107.173.167.109:28001",
		"174.50.167.10:28004", "45.79.137.109:28001", "173.26.248.114:28001",
		"207.148.13.132:28006", "136.36.91.14:28001", "216.128.150.208:28001",
		"107.173.167.109:28002", "174.55.88.190:28001", "174.50.167.10:28001",
		"73.90.24.195:28001", "45.34.15.90:28003", "174.50.167.10:28003",
		"139.99.253.35:28001", "174.50.167.10:28002", "18.218.30.7:28001",
		"144.202.54.147:28005", "107.173.167.113:28101", "45.63.65.246:28005",
		"45.34.15.90:28002", "12.234.150.214:28001", "174.50.167.10:28098",
		"107.173.167.113:28102", "159.2.46.121:28001", "174.55.88.190:28006",
		"174.55.88.190:41403", "75.131.175.92:28001", "74.51.1.126:28001",
		"174.55.88.190:38011", "174.55.88.190:29903", "174.55.88.190:1005",
		"174.55.88.190:28005", "174.55.88.190:28008", "174.55.88.190:28011",
		"75.131.175.92:28002", "174.55.88.190:1007", "174.55.88.190:1006",
		"12.13.14.15:28001", "22.23.24.25:28001",
	}
	for i, exp := range expectedServers {
		if result.Servers[i] != exp {
			t.Errorf("Servers[%d]: got %q, want %q", i, result.Servers[i], exp)
		}
	}
}

func TestMasterQueryKeyMismatch(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			resp := []byte{0x10, 0x6, 1, 1, 0xFF, 0xFF, 0x0, 0x66, 1, 'A', 1, 'B', 0, 0}
			return resp
		},
	}
	_, err := MasterQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for key mismatch")
	}
	if !strings.Contains(err.Error(), "key mismatch") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMasterQueryBadVersionByte(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			key := binary.LittleEndian.Uint16(sent[4:6])
			resp := []byte{0xFF, 0x6, 1, 1, 0x00, 0x00, 0x0, 0x66, 1, 'A', 1, 'B', 0, 0}
			binary.LittleEndian.PutUint16(resp[4:6], key)
			return resp
		},
	}
	_, err := MasterQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for bad version byte")
	}
	if !strings.Contains(err.Error(), "0x10") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMasterQueryBadTypeByte(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			key := binary.LittleEndian.Uint16(sent[4:6])
			resp := []byte{0x10, 0xFF, 1, 1, 0x00, 0x00, 0x0, 0x66, 1, 'A', 1, 'B', 0, 0}
			binary.LittleEndian.PutUint16(resp[4:6], key)
			return resp
		},
	}
	_, err := MasterQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for bad type byte")
	}
	if !strings.Contains(err.Error(), "0x06") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMasterQueryInvalidPacketNumber(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			key := binary.LittleEndian.Uint16(sent[4:6])
			resp := []byte{0x10, 0x6, 0, 1, 0x00, 0x00, 0x0, 0x66, 1, 'A', 1, 'B', 0, 0}
			binary.LittleEndian.PutUint16(resp[4:6], key)
			return resp
		},
	}
	_, err := MasterQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for invalid packet number")
	}
	if !strings.Contains(err.Error(), "invalid packet number") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMasterQueryInvalidPacketTotal(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			key := binary.LittleEndian.Uint16(sent[4:6])
			resp := []byte{0x10, 0x6, 1, 0, 0x00, 0x00, 0x0, 0x66, 1, 'A', 1, 'B', 0, 0}
			binary.LittleEndian.PutUint16(resp[4:6], key)
			return resp
		},
	}
	_, err := MasterQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for invalid packet total")
	}
	if !strings.Contains(err.Error(), "invalid total packet number") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMasterQueryPacketNumberGreaterThanTotal(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			key := binary.LittleEndian.Uint16(sent[4:6])
			resp := []byte{0x10, 0x6, 3, 1, 0x00, 0x00, 0x0, 0x66, 1, 'A', 1, 'B', 0, 0}
			binary.LittleEndian.PutUint16(resp[4:6], key)
			return resp
		},
	}
	_, err := MasterQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for packet number > total")
	}
	if !strings.Contains(err.Error(), "greater than total") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMasterQueryBadByte6(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			key := binary.LittleEndian.Uint16(sent[4:6])
			resp := []byte{0x10, 0x6, 1, 1, 0x00, 0x00, 0xFF, 0x66, 1, 'A', 1, 'B', 0, 0}
			binary.LittleEndian.PutUint16(resp[4:6], key)
			return resp
		},
	}
	_, err := MasterQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for bad byte 6")
	}
	if !strings.Contains(err.Error(), "0x00") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMasterQueryBadByte7(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			key := binary.LittleEndian.Uint16(sent[4:6])
			resp := []byte{0x10, 0x6, 1, 1, 0x00, 0x00, 0x0, 0xFF, 1, 'A', 1, 'B', 0, 0}
			binary.LittleEndian.PutUint16(resp[4:6], key)
			return resp
		},
	}
	_, err := MasterQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for bad byte 7")
	}
	if !strings.Contains(err.Error(), "0x66") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMasterQueryLeftoverBytes(t *testing.T) {
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 28000}
	mock := &dynamicMockConn{
		addr: remoteAddr,
		handler: func(sent []byte) []byte {
			key := binary.LittleEndian.Uint16(sent[4:6])
			resp := []byte{
				0x10, 0x6, 1, 1, 0x00, 0x00, 0x0, 0x66,
				1, 'A', 1, 'B',
				0, 0, // server count = 0
				0xFF, // extra byte
			}
			binary.LittleEndian.PutUint16(resp[4:6], key)
			return resp
		},
	}
	_, err := MasterQuery("127.0.0.1:28000", mockOpts(mock))
	if err == nil {
		t.Fatal("expected error for leftover bytes")
	}
	if !strings.Contains(err.Error(), "left over bytes") {
		t.Errorf("unexpected error: %v", err)
	}
}
