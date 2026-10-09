package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Kohinor46/dbibackend-go/i18n"
)

// Version is the app version. scripts/release.sh reads it from here, so a
// release is made by changing this line.
const Version = "1.3.0"

const (
	releasesPage = "https://github.com/Kohinor46/dbibackend-go/releases/latest"
	releasesAPI  = "https://api.github.com/repos/Kohinor46/dbibackend-go/releases/latest"

	prefCheckUpdates = "checkUpdates"
)

// latestRelease asks GitHub for the newest release; tests replace it.
var latestRelease = func(ctx context.Context) (tag, page string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesAPI, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub: %s", resp.Status)
	}
	var r struct {
		Tag  string `json:"tag_name"`
		Page string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", "", err
	}
	return r.Tag, r.Page, nil
}

// newerVersion reports whether latest ("v1.4.0") is newer than current ("1.3.0").
func newerVersion(current, latest string) bool {
	parse := func(v string) (n [3]int, ok bool) {
		parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(v), "v"), ".")
		if len(parts) != 3 {
			return n, false
		}
		for i, p := range parts {
			x, err := strconv.Atoi(p)
			if err != nil {
				return n, false
			}
			n[i] = x
		}
		return n, true
	}
	c, ok1 := parse(current)
	l, ok2 := parse(latest)
	if !ok1 || !ok2 {
		return false
	}
	for i := range c {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// checkUpdate looks for a newer release in the background and, if there is
// one, shows a button next to Settings.
func (u *ui) checkUpdate() {
	if !u.app.Preferences().BoolWithFallback(prefCheckUpdates, true) {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		tag, page, err := latestRelease(ctx)
		if err != nil {
			u.log.Debug("Update check failed", "err", err)
			return
		}
		if !newerVersion(Version, tag) {
			return
		}
		if _, err := url.Parse(page); err != nil || !strings.HasPrefix(page, "https://github.com/") {
			page = releasesPage
		}
		u.log.Info(i18n.T("update.available", strings.TrimPrefix(tag, "v")), "current", Version, "page", page)
		runOnUI(func() {
			u.updateTag, u.updatePage = strings.TrimPrefix(tag, "v"), page
			u.build()
		})
	}()
}

// openUpdate opens the release page in the browser.
func (u *ui) openUpdate() {
	if p, err := url.Parse(u.updatePage); err == nil {
		u.app.OpenURL(p)
	}
}
