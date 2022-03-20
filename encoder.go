package gowsps

import (
	"bytes"
	"encoding/binary"
	"reflect"
)

type PacketWriter struct {
	bytes.Buffer
}

func NewPacketWriter() *PacketWriter {
	writer := PacketWriter{
		Buffer: bytes.Buffer{},
	}
	return &writer
}

func (p *PacketWriter) WriteVarInt(value int64) error {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutVarint(buf[:], value)
	_, err := p.Write(buf[:n])
	return err
}

func (p *PacketWriter) WriteString(value string) error {
	err := p.WriteVarInt(int64(len(value)))
	if err != nil {
		return err
	}
}

func MarshalPacket(packet any, c *Connection) {
	v := reflect.ValueOf(packet)
	fc := v.NumField()
	for i := 0; i < fc; i++ {
		fv := v.Field(i)
		v := fv.Interface()
		switch v.(type) {
		case string:

		}
	}

}
