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
	Solution Substitution `json:"answer"`
	// Stdout is captured standard output text from this query.
	Stdout string
	// Stderr is captured standard error text from this query.
	Stderr string
}

type response struct {
	Answer
	Status queryStatus
	Error  json.RawMessage // ball
}

func (pl *prolog) parse(goal, stdout, stderr string) (Answer, error) {
	// log.Println("parse:", goal, "stdout:", stdout, "stderr:", stderr)
	if len(strings.TrimSpace(stdout)) == 0 {
		return Answer{}, ErrFailure{Query: goal, Stderr: stderr}
	}

	start := strings.IndexRune(stdout, stx)
	end := strings.IndexRune(stdout, etx)
	if end == -1 {
		return Answer{}, fmt.Errorf("trealla: invalid query: %s", stdout)
	}
	nl := strings.IndexRune(stdout[end+1:], '\n') + end + 1
	butt := len(stdout)
	if nl >= 0 {
		butt = nl
	}

	// fmt.Println("OUTPUT:", stdout)

	output := stdout[start+1 : end]
	js := stdout[end+1 : butt]

	if pl.stdout != nil {
		pl.stdout.Println(output)
	}
	if pl.stderr != nil {
		pl.stderr.Println(stderr)
	}
	if pl.debug != nil {
		pl.debug.Println(string(js))
	}

	resp := response{
		Answer: Answer{
			Query:  goal,
			Stdout: output,
			Stderr: stderr,
		},
	}

	dec := json.NewDecoder(strings.NewReader(js))
	dec.UseNumber()
	if err := dec.Decode(&resp); err != nil {
		return resp.Answer, fmt.Errorf("trealla: decoding error: %w", err)
	}

	// spew.Dump(resp)

	switch resp.Status {
	case statusSuccess:
		return resp.Answer, nil
	case statusFailure:
		return resp.Answer, ErrFailure{Query: goal, Stdout: output, Stderr: stderr}
	case statusError:
		ball, err := unmarshalTerm(resp.Error)
		if err != nil {
			return resp.Answer, err
		}
		return resp.Answer, ErrThrow{Query: goal, Ball: ball, Stdout: output, Stderr: stderr}
	default:
		return resp.Answer, fmt.Errorf("trealla: unexpected query status: %v", resp.Status)
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
