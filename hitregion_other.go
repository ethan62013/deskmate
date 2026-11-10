//go:build !windows

package main

func setHitRegion(uintptr, []HitRect) {}

func startNativeDrag(uintptr) {}
