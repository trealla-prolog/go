clean:
	rm -f trealla/libtpl.wasm

wasm: trealla/libtpl.wasm

trealla/libtpl.wasm:
	cd src/trealla && $(MAKE) clean && $(MAKE) -j8 libtpl && \
	cp libtpl.wasm ../../trealla/libtpl.wasm

.PHONY: clean
