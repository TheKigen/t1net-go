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

func ReadPascalString(reader *bytes.Reader) (string, error) {
	b, err := reader.ReadByte()
	if err != nil {
		return "", err
	}

	if b > 0 {
		builder := new(strings.Builder)
		builder.Grow(int(b))
		_, err = io.CopyN(builder, reader, int64(b))
		if err != nil {
			return "", err
		}

		return builder.String(), nil
	}

	return "", nil
}

func WritePascalString(buffer *bytes.Buffer, str string) error {
	strlen := len(str)
	if strlen > 255 {
		return fmt.Errorf("t1net.WritePascalString: String length is too long.  %d > 255", strlen)
	}
	if err := buffer.WriteByte(byte(strlen)); err != nil {
		return err
	}
	n, err := buffer.WriteString(str)
	if err != nil {
		return err
	}
	if n != strlen {
		return fmt.Errorf("t1net.WritePascalString: String written length does not match.  %d != %d", n, strlen)
	}

	return nil
}

func ReadAddressPort(reader *bytes.Reader) (net.IP, uint16, error) {
	ip := make(net.IP, 4)
	var port uint16
	b, err := reader.ReadByte()
	if err != nil {
		return ip, port, err
	}
	if b != 6 {
		return ip, port, errors.New("t1net.ReadServerAddress: Invalid length for server/port")
	}

	err = binary.Read(reader, binary.BigEndian, &ip)
	if err != nil {
		return ip, port, err
	}
	err = binary.Read(reader, binary.LittleEndian, &port)
	if err != nil {
		return ip, port, err
	}
	return ip, port, nil
}

func WriteAddressPort(buffer *bytes.Buffer, ip net.IP, port uint16) error {
	buffer.WriteByte(6)
	if len(ip) != net.IPv4len {
		return errors.New("t1net.WriteAddressPort: IP length is not equal to 4 bytes")
	}
	err := binary.Write(buffer, binary.BigEndian, ip)
	if err != nil {
		return err
	}
	err = binary.Write(buffer, binary.LittleEndian, port)
	if err != nil {
		return err
	}
	return nil
}
