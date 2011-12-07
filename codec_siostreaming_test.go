package socketio

import (
	"bytes"
	"fmt"
	"testing"
	"unicode/utf8"
	"unsafe"
)

func streamingFrame(data string, typ int, json bool) string {
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

type streamingEncodeTest struct {
	in  interface{}
	out string
}

var streamingEncodeTests = []streamingEncodeTest{
	{
		123,
		streamingFrame("123", 1, false),
	},
	{
		"hello, world",
		streamingFrame("hello, world", 1, false),
	},
	{
		"öäö¥£♥",
		streamingFrame("öäö¥£♥", 1, false),
	},
	{
		"öäö¥£♥",
		streamingFrame("öäö¥£♥", 1, false),
	},
	{
		heartbeat(123456),
		streamingFrame("123456", 2, false),
	},
	{
		handshake("abcdefg"),
		streamingFrame("abcdefg", 3, false),
	},
	{
		true,
		streamingFrame("true", 1, true),
	},
	{
		struct {
			Boolean bool
			Str     string
			Array   []int
		}{
			false,
			"string♥",
			[]int{1, 2, 3, 4},
		},
		streamingFrame(`{"Boolean":false,"Str":"string♥","Array":[1,2,3,4]}`, 1, true),
	},
	{
		[]byte("hello, world"),
		streamingFrame("hello, world", 1, false),
	},
}

type streamingDecodeTestMessage struct {
	messageType uint8
	data        string
	heartbeat   heartbeat
}

type streamingDecodeTest struct {
	in  string
	out []streamingDecodeTestMessage
}

// NOTE: if you change these -> adjust the benchmarks
var streamingDecodeTests = []streamingDecodeTest{
	{
		streamingFrame("", 1, false),
		[]streamingDecodeTestMessage{{MessageText, "", -1}},
	},
	{
		streamingFrame("123", 2, false),
		[]streamingDecodeTestMessage{{MessageHeartbeat, "123", 123}},
	},
	{
		streamingFrame("wadap!", 1, false),
		[]streamingDecodeTestMessage{{MessageText, "wadap!", -1}},
	},
	{
		streamingFrame("♥wadap!", 1, true),
		[]streamingDecodeTestMessage{{MessageJSON, "♥wadap!", -1}},
	},
	{
		streamingFrame("hello, world!", 1, true) + streamingFrame("313", 2, false) + streamingFrame("♥wadap!", 1, false),
		[]streamingDecodeTestMessage{
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
		streamingFrame("wadap!", 1, false),
		[]streamingDecodeTestMessage{{MessageText, "wadap!", -1}},
	},
}

func TestStreamingEncode(t *testing.T) {
	codec := SIOStreamingCodec{}
	enc := codec.NewEncoder()
	buf := new(bytes.Buffer)

	for _, test := range streamingEncodeTests {
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

func TestStreamingDecode(t *testing.T) {
	codec := SIOStreamingCodec{}
	buf := new(bytes.Buffer)
	dec := codec.NewDecoder(buf)
	var messages []Message
	var err error

	for _, test := range streamingDecodeTests {
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
			t.Logf("message: %#v", msg)
			if test.out[i].messageType != msg.Type() {
				t.Fatalf("Expected type %d but got %d", test.out[i].messageType, msg.Type())
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

func TestStreamingDecodeStreaming(t *testing.T) {
	var messages []Message
	var err error
	codec := SIOStreamingCodec{}
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
	codec := SIOStreamingCodec{}
	enc := codec.NewEncoder()
	payload := 313313
	b.SetBytes(int64(unsafe.Sizeof(payload)))
	w := nopWriter{}

	for i := 0; i < b.N; i++ {
		enc.Encode(w, payload)
	}
}

func BenchmarkStringEncode(b *testing.B) {
	codec := SIOStreamingCodec{}
	enc := codec.NewEncoder()
	payload := "Hello, World!"
	b.SetBytes(int64(len(payload)))
	w := nopWriter{}

	for i := 0; i < b.N; i++ {
		enc.Encode(w, payload)
	}
}

func BenchmarkStructEncode(b *testing.B) {
	codec := SIOStreamingCodec{}
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
	codec := SIOStreamingCodec{}
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
	codec := SIOStreamingCodec{}
	buf := new(bytes.Buffer)
	dec := codec.NewDecoder(buf)
	data := []byte(decodeTests[3].in)
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		buf.Write(data)
		dec.Decode()
	}
}
