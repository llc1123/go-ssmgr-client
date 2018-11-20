package main

import (
	"fmt"
)

func main() {
	addr := "localhost:4000"
	s := NewSsmgr(addr)
	console := make(chan string)
	ready := make(chan bool)
	fatal := make(chan string)
	go func() {
		err := s.Start(console, ready)
		if err != nil {
			fatal <- fmt.Sprint(err)
		}
	}()
	go func() {
		<-ready
		go s.AddPort(8123, "123")
		go s.AddPort(8234, "456")
		go s.RemovePort(8123)
		go s.RemovePort(8456)
	}()
	for {
		select {
		case c := <-console:
			fmt.Println(c)
		case c := <-fatal:
			fmt.Println(c)
			return
		}
	}
}
