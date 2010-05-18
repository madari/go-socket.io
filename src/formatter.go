package socketio

import (
	"os"
	"json"
)

// Formatter is used to encode and decode the on-the-wire format
type Formatter interface{
	HandshakeEncoder(*Conn) ([]byte, os.Error)
	PayloadEncoder(interface{}) ([]byte, os.Error)
	PayloadDecoder([]byte) ([]string, os.Error)
}

type DefaultFormatter struct{}

func (df DefaultFormatter) HandshakeEncoder(c *Conn) (p []byte, err os.Error) {
	return df.PayloadEncoder([]string{`{"sessionid":"` + c.sessionid + `"}`})
}

func (df DefaultFormatter) PayloadEncoder(payload interface{}) ([]byte, os.Error) {
	return json.Marshal(struct{ messages interface{} }{payload})
}

func (df DefaultFormatter) PayloadDecoder(payload []byte) (msgs []string, err os.Error) {
	err = json.Unmarshal(payload, &msgs)
	return
}
