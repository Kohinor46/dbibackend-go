package mtp

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
)

// Conn is the bulk IN/OUT pipe pair of the device's MTP interface.
// Each Read/Write is one USB transfer; a zero-length Write sends a
// zero-length packet.
type Conn interface {
	Read(ctx context.Context, buf []byte) (int, error)
	Write(ctx context.Context, buf []byte) (int, error)
}

// PacketSizer is optionally implemented by a Conn to report the bulk OUT
// endpoint's max packet size (needed to know when to send a zero-length packet).
type PacketSizer interface {
	MaxPacketSize() int
}

// ErrBroken is returned after a transfer failed midway: the device's state is
// unknown and the connection has to be reopened.
var ErrBroken = errors.New("MTP connection lost; reconnect the device")

const chunkSize = 1 << 20

// containerLimit is the largest container length that fits the 32-bit
// header field; tests lower it to exercise the "unknown length" path.
var containerLimit uint64 = maxU32Length - 1

// Client talks to one MTP device. Its methods are safe for concurrent use;
// transactions run one at a time.
type Client struct {
	mu        sync.Mutex
	conn      Conn
	maxPacket int
	tid       uint32
	buf       []byte // transfer buffer for reads and writes
	pending   []byte // bytes read from the device but not consumed yet
	lastShort bool   // the last read ended with a short packet
	broken    error
	info      *DeviceInfo
}

// NewClient wraps conn; call Open before anything else.
func NewClient(conn Conn) *Client {
	c := &Client{conn: conn, maxPacket: 512, buf: make([]byte, chunkSize)}
	if ps, ok := conn.(PacketSizer); ok && ps.MaxPacketSize() > 0 {
		c.maxPacket = ps.MaxPacketSize()
	}
	return c
}

// Open starts a session and reads the device info.
func (c *Client) Open(ctx context.Context) error {
	c.mu.Lock()
	c.tid = 0 // OpenSession uses transaction 0; the session then counts from 1
	_, err := c.transact(ctx, OpOpenSession, []uint32{1}, nil, nil, -1)
	c.mu.Unlock()
	var re *RespError
	if err != nil && !(errors.As(err, &re) && re.Code == RespSessionAlreadyOpen) {
		return err
	}
	var data byteSink
	if _, err := c.run(ctx, OpGetDeviceInfo, nil, nil, &data, -1); err != nil {
		return err
	}
	info, err := unmarshalDeviceInfo(data.b)
	if err != nil {
		return fmt.Errorf("device info: %w", err)
	}
	c.mu.Lock()
	c.info = info
	c.mu.Unlock()
	return nil
}

// Close ends the session.
func (c *Client) Close(ctx context.Context) error {
	_, err := c.run(ctx, OpCloseSession, nil, nil, nil, -1)
	return err
}

// Info returns the device info read by Open.
func (c *Client) Info() *DeviceInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.info
}

// dataOut is the payload of a host-to-device data phase.
type dataOut struct {
	r    io.Reader
	size uint64
}

func (c *Client) run(ctx context.Context, op uint16, params []uint32, out *dataOut, in io.Writer, inSize int64) ([]uint32, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.transact(ctx, op, params, out, in, inSize)
}

// transact runs one MTP transaction: command, optional data phase (out to
// the device, or in from it into `in`; inSize is the expected payload size
// when known, else -1), and the response. Must hold c.mu.
func (c *Client) transact(ctx context.Context, op uint16, params []uint32, out *dataOut, in io.Writer, inSize int64) ([]uint32, error) {
	if c.broken != nil {
		return nil, ErrBroken
	}
	if op != OpOpenSession {
		c.tid++
	}
	resp, err := c.exchange(ctx, op, params, out, in, inSize)
	var re *RespError
	if err != nil && !errors.As(err, &re) {
		// An I/O error or cancellation midway leaves the device mid-transaction.
		c.broken = err
	}
	return resp, err
}

func (c *Client) exchange(ctx context.Context, op uint16, params []uint32, out *dataOut, in io.Writer, inSize int64) ([]uint32, error) {
	cmd := make([]byte, headerSize, headerSize+4*len(params))
	putHeader(cmd, uint32(headerSize+4*len(params)), containerCommand, op, c.tid)
	for _, p := range params {
		cmd = binary.LittleEndian.AppendUint32(cmd, p)
	}
	if err := c.write(ctx, cmd); err != nil {
		return nil, fmt.Errorf("send command: %w", err)
	}
	if out != nil {
		if err := c.sendData(ctx, op, out); err != nil {
			return nil, fmt.Errorf("send data: %w", err)
		}
	}

	for {
		typ, code, length, err := c.readHeader(ctx)
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
		switch typ {
		case containerData:
			if err := c.receiveData(ctx, length, in, inSize); err != nil {
				return nil, fmt.Errorf("receive data: %w", err)
			}
		case containerResponse:
			if length < headerSize || length > headerSize+4*5 {
				return nil, fmt.Errorf("bad response length %d", length)
			}
			raw, err := c.take(ctx, int(length-headerSize))
			if err != nil {
				return nil, err
			}
			resp := make([]uint32, len(raw)/4)
			for i := range resp {
				resp[i] = binary.LittleEndian.Uint32(raw[i*4:])
			}
			if code != RespOK {
				return resp, &RespError{Op: op, Code: code}
			}
			return resp, nil
		default: // events don't belong on the bulk pipe; skip
			if length >= headerSize && length != lengthUnknown {
				if _, err := c.take(ctx, int(length-headerSize)); err != nil {
					return nil, err
				}
			}
		}
	}
}

func putHeader(b []byte, length uint32, typ, code uint16, tid uint32) {
	binary.LittleEndian.PutUint32(b[0:], length)
	binary.LittleEndian.PutUint16(b[4:], typ)
	binary.LittleEndian.PutUint16(b[6:], code)
	binary.LittleEndian.PutUint32(b[8:], tid)
}

// sendData sends one data container: header plus out.size bytes from out.r.
// Containers over 4 GB get lengthUnknown and end with a short packet.
func (c *Client) sendData(ctx context.Context, op uint16, out *dataOut) error {
	total := headerSize + out.size
	length := uint32(total)
	if total > containerLimit {
		length = lengthUnknown
	}
	buf := c.buf
	putHeader(buf, length, containerData, op, c.tid)
	n := headerSize
	remaining := out.size
	for {
		take := min(remaining, uint64(len(buf)-n))
		if _, err := io.ReadFull(out.r, buf[n:n+int(take)]); err != nil {
			return err
		}
		n += int(take)
		remaining -= take
		if err := c.write(ctx, buf[:n]); err != nil {
			return err
		}
		if remaining == 0 {
			break
		}
		n = 0
	}
	// A transfer that is a multiple of the packet size has no short packet
	// to mark its end, so terminate it with a zero-length packet.
	if total%uint64(c.maxPacket) == 0 {
		return c.write(ctx, nil)
	}
	return nil
}

func (c *Client) write(ctx context.Context, b []byte) error {
	n, err := c.conn.Write(ctx, b)
	if err == nil && n != len(b) {
		err = fmt.Errorf("short write: %d of %d bytes", n, len(b))
	}
	return err
}

// fill reads one transfer from the device and appends it to c.pending.
func (c *Client) fill(ctx context.Context) error {
	n, err := c.conn.Read(ctx, c.buf)
	if err != nil {
		return err
	}
	c.pending = append(c.pending, c.buf[:n]...)
	c.lastShort = n < len(c.buf)
	return nil
}

// take returns the next n bytes of the stream from the device.
func (c *Client) take(ctx context.Context, n int) ([]byte, error) {
	for len(c.pending) < n {
		if err := c.fill(ctx); err != nil {
			return nil, err
		}
	}
	b := c.pending[:n:n]
	c.pending = c.pending[n:]
	return b, nil
}

func (c *Client) readHeader(ctx context.Context) (typ, code uint16, length uint32, err error) {
	h, err := c.take(ctx, headerSize)
	if err != nil {
		return 0, 0, 0, err
	}
	return binary.LittleEndian.Uint16(h[4:]), binary.LittleEndian.Uint16(h[6:]),
		binary.LittleEndian.Uint32(h[0:]), nil
}

// receiveData streams a data container's payload into w (nil discards it).
// With lengthUnknown the payload size comes from inSize, or, if that is
// unknown too, the data ends with the first short packet.
func (c *Client) receiveData(ctx context.Context, length uint32, w io.Writer, inSize int64) error {
	if w == nil {
		w = io.Discard
	}
	if length != lengthUnknown {
		if length < headerSize {
			return fmt.Errorf("bad data length %d", length)
		}
		return c.copyN(ctx, w, int64(length-headerSize))
	}
	if inSize >= 0 {
		return c.copyN(ctx, w, inSize)
	}
	for {
		if len(c.pending) > 0 {
			if _, err := w.Write(c.pending); err != nil {
				return err
			}
			c.pending = c.pending[:0]
		}
		if c.lastShort {
			return nil
		}
		if err := c.fill(ctx); err != nil {
			return err
		}
	}
}

func (c *Client) copyN(ctx context.Context, w io.Writer, n int64) error {
	for n > 0 {
		if len(c.pending) == 0 {
			if err := c.fill(ctx); err != nil {
				return err
			}
			continue
		}
		k := min(int64(len(c.pending)), n)
		if _, err := w.Write(c.pending[:k]); err != nil {
			return err
		}
		c.pending = c.pending[k:]
		n -= k
	}
	return nil
}

// byteSink collects a small data phase in memory.
type byteSink struct{ b []byte }

func (s *byteSink) Write(p []byte) (int, error) {
	s.b = append(s.b, p...)
	return len(p), nil
}
