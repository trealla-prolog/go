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
//   - string
//   - int64
//   - float64
//   - *big.Int
//   - Atom
//   - Compound
//   - Variable
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

type atomic interface {
	Term
	Indicator() string
	pi() Compound
}

// Variable is an unbound Prolog variable.
type Variable struct {
	Name string
	Attr []Term
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

		var term struct {
			Functor Atom
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

// Marshal returns the Prolog text representation of term.
func Marshal(term Term) (string, error) {
	return marshal(term)
}

func marshal(term Term) (string, error) {
	switch x := term.(type) {
	case string:
		return escapeString(x), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case int:
		return strconv.FormatInt(int64(x), 10), nil
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), nil
	case *big.Int:
		return x.String(), nil
	case Atom:
		return x.String(), nil
	case Compound:
		return x.String(), nil
	case Variable:
		return x.String(), nil
	case []Term:
		return marshalSlice(x)
	case []any:
		return marshalSlice(x)
	case []string:
		return marshalSlice(x)
	case []int64:
		return marshalSlice(x)
	case []int:
		return marshalSlice(x)
	case []float64:
		return marshalSlice(x)
	case []*big.Int:
		return marshalSlice(x)
	case []Atom:
		return marshalSlice(x)
	case []Compound:
		return marshalSlice(x)
	case []Variable:
		return marshalSlice(x)
	}
	return "", fmt.Errorf("trealla: can't marshal type %T, value: %v", term, term)
}

func marshalSlice[T any](slice []T) (string, error) {
	var sb strings.Builder
	sb.WriteRune('[')
	for i, v := range slice {
		if i != 0 {
			sb.WriteString(", ")
		}
		text, err := marshal(v)
		if err != nil {
			return "", err
		}
		sb.WriteString(text)
	}
	sb.WriteRune(']')
	return sb.String(), nil
}

func escapeString(str string) string {
	return `"` + stringEscaper.Replace(str) + `"`
}

var stringEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`)
var atomEscaper = strings.NewReplacer(`\`, `\\`, `'`, `\'`)
