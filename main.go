package main

import (
	"fmt"

	"github.com/llc1123/go-ssmgr-client/ssmgr"
)

func main() {
	addr := "localhost:4000"
	s := ssmgr.NewSsmgr(addr)
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
		s.AddPort(8123, "123")
		s.AddPort(8234, "456")
		s.RemovePort(8123)
		s.RemovePort(8234)
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
