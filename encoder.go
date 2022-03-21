package gowsps

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"reflect"
)

type VarInt uint64

type PacketBuffer struct {
	*bytes.Buffer
}

func NewPacketBuffer() *PacketBuffer {
	writer := PacketBuffer{
		Buffer: &bytes.Buffer{},
	}
	return &writer
}

func (p *PacketBuffer) WriteVarInt(value VarInt) error {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], uint64(value))
	_, err := p.Write(buf[:n])
	return err
}

func (p *PacketBuffer) WriteByteArray(value []byte) error {
	err := p.WriteVarInt(VarInt(len(value)))
	if err != nil {
		return err
	}
	b := []byte(value)
	_, err = p.Write(b)
	if err != nil {
		return err
	}
	return nil
}

func (p *PacketBuffer) ReadByteArray(length VarInt) ([]byte, error) {
	buff := make([]byte, length)
	count, err := io.ReadFull(p, buff)
	if err != nil {
		return nil, err
	}
	if count != int(length) {
		return nil, errors.New("incorrect length")
	}
	return buff, nil
}

func (p *PacketBuffer) WriteString(value string) error {
	return p.WriteByteArray([]byte(value))
}
func (p *PacketBuffer) ReadString() (string, error) {
	l, err := binary.ReadVarint(p)
	if err != nil {
		return "", err
	}
	buff, err := p.ReadByteArray(VarInt(l))
	if err != nil {
		return "", err
	}
	return string(buff), nil
}

func (p *PacketBuffer) MarshalPacket(packet Packet) error {
	err := p.WriteVarInt(packet.Id)
	if err != nil {
		return err
	}

	data := packet.Data
	v := reflect.ValueOf(data)
	fc := v.NumField()
	for i := 0; i < fc; i++ {
		fv := v.Field(i)
		v := fv.Interface()
		switch v.(type) {
		case string:
			if err := p.WriteString(v.(string)); err != nil {
				return err
			}
		case bool:
			if v.(bool) {
				if err := p.WriteByte(1); err != nil {
					return err
				}
			} else {
				if err := p.WriteByte(0); err != nil {
					return err
				}
			}
		case uint8, uint16, uint32, int8, int16, int32, float32, float64:
			if err := binary.Write(p, binary.BigEndian, v); err != nil {
				return err
			}
		case []byte:
			if err := p.WriteByteArray(v.([]byte)); err != nil {
				return err
			}
		case VarInt:
			err := p.WriteVarInt(v.(VarInt))
			if err != nil {
				return err
			}
		}
	}
	fmt.Println(p.Buffer.Bytes())
	return nil
}

func (p *PacketBuffer) UnMarshalPacket(out any) error {
	t := reflect.TypeOf(out)
	v := reflect.ValueOf(out)
	fc := v.NumField()
	for i := 0; i < fc; i++ {
		ft := t.Field(i)
		fv := v.Field(i)
		v := fv.Interface()
		switch v.(type) {
		case string:
			fmt.Println(ft.Name)
			s, err := p.ReadString()
			if err != nil {
				return err
			}
			fv.SetString(s)
		case bool:
			a, err := p.ReadByte()
			if err != nil {
				return err
			}
			fv.SetBool(a == 1)
		case uint8, uint16, uint32, int8, int16, int32, float32, float64:
			err := binary.Read(p, binary.BigEndian, &v)
			if err != nil {
				return err
			}
		case []byte:
			l, err := binary.ReadVarint(p)
			if err != nil {
				return err
			}
			buff, err := p.ReadByteArray(VarInt(l))
			if err != nil {
				return err
			}
			fv.SetBytes(buff)
		case VarInt:
			l, err := binary.ReadVarint(p)
			if err != nil {
				return err
			}
			fv.Set(reflect.ValueOf(VarInt(l)))
		}
	}
	return nil
}
