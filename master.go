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
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

type MasterServer struct {
	mutex        sync.RWMutex
	address      string
	ip           net.IP
	port         int
	name         string
	motd         string
	serverCount  uint16
	servers      []string
	ping         time.Duration
	queryTime    time.Time
	totalPackets int
}

func (m *MasterServer) Name() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.name
}

func (m *MasterServer) MOTD() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.motd
}

func (m *MasterServer) ServerCount() uint16 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.serverCount
}

func (m *MasterServer) Servers() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.servers
}

func (m *MasterServer) Query() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	s, err := net.ResolveUDPAddr("udp4", m.address)
	if err != nil {
		return err
	}

	m.ip = s.IP
	m.port = s.Port
	m.serverCount = 0
	m.servers = nil

	c, err := net.DialUDP("udp4", nil, s)
	if err != nil {
		return err
	}

	defer func(c *net.UDPConn) {
		err := c.Close()
		if err != nil {
			panic(err)
		}
	}(c)

	key := uint16(rand.Uint32())
	sendBuffer := []byte{0x10, 0x03, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00}

	binary.BigEndian.PutUint16(sendBuffer[4:6], key)

	m.queryTime = time.Now()
	pingCalculated := false
	_, err = c.Write(sendBuffer)
	if err != nil {
		return err
	}

	recvBuf := make([]byte, 1024)
	m.totalPackets = 1
	for p := 0; p < m.totalPackets; p++ {
		err := c.SetDeadline(time.Now().Add(5 * time.Second))
		if err != nil {
			return err
		}
		n, addr, err := c.ReadFromUDP(recvBuf)
		if err != nil {
			return err
		}

		if !addr.IP.Equal(s.IP) || addr.Port != s.Port {
			return fmt.Errorf("t1net.MasterServer.Query: Reply address mismatch: %s != %s", s.String(), addr.String())
		}

		if !pingCalculated {
			pingCalculated = true
			m.ping = time.Since(m.queryTime)
		}

		reader := bytes.NewReader(recvBuf[0:n])

		b, err := reader.ReadByte()
		if err != nil {
			return err
		}
		if b != 0x10 {
			return fmt.Errorf("t1net.MasterServer.Query: Reply byte 0: %#v != 0x10", b)
		}

		b, err = reader.ReadByte()
		if err != nil {
			return err
		}
		if b != 0x06 {
			return fmt.Errorf("t1net.MasterServer.Query: Reply byte 1: %#v != 0x06", b)
		}

		// Packet Number
		packetNumber, err := reader.ReadByte()
		if err != nil {
			return err
		}
		if packetNumber < 1 || packetNumber > 5 {
			return fmt.Errorf("t1net.MasterServer.Query: Invalid packet number: %d", packetNumber)
		}

		// Total number of Packets
		totalPackets, err := reader.ReadByte()
		if err != nil {
			return err
		}
		if totalPackets < 1 || totalPackets > 5 {
			return fmt.Errorf("t1net.MasterServer.Query: Invalid total packet number: %d", totalPackets)
		}

		if packetNumber > totalPackets {
			return fmt.Errorf("t1net.MasterServer.Query: Packet Number is greater than total: %d / %d", packetNumber, totalPackets)
		}

		var recvKey uint16
		err = binary.Read(reader, binary.BigEndian, &recvKey)
		if err != nil {
			return err
		}
		if key != recvKey {
			return fmt.Errorf("t1net.MasterServer.Query: Key mismatch: %d : %d", recvKey, key)
		}

		m.totalPackets = int(totalPackets)

		b, err = reader.ReadByte()
		if err != nil {
			return err
		}
		if b != 0 {
			return fmt.Errorf("t1net.MasterServer.Query: Reply byte 6: %#v != 0x00", b)
		}

		b, err = reader.ReadByte()
		if err != nil {
			return err
		}
		if b != 0x66 {
			return fmt.Errorf("t1net.MasterServer.Query: Reply byte 7: %#v != 0x66", b)
		}

		m.name, err = ReadPascalString(reader)
		if err != nil {
			return err
		}

		m.motd, err = ReadPascalString(reader)
		if err != nil {
			return err
		}

		var serverCount uint16
		err = binary.Read(reader, binary.BigEndian, &serverCount)
		if err != nil {
			return err
		}

		m.serverCount += serverCount

		for i := uint16(0); i < serverCount; i++ {
			ip, port, err := ReadAddressPort(reader)
			if err != nil {
				return err
			}

			m.servers = append(m.servers, fmt.Sprintf("%s:%d", ip.String(), port))
		}

		if reader.Len() != 0 {
			return fmt.Errorf("t1net.MasterServer.Query: %d left over bytes", reader.Len())
		}
	}

	return nil
}

func NewMasterServer(address string) *MasterServer {
	return &MasterServer{address: address}
}
