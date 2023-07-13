package internal

import (
	"io"
	"math"
	"net"
)

type Pair struct {
	Key string
	Val string
}

var (
	SecWebSocketVersion    = Pair{"Sec-WebSocket-Version", "13"}
	SecWebSocketKey        = Pair{"Sec-WebSocket-Key", ""}
	SecWebSocketExtensions = Pair{"Sec-WebSocket-Extensions", "permessage-deflate; server_no_context_takeover; client_no_context_takeover"}
	Connection             = Pair{"Connection", "Upgrade"}
	Upgrade                = Pair{"Upgrade", "websocket"}
	SecWebSocketAccept     = Pair{"Sec-WebSocket-Accept", ""}
	SecWebSocketProtocol   = Pair{"Sec-WebSocket-Protocol", ""}
)

// Add four bytes as specified in RFC
// Add final block to squelch unexpected EOF error from flate reader.
var FlateTail = []byte{0x00, 0x00, 0xff, 0xff, 0x01, 0x00, 0x00, 0xff, 0xff}

const (
	MagicNumber     = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	FrameHeaderSize = 14
)

const (
	ThresholdV1 = 125
	ThresholdV2 = math.MaxUint16
	ThresholdV3 = math.MaxUint64
)

// buffer level
const (
	Lv1 = 128
	Lv2 = 1024
	Lv3 = 2 * 1024
	Lv4 = 4 * 1024
	Lv5 = 8 * 1024
	Lv6 = 16 * 1024
	Lv7 = 32 * 1024
	Lv8 = 64 * 1024
)

type (
	ReadLener interface {
		io.Reader
		Len() int
	}

	NetConn interface {
		NetConn() net.Conn
	}
)
