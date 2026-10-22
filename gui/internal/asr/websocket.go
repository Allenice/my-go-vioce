package asr

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
)

// A minimal RFC 6455 client: enough for one duplex task over HTTP/1.1, with
// the system proxy settings of net/http.

const (
	opContinuation = 0x0
	opText         = 0x1
	opBinary       = 0x2
	opClose        = 0x8
	opPing         = 0x9
	opPong         = 0xA
	// maxMessage bounds a server message; results are a few kilobytes.
	maxMessage = 1 << 20
)

// transport makes the upgrade request; HTTP/2 cannot upgrade, so it offers
// HTTP/1.1 only. Tests replace it.
var transport http.RoundTripper = &http.Transport{
	Proxy:           http.ProxyFromEnvironment,
	TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}},
}

type conn struct {
	rw     io.ReadWriteCloser
	r      *bufio.Reader
	write  sync.Mutex
	closed sync.Once
}

// bearer authorizes a request with an OAuth-style bearer token.
func bearer(key string) http.Header { return http.Header{"Authorization": {"Bearer " + key}} }

// dial upgrades a GET of url, carrying the auth headers, to a WebSocket.
func dial(ctx context.Context, url string, auth http.Header) (*conn, error) {
	url = strings.Replace(strings.Replace(url, "wss://", "https://", 1), "ws://", "http://", 1)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	var nonce [16]byte
	_, _ = rand.Read(nonce[:])
	challenge := base64.StdEncoding.EncodeToString(nonce[:])
	for k, v := range auth {
		req.Header[k] = v
	}
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", challenge)
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		defer resp.Body.Close()
		return nil, &StatusError{Code: resp.StatusCode, Reason: errorCode(io.LimitReader(resp.Body, 4096))}
	}
	rw, ok := resp.Body.(io.ReadWriteCloser)
	if !ok || !strings.EqualFold(resp.Header.Get("Upgrade"), "websocket") || resp.Header.Get("Sec-WebSocket-Accept") != accept(challenge) {
		resp.Body.Close()
		return nil, ErrProtocol
	}
	return &conn{rw: rw, r: bufio.NewReader(rw)}, nil
}

// errorCode is the code of an OpenAI-style error body.
func errorCode(r io.Reader) string {
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.NewDecoder(r).Decode(&body)
	return clip(body.Error.Code, 64)
}

func accept(challenge string) string {
	h := sha1.Sum([]byte(challenge + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h[:])
}

func (c *conn) WriteText(b []byte) error   { return c.writeFrame(opText, b, true) }
func (c *conn) WriteBinary(b []byte) error { return c.writeFrame(opBinary, b, true) }

// Close ends the connection without waiting for the server.
func (c *conn) Close() {
	c.closed.Do(func() {
		// Abort I/O immediately, including a writer blocked by a stalled peer.
		// A close-frame write here would take the same lock and defeat cancel.
		_ = c.rw.Close()
	})
}

func (c *conn) writeFrame(op byte, payload []byte, mask bool) error {
	c.write.Lock()
	defer c.write.Unlock()
	return writeFrame(c.rw, op, payload, mask)
}

// writeFrame writes one final frame; clients mask, servers do not.
func writeFrame(w io.Writer, op byte, payload []byte, mask bool) error {
	header := make([]byte, 2, 14)
	header[0] = 0x80 | op
	n := len(payload)
	switch {
	case n < 126:
		header[1] = byte(n)
	case n <= 0xFFFF:
		header[1] = 126
		header = binary.BigEndian.AppendUint16(header, uint16(n))
	default:
		header[1] = 127
		header = binary.BigEndian.AppendUint64(header, uint64(n))
	}
	body := payload
	if mask {
		header[1] |= 0x80
		var key [4]byte
		_, _ = rand.Read(key[:])
		header = append(header, key[:]...)
		body = make([]byte, n)
		for i, b := range payload {
			body[i] = b ^ key[i%4]
		}
	}
	_, err := w.Write(append(header, body...))
	return err
}

// ReadMessage returns the next text or binary message, answering pings;
// a close frame ends it with io.EOF.
func (c *conn) ReadMessage() (byte, []byte, error) {
	var op byte
	var message []byte
	for {
		fin, frameOp, payload, err := readFrame(c.r)
		if err != nil {
			return 0, nil, err
		}
		switch frameOp {
		case opPing:
			if err := c.writeFrame(opPong, payload, true); err != nil {
				return 0, nil, err
			}
			continue
		case opPong:
			continue
		case opClose:
			return 0, nil, io.EOF
		case opText, opBinary:
			if op != 0 {
				return 0, nil, ErrProtocol
			}
			op = frameOp
		case opContinuation:
			if op == 0 {
				return 0, nil, ErrProtocol
			}
		default:
			return 0, nil, ErrProtocol
		}
		if len(message)+len(payload) > maxMessage {
			return 0, nil, ErrProtocol
		}
		message = append(message, payload...)
		if fin {
			return op, message, nil
		}
	}
}

func readFrame(r *bufio.Reader) (bool, byte, []byte, error) {
	var head [2]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return false, 0, nil, err
	}
	fin, op := head[0]&0x80 != 0, head[0]&0x0F
	masked := head[1]&0x80 != 0
	n := uint64(head[1] & 0x7F)
	switch n {
	case 126:
		var b [2]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return false, 0, nil, err
		}
		n = uint64(binary.BigEndian.Uint16(b[:]))
	case 127:
		var b [8]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return false, 0, nil, err
		}
		n = binary.BigEndian.Uint64(b[:])
	}
	if n > maxMessage {
		return false, 0, nil, ErrProtocol
	}
	var key [4]byte
	if masked {
		if _, err := io.ReadFull(r, key[:]); err != nil {
			return false, 0, nil, err
		}
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return false, 0, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= key[i%4]
		}
	}
	return fin, op, payload, nil
}
