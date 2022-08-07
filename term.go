package trealla

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Term is a Prolog term.
//
// One of the following types:
//	- string
//	- int64
//	- float64
// 	- Compound
//	- Variable
type Term = any

// Solution is a mapping of variable names to substitutions.
// In other words, it's one answer to a query.
type Solution map[string]Term

// UnmarshalJSON implements the encoding/json.Marshaler interface.
func (sol *Solution) UnmarshalJSON(bs []byte) error {
	var raws map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(bs))
	dec.UseNumber()
	if err := dec.Decode(&raws); err != nil {
		return err
	}
	*sol = make(Solution, len(raws))
	for k, raw := range raws {
		term, err := unmarshalTerm(raw)
		if err != nil {
			return err
		}
		if _, ok := term.(Variable); ok {
			term = Variable(k)
		}
		(*sol)[k] = term
	}
	return nil
}

func unmarshalTerm(bs []byte) (Term, error) {
	var iface any
	dec := json.NewDecoder(bytes.NewReader(bs))
	dec.UseNumber()
	if err := dec.Decode(&iface); err != nil {
		return nil, err
	}

	switch x := iface.(type) {
	case string:
		return x, nil
	case json.Number:
		str := string(x)
		if strings.ContainsRune(str, '.') {
			return strconv.ParseFloat(str, 64)
		}
		return strconv.ParseInt(str, 10, 64)
	case []any:
		var raws []json.RawMessage
		dec := json.NewDecoder(bytes.NewReader(bs))
		dec.UseNumber()
		if err := dec.Decode(&raws); err != nil {
			return nil, err
		}
		list := make([]Term, 0, len(raws))
		for _, raw := range raws {
			term, err := unmarshalTerm(raw)
			if err != nil {
				return nil, err
			}
			list = append(list, term)
		}
		return list, nil
	case map[string]any:
		if _, ok := x["var"]; ok {
			return Variable("_"), nil
		}

		var raws map[string]json.RawMessage
		dec := json.NewDecoder(bytes.NewReader(bs))
		dec.UseNumber()
		if err := dec.Decode(&raws); err != nil {
			return nil, err
		}

		var term struct {
			Functor string
			Args    []json.RawMessage
		}
		dec = json.NewDecoder(bytes.NewReader(bs))
		dec.UseNumber()
		if err := dec.Decode(&term); err != nil {
			return nil, err
		}
		args := make([]Term, 0, len(term.Args))
		for _, raw := range term.Args {
			arg, err := unmarshalTerm(raw)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}
		return Compound{
			Functor: term.Functor,
			Args:    args,
		}, nil
		/*
			// dictionary
			m := make(map[string]Term)
			for k, raw := range raws {
				term, err := unmarshalTerm(raw)
				if err != nil {
					return nil, err
				}
				m[k] = term
			}
			return m, nil
		*/
	case bool:
		return x, nil
	case nil:
		return nil, nil
	}

	return nil, fmt.Errorf("trealla: unhandled term json: %T %v", iface, iface)
}

// Compound is a Prolog compound type.
type Compound struct {
	// Functor is the principal functor of the compound.
	// Example: the Functor of foo(bar) is "foo".
	Functor string
	// Args are the arguments of the compound.
	Args []Term
}

// Indicator returns the procedure indicator of this compound in Functor/Arity format.
func (c Compound) Indicator() string {
	// TODO: escape
	return fmt.Sprintf("%s/%d", c.Functor, len(c.Args))
}

// String returns a Prolog-ish representation of this Compound.
func (c Compound) String() string {
	if len(c.Args) == 0 {
		return c.Functor
	}

	var buf strings.Builder
	// TODO: escape
	buf.WriteString(c.Functor)
	buf.WriteRune('(')
	for i, arg := range c.Args {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(fmt.Sprintf("%v", arg))
	}
	buf.WriteRune(')')
	return buf.String()
}

// Variable is an unbound Prolog variable.
type Variable string
