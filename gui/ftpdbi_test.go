package gui

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/spf13/afero"
)

// fakeDBI mimics DBI's FTP server as seen on a real console: its FEAT, no
// EPSV, MLSD that lists only the current folder (a path argument gets an
// empty listing), capitalised facts, and data connections that are reset
// rather than closed. dirs maps folders to their MLSD listing.
func fakeDBI(t *testing.T, dirs map[string]string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go serveFakeDBI(c, dirs)
		}
	}()
	return ln.Addr().String()
}

func serveFakeDBI(c net.Conn, dirs map[string]string) {
	defer c.Close()
	r := bufio.NewReader(c)
	say := func(s string) { fmt.Fprintf(c, "%s\r\n", s) }
	say("220 Service ready for new user.")
	var data net.Listener
	cwd := "/"
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd, arg, _ := strings.Cut(strings.TrimSpace(line), " ")
		switch strings.ToUpper(cmd) {
		case "CWD":
			if _, ok := dirs[arg]; ok {
				cwd = arg
				say("250 Requested file action okay, completed.")
			} else {
				say("550 Requested action not taken.")
			}
		case "USER":
			say("230 User logged in, proceed.")
		case "FEAT":
			say("211-\r\n MDTM\r\n MLST Type*;Size*;Modify*;Perm*;UNIX.mode;\r\n PASV\r\n SIZE\r\n TVFS\r\n UTF8\r\n211 End")
		case "TYPE", "OPTS", "NOOP":
			say("200 Command okay.")
		case "PASV":
			if data, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
				return
			}
			port := data.Addr().(*net.TCPAddr).Port
			say(fmt.Sprintf("227 Entering Passive Mode (127,0,0,1,%d,%d).", port/256, port%256))
		case "MLSD":
			say("150 File status okay; about to open data connection.")
			dc, err := data.Accept()
			data.Close()
			if err != nil {
				return
			}
			if arg == "" {
				dc.Write([]byte(dirs[cwd]))
			}
			time.Sleep(50 * time.Millisecond) // the client has read everything
			dc.(*net.TCPConn).SetLinger(0)    // close with a reset
			dc.Close()
			say("226 Closing data connection.")
		case "QUIT":
			say("221 Bye.")
			return
		default:
			say("502 Command not implemented.")
		}
	}
}

func TestFTPListDBI(t *testing.T) {
	addr := fakeDBI(t, map[string]string{
		"/": "Type=cdir;Modify=20251009113205;Perm=el;UNIX.mode=0777; /\r\n" +
			"Type=dir;Modify=20251009113205;Perm=el;UNIX.mode=0777; Nintendo\r\n" +
			"Type=file;Size=4294967296;Modify=;Perm=r;UNIX.mode=0644; Big Game [0100].nsp\r\n" +
			"type=file;size=12 no-final-semicolon.txt\r\n",
		"/Nintendo": "Type=dir;Modify=20251009113205;Perm=el;UNIX.mode=0777; Contents\r\n",
	})
	ctx := context.Background()
	f, err := dialFTP(ctx, addr, "", "", quietLog())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	list, err := f.List(ctx, "/")
	if err != nil {
		t.Fatal(err)
	}
	want := []rEntry{
		{ID: "/Nintendo", Name: "Nintendo", Dir: true},
		{ID: "/Big Game [0100].nsp", Name: "Big Game [0100].nsp", Size: 4294967296},
		{ID: "/no-final-semicolon.txt", Name: "no-final-semicolon.txt", Size: 12},
	}
	if fmt.Sprint(list) != fmt.Sprint(want) {
		t.Errorf("list = %+v\nwant %+v", list, want)
	}
	sub, err := f.List(ctx, "/Nintendo")
	if err != nil || len(sub) != 1 || sub[0] != (rEntry{ID: "/Nintendo/Contents", Name: "Contents", Dir: true}) {
		t.Errorf("/Nintendo = %+v, %v", sub, err)
	}
}

func TestParseMLSD(t *testing.T) {
	entries, bad := parseMLSD("/a", []byte("garbage line\n\nTYPE=DIR;sizd=0; b c\ntype=pdir; ..\n"))
	if len(entries) != 1 || entries[0] != (rEntry{ID: "/a/b c", Name: "b c", Dir: true}) {
		t.Errorf("entries = %+v", entries)
	}
	if len(bad) != 1 || bad[0] != "garbage line" {
		t.Errorf("bad = %q", bad)
	}
}

// stallingFTP accepts an upload, then stops answering without closing
// anything, like a Switch that went to sleep.
func stallingFTP(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	t.Cleanup(func() { close(done); ln.Close() })
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		r := bufio.NewReader(c)
		say := func(s string) { fmt.Fprintf(c, "%s\r\n", s) }
		say("220 ready")
		var data net.Listener
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			cmd, _, _ := strings.Cut(strings.TrimSpace(line), " ")
			switch strings.ToUpper(cmd) {
			case "USER":
				say("230 ok")
			case "FEAT":
				say("211 End")
			case "PASV":
				data, _ = net.Listen("tcp", "127.0.0.1:0")
				p := data.Addr().(*net.TCPAddr).Port
				say(fmt.Sprintf("227 Entering Passive Mode (127,0,0,1,%d,%d).", p/256, p%256))
			case "CWD":
				say("250 ok")
			case "STOR":
				say("150 ok")
				dc, err := data.Accept()
				if err == nil {
					defer dc.Close()
				}
				<-done // silence from now on
				return
			default:
				say("200 ok")
			}
		}
	}()
	return ln.Addr().String()
}

func shortFTPLimits(t *testing.T) {
	d, r, a := ftpDataIdle, ftpReplyWait, ftpAliveWait
	t.Cleanup(func() { ftpDataIdle, ftpReplyWait, ftpAliveWait = d, r, a })
	ftpDataIdle, ftpReplyWait, ftpAliveWait = 300*time.Millisecond, 600*time.Millisecond, 300*time.Millisecond
}

// A server that goes silent mid-upload is reported as a lost connection
// instead of hanging.
func TestFTPStalledUpload(t *testing.T) {
	shortFTPLimits(t)
	ctx := context.Background()
	f, err := dialFTP(ctx, stallingFTP(t), "", "", quietLog())
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = f.Upload(ctx, "/", "big.nsp", 64<<20, bytes.NewReader(make([]byte, 64<<20)))
	if !errors.Is(err, errFTPLost) {
		t.Errorf("err = %v, want errFTPLost", err)
	}
	if el := time.Since(start); el > 5*time.Second {
		t.Errorf("took %v", el)
	}
}

// slowReader gives its data in pieces with pauses.
type slowReader struct {
	left  int
	pause time.Duration
}

func (s *slowReader) Read(p []byte) (int, error) {
	if s.left == 0 {
		return 0, io.EOF
	}
	time.Sleep(s.pause)
	n := min(len(p), s.left, 32<<10)
	s.left -= n
	return n, nil
}

// A slow transfer that keeps moving is not cut, however long it takes.
func TestFTPSlowUploadNotCut(t *testing.T) {
	shortFTPLimits(t)
	d := &testFTPDriver{fs: afero.NewMemMapFs()}
	addr := startFTP(t, d)
	ctx := context.Background()
	f, err := dialFTP(ctx, addr, "", "", quietLog())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	size := 32 << 10 * 20 // 20 pieces, 100 ms apart: 2 s, well over the limits
	if _, err := f.Upload(ctx, "/", "slow.bin", int64(size), &slowReader{left: size, pause: 100 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	if st, err := d.fs.Stat("/slow.bin"); err != nil || st.Size() != int64(size) {
		t.Errorf("slow.bin: %v", err)
	}
	if _, err := f.List(ctx, "/"); err != nil {
		t.Errorf("connection unusable afterwards: %v", err)
	}
}
