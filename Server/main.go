package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"mecm2m-Simulator/pkg/m2mapi"
	"mecm2m-Simulator/pkg/message"

	"github.com/joho/godotenv"
)

const (
	protocol        = "unix"
	graphDBSockAddr = "/tmp/mecm2m/svr_1_graphdb"
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
	loadEnv()
	//MEC Server起動時に，MEC Server内のコンポーネント (API, LocalManager, PNManager, AAA, SensingDB, GraphDB, LocalRepo) のスレッドファイルを開けておく
	var socketFiles []string
	socketFiles = append(socketFiles, "/tmp/sock1.sock", "/tmp/mecm2m/svr_1_m2mapi.sock", graphDBSockAddr)
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
		go initialize(file)
	}
	fmt.Scanln()
}

func initialize(file string) {
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

		switch file {
		case "/tmp/mecm2m/svr_1_m2mapi.sock":
			go m2mApi(conn)
		case graphDBSockAddr:
			go GraphDB(conn)
		}
	}
}

func m2mApi(conn net.Conn) {
	defer conn.Close()

	decoder := gob.NewDecoder(conn)
	encoder := gob.NewEncoder(conn)

	message.MyMessage("[MESSEGE] Call m2m api thread")

	for {
		//any型を使いたい ← Decode()でAppの内容を読もうとしてもnilとなってしまう
		//M2M Appとやりとりをする始めに型の同期をとる
		m, function := syncFormatServer(decoder, encoder)
		//fmt.Printf("%T\n", m)
		//M2MAppでexitが入力されたら，breakする
		if function == "exit" {
			break
		}
		if err := decoder.Decode(m); err != nil {
			if err == io.EOF {
				message.MyMessage("=== closed by client")
				break
			}
			message.MyError(err, "m2mApi > decoder.Decode")
			break
		}
		message.MyReadMessage(m) //1. 同じ内容

		//M2M Appから受信した内容を元にどのAPIが呼び出されているかを判断
		switch function {
		case "Point":
			connDB, err := net.Dial(protocol, graphDBSockAddr)
			if err != nil {
				message.MyError(err, "m2mApi > Point > net.Dial")
			}
			decoderDB := gob.NewDecoder(connDB)
			encoderDB := gob.NewEncoder(connDB)

			syncFormatClient(function, decoderDB, encoderDB)

			if err := encoderDB.Encode(m); err != nil {
				message.MyError(err, "m2mApi > Point > encoderDB.Encode")
			}
			message.MyWriteMessage(m) //1. 同じ内容

			//GraphDB()によるDB検索

			//受信する型は[]PSink
			ms := []m2mapi.ResolvePoint{}
			if err := decoderDB.Decode(&ms); err != nil {
				message.MyError(err, "m2mApi > Point > decoderDB.Decode")
			}
			message.MyReadMessage(ms)

			//最終的な結果をM2M Appに送信する
			if err := encoder.Encode(&ms); err != nil {
				message.MyError(err, "m2mApi > Point > encoder.Encode")
				break
			}
			message.MyWriteMessage(ms)
		case "Node":
			connDB, err := net.Dial(protocol, graphDBSockAddr)
			if err != nil {
				message.MyError(err, "m2mApi > Node > net.Dial")
			}
			decoderDB := gob.NewDecoder(connDB)
			encoderDB := gob.NewEncoder(connDB)

			syncFormatClient(function, decoderDB, encoderDB)

			if err := encoderDB.Encode(m); err != nil {
				message.MyError(err, "m2mApi > Node > encoderDB.Encode")
			}
			message.MyWriteMessage(m) //1. 同じ内容

			//GraphDB()によるDB検索

			//受信する型は[]PNode
			ms := []m2mapi.ResolveNode{}
			if err := decoderDB.Decode(&ms); err != nil {
				message.MyError(err, "m2mApi > Node > decoderDB.Decode")
			}
			message.MyReadMessage(ms)

			//最終的な結果をM2M Appに送信する
			if err := encoder.Encode(&ms); err != nil {
				message.MyError(err, "m2mApi > Node > encoder.Encode")
				break
			}
			message.MyWriteMessage(ms)
		}
	}
}

//M2M Appと型同期をするための関数
func syncFormatServer(decoder *gob.Decoder, encoder *gob.Encoder) (any, string) {
	m := &Format{}
	if err := decoder.Decode(m); err != nil {
		if err == io.EOF {
			typeM := "exit"
			typeResult := "exit"
			return typeM, typeResult
		} else {
			message.MyError(err, "syncFormatServer > decoder.Decode")
		}
	}

	typeResult := m.FormType

	if err := encoder.Encode(m); err != nil {
		message.MyError(err, "syncFormatServer > encoder.Encode")
	}

	var typeM any
	switch typeResult {
	case "Point":
		typeM = &m2mapi.ResolvePoint{}
	case "Node":
		typeM = &m2mapi.ResolveNode{}
	}
	return typeM, typeResult
}

//内部コンポーネント（DB，仮想モジュール）と型同期をするための関数
func syncFormatClient(command string, decoder *gob.Decoder, encoder *gob.Encoder) {
	switch command {
	case "Point":
		m := &Format{FormType: "Point"}
		if err := encoder.Encode(m); err != nil {
			message.MyError(err, "syncFormatClient > Point > encoder.Encode")
		}

		if err := decoder.Decode(m); err != nil {
			message.MyError(err, "syncFormatClient > Point > decoder.Decode")
		}
	case "Node":
		m := &Format{FormType: "Node"}
		if err := encoder.Encode(m); err != nil {
			message.MyError(err, "syncFormatClient > Node > encoder.Encode")
		}

		if err := decoder.Decode(m); err != nil {
			message.MyError(err, "syncFormatClient > Node > decoder.Decode")
		}
	}
}

//GraphDB Server
func GraphDB(conn net.Conn) {
	defer conn.Close()

	decoder := gob.NewDecoder(conn)
	encoder := gob.NewEncoder(conn)

	message.MyMessage("[MESSEGE] Call GraphDB thread")

	for {
		//any型を使いたい ← Decode()でAppの内容を読もうとしてもnilとなってしまう
		//M2M APIとやりとりをする始めに型の同期をとる
		m, function := syncFormatServer(decoder, encoder)
		//fmt.Printf("%T\n", m)
		if err := decoder.Decode(m); err != nil {
			if err == io.EOF {
				message.MyMessage("=== closed by client")
				break
			}
			message.MyError(err, "GraphDB > decoder.Decode")
			break
		}
		message.MyReadMessage(m)

		//DB検索
		switch function {
		case "Point":
			//型アサーション
			var swlat, swlon, nelat, nelon float64
			input := m.(*m2mapi.ResolvePoint)
			swlat = input.SW.Lat
			swlon = input.SW.Lon
			nelat = input.NE.Lat
			nelon = input.NE.Lon

			payload := `{"statements": [{"statement": "MATCH (ps:PSink)-[:isVirtualizedWith]->(vp:VPoint) WHERE ps.Lat > ` + strconv.FormatFloat(swlat, 'f', 4, 64) + ` and ps.Lon > ` + strconv.FormatFloat(swlon, 'f', 4, 64) + ` and ps.Lat < ` + strconv.FormatFloat(nelat, 'f', 4, 64) + ` and ps.Lon < ` + strconv.FormatFloat(nelon, 'f', 4, 64) + ` return ps.PSinkID, vp.Address;"}]}`
			//今後はクラウドサーバ用の分岐が必要
			var url string
			url = "http://" + os.Getenv("NEO4J_USERNAME") + ":" + os.Getenv("NEO4J_PASSWORD") + "@" + "localhost:" + os.Getenv("NEO4J_PORT") + "/db/data/transaction/commit"
			datas := ListenServer(payload, url)

			pss := []m2mapi.ResolvePoint{}
			for _, data := range datas {
				dataArray := data.([]interface{})
				ps := m2mapi.ResolvePoint{}
				ps.VPointID_n = dataArray[0].(string)
				ps.Address = dataArray[1].(string)
				flag := 0
				for _, p := range pss {
					if p.VPointID_n == ps.VPointID_n {
						flag = 1
					}
				}
				if flag == 0 {
					pss = append(pss, ps)
				}
			}

			if err := encoder.Encode(&pss); err != nil {
				message.MyError(err, "GraphDB > Point > encoder.Encode")
			}
			message.MyWriteMessage(pss)
		case "Node":
			//型アサーション
			var vpointid_n string
			input := m.(*m2mapi.ResolveNode)
			vpointid_n = input.VPointID_n
			caps := input.CapsInput
			format_vpointidn := "\\\"" + vpointid_n + "\\\""
			var format_caps []string
			for _, cap := range caps {
				cap = "\\\"" + cap + "\\\""
				format_caps = append(format_caps, cap)
			}
			payload := `{"statements": [{"statement": "MATCH (ps:PSink {PSinkID: ` + format_vpointidn + `})-[:requestsViaDevApi]->(pn:PNode) WHERE pn.Capability IN [` + strings.Join(format_caps, ", ") + `] return pn.Capability, pn.PNodeID;"}]}`
			//今後はクラウドサーバ用の分岐が必要
			var url string
			url = "http://" + os.Getenv("NEO4J_USERNAME") + ":" + os.Getenv("NEO4J_PASSWORD") + "@" + "localhost:" + os.Getenv("NEO4J_PORT") + "/db/data/transaction/commit"
			datas := ListenServer(payload, url)

			nds := []m2mapi.ResolveNode{}
			for _, data := range datas {
				dataArray := data.([]interface{})
				pn := m2mapi.ResolveNode{}
				capability := dataArray[0].(string)
				//CapOutputを1つにするか配列にして複数まとめられるようにするか要検討
				//pn.CapOutput = append(pn.CapOutput, capability)
				pn.CapOutput = capability
				pn.VNodeID_n = dataArray[1].(string)
				flag := 0
				for _, p := range nds {
					if p.VNodeID_n == pn.VNodeID_n {
						flag = 1
					} /*else {
						//CapOutputを1つにするか配列にして複数まとめられるようにするか要検討
						p.Capabilities = append(p.Capabilities, capability)
					}*/
				}
				if flag == 0 {
					nds = append(nds, pn)
				}
			}

			if err := encoder.Encode(&nds); err != nil {
				message.MyError(err, "GraphDB > Node > encoder.Encode")
			}
			message.MyWriteMessage(nds)
		}
	}
}

// MEC/Cloud Server へGraph DBの解決要求
func ListenServer(payload string, url string) []interface{} {
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")

	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		message.MyError(err, "ListenServer > client.Do")
	}
	defer resp.Body.Close()
	byteArray, _ := ioutil.ReadAll(resp.Body)

	var datas []interface{}
	if strings.Contains(url, "neo4j") {
		datas = BodyNeo4j(byteArray)
	} else {
		datas = BodyGraphQL(byteArray)
	}
	return datas
}

// Query Server から返ってきた　Reponse を探索し,中身を返す
func BodyNeo4j(byteArray []byte) []interface{} {
	var jsonBody map[string]interface{}
	if err := json.Unmarshal(byteArray, &jsonBody); err != nil {
		message.MyError(err, "BodyNeo4j > json.Unmarshal")
		return nil
	}
	var datas []interface{}
	//message.MyMessage("jsonBody: ", jsonBody)
	for _, v1 := range jsonBody {
		for _, v2 := range v1.([]interface{}) {
			for k3, v3 := range v2.(map[string]interface{}) {
				if k3 == "data" {
					for _, v4 := range v3.([]interface{}) {
						for k5, v5 := range v4.(map[string]interface{}) {
							if k5 == "row" {
								datas = append(datas, v5)
							}
						}
					}
				}
			}
		}
	}
	return datas
}

func BodyGraphQL(byteArray []byte) []interface{} {
	var jsonBody map[string]interface{}
	if err := json.Unmarshal(byteArray, &jsonBody); err != nil {
		message.MyError(err, "BodyGraphQL > json.Unmarshal")
		return nil
	}
	var values []interface{}
	//fmt.Println(jsonBody)
	for _, v1 := range jsonBody {
		for _, v2 := range v1.(map[string]interface{}) {
			switch v2.(type) {
			case []interface{}:
				values = v2.([]interface{})
			case map[string]interface{}:
				for _, v3 := range v2.(map[string]interface{}) {
					values = append(values, v3)
				}
			}
		}
	}
	return values
}

func loadEnv() {
	//.envファイルの読み込み
	if err := godotenv.Load(os.Getenv("HOME") + "/.env"); err != nil {
		log.Fatal(err)
	}
	mes := os.Getenv("SAMPLE_MESSAGE")
	//fmt.Printf("\x1b[32m%v\x1b[0m\n", message)
	message.MyMessage(mes)
}
