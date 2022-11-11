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
	"math/rand"
	"net"
	"testing"
	"time"
)

func TestMasterServer(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	master := NewMasterServer("127.0.0.1:28999")

	s, err := net.ResolveUDPAddr("udp4", "127.0.0.1:28999")
	if err != nil {
		t.Fatal(err)
	}
	c, err := net.ListenUDP("udp4", s)
	if err != nil {
		t.Fatal(err)
	}
	defer func(c *net.UDPConn) {
		err := c.Close()
		if err != nil {
			t.Fatal(err)
		}
	}(c)

	go func() {
		readBuffer := make([]byte, 64)
		err := c.SetDeadline(time.Now().Add(1 * time.Second))
		if err != nil {
			t.Error(err)
			return
		}
		n, addr, err := c.ReadFromUDP(readBuffer)
		if err != nil {
			t.Error(err)
			return
		}
		if n != 8 {
			t.Errorf("Wrong query length, %d", n)
			return
		}
		expected := []byte{0x10, 0x03, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00}
		// Key is random and must match.
		expected[4] = readBuffer[4]
		expected[5] = readBuffer[5]

		if !bytes.Equal(expected, readBuffer[0:n]) {
			t.Error(readBuffer)
			return
		}

		sendBuffer := []byte{
			0x10, 0x6, 1, 2, 0x71, 0xb2, 0x0, 0x66,
			13, 'T', 'r', 'i', 'b', 'e', 's', ' ', 'M', 'a', 's', 't', 'e', 'r', // Name
			9, 'T', 'e', 's', 't', ' ', 'M', 'O', 'T', 'D', // MOTD
			0, 42, // Server Count
			0x6, 0x43, 0xde, 0x8a, 0x2e, 0x67, 0x6d,
			0x6, 0x18, 0x24, 0xaf, 0x99, 0x61, 0x6d,
			0x6, 0x2d, 0x22, 0xf, 0x5a, 0x61, 0x6d,
			0x6, 0x6b, 0x5, 0xc3, 0xcd, 0x61, 0x6d,
			0x6, 0x6b, 0xad, 0xa7, 0x7c, 0x61, 0x6d,
			0x6, 0x6b, 0xad, 0xa7, 0x6d, 0x61, 0x6d,
			0x6, 0xae, 0x32, 0xa7, 0xa, 0x64, 0x6d,
			0x6, 0x2d, 0x4f, 0x89, 0x6d, 0x61, 0x6d,
			0x6, 0xad, 0x1a, 0xf8, 0x72, 0x61, 0x6d,
			0x6, 0xcf, 0x94, 0xd, 0x84, 0x66, 0x6d,
			0x6, 0x88, 0x24, 0x5b, 0xe, 0x61, 0x6d,
			0x6, 0xd8, 0x80, 0x96, 0xd0, 0x61, 0x6d,
			0x6, 0x6b, 0xad, 0xa7, 0x6d, 0x62, 0x6d,
			0x6, 0xae, 0x37, 0x58, 0xbe, 0x61, 0x6d,
			0x6, 0xae, 0x32, 0xa7, 0xa, 0x61, 0x6d,
			0x6, 0x49, 0x5a, 0x18, 0xc3, 0x61, 0x6d,
			0x6, 0x2d, 0x22, 0xf, 0x5a, 0x63, 0x6d,
			0x6, 0xae, 0x32, 0xa7, 0xa, 0x63, 0x6d,
			0x6, 0x8b, 0x63, 0xfd, 0x23, 0x61, 0x6d,
			0x6, 0xae, 0x32, 0xa7, 0xa, 0x62, 0x6d,
			0x6, 0x12, 0xda, 0x1e, 0x7, 0x61, 0x6d,
			0x6, 0x90, 0xca, 0x36, 0x93, 0x65, 0x6d,
			0x6, 0x6b, 0xad, 0xa7, 0x71, 0xc5, 0x6d,
			0x6, 0x2d, 0x3f, 0x41, 0xf6, 0x65, 0x6d,
			0x6, 0x2d, 0x22, 0xf, 0x5a, 0x62, 0x6d,
			0x6, 0xc, 0xea, 0x96, 0xd6, 0x61, 0x6d,
			0x6, 0xae, 0x32, 0xa7, 0xa, 0xc2, 0x6d,
			0x6, 0x6b, 0xad, 0xa7, 0x71, 0xc6, 0x6d,
			0x6, 0x9f, 0x2, 0x2e, 0x79, 0x61, 0x6d,
			0x6, 0xae, 0x37, 0x58, 0xbe, 0x66, 0x6d,
			0x6, 0xae, 0x37, 0x58, 0xbe, 0xbb, 0xa1,
			0x6, 0x4b, 0x83, 0xaf, 0x5c, 0x61, 0x6d,
			0x6, 0x4a, 0x33, 0x1, 0x7e, 0x61, 0x6d,
			0x6, 0xae, 0x37, 0x58, 0xbe, 0x7b, 0x94,
			0x6, 0xae, 0x37, 0x58, 0xbe, 0xcf, 0x74,
			0x6, 0xae, 0x37, 0x58, 0xbe, 0xed, 0x3,
			0x6, 0xae, 0x37, 0x58, 0xbe, 0x65, 0x6d,
			0x6, 0xae, 0x37, 0x58, 0xbe, 0x68, 0x6d,
			0x6, 0xae, 0x37, 0x58, 0xbe, 0x6b, 0x6d,
			0x6, 0x4b, 0x83, 0xaf, 0x5c, 0x62, 0x6d,
			0x6, 0xae, 0x37, 0x58, 0xbe, 0xef, 0x3,
			0x6, 0xae, 0x37, 0x58, 0xbe, 0xee, 0x3,
		}
		sendBuffer[4] = readBuffer[4]
		sendBuffer[5] = readBuffer[5]
		sent, err := c.WriteToUDP(sendBuffer, addr)
		if err != nil {
			t.Error(err)
			return
		}
		if sent != len(sendBuffer) {
			t.Error("Sent did not match sendBuffer length")
			return
		}

		sendBuffer = []byte{
			0x10, 0x6, 2, 2, 0x71, 0xb2, 0x0, 0x66,
			13, 'T', 'r', 'i', 'b', 'e', 's', ' ', 'M', 'a', 's', 't', 'e', 'r', // Name
			9, 'T', 'e', 's', 't', ' ', 'M', 'O', 'T', 'D', // MOTD
			0, 2, // Server Count
			6, 12, 13, 14, 15, 97, 109,
			6, 22, 23, 24, 25, 97, 109,
		}
		sendBuffer[4] = readBuffer[4]
		sendBuffer[5] = readBuffer[5]
		sent, err = c.WriteToUDP(sendBuffer, addr)
		if err != nil {
			t.Error(err)
			return
		}
		if sent != len(sendBuffer) {
			t.Error("Sent did not match sendBuffer length")
			return
		}
	}()

	err = master.Query()
	if err != nil {
		t.Fatal(err)
	}
	if master.Name() != "Tribes Master" {
		t.Errorf("master.Name(): %s != Tribes Master\n", master.Name())
	}
	if master.MOTD() != "Test MOTD" {
		t.Errorf("master.MOTD(): got %s\n", master.MOTD())
	}
	if master.ServerCount() != 44 {
		t.Errorf("master.ServerCount(): %d != 42\n", master.ServerCount())
	}

	expectedServers := []string{
		"67.222.138.46:28007",
		"24.36.175.153:28001",
		"45.34.15.90:28001",
		"107.5.195.205:28001",
		"107.173.167.124:28001",
		"107.173.167.109:28001",
		"174.50.167.10:28004",
		"45.79.137.109:28001",
		"173.26.248.114:28001",
		"207.148.13.132:28006",
		"136.36.91.14:28001",
		"216.128.150.208:28001",
		"107.173.167.109:28002",
		"174.55.88.190:28001",
		"174.50.167.10:28001",
		"73.90.24.195:28001",
		"45.34.15.90:28003",
		"174.50.167.10:28003",
		"139.99.253.35:28001",
		"174.50.167.10:28002",
		"18.218.30.7:28001",
		"144.202.54.147:28005",
		"107.173.167.113:28101",
		"45.63.65.246:28005",
		"45.34.15.90:28002",
		"12.234.150.214:28001",
		"174.50.167.10:28098",
		"107.173.167.113:28102",
		"159.2.46.121:28001",
		"174.55.88.190:28006",
		"174.55.88.190:41403",
		"75.131.175.92:28001",
		"74.51.1.126:28001",
		"174.55.88.190:38011",
		"174.55.88.190:29903",
		"174.55.88.190:1005",
		"174.55.88.190:28005",
		"174.55.88.190:28008",
		"174.55.88.190:28011",
		"75.131.175.92:28002",
		"174.55.88.190:1007",
		"174.55.88.190:1006",
		"12.13.14.15:28001",
		"22.23.24.25:28001",
	}

	gotServers := master.Servers()

	if len(gotServers) != len(expectedServers) {
		t.Fatalf("master.Servers(): len(gotServers) %d != len(expectedServers) %d\n", len(gotServers), len(expectedServers))
	}

	for i := 1; i < len(gotServers); i++ {
		if gotServers[i] != expectedServers[i] {
			t.Fatalf("master.Servers(): %s != %s at index %d\n", gotServers[i], expectedServers[i], i)
		}
	}
}
