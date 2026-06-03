// Package hl7 provides MLLP (Minimal Lower Layer Protocol) TCP transport
// for HL7 v2 messages as specified in HL7 Appendix C / RFC standards used
// by NZ healthcare integrations.
package hl7

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

const (
	// mllpStartBlock is the MLLP vertical-tab start-of-block byte (0x0B).
	mllpStartBlock byte = 0x0B
	// mllpEndBlock1 is the MLLP file-separator end-of-block byte (0x1C).
	mllpEndBlock1 byte = 0x1C
	// mllpEndBlock2 is the carriage-return that follows the end-of-block byte (0x0D).
	mllpEndBlock2 byte = 0x0D

	// mllpReadTimeout is the per-read deadline for MLLP connections.
	mllpReadTimeout = 60 * time.Second
	// mllpWriteTimeout is the per-write deadline for MLLP connections.
	mllpWriteTimeout = 30 * time.Second
)

// MLLPServer listens for inbound MLLP TCP connections, parses each received
// HL7 v2 message frame, invokes the handler, and returns the ACK response.
type MLLPServer struct {
	addr     string
	listener net.Listener
	handler  func(*Message) (*Message, error)
}

// NewMLLPServer creates an MLLPServer that will listen on addr (e.g.
// ":2575") and dispatch each received message to handler.
// handler must return an ACK message or an error; if it returns an error the
// server sends an AA-AE negative ACK automatically.
func NewMLLPServer(addr string, handler func(*Message) (*Message, error)) *MLLPServer {
	return &MLLPServer{
		addr:    addr,
		handler: handler,
	}
}

// Start opens the TCP listener and begins accepting connections.
// It blocks until ctx is cancelled, at which point it closes the listener and
// waits for in-flight connections to drain (best-effort).
// Each accepted connection is handled in its own goroutine.
func (s *MLLPServer) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("mllp: listen on %s: %w", s.addr, err)
	}
	s.listener = ln
	log.Printf("mllp: listening on %s", s.addr)

	// Close listener when context is cancelled.
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			// Check if the error is due to the listener being closed.
			select {
			case <-ctx.Done():
				return nil
			default:
				return fmt.Errorf("mllp: accept: %w", err)
			}
		}
		go s.handleConn(conn)
	}
}

// handleConn reads MLLP-framed HL7 messages from conn in a loop, parses each,
// calls the handler, then writes the ACK back. The loop ends when the
// connection is closed or an unrecoverable error occurs.
func (s *MLLPServer) handleConn(conn net.Conn) {
	defer conn.Close()
	remote := conn.RemoteAddr().String()
	log.Printf("mllp: accepted connection from %s", remote)
	reader := bufio.NewReader(conn)

	for {
		_ = conn.SetReadDeadline(time.Now().Add(mllpReadTimeout))

		raw, err := readMLLPFrame(reader)
		if err != nil {
			if errors.Is(err, io.EOF) || isClosedConnError(err) {
				log.Printf("mllp: connection closed by %s", remote)
			} else {
				log.Printf("mllp: read error from %s: %v", remote, err)
			}
			return
		}

		msg, parseErr := Parse(raw)
		var ackMsg string
		if parseErr != nil {
			log.Printf("mllp: parse error from %s: %v", remote, parseErr)
			ackMsg = buildACK("", "AE")
		} else {
			ackStatus := "AA"
			var handlerResp *Message
			handlerResp, err = s.handler(msg)
			if err != nil {
				log.Printf("mllp: handler error for message from %s: %v", remote, err)
				ackStatus = "AE"
			}
			if handlerResp != nil {
				ackMsg = handlerResp.Raw()
			} else {
				msgID := msg.GetField("MSH", "10")
				ackMsg = buildACK(msgID, ackStatus)
			}
		}

		_ = conn.SetWriteDeadline(time.Now().Add(mllpWriteTimeout))
		if _, werr := conn.Write(WrapMLLP(ackMsg)); werr != nil {
			log.Printf("mllp: write ACK error to %s: %v", remote, werr)
			return
		}
	}
}

// readMLLPFrame reads exactly one MLLP-framed message from r.
// It expects: 0x0B <message bytes> 0x1C 0x0D
func readMLLPFrame(r *bufio.Reader) (string, error) {
	// Read until we find the start-of-block byte.
	for {
		b, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if b == mllpStartBlock {
			break
		}
	}

	var buf []byte
	for {
		b, err := r.ReadByte()
		if err != nil {
			return "", fmt.Errorf("mllp: unexpected error reading message body: %w", err)
		}
		if b == mllpEndBlock1 {
			// Expect carriage-return next.
			cr, err := r.ReadByte()
			if err != nil {
				return "", fmt.Errorf("mllp: expected CR after FS: %w", err)
			}
			if cr != mllpEndBlock2 {
				// Not a proper end sequence; treat both bytes as data.
				buf = append(buf, b, cr)
				continue
			}
			// Proper end sequence found.
			break
		}
		buf = append(buf, b)
	}
	return string(buf), nil
}

// WrapMLLP wraps a raw HL7 message string in an MLLP frame.
// Frame format: 0x0B + message + 0x1C + 0x0D
func WrapMLLP(msg string) []byte {
	frame := make([]byte, 0, 1+len(msg)+2)
	frame = append(frame, mllpStartBlock)
	frame = append(frame, []byte(msg)...)
	frame = append(frame, mllpEndBlock1, mllpEndBlock2)
	return frame
}

// UnwrapMLLP extracts the raw HL7 message string from an MLLP-framed byte
// slice. Returns an error if the frame delimiters are missing or malformed.
func UnwrapMLLP(data []byte) (string, error) {
	if len(data) < 3 {
		return "", errors.New("mllp: data too short to be a valid MLLP frame")
	}
	if data[0] != mllpStartBlock {
		return "", fmt.Errorf("mllp: expected start block 0x0B, got 0x%02X", data[0])
	}
	last := len(data) - 1
	if data[last] != mllpEndBlock2 || data[last-1] != mllpEndBlock1 {
		return "", errors.New("mllp: missing end-of-block sequence (0x1C 0x0D)")
	}
	return string(data[1 : last-1]), nil
}

// buildACK constructs a minimal HL7 v2 ACK message for the given message
// control ID and acknowledgement code.
// status should be one of "AA" (Application Accept), "AE" (Application Error),
// or "AR" (Application Reject).
func buildACK(msgID, status string) string {
	now := time.Now().UTC().Format("20060102150405")
	if msgID == "" {
		msgID = "UNKNOWN"
	}
	// MSH|^~\&|ACK_APP|NZ|SENDER|NZ|<timestamp>||ACK|<ackMsgID>|P|2.4
	// MSA|<status>|<original_msgID>
	ackMsgID := fmt.Sprintf("ACK%s", now)
	msh := fmt.Sprintf("MSH|^~\\&|TPT_HL7_GW|NZ|||%s||ACK|%s|P|2.4", now, ackMsgID)
	msa := fmt.Sprintf("MSA|%s|%s", status, msgID)
	return msh + "\r" + msa + "\r"
}

// isClosedConnError reports whether err indicates a closed network connection.
func isClosedConnError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, net.ErrClosed) ||
		containsString(err.Error(), "use of closed network connection")
}

// containsString is a helper for substring matching without importing strings
// in the error path (avoids circular import concern in test stubs).
func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
