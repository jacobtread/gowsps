package gowsps

import (
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/mitchellh/mapstructure"
	"net/http"
	"sync"
)

// PacketDecoder structure of a function which decodes a packet
type PacketDecoder func(packet *Packet)

// ErrorHandler function that takes in errors
type ErrorHandler func(err error)

// Packet structure of packets that can be sent or received
type Packet struct {
	Id   int `json:"id"`
	Data any `json:"data"`
}

// PacketSystem a structure for storing a list of handlers for different packet
// id values
type PacketSystem struct {
	Handlers     map[int]PacketDecoder
	ErrorHandler ErrorHandler
}

func (s *PacketSystem) SetErrorHandler(handler ErrorHandler) {
	s.ErrorHandler = handler
}

// NewPacketSystem creates a new packet system and returns a handle to the
// newly created packet system
func NewPacketSystem() *PacketSystem {
	s := PacketSystem{
		Handlers: map[int]PacketDecoder{},
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
	Lock *sync.RWMutex
	Open bool

	*websocket.Conn
}

// Send function for sending packets to the client. Will only send if
// the connection is open. Acquires write locks before sending packet
func (conn *Connection) Send(packet Packet) {
	if conn.Open { // If the connection is open
		conn.Lock.Lock()           // Acquire write lock
		_ = conn.WriteJSON(packet) // Write the packet data as JSON
		conn.Lock.Unlock()         // Release write lock
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
		Open: true,
		Lock: &sync.RWMutex{},
		Conn: ws,
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

	p := &Packet{} // Empty packet instance used in decode

	for conn.Open { // Loop infinitely as long as the connection is open
		err = s.DecodePacket(p, conn)            // Decode any incoming packets
		if err != nil && s.ErrorHandler != nil { // If we got an error and have a handler for errors
			s.ErrorHandler(err) // Call the error handler with the error
		}
	}
}

// AddHandler adds a new packet handling function to the packet system for
// packets that have the provided id. The handler function will be called
// with the packet data whenever one is received
func AddHandler[T any](s *PacketSystem, id int, handler func(packet *T)) {
	s.Handlers[id] = func(packet *Packet) { // Set the packet decoder for this ID
		out := new(T)                                // Create a new instance of the output type
		err := mapstructure.Decode(packet.Data, out) // Decode the packet data into the struct
		if err == nil {                              // If didn't encounter an error
			handler(out) // Call the packet handler with the packet data struct
		}
	}
}

// DecodePacket handles decoding of any packets received by the packet system. Uses the connection
// and the ReadJSON function to take the incoming data and then calls the handler function. This
// function will return an error if it failed to decode the packet
func (s *PacketSystem) DecodePacket(p *Packet, c *Connection) error {
	err := c.ReadJSON(&p) // Read the packet into the packet struct
	if err != nil {       // If we encountered a JSON error
		if c.Open { // Ignore read errors if the connection is not open
			return err
		}
	} else {
		id := p.Id                        // Retrieve the ID of the packet
		handler, exists := s.Handlers[id] // Retrieve a handler for the packet
		if !exists {                      // We don't have a packet handler for this packet
			return errors.New(fmt.Sprintf("No packet handler for packet %d", id))
		} else {
			handler(p) // Call the handler function
		}
	}
	return nil
}
