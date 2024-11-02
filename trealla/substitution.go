package trealla

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Substitution is a mapping of variable names to substitutions (terms).
// For query results, it's one answer to a query.
// Substitution can also be used to bind variables in queries via WithBinding.
type Substitution map[string]Term

// String returns a Prolog representation of this substitution in the same format
// as ISO variable_names/1 option for read_term/2.
func (sub Substitution) String() string {
	return "[" + sub.bindings().String() + "]"
}

// UnmarshalJSON implements the encoding/json.Marshaler interface.
func (sub *Substitution) UnmarshalJSON(bs []byte) error {
	var raws map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(bs))
	dec.UseNumber()
	if err := dec.Decode(&raws); err != nil {
		return err
	}
	*sub = make(Substitution, len(raws))
	for k, raw := range raws {
		term, err := unmarshalTerm(raw)
		if err != nil {
			return err
		}
		(*sub)[k] = term
	}
	return nil
}

type binding struct {
	name  string
	value Term
}

// Scan sets any fields in obj that match variables in this substitution.
// obj must be a pointer to a struct or a map.
func (sub Substitution) Scan(obj any) error {
	rv := reflect.ValueOf(obj)
	return scan(sub, rv)
}

type bindings []binding

func (bs bindings) String() string {
	var sb strings.Builder
	for i, bind := range bs {
		if i != 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(bind.name)
		sb.WriteString(" = ")
		v, err := marshal(bind.value)
		if err != nil {
			sb.WriteString(fmt.Sprintf("<error: %v>", err))
		}
		sb.WriteString(v)
	}
	return sb.String()
}

func (bs bindings) Less(i, j int) bool { return bs[i].name < bs[j].name }
func (bs bindings) Swap(i, j int)      { bs[i], bs[j] = bs[j], bs[i] }
func (bs bindings) Len() int           { return len(bs) }

func (sub Substitution) bindings() bindings {
	bs := make(bindings, 0, len(sub))
	for k, v := range sub {
		bs = append(bs, binding{
			name:  k,
			value: v,
		})
	}
	sort.Sort(bs)
	return bs
}
