package trealla

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode"
)

// Term is a Prolog term.
//
// One of the following types:
//   - string
//   - int64
//   - float64
//   - *big.Int
//   - *big.Rat
//   - Atom
//   - Compound
//   - Variable
//   - Slices of any supported type
type Term any

// Atom is a Prolog atom.
type Atom string

// String returns the Prolog text representation of this atom.
func (a Atom) String() string {
	if !a.needsEscape() {
		return string(a)
	}
	return "'" + atomEscaper.Replace(string(a)) + "'"
}

// Indicator returns a predicate indicator for this atom ("foo/0").
func (a Atom) Indicator() string {
	return a.pi().String()
}

func (a Atom) pi() Compound {
	return Atom("/").Of(a, int64(0))
}

// Of returns a Compound term with this atom as the principal functor.
func (a Atom) Of(args ...Term) Compound {
	return Compound{
		Functor: a,
		Args:    args,
	}
}

func (a *Atom) UnmarshalJSON(text []byte) error {
	if string(text) == "[]" {
		*a = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(text, &s); err != nil {
		return err
	}
	*a = Atom(s)
	return nil
}

func (a Atom) needsEscape() bool {
	if len(a) == 0 {
		return true
	}
	for i, char := range a {
		if i == 0 && !unicode.IsLower(char) {
			return true
		}
		if !(char == '_' || unicode.IsLetter(char) || unicode.IsDigit(char)) {
			return true
		}
	}
	return false
}

// Compound is a Prolog compound type.
type Compound struct {
	// Functor is the principal functor of the compound.
	// Example: the Functor of foo(bar) is "foo".
	Functor Atom
	// Args are the arguments of the compound.
	Args []Term
}

// Indicator returns the procedure indicator of this compound in Functor/Arity format.
func (c Compound) Indicator() string {
	return c.pi().String()
}

func (c Compound) pi() Compound {
	return piTerm(c.Functor, len(c.Args))
}

// String returns a Prolog representation of this Compound.
func (c Compound) String() string {
	if len(c.Args) == 0 {
		return c.Functor.String()
	}

	var buf strings.Builder

	// special case these two operators for now?
	if len(c.Args) == 2 {
		switch c.Functor {
		case "/", ":":
			left, err := marshal(c.Args[0])
			if err != nil {
				buf.WriteString(fmt.Sprintf("<invalid: %v>", err))
			}
			buf.WriteString(left)
			buf.WriteString(string(c.Functor))
			right, err := marshal(c.Args[1])
			if err != nil {
				buf.WriteString(fmt.Sprintf("<invalid: %v>", err))
			}
			buf.WriteString(right)
			return buf.String()
		}
	}

	buf.WriteString(c.Functor.String())
	buf.WriteRune('(')
	for i, arg := range c.Args {
		if i > 0 {
			buf.WriteString(", ")
		}
		text, err := marshal(arg)
		if err != nil {
			buf.WriteString(fmt.Sprintf("<invalid: %v>", err))
			continue
		}
		buf.WriteString(text)
	}
	buf.WriteRune(')')
	return buf.String()
}

func piTerm(functor Atom, arity int) Compound {
	return Compound{Functor: "/", Args: []Term{functor, int64(arity)}}
}

type atomicTerm interface {
	Term
	Indicator() string
	pi() Compound
}

type Rational[T int64 | *big.Int] struct {
	Numerator   T
	Denominator T
}

// Variable is an unbound Prolog variable.
type Variable struct {
	Name string
	Attr []Term
}

// Functor is a special type that represents the functor of a compound struct.
// For example, hello/1 as in `hello(world)` could be represented as:
//
//	type Hello struct {
//		trealla.Functor `prolog:"hello/1"`
//		Planet          trealla.Atom
//	}
type Functor Atom

func (f Functor) functor() Functor { return f }

type compoundStruct interface {
	functor() Functor
}

// String returns the Prolog text representation of this variable.
func (v Variable) String() string {
	if len(v.Attr) == 0 {
		return v.Name
	}
	var sb strings.Builder
	for i, attr := range v.Attr {
		if i != 0 {
			sb.WriteString(", ")
		}
		text, err := marshal(attr)
		if err != nil {
			return fmt.Sprintf("<invalid var: %v>", err)
		}
		sb.WriteString(text)
	}
	return sb.String()
}

func numbervars(n int) []Term {
	vars := make([]Term, n)
	for i := 0; i < n; i++ {
		if i < 26 {
			vars[i] = Variable{Name: string(rune('A' + i))}
		} else {
			vars[i] = Variable{Name: "_" + strconv.Itoa(i)}
		}
	}
	return vars
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

		type internalTerm struct {
			Functor     Atom
			Args        []json.RawMessage
			Var         string
			Attr        []json.RawMessage
			Number      string
			Numerator   json.RawMessage
			Denominator json.RawMessage
		}
		var term internalTerm
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

		switch {
		case len(term.Numerator) == 0 && len(term.Denominator) == 0:
		case len(term.Numerator) == 0 && len(term.Denominator) > 0:
			return nil, fmt.Errorf("trealla: failed to decode rational, missing numerator: %s", string(bs))
		case len(term.Numerator) > 0 && len(term.Denominator) == 0:
			return nil, fmt.Errorf("trealla: failed to decode rational, missing denominator: %s", string(bs))
		case len(term.Numerator) > 0 && len(term.Denominator) > 0:
			bigN := term.Numerator[0] == '{'
			bigD := term.Denominator[0] == '{'
			if !bigN && !bigD {
				n, err1 := strconv.ParseInt(string(term.Numerator), 10, 64)
				d, err2 := strconv.ParseInt(string(term.Denominator), 10, 64)
				return big.NewRat(n, d), errors.Join(err1, err2)
			}

			var tmp struct {
				Number string
			}
			var str json.RawMessage
			if bigN {
				if err := json.Unmarshal(term.Numerator, &tmp); err != nil {
					return nil, err
				}
				str = []byte(tmp.Number)
			} else {
				str = term.Numerator
			}
			str = append(str, '/')
			if bigD {
				if err := json.Unmarshal(term.Denominator, &tmp); err != nil {
					return nil, err
				}
				str = append(str, []byte(tmp.Number)...)
			} else {
				str = append(str, term.Denominator...)
			}

			rat, ok := new(big.Rat).SetString(string(str))
			if !ok {
				return nil, fmt.Errorf("trealla: failed to create rational for %s", string(str))
			}
			return rat, nil
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
			if len(attr) == 0 {
				attr = nil
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
