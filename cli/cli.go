package main

import (
	"os"
	"fmt"
	"flag"
	"readline"
	"net"
	"strings"
	"rpc"
	"rpc/jsonrpc"
	"socketio"
)

const (
	PromptColor = "\x1b[90m"
	ResetColor  = "\x1b[0m"
)

func usage() {
	fmt.Println("usage: cli remote-address")
	os.Exit(2)
}

func error(context string, err os.Error) {
	if err != nil {
		fmt.Println(context+":", err)
	} else {
		fmt.Println(context)
	}
	os.Exit(1)
}

func repl(raddr string, r *rpc.Client) {
	prompt := fmt.Sprintf("%s%s> %s", PromptColor, raddr, ResetColor)

	for {
		line := readline.ReadLine(&prompt)
		if line == nil || *line == "" {
			continue
		}

		args := strings.Split(*line, " ", -1)

		if c, ok := cmds[args[0]]; ok {
			if err := c(args[1:], r); err != nil {
				if _, ok = err.(socketio.RPCError); ok {
					fmt.Println(args[0]+":", err)
				} else {
					error(args[0], err)
				}
			}
		} else {
			fmt.Printf("unknown command: %q\n", args[0])
		}
		readline.AddHistory(*line)
	}
}

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		usage()
	}

	raddr := flag.Arg(0)
	conn, err := net.Dial("tcp", "", raddr)
	if err != nil {
		error("net.Dial", err)
	}

	client := jsonrpc.NewClient(conn)
	repl(raddr, client)
}
