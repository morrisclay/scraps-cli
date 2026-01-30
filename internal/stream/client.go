// Package stream provides HTTP streaming client functionality.
package stream

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
)

// Client is an HTTP streaming client.
type Client struct {
	url        string
	apiKey     string
	OnMessage  func([]byte)
	OnError    func(error)
	OnClose    func()
	httpClient *http.Client
	cancel     context.CancelFunc
	done       chan struct{}
}

// NewClient creates a new streaming client.
func NewClient(url, apiKey string) *Client {
	return &Client{
		url:        url,
		apiKey:     apiKey,
		httpClient: &http.Client{},
		done:       make(chan struct{}),
	}
}

// Connect starts the streaming connection.
func (c *Client) Connect() error {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	req, err := http.NewRequestWithContext(ctx, "GET", c.url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	go c.readLoop(resp)
	return nil
}

// readLoop reads events from the stream.
func (c *Client) readLoop(resp *http.Response) {
	defer func() {
		resp.Body.Close()
		if c.OnClose != nil {
			c.OnClose()
		}
		close(c.done)
	}()

	reader := bufio.NewReader(resp.Body)
	var dataBuffer strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if c.OnError != nil {
				c.OnError(err)
			}
			return
		}

		line = strings.TrimSpace(line)

		// Empty line signals end of an event
		if line == "" {
			if dataBuffer.Len() > 0 {
				if c.OnMessage != nil {
					c.OnMessage([]byte(dataBuffer.String()))
				}
				dataBuffer.Reset()
			}
			continue
		}

		// SSE format: "data: {...}"
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)
			dataBuffer.WriteString(data)
		} else if strings.HasPrefix(line, "{") {
			// Plain JSON (newline-delimited)
			if c.OnMessage != nil {
				c.OnMessage([]byte(line))
			}
		}
	}
}

// Close closes the streaming connection.
func (c *Client) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

// Done returns a channel that's closed when the connection is closed.
func (c *Client) Done() <-chan struct{} {
	return c.done
}
