package trealla

import (
	"context"
	"errors"
	"log"
	"reflect"
	"testing"
)

func TestInterop(t *testing.T) {
	pl, err := New(WithDebugLog(log.Default()) /*WithStderrLog(log.Default()), WithStdoutLog(log.Default()), WithTrace() */)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		want []Answer
		err  error
	}{
		{
			name: "crypto_data_hash/3",
			want: []Answer{
				{
					Query:    `crypto_data_hash("foo", X, [algorithm(A)])`,
					Solution: Substitution{"A": Atom("sha256"), "X": "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"},
				},
			},
		},
		{
			name: "http_consult/1",
			want: []Answer{
				{
					Query:    `http_consult("https://raw.githubusercontent.com/guregu/worker-prolog/978c956801ffff83f190450e5c0325a9d34b064a/src/views/examples/fizzbuzz.pl"), !, fizzbuzz(1, 21), !`,
					Solution: Substitution{},
					Stdout:   "1\n2\nfizz\n4\nbuzz\nfizz\n7\n8\nfizz\nbuzz\n11\nfizz\n13\n14\nfizzbuzz\n16\n17\nfizz\n19\nbuzz\nfizz\n",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			q := pl.Query(ctx, tc.want[0].Query)
			var ans []Answer
			for q.Next(ctx) {
				ans = append(ans, q.Current())
			}
			err := q.Err()
			if tc.err == nil && err != nil {
				t.Fatal(err)
			} else if tc.err != nil && !errors.Is(err, tc.err) {
				t.Errorf("unexpected error: %#v (%v) ", err, err)
			}
			if tc.err == nil && !reflect.DeepEqual(ans, tc.want) {
				t.Errorf("bad answer. \nwant: %#v\ngot: %#v\n", tc.want, ans)
			}
		})
	}

}
