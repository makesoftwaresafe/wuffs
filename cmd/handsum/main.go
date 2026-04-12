// Copyright 2025 The Wuffs Authors.
//
// Licensed under the Apache License, Version 2.0 <LICENSE-APACHE or
// https://www.apache.org/licenses/LICENSE-2.0> or the MIT license
// <LICENSE-MIT or https://opensource.org/licenses/MIT>, at your
// option. This file may not be copied, modified, or distributed
// except according to those terms.
//
// SPDX-License-Identifier: Apache-2.0 OR MIT

// ----------------

// handsum decodes and encodes the Handsum lossy image file format.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"

	"github.com/google/wuffs/lib/handsum"

	_ "image/gif"
	_ "image/jpeg"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

var (
	decodeFlag    = flag.Bool("decode", false, "whether to decode the input")
	encodeFlag    = flag.Bool("encode", false, "whether to encode the input")
	roundtripFlag = flag.Bool("roundtrip", false, "whether to encode-and-decode the input")
	variantFlag   = flag.String("variant", "hvpc", "which variant to encode to: hvpc, hvpg, hvxc, hvxg")
)

const usageStr = `handsum decodes and encodes the Handsum lossy image file format.

Usage: choose one of

    handsum -decode    [path]
    handsum -encode    [path]
    handsum -roundtrip [path]

The path to the input image file is optional. If omitted, stdin is read.

The output image (in Handsum or PNG format) is written to stdout.

Decode inputs Handsum and outputs PNG.
Encode inputs BMP, GIF, JPEG, PNG, TIFF or WEBP and outputs Handsum.
Roundtrip is equivalent to encode (to an ephemeral file) and then decode.

For encode or roundtrip, you can also choose a quality and color variant

    -variant=hvpc to use Handsum Variant Potato Color (the default)
    -variant=hvpg to use Handsum Variant Potato Gray
    -variant=hvxc to use Handsum Variant ExtremelyPotato Color
    -variant=hvxg to use Handsum Variant ExtremelyPotato Gray

For example: handsum -encode -variant=hvpg foo.png > foo.hvpg.handsum
`

func main() {
	if err := main1(); err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func main1() error {
	flag.Usage = func() { os.Stderr.WriteString(usageStr) }
	flag.Parse()

	variant := handsum.Variant(0)
	switch strings.ToLower(*variantFlag) {
	case "hvpc":
		variant = handsum.VariantPotatoColor
	case "hvpg":
		variant = handsum.VariantPotatoGray
	case "hvxc":
		variant = handsum.VariantExtremelyPotatoColor
	case "hvxg":
		variant = handsum.VariantExtremelyPotatoGray
	default:
		return fmt.Errorf("bad -variant flag value %q", *variantFlag)
	}

	inFile := os.Stdin
	switch flag.NArg() {
	case 0:
		// No-op.
	case 1:
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			return err
		}
		defer f.Close()
		inFile = f
	default:
		return errors.New("too many filenames; the maximum is one")
	}

	if *decodeFlag && !*encodeFlag && !*roundtripFlag {
		return decode(inFile)
	}
	if !*decodeFlag && *encodeFlag && !*roundtripFlag {
		return encode(inFile, variant)
	}
	if !*decodeFlag && !*encodeFlag && *roundtripFlag {
		return roundtrip(inFile, variant)
	}
	return errors.New("must specify exactly one of -decode, -encode, -roundtrip or -help")
}

func decode(inFile *os.File) error {
	src, err := handsum.Decode(inFile)
	if err != nil {
		return err
	}
	return png.Encode(os.Stdout, src)
}

func encode(inFile *os.File, variant handsum.Variant) error {
	src, _, err := image.Decode(inFile)
	if err != nil {
		return err
	}
	return handsum.Encode(os.Stdout, src, &handsum.EncodeOptions{Variant: variant})
}

func roundtrip(inFile *os.File, variant handsum.Variant) error {
	src, _, err := image.Decode(inFile)
	if err != nil {
		return err
	}
	buf := &bytes.Buffer{}
	err = handsum.Encode(buf, src, &handsum.EncodeOptions{Variant: variant})
	if err != nil {
		return err
	}
	dst, err := handsum.Decode(buf)
	if err != nil {
		return err
	}
	return png.Encode(os.Stdout, dst)
}
