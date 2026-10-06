//go:build !darwin

package gui

const canRelease = false

func runningHolders() []string { return nil }

var releaseDevice = func() ([]string, error) { return nil, nil }

func deviceOwner() string { return "" }
