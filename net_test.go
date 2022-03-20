package gowsps

import (
	"fmt"
	"log"
	"net/http"
	"testing"
)

type TestPacket struct {
	Name string `json:"name"`
}

func onTestPacket(packet *TestPacket) {
	fmt.Println(packet.Name)
}

func TestA(t *testing.T) {
	s := NewPacketSystem()
	AddHandler(s, 0x00, onTestPacket)

	http.HandleFunc("/ws", func(writer http.ResponseWriter, request *http.Request) {

		var c *Connection

		s.UpgradeAndListen(writer, request, func(conn *Connection, err error) {
			c = conn
			if err != nil {
				panic(err)
			}

			p := TestPacket{Name: "Jacob"}
			c.Send(Packet{Id: 0x00, Data: p})
		})

		log.Println("Started connection", c)
	})

	err := http.ListenAndServe("localhost:8080", nil)
	if err != nil {
		return
	}
}
