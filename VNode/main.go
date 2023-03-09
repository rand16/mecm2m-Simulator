package main

import (
	"bytes"
	"fmt"
	"mecm2m-Simulator/pkg/message"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strconv"
)

const (
	protocol = "unix"
)

type Format struct {
	FormType string
}

func cleanup(socketFiles ...string) {
	for _, sock := range socketFiles {
		if _, err := os.Stat(sock); err == nil {
			if err := os.RemoveAll(sock); err != nil {
				message.MyError(err, "cleanup > os.RemoveAll")
			}
		}
	}
}

func main() {
	//VNodeをいくつか用意しておく
	var socketFiles []string
	socketFiles = append(socketFiles, "/tmp/mecm2m/vnode_1_0001.sock", "/tmp/mecm2m/vnode_1_0002.sock", "/tmp/mecm2m/vnode_1_0003.sock")
	gids := make(chan uint64, len(socketFiles))
	cleanup(socketFiles...)

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	go func() {
		<-quit
		fmt.Println("ctrl-c pressed!")
		close(quit)
		cleanup(socketFiles...)
		os.Exit(0)
	}()

	for _, file := range socketFiles {
		go initialize(file, gids)
		data := <-gids
		fmt.Printf("GOROUTINE ID (%s): %d\n", file, data)
	}
	fmt.Scanln()
	defer close(gids)
}

func initialize(file string, gids chan uint64) {
	gids <- getGID()
	gid := getGID()
	listener, err := net.Listen(protocol, file)
	if err != nil {
		message.MyError(err, "initialize > net.Listen")
	}
	s := "> [Initialize] Socket file launched: " + file
	message.MyMessage(s)
	for {
		conn, err := listener.Accept()
		if err != nil {
			message.MyError(err, "initialize > listener.Accept")
			break
		}

		go vnode(conn, gid)
	}
}

func vnode(conn net.Conn, gid uint64) {
	defer conn.Close()

	//decoder := gob.NewDecoder(conn)
	//encoder := gob.NewEncoder(conn)

	s := "[MESSAGE] Call VNode thread(" + string(gid) + ")"
	message.MyMessage(s)

	for {
	}

}

func getGID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	//fmt.Println(string(b))
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}
