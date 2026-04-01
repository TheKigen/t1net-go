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
	"time"
)

// GameSpyQuery sends a GameSpy query to the given address and returns the parsed result.
// address must be in "host:port" format. opts may be nil for defaults.
func GameSpyQuery(address string, opts *QueryOptions) (*GameResult, error) {
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

	// Build 3-byte query packet: [0x62, key_lo, key_hi] (little-endian key, no padding).
	key := uint16(rand.Uint32()) //nolint:gosec
	sendBuffer := []byte{0x62, 0x00, 0x00}
	binary.LittleEndian.PutUint16(sendBuffer[1:], key)

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

	// Minimum sanity check: need at least 4-byte header plus some data.
	if n < 20 {
		return nil, fmt.Errorf("reply too short: %d bytes", n)
	}

	reader := bytes.NewReader(readBuffer[:n])

	b, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if b != 0x63 {
		return nil, fmt.Errorf("expected 0x63 at byte 0, got %#x", b)
	}

	// Bytes 1-2: key echo (little-endian).
	var readKey uint16
	if err = binary.Read(reader, binary.LittleEndian, &readKey); err != nil {
		return nil, err
	}
	if key != readKey {
		return nil, fmt.Errorf("key mismatch: sent %d, got %d", key, readKey)
	}

	b, err = reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if b != 0x62 {
		return nil, fmt.Errorf("expected 0x62 at byte 3, got %#x", b)
	}

	result := &GameResult{
		Ping: ping,
	}

	// Parse fields.
	result.Game, err = ReadPascalString(reader)
	if err != nil {
		return nil, err
	}

	result.Version, err = ReadPascalString(reader)
	if err != nil {
		return nil, err
	}

	result.Name, err = ReadPascalString(reader)
	if err != nil {
		return nil, err
	}

	b, err = reader.ReadByte()
	if err != nil {
		return nil, err
	}
	result.Dedicated = b == 1

	b, err = reader.ReadByte()
	if err != nil {
		return nil, err
	}
	result.Password = b == 1

	b, err = reader.ReadByte()
	if err != nil {
		return nil, err
	}
	result.NumPlayers = b

	b, err = reader.ReadByte()
	if err != nil {
		return nil, err
	}
	result.MaxPlayers = b

	var cpuSpeed uint16
	if err = binary.Read(reader, binary.LittleEndian, &cpuSpeed); err != nil {
		return nil, err
	}
	result.CPUSpeed = uint32(cpuSpeed)

	result.Mod, err = ReadPascalString(reader)
	if err != nil {
		return nil, err
	}

	result.ServerType, err = ReadPascalString(reader)
	if err != nil {
		return nil, err
	}

	result.Mission, err = ReadPascalString(reader)
	if err != nil {
		return nil, err
	}

	result.Info, err = ReadPascalString(reader)
	if err != nil {
		return nil, err
	}

	b, err = reader.ReadByte()
	if err != nil {
		return nil, err
	}
	result.NumTeams = b

	result.TeamScoreHeader, err = ReadPascalString(reader)
	if err != nil {
		return nil, err
	}

	result.PlayerScoreHeader, err = ReadPascalString(reader)
	if err != nil {
		return nil, err
	}

	// Parse teams.
	result.Teams = make([]Team, 0, result.NumTeams)
	for i := uint8(0); i < result.NumTeams; i++ {
		var teamName, teamScore string
		teamName, err = ReadPascalString(reader)
		if err != nil {
			return nil, err
		}
		teamScore, err = ReadPascalString(reader)
		if err != nil {
			return nil, err
		}
		result.Teams = append(result.Teams, Team{Name: teamName, Score: teamScore})
	}

	// Parse players.
	result.Players = make([]Player, 0, result.NumPlayers)
	for i := uint8(0); i < result.NumPlayers; i++ {
		var ping8, pl, team byte
		ping8, err = reader.ReadByte()
		if err != nil {
			return nil, err
		}
		pl, err = reader.ReadByte()
		if err != nil {
			return nil, err
		}
		team, err = reader.ReadByte()
		if err != nil {
			return nil, err
		}
		var playerName, playerScore string
		playerName, err = ReadPascalString(reader)
		if err != nil {
			return nil, err
		}
		playerScore, err = ReadPascalString(reader)
		if err != nil {
			return nil, err
		}
		result.Players = append(result.Players, Player{
			Ping:  ping8,
			PL:    pl,
			Team:  team,
			Name:  playerName,
			Score: playerScore,
		})
	}

	if reader.Len() != 0 {
		return nil, fmt.Errorf("%d left over bytes", reader.Len())
	}

	return result, nil
}
