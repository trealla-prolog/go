:- module(wasm_go, [crypto_data_hash/3]).

:- use_module(library(error)).

crypto_data_hash(Data, Hash, Options) :-
	must_be(chars, Data),
	can_be(chars, Hash),
	must_be(list, Options),
	ignore(memberchk(algorithm(Algo), Options)),
	can_be(atom, Algo),
	( hash_algo(Algo) -> true
	; domain_error(algorithm, Algo, crypto_data_hash/3)
	),
	host_rpc(crypto_data_hash(Data, Hash, Algo)).

hash_algo(sha256).
hash_algo(sha512).
hash_algo(sha1).
