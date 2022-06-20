// Copyright 2022 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"
	"unsafe"
)

// unsafeByteSlice converts string to a byte slice without memory allocation.
func unsafeByteSlice(s string) (b []byte) {
	/* #nosec G103 */
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	/* #nosec G103 */
	sh := *(*reflect.StringHeader)(unsafe.Pointer(&s))
	bh.Data = sh.Data
	bh.Len = sh.Len
	bh.Cap = sh.Len
	return
}

// unsafeUint64Slice converts string to a byte slice without memory allocation.
func unsafeUint64Slice(s string) (b []uint64) {
	/* #nosec G103 */
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	/* #nosec G103 */
	sh := *(*reflect.StringHeader)(unsafe.Pointer(&s))
	bh.Data = sh.Data
	bh.Len = sh.Len / 8
	bh.Cap = sh.Len / 8
	return
}

// spinCPU spins the CPU to trigger turboboost / turbocore / speedshift.
func spinCPU(w io.Writer, d time.Duration) uint64 {
	command := strings.Repeat("a long string", 100)
	fmt.Fprintf(w, "Spinning for %s.\n", d)
	var v uint64
	for start := time.Now(); time.Since(start) < d; {
		// TODO(maruel): Make a mathematically difficult problem that uses the
		// ALU and FPU for at least a few ms on modern CPUs.
		for i := 0; i < 1000; i++ {
			// Hashes a string using the MurmurHash2 algorithm by Austin Appleby.
			seed := uint64(0xDECAFBADDECAFBAD)
			const m = 0xc6a4a7935bd1e995
			r := 47
			l := len(command)
			h := seed ^ (uint64(l) * m)
			i := 0
			if l > 7 {
				// I tried a few combinations (data as []byte) and this one seemed to be the
				// best. Feel free to micro-optimize.
				//data := (*[0x7fff0000]uint64)(unsafe.Pointer((*reflect.StringHeader)(unsafe.Pointer(&command)).Data))[:l/8]
				data := unsafeUint64Slice(command)
				for ; i < len(data); i++ {
					k := data[i]
					k *= m
					k ^= k >> r
					k *= m
					h ^= k
					h *= m
				}
			}

			//data2 := (*[0x7fff0000]byte)(unsafe.Pointer((*reflect.StringHeader)(unsafe.Pointer(&command)).Data))[8*i : 8*(i+1)]
			data2 := unsafeByteSlice(command[i*8:])
			//switch (l - 8*i) & 7 {
			switch (l - 8*i) & 7 {
			case 7:
				h ^= uint64(data2[6]) << 48
				fallthrough
			case 6:
				h ^= uint64(data2[5]) << 40
				fallthrough
			case 5:
				h ^= uint64(data2[4]) << 32
				fallthrough
			case 4:
				h ^= uint64(data2[3]) << 24
				fallthrough
			case 3:
				h ^= uint64(data2[2]) << 16
				fallthrough
			case 2:
				h ^= uint64(data2[1]) << 8
				fallthrough
			case 1:
				h ^= uint64(data2[0])
				h *= m
			case 0:
			}
			h ^= h >> r
			h *= m
			h ^= h >> r
			v = h
		}
	}
	return v
}
