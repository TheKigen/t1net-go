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

// Package t1net implements UDP query protocols for Starsiege: Tribes (Tribes 1)
// game servers and master servers. It supports the GameSpy protocol, the native
// Tribes ping and game info protocols, and the master server list protocol.
package t1net

import (
	"net"
	"strconv"
	"strings"
	"time"
)

// QueryOptions configures query behavior. Pass nil for defaults.
type QueryOptions struct {
	Timeout       time.Duration // default 5s if zero
	MinPacketSize int           // default DefaultMinPacketSize (8) if zero
	LocalAddress  string        // default "" (OS picks)
	dialFunc      func(*net.UDPAddr) (packetConn, error) // for testing; nil uses default
}

// GameResult holds the response from a GameSpyQuery or GameInfoQuery.
type GameResult struct {
	Ping              time.Duration
	Name              string
	Game              string
	Version           string
	Dedicated         bool
	Password          bool
	NumPlayers        uint8
	MaxPlayers        uint8
	CPUSpeed          uint32
	Mod               string
	ServerType        string
	Mission           string
	Info              string
	NumTeams          uint8
	TeamScoreHeader   string
	PlayerScoreHeader string
	Teams             []Team
	Players           []Player
	PacketVersion     uint16   // native protocol only, zero for GameSpy
	ScoreEntries      []string // native protocol only, nil for GameSpy
}

// PingResult holds the response from a PingQuery.
type PingResult struct {
	Ping          time.Duration
	Name          string
	NumPlayers    uint8
	MaxPlayers    uint8
	PacketVersion uint16
}

// MasterResult holds the response from a MasterQuery.
type MasterResult struct {
	Ping        time.Duration
	Name        string
	MOTD        string
	ServerCount uint32 // total accumulated across all response packets
	Servers     []string
}

// Team holds team name and score.
type Team struct {
	Name  string
	Score string
}

// Player holds player info from a game query.
type Player struct {
	Name  string
	Team  uint8
	Score string
	Ping  uint8 // network latency as reported by the server, in raw protocol units
	PL    uint8 // packet loss percentage
}

// parseScoreEntries performs best-effort parsing of native protocol score entries
// into Teams and Players slices. Score entries use a two-character prefix:
// first char '0' = team section, '1' = player section; second char '0' = data, '1' = header.
// Fields within each entry are tab-delimited.
//
// The Player Score field is reconstructed to match the GameSpy query format:
// the player name column is replaced with %n, and any Ping/PL columns
// (identified from the header) are replaced with %p/%l placeholders.
// The actual Ping, PL, and Team values are extracted into their respective
// struct fields.
func parseScoreEntries(result *GameResult) {
	// Column indices for player fields, discovered from the header row.
	// -1 means not found. Indices are relative to the data after the name
	// column (i.e. within the Score string fields).
	teamCol, pingCol, plCol := -1, -1, -1

	for _, entry := range result.ScoreEntries {
		if len(entry) < 2 {
			continue
		}
		section := entry[0]
		rowType := entry[1]
		data := entry[2:]

		if data == "" {
			continue
		}

		if rowType == '1' {
			// Header row — discover column positions for player fields.
			if section == '1' {
				teamCol, pingCol, plCol = parsePlayerHeader(data)
			}
			continue
		}

		// Split only on the first tab to extract the name; the remainder
		// is the raw score fields which we will reconstruct below.
		name, score, _ := strings.Cut(data, "\t")

		switch section {
		case '0': // team entry
			result.Teams = append(result.Teams, Team{
				Name:  strings.TrimSpace(name),
				Score: "%t\t" + score,
			})
		case '1': // player entry
			player := Player{
				Name: strings.TrimSpace(name),
				Team: 255, // default to unmatched, like GameSpy observer
			}
			fields := strings.Split(score, "\t")
			if teamCol >= 0 && teamCol < len(fields) {
				teamName := strings.TrimSpace(fields[teamCol])
				for i, tm := range result.Teams {
					if tm.Name == teamName {
						player.Team = uint8(i)
						break
					}
				}
			}
			if pingCol >= 0 && pingCol < len(fields) {
				if v, err := strconv.ParseUint(strings.TrimSpace(fields[pingCol]), 10, 8); err == nil {
					player.Ping = uint8(v)
				}
				fields[pingCol] = "%p"
			}
			if plCol >= 0 && plCol < len(fields) {
				if v, err := strconv.ParseUint(strings.TrimSpace(fields[plCol]), 10, 8); err == nil {
					player.PL = uint8(v)
				}
				fields[plCol] = "%l"
			}
			// Reconstruct score to match GameSpy format: %n prefix,
			// with Ping/PL columns replaced by format specifiers.
			player.Score = "%n\t" + strings.Join(fields, "\t")
			result.Players = append(result.Players, player)
		}
	}
	result.NumTeams = uint8(len(result.Teams))
}

// parsePlayerHeader parses a player header row to find the column indices
// for Team, Ping, and PL. Column names may have a leading Tribes format/color
// byte which is stripped before matching. The returned indices are relative
// to the fields after the name column (the Score string split on tabs).
func parsePlayerHeader(data string) (teamCol, pingCol, plCol int) {
	teamCol, pingCol, plCol = -1, -1, -1
	// Skip the first column (player name) — the remaining columns correspond
	// to the tab-split fields of the Score string.
	_, rest, ok := strings.Cut(data, "\t")
	if !ok {
		return
	}
	for i, col := range strings.Split(rest, "\t") {
		col = strings.TrimSpace(col)
		// Try matching both with and without a leading format byte.
		if matchHeaderCol(col, "team") {
			teamCol = i
		} else if matchHeaderCol(col, "ping") {
			pingCol = i
		} else if matchHeaderCol(col, "pl") {
			plCol = i
		}
	}
	return
}

// matchHeaderCol returns true if col matches target case-insensitively,
// optionally after stripping a single leading Tribes format/color byte.
func matchHeaderCol(col, target string) bool {
	if strings.EqualFold(col, target) {
		return true
	}
	if len(col) > 1 && strings.EqualFold(col[1:], target) {
		return true
	}
	return false
}

// FullQuery performs both a native GameInfoQuery and a GameSpy query
// concurrently, then merges the results. The native query provides the base
// result with full-length strings and score entries. The GameSpy query, when
// it succeeds, supplements per-player Ping, PL, and Team values (which are
// read from dedicated binary fields in the GameSpy protocol and are more
// accurate than the score-entry-derived values from the native query).
//
// The native query is expected to always succeed; the GameSpy query may fail
// (some ISPs block the short GameSpy packet). If the native query fails, the
// error is returned. If only the GameSpy query fails, the native result is
// returned without supplemental data.
func FullQuery(address string, opts *QueryOptions) (*GameResult, error) {
	type queryResult struct {
		result *GameResult
		err    error
	}

	nativeCh := make(chan queryResult, 1)
	gamespyCh := make(chan queryResult, 1)

	go func() {
		r, err := GameInfoQuery(address, opts)
		nativeCh <- queryResult{r, err}
	}()
	go func() {
		r, err := GameSpyQuery(address, opts)
		gamespyCh <- queryResult{r, err}
	}()

	native := <-nativeCh
	if native.err != nil {
		return nil, native.err
	}
	result := native.result

	gamespy := <-gamespyCh
	if gamespy.err != nil || gamespy.result == nil {
		return result, nil
	}

	mergeGameSpyData(result, gamespy.result)
	return result, nil
}

// mergeGameSpyData overlays data from a GameSpy query result onto a native
// query result. Per-player Ping, PL, and Team are taken from GameSpy (where
// they come from dedicated binary fields). The GameSpy team list is used if
// it contains more teams than the native result (e.g. observer teams that
// have no score entries).
func mergeGameSpyData(native, gamespy *GameResult) {
	// Build a name->player lookup from the GameSpy result.
	gsPlayers := make(map[string]*Player, len(gamespy.Players))
	for i := range gamespy.Players {
		gsPlayers[gamespy.Players[i].Name] = &gamespy.Players[i]
	}

	// Overlay per-player fields.
	for i := range native.Players {
		gsp, ok := gsPlayers[native.Players[i].Name]
		if !ok {
			continue
		}
		native.Players[i].Ping = gsp.Ping
		native.Players[i].PL = gsp.PL
		native.Players[i].Team = gsp.Team
	}

	// Use GameSpy team list if it has more teams (e.g. observer team).
	if len(gamespy.Teams) > len(native.Teams) {
		native.Teams = gamespy.Teams
		native.NumTeams = gamespy.NumTeams
	}
}

// packetConn is the interface for UDP communication.
// *net.UDPConn satisfies this. Tests provide a mock.
type packetConn interface {
	WriteTo(b []byte, addr net.Addr) (int, error)
	ReadFrom(b []byte) (int, net.Addr, error)
	SetDeadline(t time.Time) error
	Close() error
}

// applyDefaults returns a QueryOptions with defaults applied. Accepts nil.
func applyDefaults(opts *QueryOptions) QueryOptions {
	if opts == nil {
		return QueryOptions{
			Timeout:       5 * time.Second,
			MinPacketSize: DefaultMinPacketSize,
		}
	}
	result := *opts
	if result.Timeout == 0 {
		result.Timeout = 5 * time.Second
	}
	if result.MinPacketSize == 0 {
		result.MinPacketSize = DefaultMinPacketSize
	}
	return result
}

// openConn resolves addresses and dials using the dialFunc from cfg (or the default).
// Returns the open connection and the resolved remote address.
func openConn(address string, cfg QueryOptions) (packetConn, *net.UDPAddr, error) {
	var localAddr *net.UDPAddr
	if cfg.LocalAddress != "" {
		var err error
		localAddr, err = net.ResolveUDPAddr("udp4", cfg.LocalAddress)
		if err != nil {
			return nil, nil, err
		}
	}

	remoteAddr, err := net.ResolveUDPAddr("udp4", address)
	if err != nil {
		return nil, nil, err
	}

	dial := cfg.dialFunc
	if dial == nil {
		dial = func(localAddr *net.UDPAddr) (packetConn, error) {
			if localAddr == nil {
				localAddr = &net.UDPAddr{}
			}
			return net.ListenUDP("udp4", localAddr)
		}
	}

	conn, err := dial(localAddr)
	if err != nil {
		return nil, nil, err
	}

	return conn, remoteAddr, nil
}

// sendQuery sends a packet and returns the send timestamp.
func sendQuery(conn packetConn, remoteAddr net.Addr, packet []byte) (time.Time, error) {
	sendTime := time.Now()
	_, err := conn.WriteTo(packet, remoteAddr)
	return sendTime, err
}

// readResponse reads a UDP response and validates the source address matches remoteAddr.
// Discards packets from unexpected sources and retries within the deadline.
func readResponse(conn packetConn, remoteAddr *net.UDPAddr, buf []byte) (int, error) {
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			return 0, err
		}
		if udpAddr, ok := addr.(*net.UDPAddr); ok {
			if !udpAddr.IP.Equal(remoteAddr.IP) || udpAddr.Port != remoteAddr.Port {
				continue // discard packet from unexpected source
			}
		}
		return n, nil
	}
}

const maxScoreEntries = 256
