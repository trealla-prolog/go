package trealla

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var builtins = []struct {
	name  string
	arity int
	proc  Predicate
}{
	{"$coro_next", 2, sys_coro_next_2},
	{"$coro_stop", 1, sys_coro_stop_1},
	{"crypto_data_hash", 3, crypto_data_hash_3},
	{"http_consult", 1, http_consult_1},
	{"http_fetch", 3, http_fetch_3},
}

func (pl *prolog) loadBuiltins() error {
	ctx := context.Background()
	for _, predicate := range builtins {
		if err := pl.register(ctx, predicate.name, predicate.arity, predicate.proc); err != nil {
			return err
		}
	}
	return nil
}

// TODO: needs to support forms, headers, etc.
func http_fetch_3(_ Prolog, _ Subquery, goal Term) Term {
	cmp, _ := goal.(Compound)
	result := cmp.Args[1]
	opts := cmp.Args[2]

	str, ok := cmp.Args[0].(string)
	if !ok {
		return typeError("chars", cmp.Args[0], piTerm("http_fetch", 3))
	}
	href, err := url.Parse(str)
	if err != nil {
		return domainError("url", cmp.Args[0], piTerm("http_fetch", 3))
	}

	method := findOption[Atom](opts, "method", "get")
	as := findOption[Atom](opts, "as", "string")
	bodystr := findOption(opts, "body", "")
	var body io.Reader
	if bodystr != "" {
		body = strings.NewReader(bodystr)
	}

	req, err := http.NewRequest(strings.ToUpper(string(method)), href.String(), body)
	if err != nil {
		return domainError("url", cmp.Args[0], err.Error())
	}
	// req.Header.Add("Accept", "application/x-prolog")
	req.Header.Set("User-Agent", "trealla-prolog/go")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return systemError(err.Error())
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK: // ok
	case http.StatusNoContent:
		return goal
	case http.StatusNotFound, http.StatusGone:
		return existenceError("source_sink", str, piTerm("http_fetch", 3))
	case http.StatusForbidden, http.StatusUnauthorized:
		return permissionError("open,source_sink", str, piTerm("http_fetch", 3))
	default:
		return systemError(fmt.Errorf("http_consult/1: unexpected status code: %d", resp.StatusCode))
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp.Body); err != nil {
		return resourceError(Atom(err.Error()), piTerm("http_fetch", 3))
	}

	switch as {
	case "json":
		js := Variable{Name: "_JS"}
		return Atom("call").Of(Atom(",").Of(Atom("=").Of(result, js), Atom("json_chars").Of(js, buf.String())))
	}

	return Atom(cmp.Functor).Of(str, buf.String(), Variable{Name: "_"})
}

func http_consult_1(_ Prolog, _ Subquery, goal Term) Term {
	cmp, ok := goal.(Compound)
	if !ok {
		return typeError("compound", goal, piTerm("http_consult", 1))
	}
	if len(cmp.Args) != 1 {
		return systemError(piTerm("http_consult", 1))
	}
	module := Atom("user")
	var addr string
	switch x := cmp.Args[0].(type) {
	case string:
		addr = x
	case Compound:
		// http_consult(module_name:"http://...")
		if x.Functor != ":" || len(x.Args) != 2 {
			return typeError("chars", cmp.Args[0], piTerm("http_consult", 1))
		}
		var ok bool
		module, ok = x.Args[0].(Atom)
		if !ok {
			return typeError("atom", x.Args[0], piTerm("http_consult", 1))
		}
		addr, ok = x.Args[1].(string)
		if !ok {
			return typeError("chars", x.Args[1], piTerm("http_consult", 1))
		}
	}
	href, err := url.Parse(addr)
	if err != nil {
		return domainError("url", cmp.Args[0], piTerm("http_consult", 1))
	}

	// TODO: grab context somehow
	req, err := http.NewRequest(http.MethodGet, href.String(), nil)
	if err != nil {
		return domainError("url", cmp.Args[0], err.Error())
	}
	req.Header.Add("Accept", "application/x-prolog")
	req.Header.Set("User-Agent", "trealla-prolog/go")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return systemError(err.Error())
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK: // ok
	case http.StatusNoContent:
		return goal
	case http.StatusNotFound, http.StatusGone:
		return existenceError("source_sink", addr, piTerm("http_consult", 1))
	case http.StatusForbidden, http.StatusUnauthorized:
		return permissionError("open,source_sink", addr, piTerm("http_consult", 1))
	default:
		return systemError(fmt.Errorf("http_consult/1: unexpected status code: %d", resp.StatusCode))
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp.Body); err != nil {
		return resourceError(Atom(err.Error()), piTerm("http_consult", 1))
	}

	// call(load_text(Text, module(URL))).
	return Atom("call").Of(Atom("load_text").Of(buf.String(), []Term{Atom("module").Of(module)}))
}

func crypto_data_hash_3(pl Prolog, _ Subquery, goal Term) Term {
	cmp, ok := goal.(Compound)
	if !ok {
		return typeError("compound", goal, piTerm("crypto_data_hash", 3))
	}
	if len(cmp.Args) != 3 {
		return systemError(piTerm("crypto_data_hash", 3))
	}
	data := cmp.Args[0]
	hash := cmp.Args[1]
	opts := cmp.Args[2]
	str, ok := data.(string)
	if !ok {
		return typeError("chars", data, piTerm("crypto_data_hash", 3))
	}
	switch hash.(type) {
	case Variable, string: // ok
	default:
		return typeError("chars", hash, piTerm("crypto_data_hash", 3))
	}
	if !isList(opts) {
		return typeError("list", opts, piTerm("crypto_data_hash", 3))
	}
	algo := findOption[Atom](opts, "algorithm", "sha256")
	var digest []byte
	switch algo {
	case Atom("sha256"):
		sum := sha256.Sum256([]byte(str))
		digest = sum[:]
	case Atom("sha512"):
		sum := sha512.Sum512([]byte(str))
		digest = sum[:]
	case Atom("sha1"):
		sum := sha1.Sum([]byte(str))
		digest = sum[:]
	default:
		return domainError("algorithm", algo, piTerm("crypto_data_hash", 3))
	}
	return Atom("crypto_data_hash").Of(data, hex.EncodeToString(digest), opts)
}

func typeError(want Atom, got Term, ctx Term) Compound {
	return throwTerm(Atom("error").Of(Atom("type_error").Of(want, got), ctx))
}

func domainError(domain Atom, got Term, ctx Term) Compound {
	return throwTerm(Atom("error").Of(Atom("domain_error").Of(domain, got), ctx))
}

func existenceError(what Atom, got Term, ctx Term) Compound {
	return throwTerm(Atom("error").Of(Atom("existence_error").Of(what, got), ctx))
}

func permissionError(what Atom, got Term, ctx Term) Compound {
	return throwTerm(Atom("error").Of(Atom("permission_error").Of(what, got), ctx))
}

func resourceError(what Atom, ctx Term) Compound {
	return throwTerm(Atom("error").Of(Atom("resource_error").Of(what), ctx))
}

func systemError(ctx Term) Compound {
	return throwTerm(Atom("error").Of(Atom("system_error"), ctx))
}

func throwTerm(ball Term) Compound {
	return Compound{Functor: "throw", Args: []Term{ball}}
}

func findOption[T Term](opts Term, functor Atom, fallback T) T {
	if empty, ok := opts.(Atom); ok && empty == "[]" {
		return fallback
	}
	list, ok := opts.([]Term)
	if !ok {
		var empty T
		return empty
	}
	for i, x := range list {
		switch x := x.(type) {
		case Compound:
			if x.Functor != functor || len(x.Args) != 1 {
				continue
			}
			switch arg := x.Args[0].(type) {
			case T:
				return arg
			case Variable:
				list[i] = functor.Of(fallback)
				return fallback
			}
		}
	}
	return fallback
}

func isList(x Term) bool {
	switch x := x.(type) {
	case []Term:
		return true
	case Atom:
		return x == "[]"
	}
	return false
}
