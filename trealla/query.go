package trealla

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wasmerio/wasmer-go/wasmer"
)

const etx = "\x03" // END OF TEXT

// Query executes a query.
func (pl *prolog) Query(ctx context.Context, program string) (Answer, error) {
	raw, err := pl.ask(ctx, program)
	if err != nil {
		return Answer{}, err
	}

	output, js, found := strings.Cut(raw, etx)
	resp := response{
		Answer: Answer{
			Query:  program,
			Output: output,
		},
	}
	if !found {
		return resp.Answer, fmt.Errorf("trealla: unexpected output (missing ETX): %s", raw)
	}

	dec := json.NewDecoder(strings.NewReader(js))
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
	builder := wasmer.NewWasiStateBuilder("tpl").
		Argument("-g").Argument(query).
		Argument("-q").
		CaptureStdout()
	if pl.preopen != "" {
		builder = builder.PreopenDirectory(pl.preopen)
	}
	for alias, dir := range pl.dirs {
		builder = builder.MapDirectory(alias, dir)
	}
	wasiEnv, err := builder.Finalize()
	if err != nil {
		return "", err
	}

	importObject, err := wasiEnv.GenerateImportObject(pl.store, pl.module)
	if err != nil {
		return "", err
	}
	instance, err := wasmer.NewInstance(pl.module, importObject)
	if err != nil {
		return "", err
	}
	defer instance.Close()
	start, err := instance.Exports.GetWasiStartFunction()
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
		_, err := start()
		ch <- err
	}()

	select {
	case <-ctx.Done():
		return "", fmt.Errorf("trealla: canceled: %w", ctx.Err())
	case err := <-ch:
		stdout := string(wasiEnv.ReadStdout())
		return stdout, err
	}
}

func escapeQuery(query string) string {
	query = stringEscaper.Replace(query)
	return fmt.Sprintf(`use_module(library(wasm_toplevel)), wasm_ask("%s")`, query)
}

var stringEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`)
