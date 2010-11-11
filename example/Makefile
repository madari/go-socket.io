include $(GOROOT)/src/Make.inc

SIO = ../src
GC += -I$(SIO)/_obj/
LD += -L$(SIO)/_obj/
LIB = $(SIO)/_obj/socketio.a
TARG = example
GOFILES = example.go

$(TARG): $(LIB)

$(LIB): $(wildcard $(SIO)/*.go)
	make -C $(SIO)

include $(GOROOT)/src/Make.cmd
