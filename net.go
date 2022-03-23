package gowsps

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"sync"
)

// PacketDecoder structure of a function which decodes a packet
type PacketDecoder func(c *Connection)

// ErrorHandler function that takes in errors
type ErrorHandler func(err error)

// PacketSystem a structure for storing a list of handlers for different packet
// id values
type PacketSystem struct {
	Handlers     map[VarInt]PacketDecoder
	ErrorHandler ErrorHandler
}

type Packet struct {
	Id   VarInt
	Data any
}

func (s *PacketSystem) SetErrorHandler(handler ErrorHandler) {
	s.ErrorHandler = handler
}

// NewPacketSystem creates a new packet system and returns a handle to the
// newly created packet system
func NewPacketSystem() *PacketSystem {
	s := PacketSystem{
		Handlers: map[VarInt]PacketDecoder{},
	}
	return &s
}

// A global instance of a websocket upgrader to upgrade http connection
// to websockets
var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

// Connection structure for storing the current connection state as well
// as the reference to the websocket connection and write lock for
// preventing concurrently packet writes
type Connection struct {
	Lock        *sync.RWMutex
	Open        bool
	ReadBuffer  *PacketBuffer
	WriteBuffer *PacketBuffer

	*websocket.Conn
}

// Send function for sending packets to the client. Will only send if
// the connection is open. Acquires write locks before sending packet
func (conn *Connection) Send(packet Packet) {
	if conn.Open { // If the connection is open
		conn.Lock.Lock() // Acquire write lock
		err := MarshalPacket(conn.WriteBuffer, packet)
		if err == nil {
			_ = conn.WriteMessage(websocket.BinaryMessage, conn.WriteBuffer.Bytes())
		}
		conn.WriteBuffer.Reset()
		conn.Lock.Unlock() // Release write lock
	}
}

// UpgradeAndListen upgrades the provided http connection to a websocket connection and listens for packet
// data in a loop. Calls the provided callback function before starting the loop
func (s *PacketSystem) UpgradeAndListen(w http.ResponseWriter, r *http.Request, callback func(conn *Connection, err error)) {
	ws, err := upgrader.Upgrade(w, r, nil) // Upgrade the connection
	if err != nil {                        // If we couldn't upgrade the connection
		callback(nil, err) // Call the callback with the error
		return
	}

	// Create a new connection structure
	conn := &Connection{
		Open:        true,
		Lock:        &sync.RWMutex{},
		Conn:        ws,
		ReadBuffer:  NewPacketBuffer(),
		WriteBuffer: NewPacketBuffer(),
	}

	// When the websocket connection becomes closed
	ws.SetCloseHandler(func(code int, text string) error {
		// Set the connection open to false
		conn.Open = false
		return nil
	})

	// Deferred closing of the socket when we are done this will result
	// in ws.Close being called after this function is finished executing
	defer func(ws *websocket.Conn) { _ = ws.Close() }(ws)

	// Call the callback with the newly created connection
	callback(conn, nil)

	for conn.Open { // Loop infinitely as long as the connection is open
		err = s.DecodePacket(conn)               // Decode any incoming packets
		if err != nil && s.ErrorHandler != nil { // If we got an error and have a handler for errors
			s.ErrorHandler(err) // Call the error handler with the error
		}
	}
}

// AddHandler adds a new packet handling function to the packet system for
// packets that have the provided id. The handler function will be called
// with the packet data whenever one is received
func AddHandler[T any](s *PacketSystem, id VarInt, handler func(packet *T)) {
	s.Handlers[id] = func(c *Connection) { // Set the packet decoder for this ID
		out := new(T) // Create a new instance of the output type
		_ = UnMarshalPacket(c.ReadBuffer, out)
		handler(out)
	}
}

// DecodePacket handles decoding of any packets received by the packet system. Uses the connection
// and the ReadJSON function to take the incoming data and then calls the handler function. This
// function will return an error if it failed to decode the packet
func (s *PacketSystem) DecodePacket(c *Connection) error {
	t, m, err := c.ReadMessage()
	if err != nil {
		return err
	}
	if t != websocket.BinaryMessage {
		return nil
	}
	c.ReadBuffer.Buffer = bytes.NewBuffer(m)
	id, err := binary.ReadUvarint(c.ReadBuffer)
	if err != nil {
		return err
	}
	handler, exists := s.Handlers[VarInt(id)] // Retrieve a handler for the packet
	if !exists {                              // We don't have a packet handler for this packet
		return errors.New(fmt.Sprintf("No packet handler for packet %d", id))
	} else {
		handler(c) // Call the handler function
	}
	c.ReadBuffer.Buffer.Reset()
	return nil
}
