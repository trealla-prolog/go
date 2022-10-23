package trealla

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	_ "embed"
	"encoding/hex"
)

//go:embed library.pl
var libText []byte

func (pl *prolog) loadBuiltins() error {
	pl.procs["crypto_data_hash/3"] = cryptoDataHash3
	return pl.consultText("user", string(libText))
}

func cryptoDataHash3(_ *prolog, _ int32, goal Term) Term {
	pi := Atom("/").Of(Atom("crypto_data_hash"), int64(3))
	cmp, ok := goal.(Compound)
	if !ok {
		return typeError("compound", goal, pi)
	}
	if len(cmp.Args) != 3 {
		return domainError("host_call_goal", cmp, pi)
	}
	data := cmp.Args[0]
	// hash := cmp.Args[1]
	algo := cmp.Args[2]
	str, ok := data.(string)
	if !ok {
		return typeError("chars", data, pi)
	}
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
	}
	return Atom("crypto_data_hash").Of(data, hex.EncodeToString(digest), algo)
}

func typeError(want Atom, got Term, ctx Term) Compound {
	return throwTerm(Atom("error").Of(Atom("type_error").Of(want, got), ctx))
}

func domainError(domain Atom, got Term, ctx Term) Compound {
	return throwTerm(Atom("error").Of(Atom("domain_error").Of(domain, got), ctx))
}

func throwTerm(ball Term) Compound {
	return Compound{Functor: "throw", Args: []Term{ball}}
}
