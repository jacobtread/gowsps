package gowsps

import (
	"bytes"
	"encoding/binary"
	"errors"
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
	v := []byte(value)
	err := p.WriteVarInt(VarInt(len(v)))
	if err != nil {
		return err
	}
	if err = binary.Write(p, binary.BigEndian, v); err != nil {
		return err
	}
	return nil
}

func (p *PacketBuffer) ReadString() (string, error) {
	l, err := binary.ReadUvarint(p)
	if err != nil {
		return "", err
	}
	buff, err := p.ReadByteArray(VarInt(l))
	if err != nil {
		return "", err
	}
	return string(buff), nil
}

func MarshalPacket(p *PacketBuffer, packet *Packet) error {
	err := p.WriteVarInt(packet.Id)
	if err != nil {
		return err
	}
	err = marshalPacketData(p, packet.Data)
	if err != nil {
		return err
	}
	return nil
}

func marshalPacketData(p *PacketBuffer, data any) error {
	err := marshalValue(p, data)
	if err != nil {
		return err
	}
	return nil
}

func marshalValue(p *PacketBuffer, b any) error {
	x := reflect.ValueOf(b)
	rk := x.Kind()
	var err error
	switch rk {
	case reflect.Struct:
		fc := x.NumField()
		for i := 0; i < fc; i++ {
			fb := x.Field(i)
			v := fb.Interface()
			err = marshalValue(p, v)
		}
	case reflect.Slice:
		err := marshalSlice(p, b)
		if err != nil {
			return err
		}
	case reflect.Map:
		err := marshalMap(p, b)
		if err != nil {
			return err
		}
	default:
		err = marshalPrimitive(p, reflect.ValueOf(b))
		if err != nil {
			return err
		}
	}
	return err
}

func marshalSlice(p *PacketBuffer, v any) error {
	t := reflect.TypeOf(v)
	vl := reflect.ValueOf(v)
	l := vl.Len()
	err := p.WriteVarInt(VarInt(l))
	if err != nil {
		return err
	}
	tk := t.Elem().Kind()
	switch tk {
	case reflect.Struct:
		for i := 0; i < l; i++ {
			vi := vl.Index(i).Interface()
			err := marshalValue(p, vi)
			if err != nil {
				return err
			}
		}
	case reflect.Slice:
		for i := 0; i < l; i++ {
			vi := vl.Index(i).Interface()
			err := marshalSlice(p, vi)
			if err != nil {
				return err
			}
		}
	default:
		for i := 0; i < l; i++ {
			vi := vl.Index(i)
			err := marshalPrimitive(p, vi)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func marshalMap(p *PacketBuffer, v any) error {
	vl := reflect.ValueOf(v)
	count := vl.Len()
	err := p.WriteVarInt(VarInt(count))
	if err != nil {
		return err
	}
	keys := vl.MapKeys()
	for _, key := range keys {
		f := vl.MapIndex(key)
		ki := key.Interface()
		vi := f.Interface()
		err = marshalPrimitive(p, reflect.ValueOf(ki))
		if err != nil {
			return err
		}
		err = marshalValue(p, vi)
		if err != nil {
			return err
		}
	}
	return nil
}

func marshalPrimitive(p *PacketBuffer, r reflect.Value) error {
	v := r.Interface()
	switch v.(type) {
	case VarInt:
		err := p.WriteVarInt(v.(VarInt))
		if err != nil {
			return err
		}
	case bool, uint8, uint16, uint32, int8,
		int16, int32, float32, float64:
		if err := binary.Write(p, binary.BigEndian, v); err != nil {
			return err
		}
	case string:
		if err := p.WriteString(v.(string)); err != nil {
			return err
		}
	}
	return nil
}

func UnMarshalPacket(p *PacketBuffer, out any) error {
	return unmarshalValue(p, reflect.ValueOf(out).Elem())
}

func unmarshalValue(p *PacketBuffer, x reflect.Value) error {
	if x.Kind() == reflect.Pointer {
		x = x.Elem()
	}
	t := x.Type()
	rk := x.Kind()
	var err error
	switch rk {
	case reflect.Struct:
		fc := x.NumField()
		for i := 0; i < fc; i++ {
			fb := x.Field(i)
			err = unmarshalValue(p, fb)
		}
	case reflect.Slice:
		err = unmarshalSlice(p, x)
		if err != nil {
			return err
		}
	case reflect.Map:
		err = unmarshalMap(p, t)
		if err != nil {
			return err
		}
	default:
		err = unmarshalPrimitive(p, x)
		if err != nil {
			return err
		}
	}
	return err
}

func unmarshalSlice(p *PacketBuffer, vl reflect.Value) error {
	le, err := binary.ReadUvarint(p)
	if err != nil {
		return err
	}
	l := int(le)
	t := vl.Type()
	te := t.Elem()

	tk := te.Kind()
	s := reflect.MakeSlice(t, l, l)
	vl.Set(s)

	switch tk {
	case reflect.Struct:
		for i := 0; i < l; i++ {
			vi := vl.Index(i)
			err = unmarshalValue(p, vi)
			if err != nil {
				return err
			}
		}
	case reflect.Slice:
		for i := 0; i < l; i++ {
			vi := vl.Index(i)
			err = unmarshalSlice(p, vi)
			if err != nil {
				return err
			}
		}
	case reflect.Uint8:
		buff := make([]byte, l)
		count, err := io.ReadFull(p, buff)
		if err != nil {
			return err
		}
		if count != int(l) {
			return errors.New("incorrect length")
		}
		vl.SetBytes(buff)
	default:
		for i := 0; i < l; i++ {
			vi := vl.Index(i)
			err = unmarshalPrimitive(p, vi)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func unmarshalMap(p *PacketBuffer, t reflect.Type) error {
	count, err := binary.ReadUvarint(p)
	if err != nil {
		return err
	}
	kt := t.Key()
	vt := t.Elem()
	for i := uint64(0); i < count; i++ {
		key := reflect.New(kt)
		err = unmarshalPrimitive(p, key)
		if err != nil {
			return err
		}
		value := reflect.New(vt)
		err = unmarshalValue(p, value)
		if err != nil {
			return err
		}
	}
	return nil
}

func unmarshalPrimitive(p *PacketBuffer, r reflect.Value) error {
	v := r.Interface()
	switch v.(type) {
	case VarInt:
		val, err := binary.ReadUvarint(p)
		if err != nil {
			return err
		}
		r.Set(reflect.ValueOf(VarInt(val)))
	case uint8:
		var out uint8
		if err := binary.Read(p, binary.BigEndian, &out); err != nil {
			return err
		}
		r.Set(reflect.ValueOf(out))
	case uint16:
		var out uint16
		if err := binary.Read(p, binary.BigEndian, &out); err != nil {
			return err
		}
		r.Set(reflect.ValueOf(out))
	case uint32:
		var out uint32
		if err := binary.Read(p, binary.BigEndian, &out); err != nil {
			return err
		}
		r.Set(reflect.ValueOf(out))

	case int8:
		var out int8
		if err := binary.Read(p, binary.BigEndian, &out); err != nil {
			return err
		}
		r.Set(reflect.ValueOf(out))
	case int16:
		var out int16
		if err := binary.Read(p, binary.BigEndian, &out); err != nil {
			return err
		}
		r.Set(reflect.ValueOf(out))
	case int32:
		var out int32
		if err := binary.Read(p, binary.BigEndian, &out); err != nil {
			return err
		}
		r.Set(reflect.ValueOf(out))
	case float32:
		var out float32
		if err := binary.Read(p, binary.BigEndian, &out); err != nil {
			return err
		}
		r.Set(reflect.ValueOf(out))
	case float64:
		var out float64
		if err := binary.Read(p, binary.BigEndian, &out); err != nil {
			return err
		}
		r.Set(reflect.ValueOf(out))
	case bool:
		if err := binary.Read(p, binary.BigEndian, &v); err != nil {
			return err
		}
		r.Set(reflect.ValueOf(v))
	case string:
		val, err := p.ReadString()
		if err != nil {
			return err
		}
		r.Set(reflect.ValueOf(val))
	}
	return nil
}
