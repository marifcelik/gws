package main

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/marifcelik/gws"
)

var dir string

func init() {
	flag.StringVar(&dir, "d", "", "cert directory")
	flag.Parse()

	d, err := filepath.Abs(dir)
	if err != nil {
		log.Printf(err.Error())
		return
	}
	dir = d
}

func main() {
	srv := gws.NewServer(new(Websocket), nil)

	// wss://www.gws.com:8443/
	if err := srv.RunTLS(":8443", dir+"/server.crt", dir+"/server.pem"); err != nil {
		log.Panicln(err.Error())
	}
}

type Websocket struct {
	gws.BuiltinEventHandler
}

func (c *Websocket) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (c *Websocket) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	_ = socket.WriteMessage(message.Opcode, message.Bytes())
}
