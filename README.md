# t1net-go

[![Run Tests](https://github.com/TheKigen/t1net-go/actions/workflows/t1net.yml/badge.svg)](https://github.com/TheKigen/t1net-go/actions/workflows/t1net.yml) [![codecov](https://codecov.io/github/thekigen/t1net-go/graph/badge.svg?token=LNNBWQ4Q62)](https://codecov.io/github/thekigen/t1net-go)

A Go library for querying Starsiege: Tribes (Tribes 1) game servers and master servers over UDP.

## Installation

```
go get github.com/TheKigen/t1net-go
```

## Supported Protocols

| Function | Protocol | Description |
|----------|----------|-------------|
| `FullQuery` | GameSpy + Native | Combined query for maximum information (recommended) |
| `GameSpyQuery` | GameSpy (0x62/0x63) | Full server info via the GameSpy query protocol |
| `PingQuery` | Native Tribes (0x03/0x04) | Lightweight ping with server name and player counts |
| `GameInfoQuery` | Native Tribes (0x07/0x08) | Full server info via the native Tribes protocol (Huffman-encoded) |
| `MasterQuery` | Master Server (0x03/0x06) | Server list from a Tribes master server |

### FullQuery

`FullQuery` runs `GameInfoQuery` and `GameSpyQuery` concurrently and merges the results. The native query provides the base result with full-length strings and score entries. The GameSpy query supplements per-player Ping, PL, and Team values from its dedicated binary fields, and may include additional teams (such as observer teams) that have no score entries.

The native query is expected to always succeed. Some ISPs block the GameSpy packet for being too short; when it fails, the native result is returned without supplemental data.

### Protocol differences

| | GameSpy | Native (GameInfo) |
|---|---------|-------------------|
| Packet size | Small (may be blocked) | Larger (reliable) |
| Player Ping/PL/Team | Binary fields (accurate) | Parsed from score entries (best-effort) |
| Strings | Pascal (max ~255 bytes, truncated) | Huffman (full length) |
| Score format | Format string with `%n`, `%p`, `%l` placeholders | Format string with `%n`, `%p`, `%l` placeholders (reconstructed from score entries) |
| Team list | Includes all teams (e.g. observers) | Only teams with score entries |

## Usage

### Query a game server (recommended)

```go
package main

import (
    "fmt"
    "log"

    t1net "github.com/TheKigen/t1net-go"
)

func main() {
    result, err := t1net.FullQuery("127.0.0.1:28001", nil)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Server: %s (%d/%d players)\n", result.Name, result.NumPlayers, result.MaxPlayers)
    fmt.Printf("Map: %s | Mod: %s\n", result.Mission, result.Mod)

    for _, p := range result.Players {
        fmt.Printf("  Player: %s (ping: %d)\n", p.Name, p.Ping)
    }
}
```

### Individual protocol queries

```go
// GameSpy only
gsResult, err := t1net.GameSpyQuery("127.0.0.1:28001", nil)

// Native only
niResult, err := t1net.GameInfoQuery("127.0.0.1:28001", nil)
```

### Lightweight ping

```go
result, err := t1net.PingQuery("127.0.0.1:28001", nil)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("%s: %d/%d players (ping: %v)\n", result.Name, result.NumPlayers, result.MaxPlayers, result.Ping)
```

### Query the master server

```go
result, err := t1net.MasterQuery("t1m1.tribes1.co:28000", nil)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Master: %s | MOTD: %s\n", result.Name, result.MOTD)
fmt.Printf("Servers online: %d\n", result.ServerCount)

for _, addr := range result.Servers {
    fmt.Println("  ", addr)
}
```

### Custom options

```go
import "time"

result, err := t1net.FullQuery("127.0.0.1:28001", &t1net.QueryOptions{
    Timeout:       2 * time.Second,
    MinPacketSize: 16,
    LocalAddress:  "0.0.0.0:0",
})
```

Pass `nil` for defaults (5 second timeout, 8-byte minimum packet size).

## License

Apache 2.0 - see [LICENSE](LICENSE) for details.
