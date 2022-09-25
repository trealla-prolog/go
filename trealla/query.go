package trealla

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const stx = '\x02' // START OF TEXT
const etx = '\x03' // END OF TEXT

// Query executes a query.
func (pl *prolog) Query(ctx context.Context, program string) (Answer, error) {
	raw, err := pl.ask(ctx, program)
	if err != nil {
		return Answer{}, err
	}

	if idx := strings.IndexRune(raw, stx); idx >= 0 {
		raw = raw[idx+1:]
	} else {
		return Answer{}, fmt.Errorf("trealla: unexpected output (missing STX): %s", raw)
	}

	var output string
	if idx := strings.IndexRune(raw, etx); idx >= 0 {
		output = raw[:idx]
		raw = raw[idx+1:]
	} else {
		return Answer{}, fmt.Errorf("trealla: unexpected output (missing ETX): %s", raw)
	}

	if idx := strings.IndexRune(raw, stx); idx >= 0 {
		raw = raw[:idx]
	}

	resp := response{
		Answer: Answer{
			Query:  program,
			Output: output,
		},
	}

	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&resp); err != nil {
		return resp.Answer, fmt.Errorf("trealla: decoding error: %w", err)
	}

	switch resp.Result {
	case statusSuccess:
		return resp.Answer, nil
	case statusFailure:
		return resp.Answer, ErrFailure
	case statusError:
		ball, err := unmarshalTerm(resp.Error)
		if err != nil {
			return resp.Answer, err
		}
		return resp.Answer, ErrThrow{Ball: ball}
	default:
		return resp.Answer, fmt.Errorf("trealla: unexpected query status: %v", resp.Result)
	}
}

// Answer is a query result.
type Answer struct {
	// Query is the original query goal.
	Query string
	// Answers are the solutions (substitutions) for a successful query.
	Answers []Solution
	// Output is captured stdout text from this query.
	Output string
}

type response struct {
	Answer
	Result queryStatus
	Error  json.RawMessage // ball
}

// queryStatus is the status of a query answer.
type queryStatus string

// Result values.
const (
	// statusSuccess is for queries that succeed.
	statusSuccess queryStatus = "success"
	// statusFailure is for queries that fail (find no answers).
	statusFailure queryStatus = "failure"
	// statusError is for queries that throw an error.
	statusError queryStatus = "error"
)

func (pl *prolog) ask(ctx context.Context, query string) (string, error) {
	query = escapeQuery(query)
	qstr, err := newCString(pl, query)
	if err != nil {
		return "", err
	}
	defer qstr.free(pl)

	pl_eval, err := pl.instance.Exports.GetFunction("pl_eval")
	if err != nil {
		return "", err
	}

	ch := make(chan error, 2)
	go func() {
		defer func() {
			if ex := recover(); ex != nil {
				ch <- fmt.Errorf("trealla: panic: %v", ex)
			}
		}()
		_, err := pl_eval(pl.ptr, qstr.ptr)
		ch <- err
	}()

	select {
	case <-ctx.Done():
		return "", fmt.Errorf("trealla: canceled: %w", ctx.Err())
	case err := <-ch:
		stdout := string(pl.wasi.ReadStdout())
		return stdout, err
	}
}

type cstring struct {
	ptr  int32
	size int
}

func newCString(pl *prolog, str string) (*cstring, error) {
	cstr := &cstring{
		size: len(str) + 1,
	}
	size := len(str) + 1
	ptrv, err := pl.realloc(0, 0, 0, size)
	if err != nil {
		return nil, err
	}
	cstr.ptr = ptrv.(int32)
	err = cstr.set(pl, str)
	return cstr, err
}

func (cstr *cstring) set(pl *prolog, str string) error {
	data := pl.memory.Data()

	ptr := int(cstr.ptr)
	for i, b := range []byte(str) {
		data[ptr+i] = b
	}
	data[ptr+len(str)] = 0

	return nil
}

func (str *cstring) free(pl *prolog) error {
	if str.ptr == 0 {
		return nil
	}

	_, err := pl.free(str.ptr, str.size, 0)
	str.ptr = 0
	str.size = 0
	return err
}

func escapeQuery(query string) string {
	query = stringEscaper.Replace(query)
	return fmt.Sprintf(`use_module(library(js_toplevel)), js_ask("%s")`, query)
}

var stringEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`)
