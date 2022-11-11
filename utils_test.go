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

func TestWritePascalString(t *testing.T) {
	var buffer bytes.Buffer
	err := WritePascalString(&buffer, "Testing")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare([]byte{7, 'T', 'e', 's', 't', 'i', 'n', 'g'}, buffer.Bytes()) != 0 {
		t.Fatalf("bytes.Compare failed: %v", buffer.Bytes())
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

func TestWriteAddressPort(t *testing.T) {
	var buffer bytes.Buffer
	var ip net.IP = net.IPv4(12, 13, 14, 15).To4()
	err := WriteAddressPort(&buffer, ip, 28001)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare([]byte{6, 12, 13, 14, 15, 97, 109}, buffer.Bytes()) != 0 {
		t.Fatalf("bytes Compare failed: %v", buffer.Bytes())
	}
}
