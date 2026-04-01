# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

t1net-go is a Go library for querying Starsiege: Tribes (Tribes 1) game servers and master servers over UDP. It implements both the GameSpy query protocol and the native Tribes protocol (ping and full game info queries). Package name: `t1net`.

## Build & Test Commands

- **Run all tests:** `go test -v ./...`
- **Run tests with race detector:** `go test -race ./...`
- **Run a single test:** `go test -v -run TestGameSpyQuery`
- **Lint:** `golangci-lint run --verbose` (CI uses golangci-lint v2.2.1)
- **Build:** `go build ./...`

## Architecture

The library uses package-level functions that return result structs. No mutable state or mutexes.

**Public API:**
- `GameSpyQuery(address string, opts *QueryOptions) (*GameResult, error)` — GameSpy protocol (0x62/0x63)
- `PingQuery(address string, opts *QueryOptions) (*PingResult, error)` — Native Tribes ping (0x03/0x04)
- `GameInfoQuery(address string, opts *QueryOptions) (*GameResult, error)` — Native Tribes full query (0x07/0x08), uses BitStream/Huffman
- `MasterQuery(address string, opts *QueryOptions) (*MasterResult, error)` — Master server list (0x03/0x06), multi-packet

**Source files:**
- **`query.go`** — Types (`QueryOptions`, `GameResult`, `PingResult`, `MasterResult`, `Team`, `Player`), `packetConn` interface, `openConn` helper, `applyDefaults`, `sendQuery`. Testability via unexported `dialFunc` field on `QueryOptions`.
- **`gamespy.go`** — `GameSpyQuery()`. Sends 3-byte 0x62 packet (no padding), parses 0x63 response with Pascal strings.
- **`tribes.go`** — `PingQuery()` and `GameInfoQuery()`. Sends 0x03/0x07 packets (with padding), parses 0x04/0x08 responses. GameInfoQuery uses the unexported `bitStreamReader` for Huffman-decoded fields.
- **`master.go`** — `MasterQuery()`. Sends master list request (with padding), parses multi-packet 0x06 responses. Caps at 1000 servers.
- **`bitstream.go`** — Unexported `bitStreamReader` with Huffman decoding. LSB-first bit reads, static frequency table from original Tribes engine.
- **`utils.go`** — Exported protocol helpers: `ReadPascalString`/`WritePascalString`, `ReadAddressPort`/`WriteAddressPort`, `PadPacket`.

## Conventions

- All keys use little-endian encoding.
- `QueryOptions` accepts nil for defaults (5s timeout, 8-byte min packet size).
- Tests use interface-based mocks (`dynamicMockConn`) instead of real UDP sockets, injected via the unexported `dialFunc` field on `QueryOptions`.
- Licensed under Apache 2.0.
- Licensed under Apache 2.0.
- CI tests against Go 1.25 and 1.26 on macOS, Ubuntu, and Windows with race detector enabled.
