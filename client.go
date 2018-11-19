package main

import (
	"fmt"
	"net"
)

type ssmgr struct {
	addr   string
	conn   *net.UDPConn
	buffer []byte
}

func (s *ssmgr) connect() error {
	raddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return err
	}
	s.conn, _ = net.DialUDP("udp", nil, raddr)
	return nil
}

func (s *ssmgr) listen() {
	for true {
		nRead, _, err := s.conn.ReadFrom(s.buffer)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(s.buffer[:nRead])
	}
}

func (s *ssmgr) sendCommand(cmd string) error {
	_, err := fmt.Fprint(s.conn, cmd)
	if err != nil {
		return err
	}
	return nil
}

func (s *ssmgr) ping() error {
	err := s.sendCommand("ping")
	if err != nil {
		return err
	}
	return nil
}

func (s *ssmgr) addPort(port int, password string) error {
	err := s.sendCommand(fmt.Sprintf(`add: {"server_port": %v, "password":"%v"}`, port, password))
	if err != nil {
		return err
	}
	return nil
}

func (s *ssmgr) removePort(port int) error {
	err := s.sendCommand(fmt.Sprintf(`remove: {"server_port": %v}`, port))
	if err != nil {
		return err
	}
	return nil
}

func main() {
	s := ssmgr{"localhost:4000", nil, make([]byte, 2048)}
	err := s.connect()
	if err != nil {
		fmt.Println(err)
		return
	}
	s.listen()
	go s.ping()
}
