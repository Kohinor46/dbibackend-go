package gui

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/textproto"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"
)

// errFTPLost marks failures of the connection itself (as opposed to the
// server refusing an operation): the session has to be reopened.
var errFTPLost = errors.New("FTP connection lost")

const ftpTimeout = 10 * time.Second

// ftpFS is a remoteFS over an FTP connection (DBI's "Run FTP server").
// IDs are absolute paths.
type ftpFS struct {
	conn    *ftp.ServerConn
	ctrl    net.Conn // the control connection under conn
	log     *slog.Logger
	listing listingRecorder
}

// dialFTP connects and logs in. An empty user logs in anonymously.
// Cancelling ctx aborts the connect.
func dialFTP(ctx context.Context, addr, user, pass string, log *slog.Logger) (*ftpFS, error) {
	d := net.Dialer{Timeout: ftpTimeout}
	ctrl, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { ctrl.Close() })
	defer stop()

	f := &ftpFS{ctrl: ctrl, log: log}
	first := true
	f.conn, err = ftp.Dial(addr,
		// The first connection is the control one, dialled above; data
		// connections go through the recorder: List parses what it saw.
		ftp.DialWithDialFunc(func(network, address string) (net.Conn, error) {
			if first {
				first = false
				return ctrl, nil
			}
			c, err := d.Dial(network, address)
			if err != nil {
				return nil, err
			}
			f.listing.opened()
			return recordingConn{c, &f.listing}, nil
		}),
		// DBI answers EPSV with "502 Command not implemented".
		ftp.DialWithDisabledEPSV(true),
	)
	if err == nil {
		if user == "" {
			user, pass = "anonymous", "anonymous"
		}
		if err = f.conn.Login(user, pass); err != nil {
			f.conn.Quit()
		}
	}
	if err != nil {
		ctrl.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	return f, nil
}

// Close ends the session.
func (f *ftpFS) Close() error { return f.conn.Quit() }

// do runs op; cancelling ctx closes the connection (FTP has no other way to
// stop a transfer). Errors that aren't server replies mean the connection is
// gone and are marked with errFTPLost, unless the server still answers: then
// only the data transfer failed.
func (f *ftpFS) do(ctx context.Context, op func() error) error {
	stop := context.AfterFunc(ctx, func() { f.conn.Quit() })
	err := op()
	stop()
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var reply *textproto.Error
	if errors.As(err, &reply) {
		return err // the server refused; the connection is fine
	}
	if f.alive() {
		return err
	}
	return fmt.Errorf("%w: %w", errFTPLost, err)
}

// alive reports whether the server answers on the control connection.
func (f *ftpFS) alive() bool {
	f.ctrl.SetDeadline(time.Now().Add(5 * time.Second))
	defer f.ctrl.SetDeadline(time.Time{})
	return f.conn.NoOp() == nil
}

func (f *ftpFS) List(ctx context.Context, dir string) (entries []rEntry, err error) {
	err = f.do(ctx, func() (err error) {
		entries, err = f.list(dir)
		return err
	})
	return entries, err
}

// list lists dir the way FileZilla does: CWD into it, then MLSD (or LIST)
// without an argument. DBI ignores a path given to MLSD and sends nothing.
func (f *ftpFS) list(dir string) ([]rEntry, error) {
	if err := f.conn.ChangeDir(dir); err != nil {
		return nil, err
	}
	f.listing.start()
	list, err := f.conn.List("")
	rec := f.listing.stop()
	if err != nil {
		// A server that resets the data connection instead of closing it
		// (DBI does) breaks the read, but the listing was complete if the
		// server then confirmed the transfer: the data connection was opened
		// and the control connection still answers (any reply but 226
		// would be a server error).
		var reply *textproto.Error
		if errors.As(err, &reply) || !rec.opened || !f.alive() {
			return nil, err
		}
		f.log.Debug("FTP listing ended with an error, using what was received", "dir", dir, "err", err, "bytes", len(rec.data))
	}

	var entries []rEntry
	if f.conn.IsTimePreciseInList() && !rec.truncated {
		// MLSD: parse it here, jlaffaye/ftp silently drops lines it can't read.
		var bad []string
		entries, bad = parseMLSD(dir, rec.data)
		if len(bad) > 0 {
			f.log.Warn("FTP listing has lines that could not be read", "dir", dir, "count", len(bad), "first", bad[0])
		}
	} else {
		for _, e := range list {
			if e.Name == "." || e.Name == ".." || e.Name == "" {
				continue
			}
			entries = append(entries, rEntry{
				ID:   path.Join(dir, e.Name),
				Name: e.Name,
				Dir:  e.Type == ftp.EntryTypeFolder,
				Size: int64(e.Size),
			})
		}
	}
	f.log.Debug("FTP listing", "dir", dir, "mlsd", f.conn.IsTimePreciseInList(), "bytes", len(rec.data), "entries", len(entries),
		"head", string(rec.data[:min(len(rec.data), 300)]))
	return entries, nil
}

// parseMLSD reads an MLSD listing (RFC 3659): a line per entry, facts and
// the name separated by a space: "type=file;size=1024;modify=...; name".
// It is lenient about case, a missing final ";" and unknown or malformed
// facts; lines without facts are returned in bad.
func parseMLSD(dir string, data []byte) (entries []rEntry, bad []string) {
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if line == "" {
			continue
		}
		facts, name, ok := strings.Cut(line, " ")
		if !ok || name == "" || !strings.Contains(facts, "=") {
			bad = append(bad, line)
			continue
		}
		e := rEntry{ID: path.Join(dir, name), Name: name}
		skip := name == "." || name == ".."
		for _, fact := range strings.Split(facts, ";") {
			key, value, _ := strings.Cut(fact, "=")
			switch strings.ToLower(key) {
			case "type":
				switch strings.ToLower(value) {
				case "dir":
					e.Dir = true
				case "cdir", "pdir": // the listed folder and its parent
					skip = true
				}
			case "size":
				e.Size, _ = strconv.ParseInt(value, 10, 64)
			}
		}
		if !skip {
			entries = append(entries, e)
		}
	}
	return entries, bad
}

// Like list, the other operations run in the folder they concern, on a
// bare name, as FileZilla does.

func (f *ftpFS) MakeDir(ctx context.Context, parent, name string) (string, error) {
	return path.Join(parent, name), f.do(ctx, func() error {
		if err := f.conn.ChangeDir(parent); err != nil {
			return err
		}
		return f.conn.MakeDir(name)
	})
}

func (f *ftpFS) Delete(ctx context.Context, e rEntry) error {
	return f.do(ctx, func() error { return f.remove(e) })
}

// remove deletes e, a folder with its contents. (jlaffaye/ftp's
// RemoveDirRecur passes a path to MLSD, which DBI doesn't support.)
func (f *ftpFS) remove(e rEntry) error {
	if e.Dir {
		children, err := f.list(e.ID)
		if err != nil {
			return err
		}
		for _, c := range children {
			if err := f.remove(c); err != nil {
				return err
			}
		}
	}
	if err := f.conn.ChangeDir(path.Dir(e.ID)); err != nil {
		return err
	}
	if e.Dir {
		return f.conn.RemoveDir(path.Base(e.ID))
	}
	return f.conn.Delete(path.Base(e.ID))
}

func (f *ftpFS) Upload(ctx context.Context, parent, name string, _ int64, r io.Reader) (string, error) {
	return path.Join(parent, name), f.do(ctx, func() error {
		if err := f.conn.ChangeDir(parent); err != nil {
			return err
		}
		return f.conn.Stor(name, r)
	})
}

func (f *ftpFS) Download(ctx context.Context, e rEntry, w io.Writer) error {
	return f.do(ctx, func() error {
		if err := f.conn.ChangeDir(path.Dir(e.ID)); err != nil {
			return err
		}
		resp, err := f.conn.Retr(path.Base(e.ID))
		if err != nil {
			return err
		}
		_, err = io.Copy(w, resp)
		if cerr := resp.Close(); err == nil {
			err = cerr // the server confirms the transfer on close
		}
		return err
	})
}

// maxListing caps what the recorder keeps; longer listings fall back to
// jlaffaye/ftp's own parsing.
const maxListing = 8 << 20

// listingRecorder keeps a copy of what data connections deliver while a
// listing runs. Transfers outside start/stop are not recorded.
type listingRecorder struct {
	mu  sync.Mutex
	on  bool
	res listingResult
}

type listingResult struct {
	data      []byte
	opened    bool // a data connection was opened
	truncated bool // longer than maxListing
}

func (r *listingRecorder) start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.on, r.res = true, listingResult{}
}

func (r *listingRecorder) stop() listingResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.on = false
	return r.res
}

func (r *listingRecorder) opened() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.on {
		r.res.opened = true
	}
}

func (r *listingRecorder) record(p []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case !r.on:
	case len(r.res.data)+len(p) > maxListing:
		r.res.truncated = true
	default:
		r.res.data = append(r.res.data, p...)
	}
}

// recordingConn is a data connection whose reads go to a listingRecorder.
type recordingConn struct {
	net.Conn
	rec *listingRecorder
}

func (c recordingConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	c.rec.record(p[:n])
	return n, err
}
