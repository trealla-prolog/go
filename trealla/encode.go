package trealla

import (
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"
)

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
	case uint64:
		return strconv.FormatUint(x, 10), nil
	case uint:
		return strconv.FormatUint(uint64(x), 10), nil
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), nil
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32), nil
	case *big.Int:
		return x.String(), nil
	case Atom:
		return x.String(), nil
	case Compound:
		return x.String(), nil
	case Variable:
		return x.String(), nil
	case compoundStruct:
		c, err := encodeCompoundStruct(term)
		if err != nil {
			return "", fmt.Errorf("trealla: error marshaling term %#v: %w", term, err)
		}
		return c.String(), nil
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
	default:
		rv := reflect.ValueOf(term)
		if !rv.IsValid() {
			break
		}
		for rv.Kind() == reflect.Pointer && !rv.IsNil() {
			rv = rv.Elem()
		}
		if !rv.CanInterface() {
			break
		}

		switch rv.Kind() {
		case reflect.Slice, reflect.Array:
			var sb strings.Builder
			sb.WriteByte('[')
			length := rv.Len()
			for i := 0; i < length; i++ {
				if i != 0 {
					sb.WriteByte(',')
				}
				elem, err := marshal(rv.Index(i).Interface())
				if err != nil {
					return "", err
				}
				sb.WriteString(elem)
			}
			sb.WriteByte(']')
			return sb.String(), nil

		case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
			return strconv.FormatInt(rv.Int(), 10), nil
		case reflect.Uint, reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8:
			return strconv.FormatUint(rv.Uint(), 10), nil
		case reflect.Float64:
			return strconv.FormatFloat(rv.Float(), 'f', -1, 64), nil
		case reflect.Float32:
			return strconv.FormatFloat(rv.Float(), 'f', -1, 32), nil
		case reflect.String:
			return escapeString(rv.String()), nil
		}
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

var stringEscaper = strings.NewReplacer(
	`\`, `\\`,
	`"`, `\"`,
	"\n", `\n`,
	"\t", `\t`,
)

var atomEscaper = strings.NewReplacer(
	`\`, `\\`,
	`'`, `\'`,
)
