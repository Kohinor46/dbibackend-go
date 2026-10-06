package dbi

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

// Conn is a bidirectional bulk transport to the Switch.
// Each Read/Write maps to a single USB transfer.
type Conn interface {
	Read(ctx context.Context, buf []byte) (int, error)
	Write(ctx context.Context, buf []byte) (int, error)
}

// maxFileRangeHeader bounds the FILE_RANGE payload the device may announce:
// 16 bytes of size/offset/name length plus the file name.
const maxFileRangeHeader = 16 + 4096

// Event is reported to Server.OnEvent while serving.
type Event interface{ isEvent() }

// ListEvent: DBI requested the title list.
type ListEvent struct{ Titles []Title }

// ProgressEvent: a chunk of Title was sent. Pos is the end of the chunk within the file.
type ProgressEvent struct {
	Title Title
	Pos   int64
	Bytes int // bytes in this chunk
}

// ExitEvent: DBI ended the session.
type ExitEvent struct{}

func (ListEvent) isEvent()     {}
func (ProgressEvent) isEvent() {}
func (ExitEvent) isEvent()     {}

// Server answers DBI commands for titles found in Dir.
type Server struct {
	Dir     string
	Log     *slog.Logger
	OnEvent func(Event)

	titles map[string]Title
	file   *os.File // last served title, kept open between ranges
	buf    []byte   // chunk buffer reused across ranges
	stats  stats
}

func (s *Server) emit(e Event) {
	if s.OnEvent != nil {
		s.OnEvent(e)
	}
}

// Serve runs the command loop until DBI sends EXIT (returns nil),
// ctx is cancelled, or the transport fails.
func (s *Server) Serve(ctx context.Context, c Conn) error {
	s.Log.Info("Entering command loop")
	s.stats = stats{start: time.Now()}
	defer func() {
		s.closeFile()
		s.stats.log(s.Log)
	}()
	buf := make([]byte, headerSize)
	for {
		n, err := c.Read(ctx, buf)
		if err != nil {
			return fmt.Errorf("read command: %w", err)
		}
		h, ok := ParseHeader(buf[:n])
		if !ok {
			continue
		}
		s.Log.Debug("Command", "type", h.Type, "id", h.ID, "size", h.DataSize)

		switch h.ID {
		case CmdExit:
			s.Log.Info("Exit")
			err := write(ctx, c, Header{TypeResponse, CmdExit, 0}.Marshal())
			s.emit(ExitEvent{})
			return err
		case CmdList:
			err = s.list(ctx, c)
		case CmdFileRange:
			err = s.fileRange(ctx, c, h.DataSize)
		default:
			s.Log.Warn("Unknown command", "id", h.ID)
			err := write(ctx, c, Header{TypeResponse, CmdExit, 0}.Marshal())
			s.emit(ExitEvent{})
			return err
		}
		if err != nil {
			return fmt.Errorf("%s: %w", h.ID, err)
		}
	}
}

func (s *Server) scan() ([]Title, error) {
	titles, err := ScanTitles(s.Dir)
	if err != nil {
		return nil, err
	}
	s.titles = make(map[string]Title, len(titles))
	for _, t := range titles {
		s.titles[t.Name] = t
	}
	return titles, nil
}

func (s *Server) list(ctx context.Context, c Conn) error {
	s.Log.Info("Get list")
	titles, err := s.scan()
	if err != nil {
		return err
	}
	var sb strings.Builder
	for _, t := range titles {
		s.Log.Debug("Title", "name", t.Name, "path", t.Path)
		sb.WriteString(t.Name)
		sb.WriteByte('\n')
	}
	payload := []byte(sb.String())

	if err := write(ctx, c, Header{TypeResponse, CmdList, uint32(len(payload))}.Marshal()); err != nil {
		return err
	}
	if err := readAck(ctx, c); err != nil {
		return err
	}
	if err := write(ctx, c, payload); err != nil {
		return err
	}
	s.Log.Info("Sent title list", "count", len(titles))
	s.emit(ListEvent{Titles: titles})
	return nil
}

func (s *Server) lookup(name string) (Title, bool) {
	if t, ok := s.titles[name]; ok {
		return t, true
	}
	// No LIST in this session yet, or the folder changed: rescan once.
	if _, err := s.scan(); err != nil {
		return Title{}, false
	}
	t, ok := s.titles[name]
	return t, ok
}

func (s *Server) fileRange(ctx context.Context, c Conn, dataSize uint32) error {
	if dataSize > maxFileRangeHeader {
		return fmt.Errorf("file range header too large: %d bytes", dataSize)
	}
	hsStart := time.Now()
	if err := write(ctx, c, Header{TypeAck, CmdFileRange, dataSize}.Marshal()); err != nil {
		return err
	}
	hdr := make([]byte, dataSize)
	n, err := c.Read(ctx, hdr)
	if err != nil {
		return err
	}
	fr, err := ParseFileRange(hdr[:n])
	if err != nil {
		return err
	}
	s.Log.Debug("File range", "name", fr.Name, "offset", fr.Offset, "size", fr.Size)

	// Only files from the titles directory may be served — never a path chosen by the device.
	t, ok := s.lookup(fr.Name)
	if !ok {
		return fmt.Errorf("requested unknown title %q", fr.Name)
	}
	f, err := s.open(t)
	if err != nil {
		return err
	}

	if err := write(ctx, c, Header{TypeResponse, CmdFileRange, fr.Size}.Marshal()); err != nil {
		return err
	}
	if err := readAck(ctx, c); err != nil {
		return err
	}

	start := time.Now()
	if err := s.sendRange(ctx, c, f, t, fr); err != nil {
		return err
	}
	end := time.Now()
	took := end.Sub(start)
	s.stats.addRange(int64(fr.Size), hsStart, start, end)
	s.Log.Debug("Range sent", "size", fr.Size, "took", took.Round(time.Microsecond), "MB/s", mbps(int64(fr.Size), took))
	return nil
}

// sendRange streams fr from f in BufferSegmentDataSize chunks. DBI requests
// ranges of at most 1 MB, so there is never more than one chunk to queue.
func (s *Server) sendRange(ctx context.Context, c Conn, f *os.File, t Title, fr FileRange) error {
	if s.buf == nil {
		s.buf = make([]byte, BufferSegmentDataSize)
	}
	r := io.NewSectionReader(f, int64(fr.Offset), int64(fr.Size))
	pos := int64(fr.Offset)
	for remaining := int64(fr.Size); remaining > 0; {
		chunk := s.buf[:min(remaining, int64(len(s.buf)))]
		if _, err := io.ReadFull(r, chunk); err != nil {
			return fmt.Errorf("read %s: %w", t.Path, err)
		}
		if err := write(ctx, c, chunk); err != nil {
			return err
		}
		remaining -= int64(len(chunk))
		pos += int64(len(chunk))
		s.emit(ProgressEvent{Title: t, Pos: pos, Bytes: len(chunk)})
	}
	return nil
}

// open returns the title's file, reusing the handle from the previous range.
func (s *Server) open(t Title) (*os.File, error) {
	if s.file != nil && s.file.Name() == t.Path {
		return s.file, nil
	}
	s.closeFile()
	f, err := os.Open(t.Path)
	if err != nil {
		return nil, err
	}
	s.file = f
	return f, nil
}

func (s *Server) closeFile() {
	if s.file != nil {
		s.file.Close()
		s.file = nil
	}
}

func readAck(ctx context.Context, c Conn) error {
	buf := make([]byte, headerSize)
	// Contents are not checked, same as the original backend.
	if _, err := c.Read(ctx, buf); err != nil {
		return fmt.Errorf("read ack: %w", err)
	}
	return nil
}

func write(ctx context.Context, c Conn, b []byte) error {
	n, err := c.Write(ctx, b)
	if err != nil {
		return err
	}
	if n != len(b) {
		return fmt.Errorf("short write: %d of %d bytes", n, len(b))
	}
	return nil
}
