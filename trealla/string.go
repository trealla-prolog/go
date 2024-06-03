package trealla

import (
	"fmt"
)

type cstring struct {
	ptr  uint32
	size int
}

func newCString(pl *prolog, str string) (*cstring, error) {
	cstr := &cstring{
		size: len(str) + 1,
	}

	ptrv, err := pl.realloc.Call(pl.ctx, 0, 0, align, uint64(cstr.size))
	if err != nil {
		return nil, err
	}

	cstr.ptr = uint32(ptrv[0])
	if cstr.ptr == 0 {
		return nil, fmt.Errorf("trealla: failed to allocate string: %s", str)
	}

	data, _ := pl.memory.Read(cstr.ptr, uint32(len(str)+1))
	data[len(str)] = 0
	copy(data, []byte(str))
	return cstr, nil
}

func (str *cstring) free(pl *prolog) error {
	if str.ptr == 0 {
		return nil
	}

	_, err := pl.free.Call(pl.ctx, uint64(str.ptr), uint64(str.size), 1)
	str.ptr = 0
	str.size = 0
	return err
}

func (pl *prolog) gets(addr, size uint32) (string, error) {
	if addr == 0 || size == 0 {
		return "", nil
	}
	data, ok := pl.memory.Read(addr, size)
	if !ok {
		return "", fmt.Errorf("invalid string of %d length at: %d", size, addr)
	}
	// fmt.Println("gets", addr, size, string(data[ptr:end]))
	return string(data), nil
}
