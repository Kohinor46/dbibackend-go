package dbi

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeSwitch replays scripted device->host packets and records host->device writes.
type fakeSwitch struct {
	reads  [][]byte
	writes [][]byte
}

func (f *fakeSwitch) Read(_ context.Context, buf []byte) (int, error) {
	if len(f.reads) == 0 {
		return 0, io.EOF
	}
	p := f.reads[0]
	f.reads = f.reads[1:]
	if len(p) > len(buf) {
		return 0, errors.New("overflow")
	}
	return copy(buf, p), nil
}

func (f *fakeSwitch) Write(_ context.Context, buf []byte) (int, error) {
	f.writes = append(f.writes, append([]byte(nil), buf...))
	return len(buf), nil
}

func fileRangeReq(name string, off uint64, size uint32) []byte {
	b := make([]byte, 16+len(name))
	binary.LittleEndian.PutUint32(b[0:], size)
	binary.LittleEndian.PutUint64(b[4:], off)
	binary.LittleEndian.PutUint32(b[12:], uint32(len(name)))
	copy(b[16:], name)
	return b
}

func TestSession(t *testing.T) {
	sw := &fakeSwitch{}
	testSession(t, sw, &sw.writes)
}

type scripted interface {
	Conn
	script([][]byte)
}

func (f *fakeSwitch) script(r [][]byte) { f.reads = r }

func testSession(t *testing.T, sw scripted, writes *[][]byte) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	data := make([]byte, BufferSegmentDataSize+500)
	for i := range data {
		data[i] = byte(i * 7)
	}
	os.WriteFile(filepath.Join(dir, "sub", "Game.nsp"), data, 0o644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "Upd.XCI"), []byte("y"), 0o644)

	off, size := uint64(100), uint32(BufferSegmentDataSize+200)
	req := fileRangeReq("Game.nsp", off, size)
	ack := Header{TypeAck, 0, 0}.Marshal()
	sw.script([][]byte{
		{'J', 'U', 'N', 'K', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // ignored
		Header{TypeRequest, CmdList, 0}.Marshal(),
		ack,
		Header{TypeRequest, CmdFileRange, uint32(len(req))}.Marshal(),
		req,
		ack,
		Header{TypeRequest, CmdExit, 0}.Marshal(),
	})

	var events []Event
	s := &Server{Dir: dir, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), OnEvent: func(e Event) { events = append(events, e) }}
	if err := s.Serve(context.Background(), sw); err != nil {
		t.Fatal(err)
	}

	list := "Upd.XCI\nGame.nsp\n"
	if string((*writes)[1]) != list && string((*writes)[1]) != "Game.nsp\nUpd.XCI\n" {
		t.Fatalf("list payload = %q", (*writes)[1])
	}
	want := [][]byte{
		Header{TypeResponse, CmdList, uint32(len(list))}.Marshal(),
		(*writes)[1],
		Header{TypeAck, CmdFileRange, uint32(len(req))}.Marshal(),
		Header{TypeResponse, CmdFileRange, size}.Marshal(),
		data[off : off+BufferSegmentDataSize],
		data[off+BufferSegmentDataSize : off+uint64(size)],
		Header{TypeResponse, CmdExit, 0}.Marshal(),
	}
	if len((*writes)) != len(want) {
		t.Fatalf("got %d writes, want %d", len((*writes)), len(want))
	}
	for i := range want {
		if !bytes.Equal((*writes)[i], want[i]) {
			t.Errorf("write %d mismatch (len %d vs %d)", i, len((*writes)[i]), len(want[i]))
		}
	}
	last := events[len(events)-2].(ProgressEvent)
	if last.Pos != int64(off)+int64(size) {
		t.Errorf("last progress pos = %d", last.Pos)
	}
	if _, ok := events[len(events)-1].(ExitEvent); !ok {
		t.Errorf("last event = %T", events[len(events)-1])
	}
}

func TestFileRangeRejectsUnknownPath(t *testing.T) {
	dir := t.TempDir()
	req := fileRangeReq("../../etc/passwd", 0, 10)
	sw := &fakeSwitch{reads: [][]byte{
		Header{TypeRequest, CmdFileRange, uint32(len(req))}.Marshal(),
		req,
	}}
	s := &Server{Dir: dir, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err := s.Serve(context.Background(), sw); err == nil {
		t.Fatal("expected error for unknown title")
	}
}

// A device announcing a huge FILE_RANGE header must not make us allocate it.
func TestFileRangeHeaderTooLarge(t *testing.T) {
	sw := &fakeSwitch{reads: [][]byte{Header{TypeRequest, CmdFileRange, 0xFFFFFFFF}.Marshal()}}
	s := &Server{Dir: t.TempDir(), Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	err := s.Serve(context.Background(), sw)
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("err = %v", err)
	}
	if len(sw.writes) != 0 {
		t.Errorf("wrote %d packets before rejecting", len(sw.writes))
	}
}

// Idle time after the last range must not lower the reported speed.
func TestStatsExcludeIdle(t *testing.T) {
	var s stats
	t0 := time.Now()
	s.start = t0.Add(-10 * time.Second)
	s.addRange(1<<20, t0, t0.Add(10*time.Millisecond), t0.Add(100*time.Millisecond))
	s.addRange(1<<20, t0.Add(200*time.Millisecond), t0.Add(210*time.Millisecond), t0.Add(300*time.Millisecond))
	if got := s.last.Sub(s.first); got != 300*time.Millisecond {
		t.Fatalf("active = %v", got)
	}
	if got := mbps(s.bytes, s.last.Sub(s.first)); got != "6.7" {
		t.Errorf("avg = %s MB/s, want 6.7", got)
	}
	if s.handshake != 20*time.Millisecond || s.sending != 180*time.Millisecond {
		t.Errorf("handshake=%v sending=%v", s.handshake, s.sending)
	}
}
