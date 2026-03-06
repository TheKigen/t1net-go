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
	"net"
	"strings"
	"testing"
)

func TestReadPascalString(t *testing.T) {
	reader := bytes.NewReader([]byte{7, 'T', 'e', 's', 't', 'i', 'n', 'g'})
	str, err := ReadPascalString(reader)
	if err != nil {
		t.Fatal(err)
	}
	if str != "Testing" {
		t.Fatalf("%s != Testing", str)
	}
}

func TestReadPascalStringEmpty(t *testing.T) {
	reader := bytes.NewReader([]byte{0})
	str, err := ReadPascalString(reader)
	if err != nil {
		t.Fatal(err)
	}
	if str != "" {
		t.Fatalf("expected empty string, got %q", str)
	}
}

func TestReadPascalStringTruncated(t *testing.T) {
	reader := bytes.NewReader([]byte{5, 'A', 'B'})
	_, err := ReadPascalString(reader)
	if err == nil {
		t.Fatal("expected error for truncated pascal string")
	}
}

func TestReadPascalStringNoData(t *testing.T) {
	reader := bytes.NewReader([]byte{})
	_, err := ReadPascalString(reader)
	if err == nil {
		t.Fatal("expected error for empty reader")
	}
}

func TestWritePascalString(t *testing.T) {
	var buffer bytes.Buffer
	err := WritePascalString(&buffer, "Testing")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal([]byte{7, 'T', 'e', 's', 't', 'i', 'n', 'g'}, buffer.Bytes()) {
		t.Fatalf("bytes.Equal failed: %v", buffer.Bytes())
	}
}

func TestWritePascalStringEmpty(t *testing.T) {
	var buffer bytes.Buffer
	err := WritePascalString(&buffer, "")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal([]byte{0}, buffer.Bytes()) {
		t.Fatalf("expected [0], got %v", buffer.Bytes())
	}
}

func TestWritePascalStringTooLong(t *testing.T) {
	var buffer bytes.Buffer
	longStr := strings.Repeat("A", 256)
	err := WritePascalString(&buffer, longStr)
	if err == nil {
		t.Fatal("expected error for string > 255 bytes")
	}
}

func TestWritePascalStringMaxLength(t *testing.T) {
	var buffer bytes.Buffer
	maxStr := strings.Repeat("A", 255)
	err := WritePascalString(&buffer, maxStr)
	if err != nil {
		t.Fatal(err)
	}
	if buffer.Len() != 256 {
		t.Fatalf("expected 256 bytes, got %d", buffer.Len())
	}
	if buffer.Bytes()[0] != 255 {
		t.Fatalf("expected length byte 255, got %d", buffer.Bytes()[0])
	}
}

func TestReadAddressPort(t *testing.T) {
	reader := bytes.NewReader([]byte{6, 12, 13, 14, 15, 97, 109})
	ip, port, err := ReadAddressPort(reader)
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "12.13.14.15" {
		t.Fatalf("ip does not match: %s", ip.String())
	}
	if port != 28001 {
		t.Fatalf("port %d != 28001", port)
	}
}

func TestReadAddressPortInvalidLength(t *testing.T) {
	reader := bytes.NewReader([]byte{5, 12, 13, 14, 15, 97})
	_, _, err := ReadAddressPort(reader)
	if err == nil {
		t.Fatal("expected error for invalid length byte")
	}
}

func TestReadAddressPortTruncatedIP(t *testing.T) {
	reader := bytes.NewReader([]byte{6, 12, 13})
	_, _, err := ReadAddressPort(reader)
	if err == nil {
		t.Fatal("expected error for truncated IP data")
	}
}

func TestReadAddressPortTruncatedPort(t *testing.T) {
	reader := bytes.NewReader([]byte{6, 12, 13, 14, 15, 97})
	_, _, err := ReadAddressPort(reader)
	if err == nil {
		t.Fatal("expected error for truncated port data")
	}
}

func TestReadAddressPortNoData(t *testing.T) {
	reader := bytes.NewReader([]byte{})
	_, _, err := ReadAddressPort(reader)
	if err == nil {
		t.Fatal("expected error for empty reader")
	}
}

func TestWriteAddressPort(t *testing.T) {
	var buffer bytes.Buffer
	ip := net.IPv4(12, 13, 14, 15).To4()
	err := WriteAddressPort(&buffer, ip, 28001)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal([]byte{6, 12, 13, 14, 15, 97, 109}, buffer.Bytes()) {
		t.Fatalf("bytes.Equal failed: %v", buffer.Bytes())
	}
}

func TestWriteAddressPortInvalidIPLength(t *testing.T) {
	var buffer bytes.Buffer
	ip := net.ParseIP("12.13.14.15") // 16-byte IPv6-mapped form
	err := WriteAddressPort(&buffer, ip, 28001)
	if err == nil {
		t.Fatal("expected error for non-IPv4len IP")
	}
}

func TestPadPacketShorter(t *testing.T) {
	data := []byte{0x62, 0x01, 0x02}
	padded := PadPacket(data, 8)
	if len(padded) != 8 {
		t.Fatalf("expected length 8, got %d", len(padded))
	}
	expected := []byte{0x62, 0x01, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00}
	if !bytes.Equal(padded, expected) {
		t.Fatalf("expected %v, got %v", expected, padded)
	}
}

func TestPadPacketEqual(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04}
	padded := PadPacket(data, 4)
	if !bytes.Equal(padded, data) {
		t.Fatalf("expected no padding when equal")
	}
}

func TestPadPacketLonger(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	padded := PadPacket(data, 3)
	if !bytes.Equal(padded, data) {
		t.Fatalf("expected no padding when longer")
	}
}

func TestPadPacketZero(t *testing.T) {
	data := []byte{0x01}
	padded := PadPacket(data, 0)
	if !bytes.Equal(padded, data) {
		t.Fatalf("expected no padding with minSize 0")
	}
}
