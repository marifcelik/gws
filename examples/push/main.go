package main

import (
	"bufio"
	"log"
	"net"
	"net/http"

	"github.com/marifcelik/gws"
)

func main() {
	var app = gws.NewServer(new(Handler), nil)

	app.OnRequest = func(conn net.Conn, br *bufio.Reader, r *http.Request) {
		socket, err := app.GetUpgrader().UpgradeFromConn(conn, br, r)
		if err != nil {
			log.Print(err.Error())
			return
		}
		var channel = make(chan []byte, 8)
		var closer = make(chan struct{})
		socket.Session().Store("channel", channel)
		socket.Session().Store("closer", closer)
		go socket.ReadLoop()
		go func() {
			for {
				select {
				case p := <-channel:
					_ = socket.WriteMessage(gws.OpcodeText, p)
				case <-closer:
					return
				}
			}
		}()
	}

	log.Fatalf("%v", app.Run(":8000"))
}

type Handler struct {
	gws.BuiltinEventHandler
}

func (c *Handler) getSession(socket *gws.Conn, key string) any {
	v, _ := socket.Session().Load(key)
	return v
}

func (c *Handler) Send(socket *gws.Conn, payload []byte) {
	var channel = c.getSession(socket, "channel").(chan []byte)
	select {
	case channel <- payload:
	default:
		return
	}
}

func (c *Handler) OnClose(socket *gws.Conn, err error) {
	var closer = c.getSession(socket, "closer").(chan struct{})
	closer <- struct{}{}
}

func (c *Handler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	_ = socket.WriteMessage(message.Opcode, message.Bytes())
}
