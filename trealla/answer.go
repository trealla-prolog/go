package trealla

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Answer is a query result.
type Answer struct {
	// Query is the original query goal.
	Query string
	// Solution (substitutions) for a successful query.
	// Indexed by variable name.
	Solution Solution `json:"answer"`
	// Output is captured stdout text from this query.
	Output string
}

type response struct {
	Answer
	Result queryStatus
	Error  json.RawMessage // ball
}

func newAnswer(program, raw string) (Answer, error) {
	if len(strings.TrimSpace(raw)) == 0 {
		return Answer{}, ErrFailure
	}

	start := strings.IndexRune(raw, stx)
	end := strings.IndexRune(raw, etx)
	nl := strings.IndexRune(raw[end+1:], '\n') + end + 1
	butt := len(raw)
	if nl >= 0 {
		butt = nl
	}

	output := raw[start+1 : end]
	js := raw[end+1 : butt]

	resp := response{
		Answer: Answer{
			Query:  program,
			Output: output,
		},
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
