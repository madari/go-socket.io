package socketio

import (
	"rpc"
	"rpc/jsonrpc"
	"os"
	"net"
)

type RPCServer struct {
	sio *SocketIO
}

type RPCError os.Error

type RPCConnInfo struct {
	SID, Transport, RemoteAddr, UserAgent                   string
	NumConns, NumHeartbeats, QueueLength                    int
	DecoderBufferLength, NumPacketsSent, NumPacketsReceived int
	FirstConnected, LastDisconnected                        int64
	Online, Handshaked, Disconnected                        bool
}

func newRPCConnInfo(c *Conn) *RPCConnInfo {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return &RPCConnInfo{
		string(c.sessionid),
		c.transport().Resource(),
		c.remoteAddr(),
		c.userAgent(),
		c.numConns,
		c.numHeartbeats,
		len(c.queue),
		c.decBuf.Len(),
		c.numPacketsSent,
		c.numPacketsReceived,
		c.firstConnected,
		c.lastDisconnected,
		c.online,
		c.handshaked,
		c.disconnected,
	}
}

type RPCStatusInfo struct {
	NumSessions                                                          int
	TotalPacketsReceived, TotalPacketsSent, TotalSessions, TotalRequests int64
}

func (r *RPCServer) Status(args *[]string, reply *RPCStatusInfo) os.Error {
	r.sio.mutex.Lock()
	defer r.sio.mutex.Unlock()

	*reply = RPCStatusInfo{
		len(r.sio.sessions),
		r.sio.totalPacketsReceived,
		r.sio.totalPacketsSent,
		r.sio.totalSessions,
		r.sio.totalRequests,
	}

	return nil
}

func (r *RPCServer) Kick(args *[]string, reply *byte) os.Error {
	if len(*args) < 1 {
		return RPCError(os.NewError("RPCServer.Kick: first argument must be a session id"))
	}

	conn := r.sio.GetConn(SessionID((*args)[0]))
	if conn == nil {
		return RPCError(os.NewError("RPCServer.Conn: no connection with session id " + (*args)[0]))
	}

	return conn.Close()
}

func (r *RPCServer) Conns(args *[]string, reply *[]RPCConnInfo) os.Error {
	r.sio.mutex.Lock()
	defer r.sio.mutex.Unlock()

	*reply = make([]RPCConnInfo, len(r.sio.sessions))
	i := 0
	for _, c := range r.sio.sessions {
		(*reply)[i] = *newRPCConnInfo(c)
		i++
	}

	return nil
}

func (r *RPCServer) Conn(args *[]string, reply *RPCConnInfo) os.Error {
	r.sio.mutex.Lock()
	defer r.sio.mutex.Unlock()

	if len(*args) < 1 {
		return RPCError(os.NewError("RPCServer.Conn: first argument must be a session id"))
	}

	if c, ok := r.sio.sessions[SessionID((*args)[0])]; ok {
		*reply = *newRPCConnInfo(c)
		return nil
	}

	return RPCError(os.NewError("RPCServer.Conn: no connection with session id " + (*args)[0]))
}

func (sio *SocketIO) ListenAndServeRPC(laddr string) os.Error {
	rpcs := &RPCServer{sio}
	rpc.Register(rpcs)

	l, err := net.Listen("tcp", laddr)
	if err != nil {
		return err
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		go rpc.ServeCodec(jsonrpc.NewServerCodec(conn))
	}
	return nil
}
