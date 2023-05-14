.PHONY: clean

all: clean wasm

clean:
	rm -f trealla/libtpl.wasm

wasm: trealla/libtpl.wasm

trealla/libtpl.wasm:
	cd src/trealla && $(MAKE) clean && $(MAKE) -j8 libtpl && \
	cp libtpl.wasm ../../trealla/libtpl.wasm

update:
	cd src/trealla && git fetch --all && git pull origin main
