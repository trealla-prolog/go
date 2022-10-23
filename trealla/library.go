package trealla

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func (pl *prolog) loadBuiltins() error {
	if err := pl.register(context.Background(), "crypto_data_hash", 3, crypto_data_hash_3); err != nil {
		return err
	}
	if err := pl.register(context.Background(), "http_consult", 1, http_consult_1); err != nil {
		return err
	}
	return nil
}

func http_consult_1(pl Prolog, _ int32, goal Term) Term {
	cmp, ok := goal.(Compound)
	if !ok {
		return typeError("compound", goal, piTerm("http_consult", 1))
	}
	if len(cmp.Args) != 1 {
		return systemError(piTerm("http_consult", 1))
	}
	str, ok := cmp.Args[0].(string)
	if !ok {
		return typeError("chars", cmp.Args[0], piTerm("http_consult", 1))
	}
	href, err := url.Parse(str)
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
		return existenceError("source_sink", str, piTerm("http_consult", 1))
	case http.StatusForbidden, http.StatusUnauthorized:
		return permissionError("open,source_sink", str, piTerm("http_consult", 1))
	default:
		return systemError(fmt.Errorf("http_consult/1: unexpected status code: %d", resp.StatusCode))
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp.Body); err != nil {
		return resourceError(Atom(err.Error()), piTerm("http_consult/1", 1))
	}

	if err := pl.ConsultText(context.Background(), href.String(), buf.String()); err != nil {
		return systemError(err.Error())
	}
	return Atom("true")

	// call(URL:'$load_chars'(Text)).
	// return Atom("call").Of(Atom(":").Of(Atom(href.String()), Atom("$load_chars").Of(buf.String())))
}

func crypto_data_hash_3(pl Prolog, _ int32, goal Term) Term {
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
	case Variable: // ok
	case string: // ok
	default:
		return typeError("chars", hash, piTerm("crypto_data_hash", 3))
	}
	if !isList(opts) {
		return typeError("list", opts, piTerm("crypto_data_hash", 3))
	}
	algo := findOptionAtom(opts, "algorithm", "sha256")
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

func findOptionAtom(opts Term, functor, fallback Atom) Atom {
	if empty, ok := opts.(Atom); ok && empty == "[]" {
		return fallback
	}
	list, ok := opts.([]Term)
	if !ok {
		return "$error"
	}
	for i, x := range list {
		switch x := x.(type) {
		case Compound:
			if x.Functor != functor || len(x.Args) != 1 {
				continue
			}
			switch arg := x.Args[0].(type) {
			case Atom:
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