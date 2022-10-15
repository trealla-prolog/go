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

func scan(sub Substitution, rv reflect.Value) error {
	switch rv.Kind() {
	case reflect.Map:
		vtype := rv.Type().Elem()
		for k, v := range sub {
			vv := reflect.ValueOf(v)
			if !vv.CanConvert(vtype) {
				return fmt.Errorf("trealla: invalid element type for Scan: %v", vtype)
			}
			rv.SetMapIndex(reflect.ValueOf(k), vv.Convert(vtype))
		}
		return nil
	case reflect.Pointer:
		rv = rv.Elem()
		// we can't set the inner elements of *map and *interface directly,
		// so they need to be swapped out with a new inner value
		switch rv.Kind() {
		case reflect.Map:
			ev := reflect.MakeMap(rv.Type())
			if err := scan(sub, ev); err != nil {
				return err
			}
			rv.Set(ev)
			return nil
		case reflect.Interface:
			ev := reflect.New(rv.Elem().Type())
			if err := scan(sub, ev); err != nil {
				return err
			}
			rv.Set(ev.Elem())
			return nil
		case reflect.Struct:
			// happy path
		default:
			return fmt.Errorf("trealla: must pass pointer to struct or map for Scan. got: %v", rv.Type())
		}

		rtype := rv.Type()
		fieldnum := rtype.NumField()
		fields := make(map[string]reflect.Value, fieldnum)
		for i := 0; i < fieldnum; i++ {
			f := rtype.Field(i)
			name := f.Name
			if tag := f.Tag.Get("prolog"); tag != "" {
				name = tag
			}
			fields[name] = rv.Field(i)
		}

		for k, v := range sub {
			fv, ok := fields[k]
			if !ok {
				continue
			}
			ftype := fv.Type()
			vv := reflect.ValueOf(v)

			if fv.Kind() == reflect.Slice {
				length := vv.Len()
				etype := ftype.Elem()
				slice := reflect.MakeSlice(ftype, length, length)
				for i := 0; i < length; i++ {
					x := vv.Index(i)
					if x.Kind() == reflect.Interface && !x.IsNil() {
						x = x.Elem()
					}
					if !x.CanConvert(etype) {
						return fmt.Errorf("trealla: can't convert %s[%d] (value: %v, type: %v) to type: %v", k, i, x.Interface(), x.Type(), etype)
					}
					slice.Index(i).Set(x.Convert(etype))
				}
				fv.Set(slice)
				continue
			}

			// handle the empty string (rendered as [], the empty list)
			if fv.Kind() == reflect.String && vv.Kind() == reflect.Slice && vv.Len() == 0 {
				fv.SetString("")
				continue
			}

			if !vv.CanConvert(ftype) {
				return fmt.Errorf("trealla: can't convert %s (value: %v) to type: %v", k, v, ftype)
			}
			fv.Set(vv.Convert(ftype))
		}
		return nil
	}

	return fmt.Errorf("trealla: can't scan into type: %v; must be pointer to struct or map", rv.Type())
}
