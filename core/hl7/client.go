package hl7

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"time"
)

// MLLPClient sends outbound HL7 v2 messages to a remote MLLP listener.
// It opens a new TCP connection per Send call; persistent connection pooling
// is not required because outbound prescription dispatch is infrequent.
type MLLPClient struct {
	addr    string
	timeout time.Duration
}

// NewMLLPClient constructs an MLLPClient targeting addr (e.g. "192.168.1.10:2575").
// timeout is applied to both the TCP dial and the write+read-ACK round trip.
func NewMLLPClient(addr string, timeout time.Duration) (*MLLPClient, error) {
	if addr == "" {
		return nil, fmt.Errorf("mllp client: addr must not be empty")
	}
	return &MLLPClient{addr: addr, timeout: timeout}, nil
}

// Send wraps msg in an MLLP frame, dials the remote listener, sends the frame,
// and waits for the ACK. Returns an error if the ACK code is AE or AR.
func (c *MLLPClient) Send(ctx context.Context, msg string) error {
	deadline := time.Now().Add(c.timeout)
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return fmt.Errorf("mllp client: dial %s: %w", c.addr, err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(deadline)

	if _, err := conn.Write(WrapMLLP(msg)); err != nil {
		return fmt.Errorf("mllp client: send to %s: %w", c.addr, err)
	}

	// Read the ACK frame.
	reader := bufio.NewReader(conn)
	ackRaw, err := readMLLPFrame(reader)
	if err != nil {
		return fmt.Errorf("mllp client: read ACK from %s: %w", c.addr, err)
	}

	ack, err := Parse(ackRaw)
	if err != nil {
		return fmt.Errorf("mllp client: parse ACK: %w", err)
	}

	// MSA-1 holds the acknowledgement code.
	ackCode := ack.GetField("MSA", "1")
	if ackCode == "AE" || ackCode == "AR" {
		return fmt.Errorf("mllp client: negative ACK %s from %s", ackCode, c.addr)
	}
	return nil
}
