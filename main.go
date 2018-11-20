package main

import (
	"fmt"
	"time"

	"github.com/llc1123/go-ssmgr-client/ssmgr"
)

func main() {
	//INITIALIZE
	addr := "localhost:4000"
	s := ssmgr.NewSsmgr(addr)
	console := make(chan string)
	ready := make(chan bool)
	fatal := make(chan string)

	//START SSMGR MODULE
	go func() {
		err := s.Start(console, ready)
		if err != nil {
			fatal <- fmt.Sprint(err)
		}
	}()

	//START CONTROL MODULE
	go func() {
		<-ready
		s.AddPort(8123, "123")
		s.AddPort(8234, "456")
		s.RemovePort(8123)
		s.RemovePort(8234)
		go func() {
			for {
				time.Sleep(20 * time.Second)
				m := s.GetFlow()
				for key, value := range m {
					console <- fmt.Sprintf("total flow on port %v: %v", key, value)
				}
			}
		}()
	}()

	//KEEP MAIN THREAD UP AND PRINT CONSOLE LOG
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
