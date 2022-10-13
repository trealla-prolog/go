# trealla-go [![GoDoc](https://godoc.org/github.com/trealla-prolog/go?status.svg)](https://godoc.org/github.com/trealla-prolog/go)
`import "github.com/trealla-prolog/go/trealla"`

Prolog interface for Go using [Trealla Prolog](https://github.com/trealla-prolog/trealla) and [Wasmer](https://github.com/wasmerio/wasmer-go).
It's pretty fast. Not as fast as native Trealla, but pretty dang fast (2-5x slower than native).

**Development Status**: beta 🤠

### Caveats

- Beta status, API will probably change.
- Doesn't work on Windows ([wasmer-go issue](https://github.com/wasmerio/wasmer-go/issues/69)).
	- Works great on WSL.

## Install

```bash
go get github.com/trealla-prolog/go
```

**Note**: the module is under `github.com/trealla-prolog/go`, **not** `[...]/go/trealla`.
go.dev is confused about this and will pull a very old version if you try to `go get` the `trealla` package.

## Usage

This library uses WebAssembly to run Trealla, executing Prolog queries in an isolated environment.

```go

import "github.com/trealla-prolog/go/trealla"

func main() {
	// load the interpreter and (optionally) grant access to the current directory
	pl := trealla.New(trealla.WithPreopen("."))
	// run a query; cancel context to abort it
	ctx := context.Background()
	query := pl.Query(ctx, "member(X, [1, foo(bar), c]).")

	// calling Close is not necessary if you iterate through the whole result set
	// but it doesn't hurt either
	defer query.Close() 

	// iterate through answers
	for query.Next(ctx) {
		answer := query.Current()
		x := answer.Solution["X"]
		fmt.Println(x) // 1, trealla.Compound{Functor: "foo", Args: [trealla.Atom("bar")]}, "c"
	}

	// make sure to check the query for errors
	if err := query.Err(); err != nil {
		panic(err)
	}
}
```

### Single query

Use `QueryOnce` when you only want a single answer.

```go
pl := trealla.New()
answer, err := pl.QueryOnce(ctx, "succ(41, N).")
if err != nil {
	panic(err)
}

fmt.Println(answer.Stdout)
// Output: hello world
```

### Binding variables

You can bind variables in the query using the `WithBind` and `WithBinding` options.
This is a safe and convenient way to pass data into the query.
It is OK to pass these multiple times.

```go
pl := trealla.New()
answer, err := pl.QueryOnce(ctx, "write(X)", trealla.WithBind("X", trealla.Atom("hello world")))
if err != nil {
	panic(err)
}

fmt.Println(answer.Stdout)
// Output: hello world
```

### Scanning solutions

You can scan an answer's substitutions directly into a struct or map, similar to ichiban/prolog.

Use the `prolog:"VariableName"` struct tag to manually specify a variable name.
Otherwise, the field's name is used.

```prolog
answer, err := pl.QueryOnce(ctx, `X = 123, Y = abc, Z = ["hello", "world"].`)
if err != nil {
	panic(err)
}

var result struct {
	X  int
	Y  string
	Hi []string `prolog:"Z"`
}
// make sure to pass a pointer to the struct!
if err := answer.Solution.Scan(&result); err != nil {
	panic(err)
}

fmt.Printf("%+v", result)
// Output: {X:123 Y:abc Hi:[hello world]}
```

## Documentation

See **[package trealla's documentation](https://pkg.go.dev/github.com/trealla-prolog/go#section-directories)** for more details and examples.

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

- [trealla-js](https://github.com/guregu/trealla-js) is Trealla for Javascript.
- [ichiban/prolog](https://github.com/ichiban/prolog) is a pure Go Prolog.
- [guregu/pengine](https://github.com/guregu/pengine) is a Pengines (SWI-Prolog) library for Go.
