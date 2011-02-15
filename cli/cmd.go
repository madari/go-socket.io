package main

import (
	"os"
	"fmt"
	"rpc"
	"socketio"
	"tabwriter"
	"time"
)

var cmds = map[string]func([]string, *rpc.Client) os.Error{
	"quit":   cmdQuit,
	"help":   cmdHelp,
	"status": cmdStatus,
	"kick":   cmdKick,
	"conn":   cmdConn,
	"conns":  cmdConns,
}

var writer = tabwriter.NewWriter(os.Stdout, 0, 4, 8, ' ', 0)

func formatSeconds(s int64) string {
	if s == 0 {
		return "-"
	}
	return time.SecondsToLocalTime(s).Format(time.RFC3339)
}

func getState(c *socketio.RPCConnInfo) string {
	if c.Online {
		return "online"
	}
	return "keepalive"
}

func cmdQuit(args []string, _ *rpc.Client) os.Error {
	os.Exit(0)
	return nil
}

func cmdHelp(args []string, _ *rpc.Client) os.Error {
	fmt.Fprintln(writer, "Command\tArguments\tDescription")
	fmt.Fprintln(writer, "help\t\tShow this help")
	fmt.Fprintln(writer, "quit\t\tDisconnect and exit")
	fmt.Fprintln(writer, "status\t\tShow information about the server")
	fmt.Fprintln(writer, "conns\t\tShow established sessions")
	fmt.Fprintln(writer, "conn\tsession id\tDisplay detailed information of the connection")
	fmt.Fprintln(writer, "kick\tsession id\tKicks the connection")
	writer.Flush()

	return nil
}
func cmdStatus(args []string, r *rpc.Client) (err os.Error) {
	var s socketio.RPCStatusInfo
	if err = r.Call("RPCServer.Status", &args, &s); err != nil {
		return
	}

	fmt.Fprintln(writer, "Active sessions\t", s.NumSessions)
	fmt.Fprintln(writer, "Total sessions\t", s.TotalSessions)
	fmt.Fprintln(writer, "Total requests\t", s.TotalRequests)
	fmt.Fprintln(writer, "Total packets sent\t", s.TotalPacketsSent)
	fmt.Fprintln(writer, "Total packets received\t", s.TotalPacketsReceived)
	writer.Flush()
	return
}

func cmdConns(args []string, r *rpc.Client) (err os.Error) {
	var reply []socketio.RPCConnInfo
	if err = r.Call("RPCServer.Conns", &args, &reply); err != nil {
		return
	}

	fmt.Fprintln(writer, "Session ID\tTransport\tState\tEstablished\tRemote")
	for _, c := range reply {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", c.SID, c.Transport, getState(&c), formatSeconds(c.FirstConnected), c.RemoteAddr)
	}
	writer.Flush()

	return
}

func cmdKick(args []string, r *rpc.Client) os.Error {
	if len(args) < 1 {
		return os.NewError("oops, give me a session id")
	}

	var reply byte
	return r.Call("RPCServer.Kick", &args, &reply)
}

func cmdConn(args []string, r *rpc.Client) (err os.Error) {
	if len(args) < 1 {
		return os.NewError("oops, give me a session id")
	}

	var c socketio.RPCConnInfo
	if err = r.Call("RPCServer.Conn", &args, &c); err != nil {
		return
	}

	fmt.Fprintln(writer, "Session ID\t", c.SID)
	fmt.Fprintln(writer, "Transport\t", c.Transport)
	fmt.Fprintln(writer, "State\t", getState(&c))
	fmt.Fprintln(writer, "Handshaked\t", c.Handshaked)
	fmt.Fprintln(writer, "Total connections\t", c.NumConns)
	fmt.Fprintln(writer, "Heartbeats received\t", c.NumHeartbeats)
	fmt.Fprintln(writer, "Packets sent\t", c.NumPacketsSent)
	fmt.Fprintln(writer, "Packets received\t", c.NumPacketsReceived)
	fmt.Fprintln(writer, "Packets in queue\t", c.QueueLength)
	fmt.Fprintln(writer, "Bytes in decoder buf\t", c.DecoderBufferLength)
	fmt.Fprintln(writer, "Established\t", formatSeconds(c.FirstConnected))
	fmt.Fprintln(writer, "Last disconnected\t", formatSeconds(c.LastDisconnected))
	fmt.Fprintln(writer, "Remote Address\t", c.RemoteAddr)
	fmt.Fprintln(writer, "User-Agent\t", c.UserAgent)
	writer.Flush()

	return
}
