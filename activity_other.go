//go:build !windows

package main

import "time"

func foregroundApp() (string, bool) { return "", false }

func idleDuration() time.Duration { return 0 }
