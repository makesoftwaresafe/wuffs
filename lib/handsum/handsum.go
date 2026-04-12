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

// Package handsum implements the Handsum image file format.
//
// This is a very lossy format for very small thumbnails. Very small in terms
// of image dimensions, up to 32×32 pixels, but also in terms of file size.
//
// The file format has four variants, combining two quality settings (P is
// Potato, X is Extremely Potato) and two color settings (C is Color, G is
// Gray). Each Handsum Variant (named HV??) is a fixed number of bytes:
//
//   - The HVPC variant is 147 bytes long.
//   - The HVPG variant is  99 bytes long.
//   - The HVXC variant is  48 bytes long.
//   - The HVXG variant is  33 bytes long.
//
// For example, every HVXC image file is exactly 48 bytes (384 bits) long. For
// a 32×32 pixel image, this uses 0.375 bits (0.046875 bytes) per pixel.
//
// HVPC (P for Potato, not X for Extremely Potato), always 147 bytes, uses
// 1.1484375 bits per pixel for a 32×32 pixel image (a 1:1 aspect ratio), or
// 1.53125 bits per pixel for a 32×24 pixel image (a 4:3 aspect ratio).
//
// A Handsum file's starts with a 3 byte header: a 16-bit magic signature, a
// 2-bit variant and a 6-bit aspect ratio. An image's longest dimension (width
// or height) is 32 pixels and the aspect ratio gives the shorter dimension.
//
// The C (Color) payload, after the header, holds a scaled 16×16 pixel YCbCr
// 4:2:0 JPEG MCU (Minimum Coded Unit), 4 Luma and 2 Chroma blocks. Each block
// is 8×8 pixels.
//
// The G (Gray) payload is just the C payload with the 4 Luma blocks but
// without the 2 Chroma blocks. This drops the final 48 (for P, Potato) or 15
// (for X, Extremely Potato) bytes from the C variant's encoding.
//
// P (Potato) splits the MCU into 8×8 pixel blocks and further splits each
// block into 2×2 pixel mini-blocks. DCT (Discrete Cosine Transform) is applied
// to each mini-block, producing 4 DCT coefficients. Only the 3 lowest
// frequency DCT coefficients are kept.
//
// X (Extremely Potato) splits the MCU into 8×8 pixel blocks. DCT is applied to
// each block, producing 64 DCT coefficients, just like JPEG. Only the 15
// lowest frequency DCT coefficients are kept. Lowest frequency means the
// top-left corner in the usual visualization of JPEG's zig-zag ordering.
//
// Either way, P or X, each DCT coefficient is encoded as one nibble (4 bits;
// half a byte) with fixed bias and quantization factors. Each 8×8 pixel block
// encodes in 48 (P) or 15 (X) nibbles.
//
// All Handsum images use the sRGB color profile.
//
// The "Handsum" name was inspired by the "Thumbhash" image file format, which
// is also designed for very small thumbnails (or very compact representations
// of image placeholders). Handsum files are bigger (but better quality) than
// Thumbhash. "Handsum" also sounds like "handsome", meaning "good looking".
package handsum

import (
	"errors"
	"image"
	"image/color"
	"io"

	"github.com/google/wuffs/lib/lowleveljpeg"

	"golang.org/x/image/draw"
)

// Variant represents one of the Handsum file format's four variants.
//
// The zero value means unknown (before decoding) or to use the default option
// (when encoding).
type Variant uint8

const (
	VariantExtremelyPotatoGray  = Variant(1)
	VariantExtremelyPotatoColor = Variant(2)
	VariantPotatoGray           = Variant(3)
	VariantPotatoColor          = Variant(4)
)

func (v Variant) isExtremelyPotato() bool { return v <= VariantExtremelyPotatoColor }
func (v Variant) isGray() bool            { return (v & 1) != 0 }
func (v Variant) numberOfBlocks() int     { return 6 - (2 * int(v&1)) }

func (v Variant) fileSize() int {
	switch v {
	case VariantExtremelyPotatoGray:
		return FileSizeHVXG
	case VariantExtremelyPotatoColor:
		return FileSizeHVXC
	case VariantPotatoGray:
		return FileSizeHVPG
	}
	return FileSizeHVPC
}

const (
	// FileSizeHVXG is the size (in bytes) of every HVXG Handsum image file.
	FileSizeHVXG = 33
	// FileSizeHVXC is the size (in bytes) of every HVXC Handsum image file.
	FileSizeHVXC = 48
	// FileSizeHVPG is the size (in bytes) of every HVPG Handsum image file.
	FileSizeHVPG = 99
	// FileSizeHVPC is the size (in bytes) of every HVPC Handsum image file.
	FileSizeHVPC = 147

	fileSizeHeader = 3
	fileSizeMax    = 147
)

// MaxDimension is the maximum (inclusive) width or height of every Handsum
// image file.
//
// Every image is either (W × 32) or (32 × H) or both, for some positive W or H
// that is no greater than 32.
const MaxDimension = 32

// Magic is the byte string prefix of every Handsum image file.
//
// It's like how every JPEG image file starts with "\xFF\xD8".
const Magic = "\xFE\xD7"

func init() {
	image.RegisterFormat("handsum", Magic, Decode, DecodeConfig)
}

var (
	ErrBadArgument     = errors.New("handsum: bad argument")
	ErrNotAHandsumFile = errors.New("handsum: not a handsum file")
)

// EncodeOptions are optional arguments to Encode. The zero value is valid and
// means to use the default configuration.
type EncodeOptions struct {
	// Variant is which of the four Handsum Variants to use. If zero, the
	// default is HVPC (Handsum Variant Potato Color).
	Variant Variant
}

func (o *EncodeOptions) variant() Variant {
	if (o != nil) && (1 <= o.Variant) && (o.Variant <= 4) {
		return o.Variant
	}
	return VariantPotatoColor
}

// Encode writes src to w in the Handsum format.
//
// options may be nil, which means to use the default configuration.
func Encode(w io.Writer, src image.Image, options *EncodeOptions) error {
	if (w == nil) || (src == nil) {
		return ErrBadArgument
	}
	srcB := src.Bounds()
	srcW, srcH := srcB.Dx(), srcB.Dy()
	if (srcW <= 0) || (srcH <= 0) {
		return ErrBadArgument
	}

	aspectRatio := byte(0x1F)
	if srcW > srcH { // Landscape.
		a := ((int64(srcH) * 64) + int64(srcW)) / (2 * int64(srcW))
		if a <= 0 {
			a = 1
		}
		aspectRatio = byte(a-1) | 0x00
	} else if srcW < srcH { // Portrait.
		a := ((int64(srcW) * 64) + int64(srcH)) / (2 * int64(srcH))
		if a <= 0 {
			a = 1
		}
		aspectRatio = byte(a-1) | 0x20
	}

	dst := image.NewRGBA(image.Rectangle{Max: image.Point{X: 16, Y: 16}})
	draw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)

	dstU8s := lowleveljpeg.Array6BlockU8{}
	dstU8s.ExtractFrom(dst, 0, 0)

	v := options.variant()
	buf := [fileSizeMax]byte{}
	buf[0] = Magic[0]
	buf[1] = Magic[1]
	buf[2] = aspectRatio | ((uint8(v) - 1) << 6)

	bitOffset := 3 * 8
	if v.isExtremelyPotato() {
		dstI16s := lowleveljpeg.Array6BlockI16{}
		dstI16s.ForwardDCTFrom(&dstU8s)
		for i := range v.numberOfBlocks() {
			bitOffset = encodeXBlock(&buf, bitOffset, &dstI16s[i])
		}
	} else {
		// Biasing the Chroma blocks by +8 shifts the neutral (gray) Chroma
		// values from 0x80 to 0x88. encodePBlock's DC coefficient quantization
		// can encode multiples of 0x11 losslessly.
		biasUp(&dstU8s[4])
		biasUp(&dstU8s[5])
		for i := range v.numberOfBlocks() {
			bitOffset = encodePBlock(&buf, bitOffset, &dstU8s[i])
		}
	}

	_, err := w.Write(buf[:bitOffset/8])
	return err
}

func encodePBlock(buf *[fileSizeMax]byte, bitOffset int, b *lowleveljpeg.BlockU8) int {
	for i := range 16 {
		x := 2 * (i & 3)
		y := 2 * (i >> 2)
		j := (8 * y) + x

		b00 := int(b[j+0o00])
		b01 := int(b[j+0o01])
		b10 := int(b[j+0o10])
		b11 := int(b[j+0o11])

		// Compute the quantized DC coefficient.
		//
		// "(p + q + r + s + 2) >> 2" takes the average of four numbers,
		// rounding properly. Shifting by (2 + 4) instead of by 2 quantizes the
		// range [0x00, 0xFF] to [0x0, 0xF].
		avgw := (+b00 + b01 + b10 + b11 + 2) >> 6

		// Compute the quantized AC coefficients: one horizontal and one
		// vertical. These two lines are equivalent to:
		//
		// avgx := (((-b00 + b01 - b10 + b11 + 2) >> 2) + 2) >> 2
		// avgy := (((-b00 - b01 + b10 + b11 + 2) >> 2) + 2) >> 2
		//
		// The inner "(foo + 2) >> 2" calculation is like taking an average.
		//
		// In theory, the outer calculation should be "(foo + 4) >> 3" instead
		// of "(foo + 2) >> 2", since on the decode side, we multiply by 8.
		// Scaling by 2 is an arbitrary adjustment that's not mirrored on the
		// decode side, but the results seem a little more vibrant.
		avgx := (-b00 + b01 - b10 + b11 + 10) >> 4
		avgy := (-b00 - b01 + b10 + b11 + 10) >> 4

		// Clip DC to [0, 15] and AC to [-8, +7]. Pack it in a nibble.
		ew := max(0, min(15, avgw+0))
		ex := max(0, min(15, avgx+8))
		ey := max(0, min(15, avgy+8))

		buf[bitOffset>>3] |= uint8(ew) << (bitOffset & 4)
		bitOffset += 4
		buf[bitOffset>>3] |= uint8(ex) << (bitOffset & 4)
		bitOffset += 4
		buf[bitOffset>>3] |= uint8(ey) << (bitOffset & 4)
		bitOffset += 4
	}
	return bitOffset
}

func encodeXBlock(buf *[fileSizeMax]byte, bitOffset int, b *lowleveljpeg.BlockI16) int {
	for i := range nCoeffs {
		e := uint8(0)
		if i == 0 {
			e = encodeXDC(b[0])
		} else {
			e = encodeXAC(b[zigzag[i]])
		}
		buf[bitOffset>>3] |= e << (bitOffset & 4)
		bitOffset += 4
	}

	return bitOffset
}

func encodeXDC(value int16) uint8 {
	const w = dcBucketWidth

	v := int32(value) + ((w * 8) + (w / 2))
	v /= w
	if v < 0x0 {
		return 0x0
	} else if v > 0xF {
		return 0xF
	}
	return uint8(v)
}

func encodeXAC(value int16) uint8 {
	// Scaling by 2 is an arbitrary adjustment that's not mirrored on the
	// decode side, but the results seem a little more vibrant.
	const w = acBucketWidth / 2

	v := int32(value) + ((w * 8) + (w / 2))
	v /= w
	if v < 0x0 {
		return 0x0
	} else if v > 0xF {
		return 0xF
	}
	return uint8(v)
}

// For X (Extremely Potato), both DC and AC coefficients are quantized into 16
// buckets (4 bits), but they use different bucket widths:
//
//	Bucket    DC      AC
//	0x0    -1024    -128
//	0x1     -896    -112
//	0x2     -768     -96
//	0x3     -640     -80
//	...       ...     ...
//	0x7     -128     -16
//	0x8        0       0
//	0x9     +128     +16
//	...       ...     ...
//	0xE     +768     +96
//	0xF     +896    +112
const (
	dcBucketWidth = 128
	acBucketWidth = 16
)

// DecodeConfig reads a Handsum image configuration from r.
func DecodeConfig(r io.Reader) (image.Config, error) {
	buf := [fileSizeHeader]byte{}
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return image.Config{}, err
	} else if (buf[0] != Magic[0]) || (buf[1] != Magic[1]) {
		return image.Config{}, ErrNotAHandsumFile
	}
	cm := color.RGBAModel
	if v := Variant(buf[2]>>6) + 1; v.isGray() {
		cm = color.GrayModel
	}

	w, h := decodeWidthAndHeight(buf[2])
	return image.Config{
		ColorModel: cm,
		Width:      w,
		Height:     h,
	}, nil
}

// Decode reads a Handsum image from r.
func Decode(r io.Reader) (image.Image, error) {
	buf := [fileSizeMax]byte{}
	if _, err := io.ReadFull(r, buf[:fileSizeHeader]); err != nil {
		return nil, err
	} else if (buf[0] != Magic[0]) || (buf[1] != Magic[1]) {
		return nil, ErrNotAHandsumFile
	}
	v := Variant(buf[2]>>6) + 1
	if _, err := io.ReadFull(r, buf[fileSizeHeader:v.fileSize()]); err != nil {
		return nil, err
	}

	bitOffset := 3 * 8
	decodeBlock := decodePBlock
	if v.isExtremelyPotato() {
		decodeBlock = decodeXBlock
	}
	lumaQuadBlockU8 := lowleveljpeg.QuadBlockU8{}
	bitOffset = decodeBlock(lumaQuadBlockU8[0x00:], 16, &buf, bitOffset)
	bitOffset = decodeBlock(lumaQuadBlockU8[0x08:], 16, &buf, bitOffset)
	bitOffset = decodeBlock(lumaQuadBlockU8[0x80:], 16, &buf, bitOffset)
	bitOffset = decodeBlock(lumaQuadBlockU8[0x88:], 16, &buf, bitOffset)
	smoothLumaBlockSeams(&lumaQuadBlockU8)

	src := image.Image(nil)
	if v.isGray() {
		src = &image.Gray{
			Pix:    lumaQuadBlockU8[:],
			Stride: 16,
			Rect:   image.Rectangle{Max: image.Point{X: 16, Y: 16}},
		}

	} else {
		cbBlockU8 := lowleveljpeg.BlockU8{}
		crBlockU8 := lowleveljpeg.BlockU8{}

		bitOffset = decodeBlock(cbBlockU8[:], 8, &buf, bitOffset)
		bitOffset = decodeBlock(crBlockU8[:], 8, &buf, bitOffset)
		if v == VariantPotatoColor {
			biasDown(&cbBlockU8)
			biasDown(&crBlockU8)
		}

		cbQuadBlockU8 := lowleveljpeg.QuadBlockU8{}
		cbQuadBlockU8.UpsampleFrom(&cbBlockU8)
		crQuadBlockU8 := lowleveljpeg.QuadBlockU8{}
		crQuadBlockU8.UpsampleFrom(&crBlockU8)

		src = &image.YCbCr{
			Y:              lumaQuadBlockU8[:],
			Cb:             cbQuadBlockU8[:],
			Cr:             crQuadBlockU8[:],
			YStride:        16,
			CStride:        16,
			SubsampleRatio: image.YCbCrSubsampleRatio444,
			Rect:           image.Rectangle{Max: image.Point{X: 16, Y: 16}},
		}
	}

	dstW, dstH := decodeWidthAndHeight(buf[2])
	dst := image.NewRGBA(image.Rectangle{Max: image.Point{X: dstW, Y: dstH}})
	draw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	return dst, nil
}

func decodeWidthAndHeight(buf2 byte) (w int, h int) {
	if (buf2 & 0x20) == 0 { // Landscape.
		w = 32
		h = 1 + int(buf2&0x1F)
	} else { // Portrait.
		w = 1 + int(buf2&0x1F)
		h = 32
	}
	return w, h
}

func decodePBlock(dst []byte, stride int, src *[fileSizeMax]byte, bitOffset int) int {
	for i := range 16 {
		// Decode the quantized-by-0x11 DC coefficient, covering [0x00, 0xFF].
		ew := int((src[bitOffset>>3] >> (bitOffset & 4)) & 15)
		bitOffset += 4

		// Decode the quantized-by-8 AC coefficients, covering [-64, +56]. The
		// -8 here, which applies to both Luma and Chroma blocks, is not the
		// Chroma-only bias handled by biasUp and biasDown.
		ex := int((src[bitOffset>>3]>>(bitOffset&4))&15) - 8
		bitOffset += 4
		ey := int((src[bitOffset>>3]>>(bitOffset&4))&15) - 8
		bitOffset += 4

		x := 2 * (i & 3)
		y := 2 * (i >> 2)
		j0 := ((y + 0) * stride) + x
		j1 := ((y + 1) * stride) + x

		dst[j0+0] = uint8(max(0x00, min(0xFF, (ew*0x11)+((-ey-ex)*8))))
		dst[j0+1] = uint8(max(0x00, min(0xFF, (ew*0x11)+((-ey+ex)*8))))
		dst[j1+0] = uint8(max(0x00, min(0xFF, (ew*0x11)+((+ey-ex)*8))))
		dst[j1+1] = uint8(max(0x00, min(0xFF, (ew*0x11)+((+ey+ex)*8))))
	}
	return bitOffset
}

func decodeXBlock(dst []byte, stride int, src *[fileSizeMax]byte, bitOffset int) int {
	a := lowleveljpeg.BlockI16{}

	{
		nibble := (src[bitOffset>>3] >> (bitOffset & 4)) & 15
		a[0] = (int16(nibble) - 8) * dcBucketWidth
		bitOffset += 4
	}

	for i := 1; i < nCoeffs; i++ {
		nibble := (src[bitOffset>>3] >> (bitOffset & 4)) & 15
		a[zigzag[i]] = (int16(nibble) - 8) * acBucketWidth
		bitOffset += 4
	}

	b := lowleveljpeg.BlockU8{}
	b.InverseDCTFrom(&a)

	for i := range 8 {
		di := i * stride
		bi := i * 8
		copy(dst[di:di+8], b[bi:bi+8])
	}

	return bitOffset
}

const nCoeffs = 15

// zigzag represents JPEG's zig-zag order for visiting DCT coefficients. X
// (Extremely Potato) only uses the first (1 + 2 + 3 + 4 + 5) = 15 of JPEG's 64
// DCT coefficients.
//
// https://en.wikipedia.org/wiki/File:JPEG_ZigZag.svg
var zigzag = [nCoeffs]uint8{
	0o00, 0o01, 0o10, 0o20, 0o11, 0o02, 0o03, 0o12, //  0,  1,  8, 16,  9,  2,  3, 10,
	0o21, 0o30, 0o40, 0o31, 0o22, 0o13, 0o04, //       17, 24, 32, 25, 18, 11,  4,
}

func biasUp(b *lowleveljpeg.BlockU8) {
	for i, v := range b {
		b[i] = uint8(min(0xFF, int(v)+8))
	}
}

func biasDown(b *lowleveljpeg.BlockU8) {
	for i, v := range b {
		b[i] = uint8(max(0x00, int(v)-8))
	}
}

func smoothLumaBlockSeams(b *lowleveljpeg.QuadBlockU8) {
	for _, pair := range smoothingPairs {
		v0 := uint32(b[pair[0]])
		v1 := uint32(b[pair[1]])
		b[pair[0]] = uint8(((3 * v0) + v1 + 2) / 4)
		b[pair[1]] = uint8(((3 * v1) + v0 + 2) / 4)
	}

	v77 := uint32(b[0x77])
	v78 := uint32(b[0x78])
	v88 := uint32(b[0x88])
	v87 := uint32(b[0x87])

	b[0x77] = uint8(((9 * v77) + (3 * v78) + v88 + (3 * v87) + 8) / 16)
	b[0x78] = uint8(((9 * v78) + (3 * v88) + v87 + (3 * v77) + 8) / 16)
	b[0x88] = uint8(((9 * v88) + (3 * v87) + v77 + (3 * v78) + 8) / 16)
	b[0x87] = uint8(((9 * v87) + (3 * v77) + v78 + (3 * v88) + 8) / 16)
}

// smoothingPairs are the seams of the four 8×8 Luma blocks in a 16×16 MCU. The
// central 4 pixels are handled separately.
var smoothingPairs = [28][2]uint8{
	{0x07, 0x08},
	{0x17, 0x18},
	{0x27, 0x28},
	{0x37, 0x38},
	{0x47, 0x48},
	{0x57, 0x58},
	{0x67, 0x68},

	{0x70, 0x80},
	{0x71, 0x81},
	{0x72, 0x82},
	{0x73, 0x83},
	{0x74, 0x84},
	{0x75, 0x85},
	{0x76, 0x86},

	{0x79, 0x89},
	{0x7A, 0x8A},
	{0x7B, 0x8B},
	{0x7C, 0x8C},
	{0x7D, 0x8D},
	{0x7E, 0x8E},
	{0x7F, 0x8F},

	{0x97, 0x98},
	{0xA7, 0xA8},
	{0xB7, 0xB8},
	{0xC7, 0xC8},
	{0xD7, 0xD8},
	{0xE7, 0xE8},
	{0xF7, 0xF8},
}
