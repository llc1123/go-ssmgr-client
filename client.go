package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
)

type ssmgr struct {
	addr   string
	conn   *net.UDPConn
	buffer []byte
	stat   map[int]int
}

func (s *ssmgr) connect() error {
	raddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return err
	}
	s.conn, _ = net.DialUDP("udp", nil, raddr)
	return nil
}

func (s *ssmgr) listen(console chan string, control chan string, status chan string) {
	for {
		nRead, _, err := s.conn.ReadFrom(s.buffer)
		if err != nil {
			console <- fmt.Sprint(err)
		}
		res := string(s.buffer[:nRead])
		if res == "ok" || res == "pong" {
			control <- res
			console <- fmt.Sprintf("control response: [%v]", res)
		} else if res[:4] == "stat" {
			status <- res
			console <- fmt.Sprintf("status message: [%v]", res)
		} else {
			console <- fmt.Sprintf("unknown message: [%v]", res)
		}
	}
}

func (s *ssmgr) sendCommand(cmd string) error {
	_, err := fmt.Fprint(s.conn, cmd)
	if err != nil {
		return err
	}
	return nil
}

func (s *ssmgr) ping(console chan string, control chan string) error {
	err := s.sendCommand("ping")
	if err != nil {
		return err
	}
	console <- "control request: [ping]"
	res := <-control
	if res == "pong" {
		return nil
	}
	return fmt.Errorf("unknown response: [%v]", res)
}

func (s *ssmgr) addPort(port int, password string, console chan string, control chan string) error {
	err := s.sendCommand(fmt.Sprintf(`add: {"server_port": %v, "password":"%v"}`, port, password))
	if err != nil {
		return err
	}
	console <- fmt.Sprintf(`control request: [add: {"server_port": %v, "password":"%v"}]`, port, password)
	res := <-control
	if res == "ok" {
		return nil
	}
	return fmt.Errorf("unknown response: [%v]", res)
}

func (s *ssmgr) removePort(port int, console chan string, control chan string) error {
	err := s.sendCommand(fmt.Sprintf(`remove: {"server_port": %v}`, port))
	if err != nil {
		return err
	}
	console <- fmt.Sprintf(`control request: [remove: {"server_port": %v}]`, port)
	res := <-control
	if res == "ok" {
		return nil
	}
	return fmt.Errorf("unknown response: %v", res)
}

func (s *ssmgr) recordStatus(console chan string, status chan string) error {
	for {
		res := <-status
		data := []byte(res[6:])
		var objmap map[string]interface{}
		err := json.Unmarshal(data, &objmap)
		if err != nil {
			console <- fmt.Sprintf("json decode error: [%v]", res)
			continue
		}
		for key, value := range objmap {
			k, e := strconv.Atoi(key)
			if e != nil {
				console <- fmt.Sprintf("invalid key: [%v]", key)
				continue
			}
			v, ok := value.(float64)
			if !ok {
				console <- fmt.Sprintf("invalid value: [%v]", value)
				continue
			}
			s.stat[k] += int(v)
			console <- fmt.Sprintf("total flow at port %v: %v", k, s.stat[k])
		}
	}
}

func main() {
	s := ssmgr{"localhost:4000", nil, make([]byte, 2048), make(map[int]int)}
	err := s.connect()
	if err != nil {
		fmt.Println(err)
		return
	}
	console, control, status := make(chan string), make(chan string), make(chan string)
	go s.ping(console, control)
	go s.listen(console, control, status)
	go s.recordStatus(console, status)
	go func() {
		s.addPort(8123, "123", console, control)
		s.removePort(8123, console, control)
	}()
	for {
		fmt.Println(<-console)
	}
}
