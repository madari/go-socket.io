package socketio

import (
	"reflect"
	"testing"
)

type eventTest struct {
	in   *Message
	out  *event
	args []interface{}
	want []interface{}
}

type stru struct {
	A bool
	B int
}

var eventTests = []eventTest{
	{ //`5:1+::{"name":"tobi"}`,
		in:  &Message{typ: MessageNOOP},
		out: nil,
	},
	{
		in:  &Message{typ: MessageEvent},
		out: nil,
	},
	{
		in:  &Message{typ: MessageEvent, data: []byte("{}")},
		out: &event{Name: ""},
	},
	{
		in:  &Message{typ: MessageEvent, data: []byte(`{"Name":"test"}`)},
		out: &event{Name: "test"},
	},
	{
		in:  &Message{typ: MessageEvent, data: []byte(`{"Name":"test2","Args":[]}`)},
		out: &event{Name: "test2"},
	},
	{
		in:   &Message{typ: MessageEvent, data: []byte(`{"Name":"test3","Args":[1,"123",true]}`)},
		out:  &event{Name: "test3"},
		args: []interface{}{new(int), new(string), new(bool)},
		want: []interface{}{1, "123", true},
	},
	{
		in:   &Message{typ: MessageEvent, data: []byte(`{"Name":"test4","Args":[null, 3.13]}`)},
		out:  &event{Name: "test4"},
		args: []interface{}{new(*int), new(float64)},
		want: []interface{}{(*int)(nil), 3.13},
	},
	{
		in:   &Message{typ: MessageEvent, data: []byte(`{"Name":"test5","Args":["first",{"a":true,"b":99}]}`)},
		out:  &event{Name: "test5"},
		args: []interface{}{new(string), new(stru)},
		want: []interface{}{"first", stru{true, 99}},
	},
}

func TestEvents(t *testing.T) {
	for _, test := range eventTests {
		t.Logf("in => %s", test.in.Inspect())
		name, err := test.in.Event()
		if err != nil {
			if test.out != nil {
				t.Fatal("unexpected error: ", err)
			}
			continue
		}
		if test.out.Name != name {
			t.Fatal("name mismatch")
		}
		if test.args != nil {
			if err := test.in.ReadArguments(test.args...); err != nil {
				t.Fatal("unexpected error (2): ", err)
			}
			for i, v := range test.args {
				if !reflect.DeepEqual(test.want[i], reflect.ValueOf(v).Elem().Interface()) {
					t.Fatalf("argument=%d wanted %#v, but got %#v", i, test.want[i], reflect.ValueOf(v).Elem().Interface())
				}
			}
		}
	}
}
