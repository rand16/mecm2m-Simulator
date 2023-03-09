package main

import (
	"encoding/gob"
	"fmt"
	"net"

	"mecm2m-Simulator/pkg/m2mapi"
	"mecm2m-Simulator/pkg/m2mapp"
	"mecm2m-Simulator/pkg/message"
)

const (
	protocol = "unix"
)

func main() {
	//configファイルを用意して，configファイルを読み込むことで，GraphDBやSensingDBの情報の初期値を用意できるようにする
	for {
		// クライアントがコマンドを入力
		var command string
		fmt.Printf("input command > ")
		fmt.Scan(&command)
		message.MyExit(command)

		sockAddr := selectSocketFile(command)
		conn, err := net.Dial(protocol, sockAddr)
		if err != nil {
			message.MyError(err, "main > net.Dial")
		}
		//defer conn.Close()

		decoder := gob.NewDecoder(conn)
		encoder := gob.NewEncoder(conn)
		commandExecution(command, decoder, encoder)
	}
}

// 入力したコマンドに対応するソケットファイルを選択
func selectSocketFile(command string) string {
	var sockAddr string
	defaultAddr := "/tmp/mecm2m"
	defaultExt := ".sock"
	switch command {
	case "m2mapi":
		sockAddr = defaultAddr + "/svr_1_m2mapi" + defaultExt
	case "m2mapp":
		sockAddr = defaultAddr + "/m2mapp_1" + defaultExt
	default:
		sockAddr = "/tmp/sock1" + defaultExt
	}
	return sockAddr
}

// 入力したコマンドに応じて，送受信するメッセージの内容を選択
func commandExecution(command string, decoder *gob.Decoder, encoder *gob.Encoder) {
	switch command {
	case "m2mapp":
		//適当なM2M App　IDを指定
		m := &m2mapp.App{
			AppID: "m2mapp_1",
		}
		if err := encoder.Encode(m); err != nil {
			message.MyError(err, "commandExecution > m2mapp > encoder.Encode")
		}
		message.MyWriteMessage(m)

		if err := decoder.Decode(m); err != nil {
			message.MyError(err, "commandExecution > m2mapp > decoder.Decode")
		}
		message.MyReadMessage(m)
	case "m2mapi":
		m := &m2mapi.Area{
			AreaID: command,
		}

		if err := encoder.Encode(m); err != nil {
			message.MyError(err, "commandExecution > m2mapi > encoder.Encode")
		}
		message.MyWriteMessage(m)

		if err := decoder.Decode(m); err != nil {
			message.MyError(err, "commandExecution > m2mapi > decoder.Decode")
		}
		message.MyReadMessage(m)
	default:

	}
}
