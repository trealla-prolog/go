package trealla

import (
	"context"
	"errors"
	"log"
	"reflect"
	"testing"
)

func TestInterop(t *testing.T) {
	pl, err := New(WithDebugLog(log.Default()))
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
					Query:    `crypto_data_hash("foo", X, [algorithm(A)]).`,
					Solution: Substitution{"A": Atom("sha256"), "X": "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"},
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
