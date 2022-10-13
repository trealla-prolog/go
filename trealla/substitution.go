package trealla

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

// Substitution is a mapping of variable names to substitutions (terms).
// For query results, it's one answer to a query.
// Substitution can also be used to bind variables in queries via WithBinding.
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

// Scan sets any fields in obj that match variables in this substitution.
// obj must be a pointer to a struct or a map.
func (sub Substitution) Scan(obj any) error {
	rv := reflect.ValueOf(obj)
	return scan(sub, rv)
}

func scan(sub Substitution, rv reflect.Value) error {
	if rv.Kind() == reflect.Map {
		vtype := rv.Type().Elem()
		for k, v := range sub {
			vv := reflect.ValueOf(v)
			if !vv.CanConvert(vtype) {
				return fmt.Errorf("trealla: invalid element type for Scan: %v", vtype)
			}
			rv.SetMapIndex(reflect.ValueOf(k), vv.Convert(vtype))
		}
		return nil
	}

	switch rv.Kind() {
	case reflect.Pointer:
		rv = rv.Elem()
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

			if !vv.CanConvert(ftype) {
				return fmt.Errorf("trealla: can't convert %s (value: %v) to type: %v", k, v, ftype)
			}
			fv.Set(vv.Convert(ftype))
		}
		return nil
	}

	return fmt.Errorf("trealla: can't scan into type: %v; must be pointer to struct or map", rv.Type())
}
