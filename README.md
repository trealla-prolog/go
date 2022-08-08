# trealla-go [![GoDoc](https://godoc.org/github.com/trealla-prolog/go/trealla?status.svg)](https://godoc.org/github.com/trealla-prolog/go/trealla)
`import "github.com/trealla-prolog/go/trealla"`

Prolog interface for Go using [Trealla Prolog](https://github.com/trealla-prolog/trealla) and [Wasmer](https://github.com/wasmerio/wasmer-go).
It's pretty fast. Not as fast as native Trealla, but pretty dang fast (2-5x slower than native).

**Development Status**: alpha 🤠

### Caveats

- Alpha status, API will change.
- Queries are findall'd and won't return answers until they terminate.
- Doesn't work on Windows ([wasmer-go issue](https://github.com/wasmerio/wasmer-go/issues/69)).
	- Works great on WSL.
- Currently interpreters are ephemeral, so you have to reconsult everything each query (working on this).

## Usage

This library uses WebAssembly to run Trealla, executing Prolog queries in an isolated environment.

```go

import "github.com/trealla-prolog/go/trealla"

func main() {
	// load the interpreter and (optionally) grant access to the current directory
	pl := trealla.New(trealla.WithPreopen("."))
	// run a query; cancel context to abort it
	answer, err := pl.Query(ctx, "member(X, [1, foo(bar), c]).")
	// get the second substitution (answer) for X
	x := answer.Solutions[1]["X"] // trealla.Compound{Functor: "foo", Args: ["bar"]}
}
```

## Thanks
 
- Andrew Davison ([@infradig](https://github.com/infradig)) and other contributors to [Trealla Prolog](https://github.com/trealla-prolog/trealla).
- Jos De Roo ([@josd](https://github.com/josd)) for test cases and encouragement.
- Aram Panasenco ([@panasenco](https://github.com/panasenco)) for his JSON library.

## License

MIT. See ATTRIBUTION as well.

## See also

- [ichiban/prolog](https://github.com/ichiban/prolog) is a pure Go Prolog.
- [guregu/pengine](https://github.com/guregu/pengine) is a Pengines (SWI-Prolog) library for Go.
