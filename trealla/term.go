package trealla

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode"
)

// Term is a Prolog term.
//
// One of the following types:
//	- string
//	- int64
//	- float64
//	- *big.Int
//  - Atom
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
		var raws map[string]json.RawMessage
		dec := json.NewDecoder(bytes.NewReader(bs))
		dec.UseNumber()
		if err := dec.Decode(&raws); err != nil {
			return nil, err
		}

		var term struct {
			Functor string
			Args    []json.RawMessage
			Var     string
			Attr    []json.RawMessage
			Number  string
		}
		dec = json.NewDecoder(bytes.NewReader(bs))
		dec.UseNumber()
		if err := dec.Decode(&term); err != nil {
			return nil, err
		}

		if term.Number != "" {
			n := new(big.Int)
			if _, ok := n.SetString(term.Number, 10); !ok {
				return nil, fmt.Errorf("trealla: failed to decode number: %s", term.Number)
			}
			return n, nil
		}

		if term.Var != "" {
			attr := make([]Term, 0, len(term.Attr))
			for _, raw := range term.Attr {
				at, err := unmarshalTerm(raw)
				if err != nil {
					return nil, err
				}
				attr = append(attr, at)
			}
			return Variable{Name: term.Var, Attr: attr}, nil
		}

		if len(term.Args) == 0 {
			return Atom(term.Functor), nil
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
	case bool:
		return x, nil
	case nil:
		return nil, nil
	}

	return nil, fmt.Errorf("trealla: unhandled term json: %T %v", iface, iface)
}

// Atom is a Prolog atom.
type Atom string

func (a Atom) String() string {
	return escapeAtom(string(a))
}

func (a Atom) Indicator() string {
	return fmt.Sprintf("%s/0", escapeAtom(string(a)))
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
	return fmt.Sprintf("%s/%d", escapeAtom(c.Functor), len(c.Args))
}

// String returns a Prolog representation of this Compound.
func (c Compound) String() string {
	if len(c.Args) == 0 {
		// TODO: this shouldn't happen anymore
		return escapeAtom(c.Functor)
	}

	var buf strings.Builder
	buf.WriteString(escapeAtom(c.Functor))
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
type Variable struct {
	Name string
	Attr []Term
}

func escapeString(str string) string {
	return `"` + stringEscaper.Replace(str) + `"`
}

func escapeAtom(atom string) string {
	if !atomNeedsEscape(atom) {
		return atom
	}
	return "'" + atomEscaper.Replace(atom) + "'"
}

func atomNeedsEscape(atom string) bool {
	if len(atom) == 0 {
		return true
	}
	for i, r := range atom {
		if i == 0 && !unicode.IsLower(r) {
			return true
		}
		if !unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

var stringEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`)
var atomEscaper = strings.NewReplacer(`\`, `\\`, `'`, `\'`)
