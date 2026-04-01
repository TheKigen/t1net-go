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
	"math/rand"
	"time"
)

// PingQuery sends a Tribes 1 ping query to the given address and returns the parsed result.
// address must be in "host:port" format. opts may be nil for defaults.
func PingQuery(address string, opts *QueryOptions) (*PingResult, error) {
	cfg := applyDefaults(opts)

	conn, remoteAddr, err := openConn(address, cfg)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			_ = cerr
		}
	}()

	// Build 8-byte query packet: [0x10, 0x03, 0xFF, 0x00, key_lo, key_hi, 0x00, 0x00]
	key := uint16(rand.Uint32()) //nolint:gosec
	sendBuffer := []byte{0x10, 0x03, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00}
	binary.LittleEndian.PutUint16(sendBuffer[4:6], key)
	sendBuffer = PadPacket(sendBuffer, cfg.MinPacketSize)

	// Send.
	sendTime, err := sendQuery(conn, remoteAddr, sendBuffer)
	if err != nil {
		return nil, err
	}

	// Set deadline and read response.
	if err = conn.SetDeadline(time.Now().Add(cfg.Timeout)); err != nil {
		return nil, err
	}

	readBuffer := make([]byte, 512)
	n, err := readResponse(conn, remoteAddr, readBuffer)
	if err != nil {
		return nil, err
	}

	ping := time.Since(sendTime)

	// Minimum sanity check: need at least 10 bytes.
	if n < 10 {
		return nil, fmt.Errorf("reply too short: %d bytes", n)
	}

	if readBuffer[0] != 0x10 {
		return nil, fmt.Errorf("expected 0x10 at byte 0, got %#x", readBuffer[0])
	}

	if readBuffer[1] != 0x04 {
		return nil, fmt.Errorf("expected 0x04 at byte 1, got %#x", readBuffer[1])
	}

	if readBuffer[3] != 0xf0 {
		return nil, fmt.Errorf("expected 0xf0 at byte 3 (verify magic), got %#x", readBuffer[3])
	}

	// Bytes 4-5: key echo (little-endian).
	readKey := binary.LittleEndian.Uint16(readBuffer[4:6])
	if key != readKey {
		return nil, fmt.Errorf("key mismatch: sent %d, got %d", key, readKey)
	}

	result := &PingResult{
		Ping:          ping,
		MaxPlayers:    readBuffer[6],
		NumPlayers:    readBuffer[7],
		PacketVersion: binary.LittleEndian.Uint16(readBuffer[8:10]),
	}

	// Name: null-terminated starting at byte 10.
	nameEnd := 10
	for nameEnd < n && readBuffer[nameEnd] != 0 {
		nameEnd++
	}
	result.Name = string(readBuffer[10:nameEnd])

	return result, nil
}

// GameInfoQuery sends a Tribes 1 game info query to the given address and returns the parsed result.
// address must be in "host:port" format. opts may be nil for defaults.
func GameInfoQuery(address string, opts *QueryOptions) (*GameResult, error) {
	cfg := applyDefaults(opts)

	conn, remoteAddr, err := openConn(address, cfg)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			_ = cerr
		}
	}()

	// Build 8-byte query packet: [0x10, 0x07, 0xFF, 0x00, key_lo, key_hi, 0x00, 0x00]
	key := uint16(rand.Uint32()) //nolint:gosec
	sendBuffer := []byte{0x10, 0x07, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00}
	binary.LittleEndian.PutUint16(sendBuffer[4:6], key)
	sendBuffer = PadPacket(sendBuffer, cfg.MinPacketSize)

	// Send.
	sendTime, err := sendQuery(conn, remoteAddr, sendBuffer)
	if err != nil {
		return nil, err
	}

	// Set deadline and read response.
	if err = conn.SetDeadline(time.Now().Add(cfg.Timeout)); err != nil {
		return nil, err
	}

	readBuffer := make([]byte, 16384)
	n, err := readResponse(conn, remoteAddr, readBuffer)
	if err != nil {
		return nil, err
	}

	ping := time.Since(sendTime)

	// Minimum sanity check: need at least 8 bytes.
	if n < 8 {
		return nil, fmt.Errorf("reply too short: %d bytes", n)
	}

	if readBuffer[0] != 0x10 {
		return nil, fmt.Errorf("expected 0x10 at byte 0, got %#x", readBuffer[0])
	}

	if readBuffer[1] != 0x08 {
		return nil, fmt.Errorf("expected 0x08 at byte 1, got %#x", readBuffer[1])
	}

	// Bytes 4-5: key echo (little-endian).
	readKey := binary.LittleEndian.Uint16(readBuffer[4:6])
	if key != readKey {
		return nil, fmt.Errorf("key mismatch: sent %d, got %d", key, readKey)
	}

	// BitStream payload starts at byte 8.
	bs := newBitStreamReader(readBuffer[8:n])

	result := &GameResult{
		Ping: ping,
	}

	// packetVersion: uint16
	result.PacketVersion, err = bs.ReadUint16()
	if err != nil {
		return nil, err
	}

	// game: huff string
	result.Game, err = bs.ReadHuffString()
	if err != nil {
		return nil, err
	}

	// version: huff string
	result.Version, err = bs.ReadHuffString()
	if err != nil {
		return nil, err
	}

	// name: huff string
	result.Name, err = bs.ReadHuffString()
	if err != nil {
		return nil, err
	}

	// numPlayers: 8 bits
	numPlayers, err := bs.ReadBits(8)
	if err != nil {
		return nil, err
	}
	result.NumPlayers = uint8(numPlayers)

	// maxPlayers: 8 bits
	maxPlayers, err := bs.ReadBits(8)
	if err != nil {
		return nil, err
	}
	result.MaxPlayers = uint8(maxPlayers)

	// mission: huff string
	result.Mission, err = bs.ReadHuffString()
	if err != nil {
		return nil, err
	}

	// dedicated: 8 bits
	dedicated, err := bs.ReadBits(8)
	if err != nil {
		return nil, err
	}
	result.Dedicated = dedicated != 0

	// password: 8 bits
	password, err := bs.ReadBits(8)
	if err != nil {
		return nil, err
	}
	result.Password = password != 0

	// cpuSpeed: 32 bits
	cpuSpeed, err := bs.ReadBits(32)
	if err != nil {
		return nil, err
	}
	result.CPUSpeed = cpuSpeed

	// mod: huff string
	result.Mod, err = bs.ReadHuffString()
	if err != nil {
		return nil, err
	}

	// serverType: huff string
	result.ServerType, err = bs.ReadHuffString()
	if err != nil {
		return nil, err
	}

	// info: huff string
	result.Info, err = bs.ReadHuffString()
	if err != nil {
		return nil, err
	}

	// teamScoreHeader: huff string
	result.TeamScoreHeader, err = bs.ReadHuffString()
	if err != nil {
		return nil, err
	}

	// playerScoreHeader: huff string
	result.PlayerScoreHeader, err = bs.ReadHuffString()
	if err != nil {
		return nil, err
	}

	// Score entries: loop while BitsRemaining() > 9, read huff string, break on empty.
	result.ScoreEntries = nil
	for bs.BitsRemaining() > 9 {
		entry, err := bs.ReadHuffString()
		if err != nil {
			return nil, err
		}
		if entry == "" {
			break
		}
		result.ScoreEntries = append(result.ScoreEntries, entry)
		if len(result.ScoreEntries) >= maxScoreEntries {
			break
		}
	}

	parseScoreEntries(result)

	return result, nil
}
