package socketio

import (
	"testing"
	"utf8"
	"fmt"
	"bytes"
	"unsafe"
	"os"
)

func frame(data string, typ int, json bool) string {
	utf8str := utf8.NewString(data)
	switch typ {
	case 0:
		return "0:0:,"

	case 2, 3:
		return fmt.Sprintf("%d:%d:%s,", typ, utf8str.RuneCount(), data)
	}

	if json {
		return fmt.Sprintf("%d:%d:j\n:%s,", typ, 3+utf8str.RuneCount(), data)
	}
	return fmt.Sprintf("%d:%d::%s,", typ, 1+utf8str.RuneCount(), data)
}

type encodeTest struct {
	in  interface{}
	out string
}

var encodeTests = []encodeTest{
	{
		123,
		frame("123", 1, false),
	},
	{
		"hello, world",
		frame("hello, world", 1, false),
	},
	{
		"öäö¥£♥",
		frame("öäö¥£♥", 1, false),
	},
	{
		"öäö¥£♥",
		frame("öäö¥£♥", 1, false),
	},
	{
		heartbeat(123456),
		frame("123456", 2, false),
	},
	{
		handshake("abcdefg"),
		frame("abcdefg", 3, false),
	},
	{
		true,
		frame("true", 1, true),
	},
	{
		struct {
			boolean bool
			str     string
			array   []int
		}{
			false,
			"string♥",
			[]int{1, 2, 3, 4},
		},
		frame(`{"boolean":false,"str":"string♥","array":[1,2,3,4]}`, 1, true),
	},
	{
		[]byte("hello, world"),
		frame("hello, world", 1, false),
	},
}


type decodeTestMessage struct {
	messageType uint8
	data        string
	heartbeat   heartbeat
}

type decodeTest struct {
	in  string
	out []decodeTestMessage
}

// NOTE: if you change these -> adjust the benchmarks
var decodeTests = []decodeTest{
	{
		frame("123", 2, false),
		[]decodeTestMessage{{MessageHeartbeat, "123", 123}},
	},
	{
		frame("wadap!", 1, false),
		[]decodeTestMessage{{MessageText, "wadap!", -1}},
	},
	{
		frame("♥wadap!", 1, true),
		[]decodeTestMessage{{MessageJSON, "♥wadap!", -1}},
	},
	{
		frame("hello, world!", 1, true) + frame("313", 2, false) + frame("♥wadap!", 1, false),
		[]decodeTestMessage{
			{MessageJSON, "hello, world!", -1},
			{MessageHeartbeat, "313", 313},
			{MessageText, "♥wadap!", -1},
		},
	},
	{
		frame("ok", 1, false) + "foobar!",
		nil,
	},
}

func TestEncode(t *testing.T) {
	codec := SIOCodec{}
	buf := new(bytes.Buffer)

	for _, test := range encodeTests {
		buf.Reset()
		if err := codec.Encode(buf, test.in); err != nil {
			t.Fatal("Encode:", err)
		}
		if string(buf.Bytes()) != test.out {
			t.Fatalf("Expected %q but got %q from %q", test.out, string(buf.Bytes()), test.in)
		}
	}
}

func TestDecode(t *testing.T) {
	codec := SIOCodec{}
	var messages []Message
	var err os.Error

	for _, test := range decodeTests {
		if messages, err = codec.Decode([]byte(test.in)); err != nil {
			if test.out == nil {
				continue
			}
			t.Fatal("Decode:", err)
		}
		if test.out == nil {
			t.Fatalf("Expected decode error, but got: %v, %v", messages, err)
		}
		if len(messages) != len(test.out) {
			t.Fatalf("Expected %d messages, but got %d", len(messages), len(test.out))
		}
		for i, msg := range messages {
			if test.out[i].messageType != msg.Type() {
				t.Fatalf("Expected type %q but got %q", test.out[i].messageType, msg.Type())
			}
			if test.out[i].data != msg.Data() {
				t.Fatalf("Expected data %q but got %q", test.out[i].data, msg.Data())
			}
			if test.out[i].messageType == MessageHeartbeat {
				if hb, ok := msg.heartbeat(); !ok || test.out[i].heartbeat != hb {
					t.Fatalf("Expected heartbeat %d but got %d (%v)", test.out[i].heartbeat, hb, err)
				}
			}
		}
	}
}

func BenchmarkIntEncode(b *testing.B) {
	codec := SIOCodec{}
	payload := 313313
	b.SetBytes(int64(unsafe.Sizeof(payload)))
	w := nopWriter{}

	for i := 0; i < b.N; i++ {
		codec.Encode(w, payload)

	}
}

func BenchmarkStringEncode(b *testing.B) {
	codec := SIOCodec{}
	payload := "Hello, World!"
	b.SetBytes(int64(len(payload)))
	w := nopWriter{}

	for i := 0; i < b.N; i++ {
		codec.Encode(w, payload)

	}
}

func BenchmarkStructEncode(b *testing.B) {
	codec := SIOCodec{}
	payload := struct {
		boolean bool
		str     string
		array   []int
	}{
		false,
		"string♥",
		[]int{1, 2, 3, 4},
	}

	b.SetBytes(int64(unsafe.Sizeof(payload)))
	w := nopWriter{}

	for i := 0; i < b.N; i++ {
		codec.Encode(w, payload)
	}
}

func BenchmarkSingleFrameDecode(b *testing.B) {
	codec := SIOCodec{}
	data := []byte(decodeTests[2].in)
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		codec.Decode(data)
	}
}

func BenchmarkThreeFramesDecode(b *testing.B) {
	codec := SIOCodec{}
	data := []byte(decodeTests[3].in)
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		codec.Decode(data)
	}
}
