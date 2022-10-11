package trealla

import "fmt"

type cstring struct {
	ptr  int32
	size int
}

func newCString(pl *prolog, str string) (*cstring, error) {
	cstr := &cstring{
		size: len(str) + 1,
	}

	ptrv, err := pl.realloc(0, 0, 1, cstr.size)
	if err != nil {
		return nil, err
	}

	cstr.ptr = ptrv.(int32)
	if cstr.ptr == 0 {
		return nil, fmt.Errorf("trealla: failed to allocate string: %s", str)
	}

	data := pl.memory.Data()
	ptr := int(cstr.ptr)
	copy(data[ptr:], []byte(str))
	data[ptr+len(str)] = 0
	return cstr, nil
}

func (str *cstring) free(pl *prolog) error {
	if str.ptr == 0 {
		return nil
	}

	_, err := pl.free(str.ptr, str.size, 1)
	str.ptr = 0
	str.size = 0
	return err
}
