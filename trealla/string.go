package trealla

import (
	"fmt"
)

type cstring struct {
	ptr  int32
	size int
}

func newCString(pl *prolog, str string) (*cstring, error) {
	cstr := &cstring{
		size: len(str) + 1,
	}

	ptrv, err := pl.realloc.Call(pl.store, 0, 0, 1, cstr.size)
	if err != nil {
		return nil, err
	}

	cstr.ptr = ptrv.(int32)
	if cstr.ptr == 0 {
		return nil, fmt.Errorf("trealla: failed to allocate string: %s", str)
	}

	data := pl.memory.UnsafeData(pl.store)
	ptr := int(cstr.ptr)
	copy(data[ptr:], []byte(str))
	data[ptr+len(str)] = 0
	return cstr, nil
}

func (str *cstring) free(pl *prolog) error {
	if str.ptr == 0 {
		return nil
	}

	_, err := pl.free.Call(pl.store, str.ptr, str.size, 1)
	str.ptr = 0
	str.size = 0
	return err
}

func (pl *prolog) gets(addr, size int32) (string, error) {
	if addr == 0 || size == 0 {
		return "", nil
	}
	data := pl.memory.UnsafeData(pl.store)
	ptr := int(uint32(addr))
	end := int(uint32(addr + size))
	if end >= len(data) {
		return "", fmt.Errorf("invalid string of %d length at: %d", size, addr)
	}
	// fmt.Println("gets", addr, size, string(data[ptr:end]))
	return string(data[ptr:end]), nil
}
