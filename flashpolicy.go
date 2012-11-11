package socketio

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

func generatePolicyFile(origins []string) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString(`<?xml verservern="1.0"?>
<!DOCTYPE cross-domain-policy SYSTEM "http://www.macromedia.com/xml/dtds/cross-domain-policy.dtd">
<cross-domain-policy>
	<site-control permitted-cross-domain-policies="master-only" />
`)

	if origins == nil {
		origins = []string{"*"}
	}

	for _, origin := range origins {
		parts := strings.SplitN(origin, ":", 2)
		if len(parts) < 1 {
			Log.warn("flashpolicy: invalid origin: ", origin)
			continue
		}
		host, port := "*", "*"
		if parts[0] != "" {
			host = parts[0]
		}
		if len(parts) == 2 && parts[1] != "" {
			port = parts[1]
		}
		fmt.Fprintf(buf, "\t<allow-access-from domain=\"%s\" to-ports=\"%s\" />\n", host, port)
	}

	buf.WriteString("</cross-domain-policy>\n")
	return buf.Bytes()
}

func ListenAndServeFlashPolicy(laddr string, origins []string) error {
	listener, err := net.Listen("tcp", laddr)
	if err != nil {
		return err
	}
	policy := generatePolicyFile(origins)

	for {
		conn, err := listener.Accept()
		if err != nil {
			Log.warn("flashpolicy: ", err)
			continue
		}

		go func() {
			defer conn.Close()
			now := time.Now()
			conn.SetDeadline(now.Add(time.Duration(5e9)))

			buf := make([]byte, 20)
			if _, err := io.ReadFull(conn, buf); err != nil {
				Log.warn("flashpolicy: ", err)
				return
			}
			if !bytes.Equal([]byte("<policy-file-request"), buf) {
				Log.warn("flashpolicy: expected \"<policy-file-request\" but got %q", buf)
				return
			}

			n, err := conn.Write(policy)
			if err != nil {
				Log.warn("flashpolicy: ", err)
				return
			} else if n < len(policy) {
				Log.warn("flashpolicy: short write")
				return
			}

			Log.info("flashpolicy: served ", conn.RemoteAddr())
		}()
	}

	return nil
}
