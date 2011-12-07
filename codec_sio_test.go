package socketio

import (
	"bytes"
	"fmt"
	"testing"
	"unicode/utf8"
)

func frame(data string, json bool) string {
	utf8str := utf8.NewString(data)
	if json {
		return fmt.Sprintf("~m~%d~m~~j~%s", 3+utf8str.RuneCount(), data)
	}
	return fmt.Sprintf("~m~%d~m~%s", utf8str.RuneCount(), data)
}

type encodeTest struct {
	in  interface{}
	out string
}

var encodeTests = []encodeTest{
	{
		123,
		frame("123", false),
	},
	{
		"hello, world",
		frame("hello, world", false),
	},
	{
		"öäö¥£♥",
		frame("öäö¥£♥", false),
	},
	{
		"öäö¥£♥",
		frame("öäö¥£♥", false),
	},
	{
		heartbeat(123456),
		frame("~h~123456", false),
	},
	{
		handshake("abcdefg"),
		frame("abcdefg", false),
	},
	{
		true,
		frame("true", true),
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
		frame(`{"Boolean":false,"Str":"string♥","Array":[1,2,3,4]}`, true),
	},
	{
		[]byte("hello, world"),
		frame("hello, world", false),
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
		frame("", false),
		[]decodeTestMessage{{MessageText, "", -1}},
	},
	{
		frame("~h~123", false),
		[]decodeTestMessage{{MessageHeartbeat, "123", 123}},
	},
	{
		frame("wadap!", false),
		[]decodeTestMessage{{MessageText, "wadap!", -1}},
	},
	{
		frame("♥wadap!", true),
		[]decodeTestMessage{{MessageJSON, "♥wadap!", -1}},
	},
	{
		frame("hello, world!", true) + frame("~h~313", false) + frame("♥wadap!", false),
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
		frame("wadap!", false),
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
	var err error

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
				t.Logf("Message was: %#v", msg)
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

func TestDecodeStreaming(t *testing.T) {
	var messages []Message
	var err error
	codec := SIOCodec{}
	buf := new(bytes.Buffer)
	dec := codec.NewDecoder(buf)

	expectNothing := func(written string) {
		if messages, err = dec.Decode(); err != nil || messages == nil || len(messages) != 0 {
			t.Fatalf("Partial decode failed after writing %s. err=%#v messages=%#v", written, err, messages)
		}
	}

	buf.WriteString("~m~")
	expectNothing("~m~")
	buf.WriteString("6")
	expectNothing("~m~6")
	buf.WriteString("~m")
	expectNothing("~m~6~m")
	buf.WriteString("~")
	expectNothing("~m~6~m~")
	buf.WriteString("12345")
	expectNothing("~m~6~m~12345")
	buf.WriteString("6~m~")
	messages, err = dec.Decode()
	if err != nil {
		t.Fatalf("Did not expect errors: %s", err)
	}
	if messages == nil || len(messages) != 1 {
		t.Fatalf("Expected 1 message, got: %#v", messages)
	}
	if messages[0].Type() != MessageText || messages[0].Data() != "123456" {
		t.Fatalf("Expected data 123456 and text, got: %#v", messages[0])
	}
}
