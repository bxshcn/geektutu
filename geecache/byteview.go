package geecache

type ByteView struct {
	b []byte
}

func (v ByteView) Len() int {
	return len(v.b)
}

func (v ByteView) ByteSlice() []byte {
	slice := make([]byte, v.Len())
	copy(slice, v.b)
	return slice
}

func (v ByteView) String() string {
	return string(v.b)
}
