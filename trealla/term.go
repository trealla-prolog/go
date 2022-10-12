package trealla

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
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
type Term any

// Substitution is a mapping of variable names to substitutions (terms).
// In other words, it's one answer to a query.
type Substitution map[string]Term

type binding struct {
	name  string
	value Term
}

func (sub Substitution) bindings() []binding {
	bs := make([]binding, 0, len(sub))
	for k, v := range sub {
		bs = append(bs, binding{
			name:  k,
			value: v,
		})
	}
	sort.Slice(bs, func(i, j int) bool {
		return bs[i].name < bs[j].name
	})
	return bs
}

// UnmarshalJSON implements the encoding/json.Marshaler interface.
func (sol *Substitution) UnmarshalJSON(bs []byte) error {
	var raws map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(bs))
	dec.UseNumber()
	if err := dec.Decode(&raws); err != nil {
		return err
	}
	*sol = make(Substitution, len(raws))
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

// Atom is a Prolog atom.
type Atom string

// String returns the Prolog text representation of this atom.
func (a Atom) String() string {
	return escapeAtom(a)
}

// Indicator returns a predicate indicator for this atom ("foo/0").
func (a Atom) Indicator() string {
	return fmt.Sprintf("%s/0", escapeAtom(a))
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
	return fmt.Sprintf("%s/%d", escapeAtom(c.Functor), len(c.Args))
}

// String returns a Prolog representation of this Compound.
func (c Compound) String() string {
	if len(c.Args) == 0 {
		return escapeAtom(c.Functor)
	}

	var buf strings.Builder
	buf.WriteString(escapeAtom(c.Functor))
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
		var sb strings.Builder
		sb.WriteRune('[')
		for i, t := range x {
			if i != 0 {
				sb.WriteString(", ")
			}
			text, err := marshal(t)
			if err != nil {
				return "", err
			}
			sb.WriteString(text)
		}
		sb.WriteRune(']')
		return sb.String(), nil
	}
	return "", fmt.Errorf("trealla: can't marshal type %T, value: %v", term, term)
}

func escapeString(str string) string {
	return `"` + stringEscaper.Replace(str) + `"`
}

func escapeAtom(atom Atom) string {
	if !atomNeedsEscape(atom) {
		return string(atom)
	}
	return "'" + atomEscaper.Replace(string(atom)) + "'"
}

func atomNeedsEscape(atom Atom) bool {
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
