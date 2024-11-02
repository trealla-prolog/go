package trealla

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
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

var (
	compoundType = reflect.TypeFor[Compound]()
	functorType  = reflect.TypeFor[Functor]()
	termType     = reflect.TypeFor[Term]()
	atomType     = reflect.TypeFor[Atom]()
)

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
		info := make(map[string]reflect.StructField, fieldnum)
		for i := 0; i < fieldnum; i++ {
			f := rtype.Field(i)
			name := f.Name
			if tag := f.Tag.Get("prolog"); tag != "" {
				name = tag
			}
			fields[name] = rv.Field(i)
			info[name] = f
		}

		for k, v := range sub {
			fv, ok := fields[k]
			if !ok {
				continue
			}
			if err := convert(fv, reflect.ValueOf(v), info[k]); err != nil {
				return fmt.Errorf("trealla: error converting field %q in %v: %w", k, rv.Type(), err)
			}
		}
		return nil
	}

	return fmt.Errorf("trealla: can't scan into type: %v; must be pointer to struct or map", rv.Type())
}

func convert(dstv, srcv reflect.Value, meta reflect.StructField) error {
	if !srcv.IsValid() || !srcv.CanInterface() {
		return fmt.Errorf("invalid src: %v", srcv)
	}

	ftype := dstv.Type()

	if ftype == termType {
		dstv.Set(srcv)
		return nil
	}

	if srcv.Kind() == reflect.Interface && !srcv.IsNil() {
		srcv = srcv.Elem()
	}

	if dstv.Kind() == reflect.Slice {
		length := srcv.Len()
		srctype := srcv.Type()
		etype := ftype.Elem()
		detype := dstv.Type().Elem()
		var preconvert bool
		switch {
		case srctype == atomType && srcv.Interface().(Atom) == "[]":
			// special case: empty list
			length = 0
		case srctype.Kind() == reflect.String:
			// special case: convert string → list
			runes := []rune(srcv.String())
			srcv = reflect.ValueOf(runes)
			length = len(runes)
			// if []Atom or []Term
			if dstv.Type().Elem().ConvertibleTo(atomType) || termType.AssignableTo(detype) {
				preconvert = true
				etype = atomType
			}
		}
		slice := reflect.MakeSlice(ftype, length, length)
		for i := 0; i < length; i++ {
			x := srcv.Index(i)
			if preconvert {
				x = x.Convert(etype)
			}
			if err := convert(slice.Index(i), x, meta); err != nil {
				return fmt.Errorf("can't convert %v[%d]; error: %w", srctype, i, err)
			}
		}
		dstv.Set(slice)
		return nil
	}

	// handle the empty string (rendered as [], the empty list)
	if dstv.Kind() == reflect.String && srcv.Kind() == reflect.Slice && srcv.Len() == 0 {
		dstv.SetString("")
		return nil
	}

	// compound → struct
	if srcv.Type() == compoundType && dstv.Kind() == reflect.Struct {
		return decodeCompoundStruct(dstv, srcv.Interface().(Compound), meta)
	}

	if !srcv.CanConvert(ftype) {
		return fmt.Errorf("can't convert from type %v to type: %v", srcv.Type(), ftype)
	}
	dstv.Set(srcv.Convert(ftype))
	return nil
}

// TODO: break out reflect stuff into something like this:
// type structInfo struct {
// 	fields   []reflect.Value
// 	meta     []reflect.StructField
// 	functor  string
// 	arity    int
// }

func decodeCompoundStruct(dstv reflect.Value, src Compound, meta reflect.StructField) error {
	rtype := dstv.Type()
	fieldnum := rtype.NumField()
	fields := make([]reflect.Value, 0, fieldnum)
	fieldInfo := make([]reflect.StructField, 0, fieldnum)

	var functor reflect.Value

	var collect func(dstv reflect.Value) error
	collect = func(dstv reflect.Value) error {
		for i := 0; i < fieldnum; i++ {
			field := rtype.Field(i)
			fv := dstv.Field(i)
			tag := field.Tag.Get("prolog")
			if tag == "-" {
				continue
			}
			exported := field.IsExported()
			if field.Type == functorType && exported {
				functor = fv
				continue
			}
			if field.Anonymous && field.Type.Kind() == reflect.Struct {
				if err := collect(fv); err != nil {
					return err
				}
				continue
			}
			if !exported {
				continue
			}
			fields = append(fields, fv)
			fieldInfo = append(fieldInfo, field)
		}
		return nil
	}
	if err := collect(dstv); err != nil {
		return err
	}

	if functor.IsValid() && functor.CanSet() {
		// TODO: check tag?
		functor.Set(reflect.ValueOf(Functor(src.Functor)))
	}

	for i := 0; i < min(len(fields), len(src.Args)); i++ {
		info := fieldInfo[i]
		if err := convert(fields[i], reflect.ValueOf(src.Args[i]), info); err != nil {
			return fmt.Errorf("can't convert compound (%v) argument #%d (type %T, value: %v) into field %q: %w",
				src.pi().String(), i, src.Args[i], src.Args[i], info.Name, err)
		}
	}
	return nil
}

func encodeCompoundStruct(src any) (Compound, error) {
	marker, ok := src.(compoundStruct)
	if !ok {
		return Compound{}, fmt.Errorf("can't encode %T to compound; no Functor field found", src)
	}
	srcv := reflect.ValueOf(src)
	for srcv.Kind() == reflect.Pointer && !srcv.IsNil() {
		srcv = srcv.Elem()
	}
	if srcv.Kind() != reflect.Struct {
		return Compound{}, fmt.Errorf("not a struct: %T", src)
	}

	rtype := srcv.Type()
	fieldnum := rtype.NumField()
	fields := make([]reflect.Value, 0, fieldnum)
	fieldInfo := make([]reflect.StructField, 0, fieldnum)
	functor := marker.functor()

	// var functor reflect.Value
	// var expectFunctor Functor
	// expectArity := -1
	var expect string
	var arity int
	var collect func(dstv reflect.Value) error
	collect = func(dstv reflect.Value) error {
		for i := 0; i < fieldnum; i++ {
			field := rtype.Field(i)
			fv := dstv.Field(i)
			tag := field.Tag.Get("prolog")
			if tag == "-" {
				continue
			}
			exported := field.IsExported()
			if field.Type == functorType && exported {
				expect, arity = structTag(tag)
				continue
			}
			if field.Anonymous && field.Type.Kind() == reflect.Struct {
				if err := collect(fv); err != nil {
					return err
				}
				continue
			}
			if !exported {
				continue
			}
			fields = append(fields, fv)
			fieldInfo = append(fieldInfo, field)
		}
		return nil
	}
	if err := collect(srcv); err != nil {
		return Compound{}, err
	}

	c := Compound{Functor: Atom(functor), Args: make([]Term, 0, len(fields))}
	for i := 0; i < len(fields); i++ {
		// info := fieldInfo[i]
		iface := fields[i].Interface()
		// tt, err := marshal(iface.(Term))
		// if err != nil {
		// 	return c, fmt.Errorf("can't encode compound (%v) argument #%d (type %T, value: %v) from field %q: %w",
		// 		functor, i, iface, iface, info.Name, err)
		// }
		c.Args = append(c.Args, iface)
	}
	if c.Functor == "" && expect != "" {
		c.Functor = Atom(expect)
	}
	if arity > 0 && len(c.Args) != arity {
		names := make([]string, len(fields))
		for i := 0; i < len(fields); i++ {
			names[i] = fieldInfo[i].Name
		}
		return c, fmt.Errorf("# of fields in %T does not match arity of struct tag (%s/%d): have %d fields %v but expected %d",
			src, expect, arity, len(c.Args), names, arity)
	}

	return c, nil
}

func structTag(tag string) (name string, arity int) {
	if tag == "" {
		return
	}
	name, _, _ = strings.Cut(tag, ",")
	slash := strings.LastIndexByte(name, '/')
	if slash > 0 && slash < len(name)-1 {
		arity, _ = strconv.Atoi(name[slash+1:])
		name = name[:slash]
	}
	return
}
