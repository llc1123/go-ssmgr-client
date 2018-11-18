package main

import (
	"fmt"
	"net"
	"time"
)

var mgrAddr string

func sendCommand(command string) (string, error) {
	raddr, err := net.ResolveUDPAddr("udp", mgrAddr)
	if err != nil {
		return "", fmt.Errorf("invalid manager address")
	}
	conn, _ := net.DialUDP("udp", nil, raddr)
	defer conn.Close()
	fmt.Fprint(conn, command)
	buffer := make([]byte, 2048)

	deadline := time.Now().Add(time.Duration(time.Second))
	err = conn.SetReadDeadline(deadline)
	if err != nil {
		return "", err
	}

	nRead, _, err := conn.ReadFrom(buffer)
	if err != nil {
		return "", err
	}

	return string(buffer[:nRead]), nil
}

func addPort(port int, password string) error {
	res, err := sendCommand(fmt.Sprintf(`add: {"server_port": %v, "password":"%v"}`, port, password))
	if err != nil {
		return err
	}
	if res != "ok" {
		return fmt.Errorf("unknown response")
	}
	return nil
}

func removePort(port int) error {
	res, err := sendCommand(fmt.Sprintf(`remove: {"server_port": %v}`, port))
	if err != nil {
		return err
	}
	if res != "ok" {
		return fmt.Errorf("unknown response")
	}
	return nil
}

func ping() error {
	res, err := sendCommand("ping")
	if err != nil {
		return err
	}
	if res != "pong" {
		return fmt.Errorf("unknown response")
	}
	return nil
}

func main() {
	mgrAddr = "localhost:4000"
	var err error
	err = addPort(8000, "123")
	if err != nil {
		fmt.Println(err)
	}
	err = addPort(8001, "234")
	if err != nil {
		fmt.Println(err)
	}
	err = removePort(8000)
	if err != nil {
		fmt.Println(err)
	}
	err = ping()
	if err != nil {
		fmt.Println(err)
	}
}
