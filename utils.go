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
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
)

// ReadPascalString reads a length-prefixed Pascal string from reader, where the first byte
// is the string length followed by that many bytes of character data.
func ReadPascalString(reader *bytes.Reader) (str string, err error) {
	b, err := reader.ReadByte()
	if err != nil {
		return
	}

	if b > 0 {
		builder := new(strings.Builder)
		builder.Grow(int(b))
		_, err = io.CopyN(builder, reader, int64(b))
		if err != nil {
			return
		}
		str = builder.String()
		return
	}

	return
}

// WritePascalString writes a Pascal string to buffer as a length byte followed by the string bytes.
// Returns an error if str exceeds 255 characters.
func WritePascalString(buffer *bytes.Buffer, str string) (err error) {
	strlen := len(str)
	if strlen > 255 {
		return fmt.Errorf("string length too long: %d > 255", strlen)
	}
	if err = buffer.WriteByte(byte(strlen)); err != nil {
		return
	}
	buffer.WriteString(str) //nolint:errcheck

	return
}

// ReadAddressPort reads a 7-byte encoded address from reader: a length byte (must be 6),
// followed by 4 bytes of IPv4 address (big-endian) and 2 bytes of port (little-endian).
func ReadAddressPort(reader *bytes.Reader) (ip net.IP, port uint16, err error) {
	ip = make(net.IP, 4)
	b, err := reader.ReadByte()
	if err != nil {
		return
	}
	if b != 6 {
		err = errors.New("invalid length for server/port")
		return
	}

	err = binary.Read(reader, binary.BigEndian, &ip)
	if err != nil {
		return
	}
	err = binary.Read(reader, binary.LittleEndian, &port)
	if err != nil {
		return
	}
	return
}

// DefaultMinPacketSize is the default minimum UDP packet size used by PadPacket when
// QueryOptions.MinPacketSize is zero.
const DefaultMinPacketSize = 8

// PadPacket returns data zero-padded to at least minSize bytes.
// If data is already at least minSize bytes long, it is returned unchanged.
func PadPacket(data []byte, minSize int) []byte {
	if len(data) >= minSize {
		return data
	}
	padded := make([]byte, minSize)
	copy(padded, data)
	return padded
}

// WriteAddressPort writes an IPv4 address and port to buffer in the 7-byte wire format:
// a length byte (6), 4 bytes of IPv4 address (big-endian), and 2 bytes of port (little-endian).
// Returns an error if ip is not exactly 4 bytes.
func WriteAddressPort(buffer *bytes.Buffer, ip net.IP, port uint16) (err error) {
	if len(ip) != net.IPv4len {
		return errors.New("IP length is not equal to 4 bytes")
	}
	if err = buffer.WriteByte(6); err != nil {
		return
	}
	err = binary.Write(buffer, binary.BigEndian, ip)
	if err != nil {
		return
	}
	err = binary.Write(buffer, binary.LittleEndian, port)
	if err != nil {
		return
	}
	return
}
