package main

import (
	"fmt"

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
		s.SetPorts(map[int]string{
			10001: "abc",
			10002: "bcd",
			10003: "cde",
			10004: "def",
		})
		s.SetPorts(map[int]string{
			10002: "cde",
			10003: "cde",
			10005: "efg",
		})
		// go func() {
		// 	for {
		// 		time.Sleep(20 * time.Second)
		// 		m := s.GetFlow()
		// 		for key, value := range m {
		// 			console <- fmt.Sprintf("total flow on port %v: %v", key, value)
		// 		}
		// 	}
		// }()
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
