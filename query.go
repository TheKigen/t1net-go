// Package t1net implements UDP query protocols for Starsiege: Tribes (Tribes 1)
// game servers and master servers. It supports the GameSpy protocol, the native
// Tribes ping and game info protocols, and the master server list protocol.
package t1net

import (
	"net"
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
func parseScoreEntries(result *GameResult) {
	for _, entry := range result.ScoreEntries {
		if len(entry) < 2 {
			continue
		}
		section := entry[0]
		rowType := entry[1]
		data := entry[2:]

		// Skip headers and empty data rows
		if rowType == '1' || data == "" {
			continue
		}

		fields := strings.Split(data, "\t")

		switch section {
		case '0': // team entry
			team := Team{Name: strings.TrimSpace(fields[0])}
			if len(fields) > 1 {
				team.Score = strings.TrimSpace(fields[1])
			}
			result.Teams = append(result.Teams, team)
		case '1': // player entry
			player := Player{Name: strings.TrimSpace(fields[0])}
			if len(fields) > 2 {
				player.Score = strings.TrimSpace(fields[2])
			}
			result.Players = append(result.Players, player)
		}
	}
	result.NumTeams = uint8(len(result.Teams))
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
