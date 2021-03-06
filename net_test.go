package gowsps

import (
	"fmt"
	"log"
	"net/http"
	"testing"
)

type TestNest struct {
	Name  string
	Value uint8
}

type TestPacket struct {
	Name   string
	User   uint8
	Values []TestNest
}

func onTestPacket(packet *TestPacket) {
	fmt.Printf("Name %s %d\n", packet.Name, packet.User)
}

func TestA(t *testing.T) {
	s := NewPacketSystem()
	AddHandler(s, 0x02, onTestPacket)

	http.HandleFunc("/ws", func(writer http.ResponseWriter, request *http.Request) {

		var c *Connection

		s.UpgradeAndListen(writer, request, func(conn *Connection, err error) {
			c = conn
			if err != nil {
				panic(err)
			}

			p := TestPacket{Name: "Jacob", User: 2, Values: []TestNest{
				{"HI", 12},
				{"HI 2", 15},
			}}
			c.Send(Packet{Id: 0x2, Data: p})
		})

		log.Println("Started connection", c)
	})

	err := http.ListenAndServe("localhost:8080", nil)
	if err != nil {
		return
	}
}
