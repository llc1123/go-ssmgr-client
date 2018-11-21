package ssmgr

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"
)

//Ssmgr instance
type Ssmgr struct {
	addr     string
	conn     *net.UDPConn
	buffer   []byte
	flow     map[int]int
	ports    map[int]string
	console  chan string
	control  chan string
	status   chan string
	sendMux  sync.Mutex
	flowMux  sync.Mutex
	portsMux sync.Mutex
}

//NewSsmgr initialize new ssmgr instance
func NewSsmgr(addr string) *Ssmgr {
	return &Ssmgr{
		addr:    addr,
		conn:    nil,
		buffer:  make([]byte, 1506),
		flow:    make(map[int]int),
		ports:   make(map[int]string),
		console: make(chan string),
		control: make(chan string, 1),
		status:  make(chan string),
	}
}

func (s *Ssmgr) connect() error {
	raddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return err
	}
	s.conn, err = net.DialUDP("udp", nil, raddr)
	if err != nil {
		return err
	}
	return nil
}

func (s *Ssmgr) listen() {
	for {
		nRead, _, err := s.conn.ReadFrom(s.buffer)
		if err != nil {
			s.console <- fmt.Sprint(err)
			continue
		}
		res := string(s.buffer[:nRead])
		if res == "ok" || res == "pong" {
			select {
			case <-s.control:
				s.control <- res
			default:
				s.control <- res
			}
			s.console <- fmt.Sprintf("control response: [%v]", res)
		} else if res[:4] == "stat" {
			s.status <- res
			s.console <- fmt.Sprintf("status message: [%v]", res)
		} else {
			s.console <- fmt.Sprintf("unknown message: [%v]", res)
		}
	}
}

func (s *Ssmgr) keepAlive() {
	for {
		err := s.ping()
		if err != nil {
			s.console <- fmt.Sprint(err)
		}
		s.portsMux.Lock()
		for pt, pw := range s.ports {
			s.addPort(pt, pw)
		}
		s.portsMux.Unlock()
		time.Sleep(5 * time.Second)
	}
}

func (s *Ssmgr) recordStatus() {
	for {
		res := <-s.status
		data := []byte(res[6:])
		var objmap map[string]interface{}
		err := json.Unmarshal(data, &objmap)
		if err != nil {
			s.console <- fmt.Sprintf("json decode error: [%v][%v]", res, err)
			continue
		}
		for key, value := range objmap {
			k, e := strconv.Atoi(key)
			if e != nil {
				s.console <- fmt.Sprintf("invalid key: [%v]", key)
				continue
			}
			v, ok := value.(float64)
			if !ok {
				s.console <- fmt.Sprintf("invalid value: [%v]", value)
				continue
			}
			s.flowMux.Lock()
			s.flow[k] += int(v)
			s.flowMux.Unlock()
			// s.console <- fmt.Sprintf("total flow at port %v: %v", k, s.flow[k])
		}
	}
}

func (s *Ssmgr) sendCommand(cmd string) error {
	_, err := fmt.Fprint(s.conn, cmd)
	if err != nil {
		return err
	}
	return nil
}

func (s *Ssmgr) ping() error {
	s.sendMux.Lock()
	defer s.sendMux.Unlock()

	err := s.sendCommand("ping")
	if err != nil {
		return err
	}
	s.console <- "control request: [ping]"
	select {
	case res := <-s.control:
		if res == "pong" {
			return nil
		}
		return fmt.Errorf("unknown response: [%v]", res)
	case <-time.After(1 * time.Second):
		return fmt.Errorf("control request timed out: [ping]")
	}
}

func (s *Ssmgr) addPort(port int, password string) error {
	s.sendMux.Lock()
	defer s.sendMux.Unlock()

	err := s.sendCommand(fmt.Sprintf(`add: {"server_port": %v, "password":"%v"}`, port, password))
	if err != nil {
		return err
	}
	s.console <- fmt.Sprintf(`control request: [add: {"server_port": %v, "password":"%v"}]`, port, password)
	select {
	case res := <-s.control:
		if res == "ok" {
			return nil
		}
		return fmt.Errorf("unknown response: [%v]", res)
	case <-time.After(1 * time.Second):
		return fmt.Errorf("control request timed out: [add]")
	}
}

func (s *Ssmgr) removePort(port int) error {
	s.sendMux.Lock()
	defer s.sendMux.Unlock()

	err := s.sendCommand(fmt.Sprintf(`remove: {"server_port": %v}`, port))
	if err != nil {
		return err
	}
	s.console <- fmt.Sprintf(`control request: [remove: {"server_port": %v}]`, port)
	select {
	case res := <-s.control:
		if res == "ok" {
			return nil
		}
		return fmt.Errorf("unknown response: [%v]", res)
	case <-time.After(1 * time.Second):
		return fmt.Errorf("control request timed out: [remove]")
	}
}

//GetFlow outputs the current flow stats and reset
func (s *Ssmgr) GetFlow() map[int]int {
	s.flowMux.Lock()
	defer s.flowMux.Unlock()
	m := s.flow
	s.flow = make(map[int]int)
	return m
}

//SetPorts sets ports and passwords and returns flow before the change
func (s *Ssmgr) SetPorts(p map[int]string) map[int]int {
	s.portsMux.Lock()
	defer s.portsMux.Unlock()

	remove := make([]int, 0)
	add := make(map[int]string)
	for pt, pw := range s.ports {
		if p[pt] != pw {
			remove = append(remove, pt)
		}
	}
	for pt, pw := range p {
		if s.ports[pt] != pw {
			add[pt] = pw
		}
	}
	for _, pt := range remove {
		s.removePort(pt)
	}
	flow := s.GetFlow()
	for pt, pw := range add {
		s.addPort(pt, pw)
	}
	s.ports = p
	return flow
}

//Start the ssmgr daemon
func (s *Ssmgr) Start(console chan string, ready chan bool) error {
	err := s.connect()
	if err != nil {
		return err
	}
	go s.listen()
	go s.recordStatus()
	go s.keepAlive()
	ready <- true
	for {
		c := <-s.console
		console <- c
	}
}
