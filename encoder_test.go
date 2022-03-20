package gowsps

import "testing"

type TP struct {
	Name   string
	Number uint16
	A      string
	B      string
}

func TestMarshalPacket(t *testing.T) {
	MarshalPacket(TP{Name: "Test", Number: 1221, A: "", B: ""})
}
