package protocol

import (
	"bytes"
	"encoding/binary"
)

type Fragment struct {
	SessionID, Sequence uint32
	Index, Count        byte
	Payload             []byte
}
type Frame struct {
	SessionID, Sequence uint32
	Payload             []byte
}

func ParseFragment(b []byte) (Fragment, error) {
	f := Fragment{}
	if len(b) < 17 || len(b) > 182 || binary.LittleEndian.Uint16(b) != 0x5041 || b[2] != 1 || b[3] != 0 {
		return f, ErrInvalid
	}
	f = Fragment{binary.LittleEndian.Uint32(b[4:]), binary.LittleEndian.Uint32(b[8:]), b[12], b[13], bytes.Clone(b[16:])}
	if f.Count < 1 || f.Count > 64 || f.Index >= f.Count || f.Sequence >= MaxFrames || int(binary.LittleEndian.Uint16(b[14:])) != len(f.Payload) {
		return Fragment{}, ErrInvalid
	}
	return f, nil
}

type frameKey struct{ session, sequence uint32 }
type partial struct {
	parts [][]byte
	size  int
}
type Assembler struct {
	pending   map[frameKey]*partial
	order     []frameKey
	completed map[frameKey]bool
	done      []frameKey
	Evicted   int
}

func (a *Assembler) Push(b []byte) (*Frame, error) {
	f, e := ParseFragment(b)
	if e != nil {
		return nil, e
	}
	k := frameKey{f.SessionID, f.Sequence}
	if a.pending == nil {
		a.pending = map[frameKey]*partial{}
		a.completed = map[frameKey]bool{}
	}
	if a.completed[k] {
		return nil, nil
	}
	p := a.pending[k]
	if p == nil {
		if len(a.order) == 64 {
			a.remove(a.order[0])
			a.Evicted++
		}
		p = &partial{parts: make([][]byte, int(f.Count))}
		a.pending[k] = p
		a.order = append(a.order, k)
	}
	if len(p.parts) != int(f.Count) {
		return nil, ErrInvalid
	}
	if p.parts[f.Index] != nil {
		if !bytes.Equal(p.parts[f.Index], f.Payload) {
			return nil, ErrInvalid
		}
		return nil, nil
	}
	if p.size+len(f.Payload) > FrameBytes {
		a.remove(k)
		return nil, ErrInvalid
	}
	p.parts[f.Index] = f.Payload
	p.size += len(f.Payload)
	for _, part := range p.parts {
		if part == nil {
			return nil, nil
		}
	}
	a.remove(k)
	if p.size != FrameBytes {
		return nil, ErrInvalid
	}
	a.completed[k] = true
	a.done = append(a.done, k)
	if len(a.done) > 128 {
		delete(a.completed, a.done[0])
		a.done = a.done[1:]
	}
	return &Frame{f.SessionID, f.Sequence, bytes.Join(p.parts, nil)}, nil
}
func (a *Assembler) remove(k frameKey) {
	delete(a.pending, k)
	for i, v := range a.order {
		if v == k {
			a.order = append(a.order[:i], a.order[i+1:]...)
			break
		}
	}
}

var steps = []int{7, 8, 9, 10, 11, 12, 13, 14, 16, 17, 19, 21, 23, 25, 28, 31, 34, 37, 41, 45, 50, 55, 60, 66, 73, 80, 88, 97, 107, 118, 130, 143, 157, 173, 190, 209, 230, 253, 279, 307, 337, 371, 408, 449, 494, 544, 598, 658, 724, 796, 876, 963, 1060, 1166, 1282, 1411, 1552, 1707, 1878, 2066, 2272, 2499, 2749, 3024, 3327, 3660, 4026, 4428, 4871, 5358, 5894, 6484, 7132, 7845, 8630, 9493, 10442, 11487, 12635, 13899, 15289, 16818, 18500, 20350, 22385, 24623, 27086, 29794, 32767}
var indexes = []int{-1, -1, -1, -1, 2, 4, 6, 8, -1, -1, -1, -1, 2, 4, 6, 8}

func Decode(b []byte) ([Samples]int16, error) {
	var out [Samples]int16
	if len(b) != 166 || b[2] > 88 || binary.LittleEndian.Uint16(b[3:]) != 320 || b[5] != 0 || b[165]&0xf0 != 0 {
		return out, ErrInvalid
	}
	p := int(int16(binary.LittleEndian.Uint16(b)))
	idx := int(b[2])
	out[0] = int16(p)
	for i := 1; i < Samples; i++ {
		n := int(b[6+(i-1)/2])
		if (i-1)%2 == 0 {
			n &= 15
		} else {
			n >>= 4
		}
		s := steps[idx]
		delta := s >> 3
		if n&4 != 0 {
			delta += s
		}
		if n&2 != 0 {
			delta += s >> 1
		}
		if n&1 != 0 {
			delta += s >> 2
		}
		if n&8 != 0 {
			p -= delta
		} else {
			p += delta
		}
		p = max(-32768, min(32767, p))
		idx = max(0, min(88, idx+indexes[n]))
		out[i] = int16(p)
	}
	return out, nil
}
func PCMBytes(p [Samples]int16) []byte {
	b := make([]byte, Samples*2)
	for i, v := range p {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(v))
	}
	return b
}
