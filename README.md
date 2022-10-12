# trealla-go [![GoDoc](https://godoc.org/github.com/trealla-prolog/go?status.svg)](https://godoc.org/github.com/trealla-prolog/go)
`import "github.com/trealla-prolog/go/trealla"`

Prolog interface for Go using [Trealla Prolog](https://github.com/trealla-prolog/trealla) and [Wasmer](https://github.com/wasmerio/wasmer-go).
It's pretty fast. Not as fast as native Trealla, but pretty dang fast (2-5x slower than native).

**Development Status**: beta 🤠

### Caveats

- Beta status, API will probably change.
- Doesn't work on Windows ([wasmer-go issue](https://github.com/wasmerio/wasmer-go/issues/69)).
	- Works great on WSL.

## Usage

This library uses WebAssembly to run Trealla, executing Prolog queries in an isolated environment.

```go

import "github.com/trealla-prolog/go/trealla"

func main() {
	// load the interpreter and (optionally) grant access to the current directory
	pl := trealla.New(trealla.WithPreopen("."))
	// run a query; cancel context to abort it
	query := pl.Query(ctx, "member(X, [1, foo(bar), c]).")
	// iterate through answers
	for query.Next(ctx) {
		answer := query.Current()
		x := answer.Solution["X"]
		fmt.Println(x) // 1, trealla.Compound{Functor: "foo", Args: [trealla.Atom("bar")]}, "c"}
	}
	// make sure to check the query for errors
	if err := query.Err(); err != nil {
		panic(err)
	}
}
```

## WASM Binary

This library embeds the Trealla WebAssembly binary in itself, so you can use it without any external dependencies.
The binaries are currently sourced from [guregu/trealla](https://github.com/guregu/trealla), also [available from WAPM](https://wapm.io/guregu/trealla).

## Thanks
 
- Andrew Davison ([@infradig](https://github.com/infradig)) and other contributors to [Trealla Prolog](https://github.com/trealla-prolog/trealla).
- Jos De Roo ([@josd](https://github.com/josd)) for test cases and encouragement.
- Aram Panasenco ([@panasenco](https://github.com/panasenco)) for his JSON library.

## License

MIT. See ATTRIBUTION as well.

## See also

- [trealla-js](https://wapm.io/guregu/trealla-js) is Trealla for Javascript.
- [ichiban/prolog](https://github.com/ichiban/prolog) is a pure Go Prolog.
- [guregu/pengine](https://github.com/guregu/pengine) is a Pengines (SWI-Prolog) library for Go.
