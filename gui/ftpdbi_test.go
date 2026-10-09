package gui

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
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
