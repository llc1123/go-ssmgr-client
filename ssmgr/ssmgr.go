package ssmgr

import (
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

//Ssmgr instance
type Ssmgr struct {
	mgrPort   int
	mgrCrypto string
	conn      *net.UDPConn
	flow      map[int]int
	ports     map[int]string
	console   chan string
	control   chan string
	status    chan string
	sendMux   sync.Mutex
	flowMux   sync.Mutex
	portsMux  sync.Mutex
}

//NewSsmgr initialize new ssmgr instance
func NewSsmgr(mgrPort int, mgrCrypto string) (*Ssmgr, error) {
	s := Ssmgr{
		mgrPort:   mgrPort,
		mgrCrypto: mgrCrypto,
		conn:      nil,
		flow:      make(map[int]int),
		ports:     make(map[int]string),
		console:   make(chan string),
		control:   make(chan string, 1),
		status:    make(chan string),
	}
	if err := s.connect(); err != nil {
		return nil, err
	}
	go s.listen()
	return &s, nil
}

func (s *Ssmgr) connect() error {
	raddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("localhost:%v", s.mgrPort))
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
	buffer := make([]byte, 1506)
	for {
		nRead, _, err := s.conn.ReadFrom(buffer)
		if err != nil {
			s.console <- fmt.Sprint(err)
			continue
		}
		res := string(buffer[:nRead])
		if res[:4] == "stat" {
			s.console <- fmt.Sprintf("status message: [%v]", res)
			recordStatus(res)
		} else {
			s.console <- fmt.Sprintf("control response: [%v]", res)
			select {
			case s.control <- res:
			default:
			}
		}
	}
}

func (s *Ssmgr) recordStatus(res string) {
	data := []byte(res[6:])
	var objmap map[string]interface{}
	err := json.Unmarshal(data, &objmap)
	if err != nil {
		s.console <- fmt.Sprintf("json decode error: [%v][%v]", res, err)
		return
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

func (s *Ssmgr) sendCommand(cmd string) (string, error) {
	s.sendMux.Lock()
	defer s.sendMux.Unlock()
	//empty the control channel before sending new command
	select {
	case <-s.control:
	default:
	}
	_, err := fmt.Fprint(s.conn, cmd)
	if err != nil {
		return "", err
	}
	select {
	case res := <-s.control:
		return res, nil
	case <-time.After(1 * time.Second):
		return "", fmt.Errorf("control request timed out: [ping]")
	}
}

func (s *Ssmgr) ping() error {
	s.console <- "control request: [ping]"
	res, err := s.sendCommand("ping")
	if err != nil {
		return err
	}
	if res == "pong" {
		return nil
	}
	return fmt.Errorf("unknown response: [%v]", res)
}

func (s *Ssmgr) addPort(port int, password string) error {
	s.console <- fmt.Sprintf(`control request: [add: {"server_port": %v, "password":"%v"}]`, port, password)
	res, err := s.sendCommand(fmt.Sprintf(`add: {"server_port": %v, "password":"%v"}`, port, password))
	if err != nil {
		return err
	}
	if res == "ok" {
		return nil
	}
	return fmt.Errorf("unknown response: [%v]", res)
}

func (s *Ssmgr) removePort(port int) error {
	s.console <- fmt.Sprintf(`control request: [remove: {"server_port": %v}]`, port)
	err := s.sendCommand(fmt.Sprintf(`remove: {"server_port": %v}`, port))
	if err != nil {
		return err
	}
	if res == "ok" {
		return nil
	}
	return fmt.Errorf("unknown response: [%v]", res)
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

func (s *Ssmgr) startManager() error {

	ch := make(chan error)

	//create ssmgr process
	go func() {
		cmd := exec.Command(
			"ssserver",
			"-m", s.mgrCrypto,
			"-p", "8888",
			"-k", "8888",
			"--manager-address", fmt.Sprintf("127.0.0.1:%v", s.mgrPort),
		)
		err := cmd.Run()
		if err != nil {
			ch <- err
		}
	}()

	//make the first connection and add ports
	go func() {

		time.Sleep(1 * time.Second)
		err := s.connect()
		if err != nil {
			ch <- err
			return
		}
		err = s.ping()
		if err != nil {
			ch <- err
			return
		}
		err = s.removePort(8888)
		if err != nil {
			ch <- err
			return
		}
		for pt, pw := range s.ports {
			err = s.addPort(pt, pw)
			if err != nil {
				ch <- err
				return
			}
		}
		s.portsMux.Unlock()
	}()

	defer s.portsMux.Lock()
	return <-ch
}

//Start the ssmgr daemon
func (s *Ssmgr) Start(console chan string) error {
	//start ssmgr
	go func() {
		for {
			err := s.startManager()
			if err != nil {
				console <- fmt.Sprint(err)
			}
		}
	}()
	for {
		c := <-s.console
		console <- c
	}
}
