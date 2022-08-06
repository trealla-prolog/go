:- module(wasm_toplevel, [wasm_ask/1]).

:- use_module(library(json)).
:- use_module(library(lists)).
:- use_module(library(dcgs)).

wasm_ask(Input) :-
	read_term_from_chars(Input, Query, [variable_names(Vars)]),
	query(Query, Vars, Event, Solutions, _),
	% output_string(Output0, Output),
	write_result(Event, Solutions, _),
	flush_output,
	halt.

write_result(success, Solutions0, _) :-
	maplist(solution_json, Solutions0, Solutions),
	once(phrase(json_chars(pairs([
		string("result")-string("success"),
		string("answers")-list(Solutions)
		% string("output")-string(Output)
	])), JSON)),
	format("~s~n", [JSON]).

write_result(failure, _, _) :-
	once(phrase(json_chars(pairs([
		string("result")-string("failure")
		% string("output")-string(Output)
	])), JSON)),
	format("~s~n", [JSON]).

write_result(error, Error0, _) :-
	term_json(Error0, Error),
	once(phrase(json_chars(pairs([
		string("result")-string("error"),
		string("error")-Error
		% string("output")-string(Output)
	])), JSON)),
	format("~s~n", [JSON]).

query(Query, Vars, Event, Solutions, Output) :-
	% StdoutFile = tmp,
	( setup_call_cleanup(
		(	
			%open(StdoutFile, write, Stream),
			%set_output(Stream)
			%format("+++BEGIN STDOUT+++~n", [])
			true
		),
		catch(bagof(Vars, call(Query), Solutions), Error, true),
		(
			%close(StdoutFile),
			%set_output(stdout)
			write('\x3\') % END OF TEXT
		)
	) -> OK = true
	  ;  OK = false
	),  
	query_event(OK, Error, Event),
	% read_file_to_string(StdoutFile, Output0, []),
	% output_string(Output0, Output),
	Output = "",
	(  nonvar(Error)
	-> Solutions = Error
	;  true
	).

query_event(_OK, Error, error) :- nonvar(Error), !.
query_event(true, _, success).
query_event(false, _, failure).

solution_json(Vars0, pairs(Vars)) :- maplist(var_json, Vars0, Vars).

var_json(Var0=Value0, string(Var)-Value) :-
	atom_chars(Var0, Var),
	term_json(Value0, Value).

term_json(Value0, string(Value)) :-
	atom(Value0),
	atom_chars(Value0, Value),
	!.
term_json(Value, string(Value)) :-
	string(Value),
	!.
term_json(Value, number(Value)) :-
	number(Value),
	!.
term_json(Value0, list(Value)) :-
	is_list(Value0),
	maplist(term_json, Value0, Value),
	!.
term_json(Value, pairs([string("functor")-string(Functor), string("args")-list(Args)])) :-
	compound(Value),
	Value =.. [Functor0|Args0],
	atom_chars(Functor0, Functor),
	maplist(term_json, Args0, Args),
	!.
term_json(Value, pairs([string("var")-string("_")])) :-
	var(Value),
	!.

output_string(X, X) :- string(X), !.
output_string('', "") :- !.
output_string([], "") :- !.
