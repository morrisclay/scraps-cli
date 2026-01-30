// Package ws provides WebSocket client functionality.
package ws

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
)

// Client is a WebSocket client.
type Client struct {
	conn      *websocket.Conn
	url       string
	OnMessage func([]byte)
	OnError   func(error)
	OnClose   func()
	done      chan struct{}
}

// NewClient creates a new WebSocket client.
func NewClient(url string) *Client {
	return &Client{
		url:  url,
		done: make(chan struct{}),
	}
}

// Connect establishes the WebSocket connection.
func (c *Client) Connect() error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(c.url, nil)
	if err != nil {
		return err
	}

	c.conn = conn
	go c.readLoop()
	return nil
}

// readLoop reads messages from the WebSocket.
func (c *Client) readLoop() {
	defer func() {
		if c.OnClose != nil {
			c.OnClose()
		}
		close(c.done)
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if c.OnError != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				c.OnError(err)
			}
			return
		}

		if c.OnMessage != nil {
			c.OnMessage(message)
		}
	}
}

// Send sends a message over the WebSocket.
func (c *Client) Send(data []byte) error {
	if c.conn == nil {
		return websocket.ErrCloseSent
	}
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// SendJSON sends a JSON message over the WebSocket.
func (c *Client) SendJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.Send(data)
}

// Close closes the WebSocket connection.
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}

	// Send close message
	err := c.conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)
	if err != nil {
		return c.conn.Close()
	}

	// Wait for read loop to finish or timeout
	select {
	case <-c.done:
	case <-time.After(time.Second):
	}

	return c.conn.Close()
}

// Done returns a channel that's closed when the connection is closed.
func (c *Client) Done() <-chan struct{} {
	return c.done
}

// IsConnected returns whether the client is connected.
func (c *Client) IsConnected() bool {
	return c.conn != nil
}
