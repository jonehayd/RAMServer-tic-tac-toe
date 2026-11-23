package main

import (
	"encoding/binary"
	"errors"
)

// enc builds a protobuf message. Scalars at their default value are omitted, as proto3 does;
// submessages are always written, so an empty one still selects its oneof field.
type enc []byte

func (e *enc) tag(field, wire int) { *e = binary.AppendUvarint(*e, uint64(field<<3|wire)) }

func (e *enc) uint(field int, v uint64) {
	if v == 0 {
		return
	}
	e.tag(field, 0)
	*e = binary.AppendUvarint(*e, v)
}

func (e *enc) boolean(field int, v bool) {
	if v {
		e.uint(field, 1)
	}
}

func (e *enc) bytes(field int, b []byte) {
	if len(b) == 0 {
		return
	}
	e.tag(field, 2)
	*e = binary.AppendUvarint(*e, uint64(len(b)))
	*e = append(*e, b...)
}

func (e *enc) str(field int, s string) { e.bytes(field, []byte(s)) }

func (e *enc) msg(field int, m []byte) {
	e.tag(field, 2)
	*e = binary.AppendUvarint(*e, uint64(len(m)))
	*e = append(*e, m...)
}

// fields is a decoded message: the last value seen for each field number
type fields map[int]value

type value struct {
	v uint64
	b []byte
}

func parse(b []byte) (fields, error) {
	f := fields{}
	for len(b) > 0 {
		key, n := binary.Uvarint(b)
		if n <= 0 {
			return nil, errors.New("bad key")
		}
		b = b[n:]
		field, wire := int(key>>3), int(key&7)
		switch wire {
		case 0:
			v, n := binary.Uvarint(b)
			if n <= 0 {
				return nil, errors.New("bad varint")
			}
			b = b[n:]
			f[field] = value{v: v}
		case 2:
			l, n := binary.Uvarint(b)
			if n <= 0 || uint64(len(b)-n) < l {
				return nil, errors.New("bad length")
			}
			f[field] = value{b: b[n : n+int(l)]}
			b = b[n+int(l):]
		case 1:
			if len(b) < 8 {
				return nil, errors.New("short fixed64")
			}
			b = b[8:]
		case 5:
			if len(b) < 4 {
				return nil, errors.New("short fixed32")
			}
			b = b[4:]
		default:
			return nil, errors.New("unsupported wire type")
		}
	}
	return f, nil
}

func (f fields) uint(n int) uint64  { return f[n].v }
func (f fields) boolean(n int) bool { return f[n].v != 0 }
func (f fields) str(n int) string   { return string(f[n].b) }
func (f fields) bytes(n int) ([]byte, bool) {
	v, ok := f[n]
	return v.b, ok
}
