package socketio

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"unsafe"
)

func frame(data string) string {
	return fmt.Sprintf("\uFFFD%d\uFFFD%s", len([]byte(data)), data)
}

type decodeTest struct {
	in  string
	out []*Message
}

var decodeTests = []decodeTest{
	{
		"7:::",
		[]*Message{&Message{
			typ: MessageError,
		}},
	},
	{
		"7::",
		[]*Message{&Message{
			typ: MessageError,
		}},
	},
	{
		"7:::0",
		[]*Message{&Message{
			typ:  MessageError,
			data: []byte("0"),
		}},
	},
	{
		"7:::2+0",
		[]*Message{&Message{
			typ:  MessageError,
			data: []byte("2+0"),
		}},
	},
	{
		"7::/woot",
		[]*Message{&Message{
			typ:      MessageError,
			endpoint: "/woot",
		}},
	},
	{
		`4:::"2"`,
		[]*Message{&Message{
			typ:  MessageJSON,
			data: []byte(`"2"`),
		}},
	},
	{
		`4:1+::{"a":"b"}`,
		[]*Message{&Message{
			typ:  MessageJSON,
			id:   1,
			ack:  true,
			data: []byte(`{"a":"b"}`),
		}},
	},
	{
		`5:1+::{"name":"irene"}`,
		[]*Message{&Message{
			typ:  MessageEvent,
			id:   1,
			ack:  true,
			data: []byte(`{"name":"irene"}`),
		}},
	},
	{
		"3:5:/irene",
		[]*Message{&Message{
			typ:      MessageText,
			id:       5,
			endpoint: "/irene",
		}},
	},
	{
		`2:::`,
		[]*Message{&Message{
			typ: MessageHeartbeat,
		}},
	},
	{
		"1::/irene",
		[]*Message{&Message{
			typ:      MessageConnect,
			endpoint: "/irene",
		}},
	},
	{
		"1::/irene:?test=1",
		[]*Message{&Message{
			typ:      MessageConnect,
			endpoint: "/irene",
			data:     []byte("?test=1"),
		}},
	},
	{
		"0::/irene",
		[]*Message{&Message{
			typ:      MessageDisconnect,
			endpoint: "/irene",
		}},
	},
	{
		frame("3:::i\u2665am") + frame("4:1+::only") + frame("0::/human\u2665"),
		[]*Message{
			&Message{
				typ:  MessageText,
				data: []byte("i\u2665am"),
			},
			&Message{
				typ:  MessageJSON,
				id:   1,
				ack:  true,
				data: []byte("only"),
			},
			&Message{
				typ:      MessageDisconnect,
				endpoint: "/human\u2665",
			},
		},
	},
}

func TestDecode(t *testing.T) {
	var msg Message
	var err error
	dec := &Decoder{}

Test:
	for _, test := range decodeTests {
		t.Logf("in => %s", test.in)

		dec.Reset()
		dec.Write([]byte(test.in))

		for i := 0; ; i++ {
			if err = dec.Decode(&msg); err != nil {
				if test.out == nil || (err == io.EOF && i == len(test.out)) {
					continue Test
				}
				t.Fatal("decode:", err)
			}
			if test.out == nil {
				t.Fatalf("Expected decode error, but got: %v, %v", msg, err)
			}
			if len(test.out) < i {
				t.Fatalf("Unexpected msg: %v", msg)
			}
			t.Logf("  [%d] expect=%s got=%s", i, test.out[i].Inspect(), msg.Inspect())
			if test.out[i].Type() != msg.Type() {
				t.Fatalf("Expected type %d but got %d", test.out[i].Type(), msg.Type())
			}
			if !bytes.Equal(test.out[i].Bytes(), msg.Bytes()) {
				t.Fatalf("Expected data %q but got %q", test.out[i].Bytes(), msg.Bytes())
			}
			if test.out[i].ack != msg.ack {
				t.Fatalf("Expected ack %t but got %t", test.out[i].ack, msg.ack)
			}
			if test.out[i].id != msg.id {
				t.Fatalf("Expected id %d but got %d", test.out[i].id, msg.id)
			}
			if test.out[i].endpoint != msg.endpoint {
				t.Fatalf("Expected endpoint %q but got %q", test.out[i].endpoint, msg.endpoint)
			}
		}
	}
}

type encodeTest struct {
	in  []interface{}
	out string
}

var encodeTests = []encodeTest{
	{
		[]interface{}{
			&error_{-1, "", -1},
		},
		frame("7::"),
	},
	{
		[]interface{}{
			&error_{0, "", 2},
		},
		frame("7:::2+0"),
	},
	{
		[]interface{}{
			&error_{0, "", -1},
		},
		frame("7:::+0"),
	},
	{
		[]interface{}{
			&error_{-1, "/irene", 0},
		},
		frame("7::/irene:0"),
	},
	{
		[]interface{}{&Message{
			typ:      MessageJSON,
			endpoint: "/irene",
			data:     []byte(`"0"`),
		}},
		frame(`4::/irene:"0"`),
	},
	{
		[]interface{}{&Message{
			typ:  MessageJSON,
			id:   1,
			ack:  true,
			data: []byte(`{"a":"b"}`),
		}},
		frame(`4:1+::{"a":"b"}`),
	},
	{
		[]interface{}{&event{
			id:   1,
			ack:  true,
			Name: "irene",
		}},
		frame(`5:1+::{"name":"irene"}`),
	},
	{
		[]interface{}{&event{
			id:   1,
			ack:  true,
			Name: "irene",
			Args: []interface{}{"string", 123},
		}},
		frame(`5:1+::{"args":["string",123],"name":"irene"}`),
	},
	{
		[]interface{}{&Message{
			typ:      MessageText,
			id:       5,
			endpoint: "/irene",
		}},
		frame(`3:5:/irene`),
	},
	{
		[]interface{}{heartbeat(0)},
		frame(`2::`),
	},
	{
		[]interface{}{connect("/irene\u2665")},
		frame("1::/irene\u2665"),
	},
	{
		[]interface{}{disconnect("/irene")},
		frame("0::/irene"),
	},
	{
		[]interface{}{
			&Message{
				typ:  MessageText,
				data: []byte("i\u2665am"),
			},
			&Message{
				typ:  MessageJSON,
				id:   1,
				ack:  true,
				data: []byte("only"),
			},
			disconnect("/human\u2665"),
		},
		frame("3:::i\u2665am") + frame("4:1+::only") + frame("0::/human\u2665"),
	},
}

func TestEncode(t *testing.T) {
	var err error
	var buf bytes.Buffer
	enc := &Encoder{MustFrame: true}

	for _, test := range encodeTests {
		buf.Reset()

		t.Logf("in => %#v", test.in)

		if err = enc.Encode(&buf, test.in); err != nil {
			if test.out == "" {
				continue
			}
			t.Fatal("encode:", err)
		}
		if test.out == "" {
			t.Fatalf("Expected encode error, but got: %s, %v", buf.String(), err)
		}
		t.Logf("  [0] expect=%s got=%s", test.out, buf.String())
		if buf.String() != test.out {
			t.Fatal("Mismatch")
		}
	}
}

type nopWriter struct{}

func (nw nopWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

var w = &nopWriter{}

func BenchmarkIntEncode(b *testing.B) {
	enc := &Encoder{}
	payload := 313313
	b.SetBytes(int64(unsafe.Sizeof(payload)))

	for i := 0; i < b.N; i++ {
		enc.Encode(w, []interface{}{payload})
	}
}

func BenchmarkStringEncode(b *testing.B) {
	enc := &Encoder{}
	payload := "Hello, World!"
	b.SetBytes(int64(len(payload)))

	for i := 0; i < b.N; i++ {
		enc.Encode(w, []interface{}{payload})
	}
}

func BenchmarkStructEncode(b *testing.B) {
	enc := &Encoder{}
	payload := struct {
		boolean bool
		str     string
		array   []int
	}{
		false,
		"string",
		[]int{1, 2, 3, 4},
	}
	b.SetBytes(int64(unsafe.Sizeof(payload)))

	for i := 0; i < b.N; i++ {
		enc.Encode(w, []interface{}{payload})
	}
}

func BenchmarkSingleFrameDecode(b *testing.B) {
	var msg Message
	dec := &Decoder{}
	data := []byte("3:::wadap")
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		dec.Write(data)
		dec.Decode(&msg)
	}
}

func BenchmarkThreeFramesDecode(b *testing.B) {
	var msg Message
	dec := &Decoder{}
	data := []byte(frame("3:::i\u2665am") + frame("4:1+::only") + frame("0::/human\u2665"))
	b.SetBytes(int64(len(data)))
	var err error

	for i := 0; i < b.N; i++ {
		dec.Write(data)
		for err == nil {
			err = dec.Decode(&msg)
		}
		err = nil
	}
}
