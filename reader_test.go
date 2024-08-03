package gws

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/marifcelik/gws/internal"
	"github.com/stretchr/testify/assert"
)

// 测试同步读
func TestReadSync(t *testing.T) {
	var mu = &sync.Mutex{}
	var listA []string
	var listB []string
	const count = 1000
	var wg = &sync.WaitGroup{}
	wg.Add(count)

	var serverHandler = new(webSocketMocker)
	var clientHandler = new(webSocketMocker)
	var serverOption = &ServerOption{PermessageDeflate: PermessageDeflate{
		Enabled:               true,
		ServerContextTakeover: true,
		ClientContextTakeover: false,
		ServerMaxWindowBits:   10,
		ClientMaxWindowBits:   10,
	}}
	var clientOption = &ClientOption{PermessageDeflate: PermessageDeflate{
		Enabled:               true,
		ServerContextTakeover: true,
		ClientContextTakeover: true,
	}}

	serverHandler.onMessage = func(socket *Conn, message *Message) {
		mu.Lock()
		listB = append(listB, message.Data.String())
		mu.Unlock()
		wg.Done()
	}

	server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
	go server.ReadLoop()
	go client.ReadLoop()

	for i := 0; i < count; i++ {
		var n = internal.AlphabetNumeric.Intn(1024)
		var message = internal.AlphabetNumeric.Generate(n)
		listA = append(listA, string(message))
		client.WriteAsync(OpcodeText, message, nil)
	}

	wg.Wait()
	assert.ElementsMatch(t, listA, listB)
}

//go:embed assets/read_test.json
var testdata []byte

type testRow struct {
	Title    string `json:"title"`
	Fin      bool   `json:"fin"`
	Opcode   uint8  `json:"opcode"`
	Length   int    `json:"length"`
	Payload  string `json:"payload"`
	RSV2     bool   `json:"rsv2"`
	Expected struct {
		Event  string `json:"event"`
		Code   uint16 `json:"code"`
		Reason string `json:"reason"`
	} `json:"expected"`
}

func TestRead(t *testing.T) {
	var as = assert.New(t)

	var items = make([]testRow, 0)
	if err := json.Unmarshal(testdata, &items); err != nil {
		as.NoError(err)
		return
	}

	for _, item := range items {
		println(item.Title)
		var payload []byte
		if item.Payload == "" {
			payload = internal.AlphabetNumeric.Generate(item.Length)
		} else {
			p, err := hex.DecodeString(item.Payload)
			if err != nil {
				as.NoError(err)
				return
			}
			payload = p
		}

		var wg = &sync.WaitGroup{}
		wg.Add(1)

		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{
			ParallelEnabled:     true,
			CheckUtf8Enabled:    false,
			ReadMaxPayloadSize:  1024 * 1024,
			WriteMaxPayloadSize: 16 * 1024 * 1024,
			PermessageDeflate:   PermessageDeflate{Enabled: true},
		}
		var clientOption = &ClientOption{
			ParallelEnabled:     true,
			PermessageDeflate:   PermessageDeflate{Enabled: true, ServerContextTakeover: true, ClientContextTakeover: true},
			CheckUtf8Enabled:    true,
			ReadMaxPayloadSize:  1024 * 1024,
			WriteMaxPayloadSize: 1024 * 1024,
		}

		switch item.Expected.Event {
		case "onMessage":
			clientHandler.onMessage = func(socket *Conn, message *Message) {
				as.Equal(string(payload), message.Data.String())
				wg.Done()
			}
		case "onPing":
			clientHandler.onPing = func(socket *Conn, d []byte) {
				as.Equal(string(payload), string(d))
				wg.Done()
			}
		case "onPong":
			clientHandler.onPong = func(socket *Conn, d []byte) {
				as.Equal(string(payload), string(d))
				wg.Done()
			}
		case "onClose":
			clientHandler.onClose = func(socket *Conn, err error) {
				if v, ok := err.(*CloseError); ok {
					println(v.Error())
				}
				as.Error(err)
				wg.Done()
			}
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go client.ReadLoop()
		go server.ReadLoop()

		if item.Fin {
			server.WriteAsync(Opcode(item.Opcode), testCloneBytes(payload), nil)
		} else {
			testWrite(server, false, Opcode(item.Opcode), testCloneBytes(payload))
		}
		wg.Wait()
	}
}

func TestSegments(t *testing.T) {
	var as = assert.New(t)

	t.Run("valid segments", func(t *testing.T) {
		var wg = &sync.WaitGroup{}
		wg.Add(1)

		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{}
		var clientOption = &ClientOption{}

		var s1 = internal.AlphabetNumeric.Generate(16)
		var s2 = internal.AlphabetNumeric.Generate(16)
		serverHandler.onMessage = func(socket *Conn, message *Message) {
			as.Equal(string(s1)+string(s2), message.Data.String())
			wg.Done()
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		go func() {
			testWrite(client, false, OpcodeText, testCloneBytes(s1))
			testWrite(client, true, OpcodeContinuation, testCloneBytes(s2))
		}()
		wg.Wait()
	})

	t.Run("long segments", func(t *testing.T) {
		var wg = &sync.WaitGroup{}
		wg.Add(1)

		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{ReadMaxPayloadSize: 16}
		var clientOption = &ClientOption{}

		var s1 = internal.AlphabetNumeric.Generate(16)
		var s2 = internal.AlphabetNumeric.Generate(16)
		serverHandler.onClose = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		go func() {
			testWrite(client, false, OpcodeText, testCloneBytes(s1))
			testWrite(client, true, OpcodeContinuation, testCloneBytes(s2))
		}()
		wg.Wait()
	})

	t.Run("invalid segments", func(t *testing.T) {
		var wg = &sync.WaitGroup{}
		wg.Add(1)

		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{}
		var clientOption = &ClientOption{}

		var s1 = internal.AlphabetNumeric.Generate(16)
		var s2 = internal.AlphabetNumeric.Generate(16)
		serverHandler.onClose = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		go func() {
			testWrite(client, false, OpcodeText, testCloneBytes(s1))
			testWrite(client, true, OpcodeText, testCloneBytes(s2))
		}()
		wg.Wait()
	})

	t.Run("illegal compression", func(t *testing.T) {
		var wg = &sync.WaitGroup{}
		wg.Add(1)

		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{}
		var clientOption = &ClientOption{PermessageDeflate: PermessageDeflate{Enabled: true}}

		var s1 = internal.AlphabetNumeric.Generate(1024)
		serverHandler.onClose = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		go func() {
			testWrite(client, true, OpcodeText, testCloneBytes(s1))
		}()
		wg.Wait()
	})

	t.Run("decompress error", func(t *testing.T) {
		var wg = &sync.WaitGroup{}
		wg.Add(1)

		var serverHandler = new(webSocketMocker)
		var clientHandler = new(webSocketMocker)
		var serverOption = &ServerOption{PermessageDeflate: PermessageDeflate{Enabled: true}}
		var clientOption = &ClientOption{PermessageDeflate: PermessageDeflate{Enabled: true}}

		serverHandler.onClose = func(socket *Conn, err error) {
			as.Error(err)
			wg.Done()
		}

		server, client := newPeer(serverHandler, serverOption, clientHandler, clientOption)
		go server.ReadLoop()
		go client.ReadLoop()

		go func() {
			frame, _ := client.genFrame(OpcodeText, internal.Bytes(testdata), false)
			data := frame.Bytes()
			data[20] = 'x'
			client.conn.Write(data)
		}()
		wg.Wait()
	})
}

func TestMessage(t *testing.T) {
	var msg = &Message{
		Opcode: OpcodeText,
		Data:   bytes.NewBufferString("1234"),
	}
	_, _ = msg.Read(make([]byte, 2))
	msg.Close()
}

func TestFrameHeader_Parse(t *testing.T) {
	t.Run("", func(t *testing.T) {
		s, c := net.Pipe()
		c.Close()
		var fh = frameHeader{}
		var _, err = fh.Parse(s)
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		s, c := net.Pipe()
		go func() {
			h := frameHeader{}
			h.GenerateHeader(false, true, false, OpcodeText, 500)
			c.Write(h[:2])
			c.Close()
		}()

		time.Sleep(100 * time.Millisecond)
		var fh = frameHeader{}
		var _, err = fh.Parse(s)
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		s, c := net.Pipe()
		go func() {
			h := frameHeader{}
			h.GenerateHeader(false, true, false, OpcodeText, 1024*1024)
			c.Write(h[:2])
			c.Close()
		}()

		time.Sleep(100 * time.Millisecond)
		var fh = frameHeader{}
		var _, err = fh.Parse(s)
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		s, c := net.Pipe()
		go func() {
			h := frameHeader{}
			h.GenerateHeader(false, true, false, OpcodeText, 1024*1024)
			c.Write(h[:10])
			c.Close()
		}()

		time.Sleep(100 * time.Millisecond)
		var fh = frameHeader{}
		var _, err = fh.Parse(s)
		assert.Error(t, err)
	})
}

func TestConn_ReadMessage(t *testing.T) {
	t.Run("", func(t *testing.T) {
		var addr = ":" + nextPort()
		var serverHandler = &webSocketMocker{}
		serverHandler.onOpen = func(socket *Conn) {
			var p = []byte("123")
			frame, _ := socket.genFrame(OpcodePing, internal.Bytes(p), false)
			socket.conn.Write(frame.Bytes()[:2])
			socket.conn.Close()
		}
		var server = NewServer(serverHandler, nil)
		go server.Run(addr)

		time.Sleep(100 * time.Millisecond)
		client, _, err := NewClient(new(BuiltinEventHandler), &ClientOption{
			Addr: "ws://localhost" + addr,
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
			},
		})
		assert.NoError(t, err)
		client.ReadLoop()
	})

	t.Run("", func(t *testing.T) {
		var addr = ":" + nextPort()
		var serverHandler = &webSocketMocker{}
		serverHandler.onOpen = func(socket *Conn) {
			var p = []byte("123")
			frame, _ := socket.genFrame(OpcodeText, internal.Bytes(p), false)
			socket.conn.Write(frame.Bytes()[:2])
			socket.conn.Close()
		}
		var server = NewServer(serverHandler, nil)
		go server.Run(addr)

		time.Sleep(100 * time.Millisecond)
		client, _, err := NewClient(new(BuiltinEventHandler), &ClientOption{
			Addr: "ws://localhost" + addr,
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
			},
		})
		assert.NoError(t, err)
		client.ReadLoop()
	})
}
