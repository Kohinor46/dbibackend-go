package gui

import (
	"bufio"
	"context"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// findSwitches looks for DBI's FTP server (SD card port) on the local
// networks; tests replace it.
var findSwitches = func(ctx context.Context) []string {
	return probeFTP(ctx, ftpCandidates(), ftpPortSD)
}

// ftpCandidates returns the hosts to probe: every address of the /24
// network around each private IPv4 address of an active interface, except
// the computer's own.
func ftpCandidates() []string {
	ifaces, _ := net.Interfaces()
	own := map[netip.Addr]bool{}
	nets := map[netip.Prefix]bool{}
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip, ok := netip.AddrFromSlice(ipn.IP.To4())
			if !ok || !ip.IsPrivate() {
				continue
			}
			own[ip] = true
			p, _ := ip.Prefix(24)
			nets[p] = true
		}
	}
	var hosts []string
	for p := range nets {
		for ip := p.Addr().Next(); p.Contains(ip); ip = ip.Next() {
			if b := ip.As4(); b[3] != 255 && !own[ip] {
				hosts = append(hosts, ip.String())
			}
		}
	}
	return hosts
}

// probeFTP returns, sorted, the hosts that accept a connection on port and
// greet like an FTP server ("220 ...").
func probeFTP(ctx context.Context, hosts []string, port int) []string {
	var (
		mu    sync.Mutex
		found []string
		wg    sync.WaitGroup
		slots = make(chan struct{}, 128)
	)
	d := net.Dialer{Timeout: 800 * time.Millisecond}
	for _, h := range hosts {
		select {
		case <-ctx.Done():
		case slots <- struct{}{}:
			wg.Add(1)
			go func() {
				defer func() { <-slots; wg.Done() }()
				c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(h, strconv.Itoa(port)))
				if err != nil {
					return
				}
				defer c.Close()
				c.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
				line, err := bufio.NewReader(c).ReadString('\n')
				if err == nil && strings.HasPrefix(line, "220") {
					mu.Lock()
					found = append(found, h)
					mu.Unlock()
				}
			}()
		}
	}
	wg.Wait()
	sort.Slice(found, func(i, j int) bool {
		a, _ := netip.ParseAddr(found[i])
		b, _ := netip.ParseAddr(found[j])
		return a.Less(b)
	})
	return found
}

// rememberHost puts host first in the list of recent addresses (at most 5).
func rememberHost(recent []string, host string) []string {
	out := []string{host}
	for _, h := range recent {
		if h != host && len(out) < 5 {
			out = append(out, h)
		}
	}
	return out
}
