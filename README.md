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
| `GameSpyQuery` | GameSpy (0x62/0x63) | Full server info via the GameSpy query protocol |
| `PingQuery` | Native Tribes (0x03/0x04) | Lightweight ping with server name and player counts |
| `GameInfoQuery` | Native Tribes (0x07/0x08) | Full server info via the native Tribes protocol (Huffman-encoded) |
| `MasterQuery` | Master Server (0x03/0x06) | Server list from a Tribes master server |

## Usage

### Query a game server

```go
package main

import (
    "fmt"
    "log"

    t1net "github.com/TheKigen/t1net-go"
)

func main() {
    // GameSpy query (full server info)
    result, err := t1net.GameSpyQuery("127.0.0.1:28001", nil)
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
result, err := t1net.GameInfoQuery("127.0.0.1:28001", &t1net.QueryOptions{
    Timeout:       2 * time.Second,
    MinPacketSize: 16,
    LocalAddress:  "0.0.0.0:0",
})
```

Pass `nil` for defaults (5 second timeout, 8-byte minimum packet size).

## License

Apache 2.0 - see [LICENSE](LICENSE) for details.
