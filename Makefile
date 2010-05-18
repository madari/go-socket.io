all: socketio

socketio:
	make -C src

clean:
	make -C src clean
	
test:
	make -C src test

install:
	make -C src install
