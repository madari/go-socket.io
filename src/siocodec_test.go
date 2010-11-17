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
		frame("", 1, false),
		[]decodeTestMessage{{MessageText, "", -1}},
	},
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
		"1:3::fael!,",
		nil,
	},
	{
		frame("wadap!", 1, false),
		[]decodeTestMessage{{MessageText, "wadap!", -1}},
	},
}

func TestEncode(t *testing.T) {
	codec := SIOCodec{}
	enc := codec.NewEncoder()
	buf := new(bytes.Buffer)

	for _, test := range encodeTests {
		t.Logf("in=%v out=%s", test.in, test.out)

		buf.Reset()
		if err := enc.Encode(buf, test.in); err != nil {
			t.Fatal("Encode:", err)
		}
		if string(buf.Bytes()) != test.out {
			t.Fatalf("Expected %q but got %q from %q", test.out, string(buf.Bytes()), test.in)
		}
	}
}

func TestDecode(t *testing.T) {
	codec := SIOCodec{}
	buf := new(bytes.Buffer)
	dec := codec.NewDecoder(buf)
	var messages []Message
	var err os.Error

	for _, test := range decodeTests {
		t.Logf("in=%s out=%v", test.in, test.out)

		buf.WriteString(test.in)
		if messages, err = dec.Decode(); err != nil {
			if test.out == nil {
				continue
			}
			t.Fatal("Decode:", err)
		}
		if test.out == nil {
			t.Fatalf("Expected decode error, but got: %v, %v", messages, err)
		}
		if len(messages) != len(test.out) {
			t.Fatalf("Expected %d messages, but got %d", len(test.out), len(messages))
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

func TestDecodeStreaming(t *testing.T) {
	var messages []Message
	var err os.Error
	codec := SIOCodec{}
	buf := new(bytes.Buffer)
	dec := codec.NewDecoder(buf)

	expectNothing := func(written string) {
		if messages, err = dec.Decode(); err != nil || messages == nil || len(messages) != 0 {
			t.Fatalf("Partial decode failed after writing %s. err=%#v messages=%#v", written, err, messages)
		}
	}

	buf.WriteString("5")
	expectNothing("5")
	buf.WriteString(":9")
	expectNothing("5:9")
	buf.WriteString(":12345")
	expectNothing("5:9:12345")
	buf.WriteString("678")
	expectNothing("5:9:12345678")
	buf.WriteString("9")
	expectNothing("5:9:123456789")
	buf.WriteString(",typefornextmessagewhichshouldbeignored")
	messages, err = dec.Decode()
	if err != nil {
		t.Fatalf("Did not expect errors: %s", err)
	}
	if messages == nil || len(messages) != 1 {
		t.Fatalf("Expected 1 message, got: %#v", messages)
	}
	if messages[0].(*sioMessage).typ != 5 || messages[0].Data() != "123456789" {
		t.Fatalf("Expected data 123456789 and typ 5, got: %#v", messages[0])
	}
}

func BenchmarkIntEncode(b *testing.B) {
	codec := SIOCodec{}
	enc := codec.NewEncoder()
	payload := 313313
	b.SetBytes(int64(unsafe.Sizeof(payload)))
	w := nopWriter{}

	for i := 0; i < b.N; i++ {
		enc.Encode(w, payload)
	}
}

func BenchmarkStringEncode(b *testing.B) {
	codec := SIOCodec{}
	enc := codec.NewEncoder()
	payload := "Hello, World!"
	b.SetBytes(int64(len(payload)))
	w := nopWriter{}

	for i := 0; i < b.N; i++ {
		enc.Encode(w, payload)
	}
}

func BenchmarkStructEncode(b *testing.B) {
	codec := SIOCodec{}
	enc := codec.NewEncoder()
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
		enc.Encode(w, payload)
	}
}

func BenchmarkSingleFrameDecode(b *testing.B) {
	codec := SIOCodec{}
	buf := new(bytes.Buffer)
	dec := codec.NewDecoder(buf)
	data := []byte(decodeTests[2].in)
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		buf.Write(data)
		dec.Decode()
	}
}

func BenchmarkThreeFramesDecode(b *testing.B) {
	codec := SIOCodec{}
	buf := new(bytes.Buffer)
	dec := codec.NewDecoder(buf)
	data := []byte(decodeTests[3].in)
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		buf.Write(data)
		dec.Decode()
	}
}
