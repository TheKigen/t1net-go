package t1net

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"time"
)

const maxServers = 1000

// MasterQuery sends a master server list request to the given address and returns the parsed result.
// address must be in "host:port" format. opts may be nil for defaults.
func MasterQuery(address string, opts *QueryOptions) (*MasterResult, error) {
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

	// Set deadline.
	if err = conn.SetDeadline(time.Now().Add(cfg.Timeout)); err != nil {
		return nil, err
	}

	result := &MasterResult{}
	var pingRecorded bool
	totalPackets := 0

	readBuffer := make([]byte, 65535)

	for p := 0; ; p++ {
		n, rerr := readResponse(conn, remoteAddr, readBuffer)
		if rerr != nil {
			if p == 0 {
				return nil, rerr
			}
			// Timeout after receiving some packets — treat as done.
			break
		}

		if !pingRecorded {
			result.Ping = time.Since(sendTime)
			pingRecorded = true
		}

		reader := bytes.NewReader(readBuffer[:n])

		b, berr := reader.ReadByte()
		if berr != nil {
			return nil, berr
		}
		if b != 0x10 {
			return nil, fmt.Errorf("expected 0x10 at byte 0, got %#x", b)
		}

		b, berr = reader.ReadByte()
		if berr != nil {
			return nil, berr
		}
		if b != 0x06 {
			return nil, fmt.Errorf("expected 0x06 at byte 1, got %#x", b)
		}

		// Byte 2: packet number (1-based, 1-5).
		packetNumber, berr := reader.ReadByte()
		if berr != nil {
			return nil, berr
		}
		if packetNumber == 0 {
			return nil, fmt.Errorf("invalid packet number: %d", packetNumber)
		}

		// Byte 3: total packets (1-5).
		packetTotal, berr := reader.ReadByte()
		if berr != nil {
			return nil, berr
		}
		if packetTotal == 0 {
			return nil, fmt.Errorf("invalid total packet number: %d", packetTotal)
		}
		if packetNumber > packetTotal {
			return nil, fmt.Errorf("packet number %d greater than total %d", packetNumber, packetTotal)
		}

		// Set totalPackets from first packet.
		if p == 0 {
			totalPackets = int(packetTotal)
		}

		// Bytes 4-5: key echo (little-endian).
		var readKey uint16
		if berr = binary.Read(reader, binary.LittleEndian, &readKey); berr != nil {
			return nil, berr
		}
		if key != readKey {
			return nil, fmt.Errorf("key mismatch: sent %d, got %d", key, readKey)
		}

		b, berr = reader.ReadByte()
		if berr != nil {
			return nil, berr
		}
		if b != 0x00 {
			return nil, fmt.Errorf("expected 0x00 at byte 6, got %#x", b)
		}

		b, berr = reader.ReadByte()
		if berr != nil {
			return nil, berr
		}
		if b != 0x66 {
			return nil, fmt.Errorf("expected 0x66 at byte 7, got %#x", b)
		}

		// Name (Pascal string).
		name, berr := ReadPascalString(reader)
		if berr != nil {
			return nil, berr
		}
		if p == 0 {
			result.Name = name
		}

		// MOTD (Pascal string).
		motd, berr := ReadPascalString(reader)
		if berr != nil {
			return nil, berr
		}
		if p == 0 {
			result.MOTD = motd
		}

		// Server count for this packet (big-endian).
		var serverCount uint16
		if berr = binary.Read(reader, binary.BigEndian, &serverCount); berr != nil {
			return nil, berr
		}
		result.ServerCount += uint32(serverCount)

		// Read server addresses.
		for i := uint16(0); i < serverCount; i++ {
			ip, port, berr := ReadAddressPort(reader)
			if berr != nil {
				return nil, berr
			}
			result.Servers = append(result.Servers, fmt.Sprintf("%s:%d", ip.String(), port))
			if len(result.Servers) > maxServers {
				return nil, fmt.Errorf("server count exceeds maximum: %d", len(result.Servers))
			}
		}

		// Leftover bytes check.
		if reader.Len() != 0 {
			return nil, fmt.Errorf("%d left over bytes", reader.Len())
		}

		// Stop after reading all expected packets.
		if p+1 >= totalPackets {
			break
		}
	}

	return result, nil
}
