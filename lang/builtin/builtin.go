// Copyright 2017 The Wuffs Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package builtin lists Wuffs' built-in concepts such as status codes.
package builtin

import (
	"fmt"
	"strings"

	"github.com/google/wuffs/lang/parse"

	a "github.com/google/wuffs/lang/ast"
	t "github.com/google/wuffs/lang/token"
)

var FourCCs = [...][2]string{
	{"ICCP", "International Color Consortium Profile"},
	{"XMP ", "Extensible Metadata Platform"},
}

var Statuses = [...]string{
	// Notes.
	`"@end of data"`,
	`"@metadata reported"`,

	// Suspensions.
	`"$short read"`,
	`"$short write"`,

	// Errors.
	`"#bad I/O position"`,
	`"#bad argument (length too short)"`,
	`"#bad argument"`,
	`"#bad call sequence"`,
	`"#bad receiver"`,
	`"#bad restart"`,
	`"#bad sizeof receiver"`,
	`"#bad vtable"`,
	`"#bad workbuf length"`,
	`"#bad wuffs version"`,
	`"#cannot return a suspension"`,
	`"#disabled by previous error"`,
	`"#initialize falsely claimed already zeroed"`,
	`"#initialize not called"`,
	`"#interleaved coroutine calls"`,
	`"#not enough data"`,
	`"#out of bounds"`,
	`"#unsupported method"`,
	`"#unsupported option"`,
	`"#unsupported pixel swizzler option"`,
	`"#too much data"`,
}

// TODO: a collection of forbidden variable names like and, or, not, as, false,
// true, in, out, this, u8, u16, etc?

var Types = []string{
	// TODO: i8, i16, i32, i64.
	"u8",
	"u16",
	"u32",
	"u64",

	"empty_struct",
	"bool",
	"utility",

	"range_ii_u32",
	"range_ie_u32",
	"range_ii_u64",
	"range_ie_u64",
	"rect_ie_u32",
	"rect_ii_u32",

	"status",

	"io_reader",
	"io_writer",
	"token_reader",
	"token_writer",

	"frame_config",
	"image_config",
	"pixel_blend",
	"pixel_buffer",
	"pixel_config",
	"pixel_format",
	"pixel_swizzler",

	"decode_frame_options",
}

var Funcs = []string{
	"u8.high_bits(n: u32[..= 8]) u8",
	"u8.low_bits(n: u32[..= 8]) u8",
	"u8.max(a: u8) u8",
	"u8.min(a: u8) u8",

	"u16.high_bits(n: u32[..= 16]) u16",
	"u16.low_bits(n: u32[..= 16]) u16",
	"u16.max(a: u16) u16",
	"u16.min(a: u16) u16",

	"u32.high_bits(n: u32[..= 32]) u32",
	"u32.low_bits(n: u32[..= 32]) u32",
	"u32.max(a: u32) u32",
	"u32.min(a: u32) u32",

	"u64.high_bits(n: u32[..= 64]) u64",
	"u64.low_bits(n: u32[..= 64]) u64",
	"u64.max(a: u64) u64",
	"u64.min(a: u64) u64",

	// ---- utility

	"utility.empty_io_reader() io_reader",
	"utility.empty_io_writer() io_writer",
	"utility.empty_range_ii_u32() range_ii_u32",
	"utility.empty_range_ie_u32() range_ie_u32",
	"utility.empty_range_ii_u64() range_ii_u64",
	"utility.empty_range_ie_u64() range_ie_u64",
	"utility.empty_rect_ii_u32() rect_ii_u32",
	"utility.empty_rect_ie_u32() rect_ie_u32",
	"utility.empty_slice_u8() slice u8",
	"utility.make_pixel_format(repr: u32) pixel_format",
	"utility.make_range_ii_u32(min_incl: u32, max_incl: u32) range_ii_u32",
	"utility.make_range_ie_u32(min_incl: u32, max_excl: u32) range_ie_u32",
	"utility.make_range_ii_u64(min_incl: u64, max_incl: u64) range_ii_u64",
	"utility.make_range_ie_u64(min_incl: u64, max_excl: u64) range_ie_u64",
	"utility.make_rect_ii_u32(" +
		"min_incl_x: u32, min_incl_y: u32, max_incl_x: u32, max_incl_y: u32) rect_ii_u32",
	"utility.make_rect_ie_u32(" +
		"min_incl_x: u32, min_incl_y: u32, max_excl_x: u32, max_excl_y: u32) rect_ie_u32",

	// ---- ranges

	"range_ie_u32.reset!()",
	"range_ie_u32.get_min_incl() u32",
	"range_ie_u32.get_max_excl() u32",
	"range_ie_u32.intersect(r: range_ie_u32) range_ie_u32",
	"range_ie_u32.unite(r: range_ie_u32) range_ie_u32",

	"range_ii_u32.reset!()",
	"range_ii_u32.get_min_incl() u32",
	"range_ii_u32.get_max_incl() u32",
	"range_ii_u32.intersect(r: range_ii_u32) range_ii_u32",
	"range_ii_u32.unite(r: range_ii_u32) range_ii_u32",

	"range_ie_u64.reset!()",
	"range_ie_u64.get_min_incl() u64",
	"range_ie_u64.get_max_excl() u64",
	"range_ie_u64.intersect(r: range_ie_u64) range_ie_u64",
	"range_ie_u64.unite(r: range_ie_u64) range_ie_u64",

	"range_ii_u64.reset!()",
	"range_ii_u64.get_min_incl() u64",
	"range_ii_u64.get_max_incl() u64",
	"range_ii_u64.intersect(r: range_ii_u64) range_ii_u64",
	"range_ii_u64.unite(r: range_ii_u64) range_ii_u64",

	// ---- status

	// TODO: should we add is_complete?
	"status.is_error() bool",
	"status.is_note() bool",
	"status.is_ok() bool",
	"status.is_suspension() bool",

	// ---- io_reader

	"io_reader.can_undo_byte() bool",
	"io_reader.undo_byte!()",

	"io_reader.read_u8?() u8",

	"io_reader.read_u16be?() u16",
	"io_reader.read_u16le?() u16",

	"io_reader.read_u8_as_u32?() u32[..= 0xFF]",
	"io_reader.read_u16be_as_u32?() u32[..= 0xFFFF]",
	"io_reader.read_u16le_as_u32?() u32[..= 0xFFFF]",
	"io_reader.read_u24be_as_u32?() u32[..= 0xFFFFFF]",
	"io_reader.read_u24le_as_u32?() u32[..= 0xFFFFFF]",
	"io_reader.read_u32be?() u32",
	"io_reader.read_u32le?() u32",

	"io_reader.read_u8_as_u64?() u64[..= 0xFF]",
	"io_reader.read_u16be_as_u64?() u64[..= 0xFFFF]",
	"io_reader.read_u16le_as_u64?() u64[..= 0xFFFF]",
	"io_reader.read_u24be_as_u64?() u64[..= 0xFFFFFF]",
	"io_reader.read_u24le_as_u64?() u64[..= 0xFFFFFF]",
	"io_reader.read_u32be_as_u64?() u64[..= 0xFFFFFFFF]",
	"io_reader.read_u32le_as_u64?() u64[..= 0xFFFFFFFF]",
	"io_reader.read_u40be_as_u64?() u64[..= 0xFFFFFFFFFF]",
	"io_reader.read_u40le_as_u64?() u64[..= 0xFFFFFFFFFF]",
	"io_reader.read_u48be_as_u64?() u64[..= 0xFFFFFFFFFFFF]",
	"io_reader.read_u48le_as_u64?() u64[..= 0xFFFFFFFFFFFF]",
	"io_reader.read_u56be_as_u64?() u64[..= 0xFFFFFFFFFFFFFF]",
	"io_reader.read_u56le_as_u64?() u64[..= 0xFFFFFFFFFFFFFF]",
	"io_reader.read_u64be?() u64",
	"io_reader.read_u64le?() u64",

	// TODO: these should have an explicit pre-condition "available() >= N".
	// For now, that's implicitly checked (i.e. hard coded).
	//
	// The io_reader has peek_etc methods and skip_fast, not read_etc_fast,
	// because we sometimes advance the pointer by less than what's read. See
	// https://fgiesen.wordpress.com/2018/02/20/reading-bits-in-far-too-many-ways-part-2/

	"io_reader.peek_u8() u8",

	"io_reader.peek_u16be() u16",
	"io_reader.peek_u16le() u16",

	"io_reader.peek_u8_as_u32() u32[..= 0xFF]",
	"io_reader.peek_u16be_as_u32() u32[..= 0xFFFF]",
	"io_reader.peek_u16le_as_u32() u32[..= 0xFFFF]",
	"io_reader.peek_u24be_as_u32() u32[..= 0xFFFFFF]",
	"io_reader.peek_u24le_as_u32() u32[..= 0xFFFFFF]",
	"io_reader.peek_u32be() u32",
	"io_reader.peek_u32le() u32",

	"io_reader.peek_u8_as_u64() u64[..= 0xFF]",
	"io_reader.peek_u16be_as_u64() u64[..= 0xFFFF]",
	"io_reader.peek_u16le_as_u64() u64[..= 0xFFFF]",
	"io_reader.peek_u24be_as_u64() u64[..= 0xFFFFFF]",
	"io_reader.peek_u24le_as_u64() u64[..= 0xFFFFFF]",
	"io_reader.peek_u32be_as_u64() u64[..= 0xFFFFFFFF]",
	"io_reader.peek_u32le_as_u64() u64[..= 0xFFFFFFFF]",
	"io_reader.peek_u40be_as_u64() u64[..= 0xFFFFFFFFFF]",
	"io_reader.peek_u40le_as_u64() u64[..= 0xFFFFFFFFFF]",
	"io_reader.peek_u48be_as_u64() u64[..= 0xFFFFFFFFFFFF]",
	"io_reader.peek_u48le_as_u64() u64[..= 0xFFFFFFFFFFFF]",
	"io_reader.peek_u56be_as_u64() u64[..= 0xFFFFFFFFFFFFFF]",
	"io_reader.peek_u56le_as_u64() u64[..= 0xFFFFFFFFFFFFFF]",
	"io_reader.peek_u64be() u64",
	"io_reader.peek_u64le() u64",

	"io_reader.available() u64",
	"io_reader.count_since(mark: u64) u64",
	"io_reader.is_closed() bool",
	"io_reader.mark() u64",
	"io_reader.match7(a: u64) u32[..= 2]",
	"io_reader.position() u64",
	"io_reader.since(mark: u64) slice u8",
	"io_reader.take!(n: u64) slice u8",

	"io_reader.skip?(n: u64)",
	"io_reader.skip32?(n: u32)",

	// TODO: this should have explicit pre-conditions "actual <= worst_case"
	// and "worst_case <= available()". As an implementation restriction, we
	// also require that worst_case has a constant value. For now, that's all
	// implicitly checked (i.e. hard coded).
	"io_reader.skip32_fast!(actual: u32, worst_case: u32)",

	// ---- io_writer

	"io_writer.write_u8?(a: u8)",
	"io_writer.write_u16be?(a: u16)",
	"io_writer.write_u16le?(a: u16)",
	"io_writer.write_u24be?(a: u32[..= 0xFFFFFF])",
	"io_writer.write_u24le?(a: u32[..= 0xFFFFFF])",
	"io_writer.write_u32be?(a: u32)",
	"io_writer.write_u32le?(a: u32)",
	"io_writer.write_u40be?(a: u64[..= 0xFFFFFFFFFF])",
	"io_writer.write_u40le?(a: u64[..= 0xFFFFFFFFFF])",
	"io_writer.write_u48be?(a: u64[..= 0xFFFFFFFFFFFF])",
	"io_writer.write_u48le?(a: u64[..= 0xFFFFFFFFFFFF])",
	"io_writer.write_u56be?(a: u64[..= 0xFFFFFFFFFFFFFF])",
	"io_writer.write_u56le?(a: u64[..= 0xFFFFFFFFFFFFFF])",
	"io_writer.write_u64be?(a: u64)",
	"io_writer.write_u64le?(a: u64)",

	// TODO: these should have an explicit pre-condition "available() >= N".
	// For now, that's implicitly checked (i.e. hard coded).
	//
	// The io_writer has write_fast_etc methods, not poke_etc and skip_fast,
	// because skip_fast could leave uninitialized bytes in the io_buffer.
	"io_writer.write_fast_u8!(a: u8)",
	"io_writer.write_fast_u16be!(a: u16)",
	"io_writer.write_fast_u16le!(a: u16)",
	"io_writer.write_fast_u24be!(a: u32[..= 0xFFFFFF])",
	"io_writer.write_fast_u24le!(a: u32[..= 0xFFFFFF])",
	"io_writer.write_fast_u32be!(a: u32)",
	"io_writer.write_fast_u32le!(a: u32)",
	"io_writer.write_fast_u40be!(a: u64[..= 0xFFFFFFFFFF])",
	"io_writer.write_fast_u40le!(a: u64[..= 0xFFFFFFFFFF])",
	"io_writer.write_fast_u48be!(a: u64[..= 0xFFFFFFFFFFFF])",
	"io_writer.write_fast_u48le!(a: u64[..= 0xFFFFFFFFFFFF])",
	"io_writer.write_fast_u56be!(a: u64[..= 0xFFFFFFFFFFFFFF])",
	"io_writer.write_fast_u56le!(a: u64[..= 0xFFFFFFFFFFFFFF])",
	"io_writer.write_fast_u64be!(a: u64)",
	"io_writer.write_fast_u64le!(a: u64)",

	"io_writer.available() u64",
	"io_writer.count_since(mark: u64) u64",
	"io_writer.history_available() u64",
	"io_writer.mark() u64",
	"io_writer.position() u64",
	"io_writer.since(mark: u64) slice u8",

	"io_writer.copy_from_slice!(s: slice u8) u64",
	"io_writer.copy_n32_from_history!(n: u32, distance: u32) u32",
	"io_writer.copy_n32_from_reader!(n: u32, r: io_reader) u32",
	"io_writer.copy_n32_from_slice!(n: u32, s: slice u8) u32",

	// TODO: this should have explicit pre-conditions:
	//  - n <= this.available()
	//  - distance > 0
	//  - distance <= this.since_mark().length()
	// For now, that's all implicitly checked (i.e. hard coded).
	"io_writer.copy_n32_from_history_fast!(n: u32, distance: u32) u32",

	// ---- token_writer

	"token_writer.write_token?(value: u64[..= 0xFFFFFFFFFFFF], length: u64[..= 0xFFFF])",

	"token_writer.write_fast_token!(value: u64[..= 0xFFFFFFFFFFFF], length: u64[..= 0xFFFF])",

	"token_writer.available() u64",

	// ---- frame_config
	// Duration's upper bound is the maximum possible i64 value.

	"frame_config.blend() u8",
	"frame_config.disposal() u8",
	"frame_config.duration() u64[..= 0x7FFFFFFFFFFFFFFF]",
	"frame_config.index() u64",
	"frame_config.io_position() u64",

	"frame_config.update!(bounds: rect_ie_u32, duration: u64[..= 0x7FFFFFFFFFFFFFFF]," +
		"index: u64, io_position: u64, disposal: u8, opaque_within_bounds: bool," +
		"overwrite_instead_of_blend: bool, background_color: u32)",

	// ---- image_config

	"image_config.set!(pixfmt: u32, pixsub: u32, width: u32, height: u32," +
		"first_frame_io_position: u64, first_frame_is_opaque: bool)",

	// ---- pixel_buffer

	"pixel_buffer.palette() slice u8",
	"pixel_buffer.pixel_format() pixel_format",
	"pixel_buffer.plane(p: u32[..= 3]) table u8",

	// ---- pixel_format

	"pixel_format.bits_per_pixel() u32[..= 256]",

	// ---- pixel_swizzler

	"pixel_swizzler.prepare!(" +
		"dst_pixfmt: pixel_format, dst_palette: slice u8," +
		"src_pixfmt: pixel_format, src_palette: slice u8, blend: pixel_blend) status",
	"pixel_swizzler.swizzle_interleaved!(" +
		"dst: slice u8, dst_palette: slice u8, src: slice u8) u64",
}

var Interfaces = []string{
	"hasher_u32",
	"image_decoder",
	"io_transformer",
	"token_decoder",
}

var InterfacesMap = map[string]bool{
	"hasher_u32":     true,
	"image_decoder":  true,
	"io_transformer": true,
	"token_decoder":  true,
}

var InterfaceFuncs = []string{
	// ---- hasher_u32

	"hasher_u32.update_u32!(x: slice u8) u32",

	// ---- image_decoder

	"image_decoder.ack_metadata_chunk?(src: io_reader)",
	"image_decoder.decode_frame?(" +
		"dst: ptr pixel_buffer, src: io_reader, blend: pixel_blend," +
		"workbuf: slice u8, opts: nptr decode_frame_options)",
	"image_decoder.decode_frame_config?(dst: nptr frame_config, src: io_reader)",
	"image_decoder.decode_image_config?(dst: nptr image_config, src: io_reader)",
	"image_decoder.frame_dirty_rect() rect_ie_u32",
	"image_decoder.metadata_chunk_length() u64",
	"image_decoder.metadata_fourcc() u32",
	"image_decoder.num_animation_loops() u32",
	"image_decoder.num_decoded_frame_configs() u64",
	"image_decoder.num_decoded_frames() u64",
	"image_decoder.restart_frame!(index: u64, io_position: u64) status",
	"image_decoder.set_report_metadata!(fourcc: u32, report: bool)",
	"image_decoder.workbuf_len() range_ii_u64",

	// ---- io_transformer

	"io_transformer.transform_io?(dst: io_writer, src: io_reader, workbuf: slice u8)",
	"io_transformer.workbuf_len() range_ii_u64",

	// ---- token_decoder

	"token_decoder.decode_tokens?(dst: token_writer, src: io_reader)",
}

// The "T1" and "T2" types here are placeholders for generic "slice T" or
// "table T" types. After tokenizing (but before parsing) these XxxFunc strings
// (e.g. in the lang/check package), replace "T1" and "T2" with "†" or "‡"
// daggers, to avoid collision with a user-defined "T1" or "T2" type.

const (
	genericOldName1 = t.IDT1
	genericOldName2 = t.IDT2
	genericNewName1 = t.IDDagger1
	genericNewName2 = t.IDDagger2
)

var SliceFuncs = []string{
	"GENERIC T1.copy_from_slice!(s: T1) u64",
	"GENERIC T1.length() u64",
	"GENERIC T1.prefix(up_to: u64) T1",
	"GENERIC T1.suffix(up_to: u64) T1",
}

var TableFuncs = []string{
	"GENERIC T2.height() u64",
	"GENERIC T2.stride() u64",
	"GENERIC T2.width() u64",

	"GENERIC T2.row(y: u32) T1",
}

func ParseFuncs(tm *t.Map, ss []string, callback func(*a.Func) error) error {
	const GENERIC = "GENERIC "
	buf := []byte(nil)
	for _, s := range ss {
		generic := strings.HasPrefix(s, GENERIC)
		if generic {
			s = s[len(GENERIC):]
		}

		buf = buf[:0]
		buf = append(buf, "pub func "...)
		buf = append(buf, s...)
		buf = append(buf, "{}\n"...)

		const filename = "builtin.wuffs"
		tokens, _, err := t.Tokenize(tm, filename, buf)
		if err != nil {
			return fmt.Errorf("parsing %q: could not tokenize built-in funcs: %v", s, err)
		}
		if generic {
			for i := range tokens {
				if id := tokens[i].ID; id == genericOldName1 {
					tokens[i].ID = genericNewName1
				} else if id == genericOldName2 {
					tokens[i].ID = genericNewName2
				}
			}
		}
		file, err := parse.Parse(tm, filename, tokens, &parse.Options{
			AllowBuiltInNames: true,
		})
		if err != nil {
			return fmt.Errorf("parsing %q: could not parse built-in funcs: %v", s, err)
		}

		tlds := file.TopLevelDecls()
		if len(tlds) != 1 || tlds[0].Kind() != a.KFunc {
			return fmt.Errorf("parsing %q: got %d top level decls, want %d", s, len(tlds), 1)
		}
		f := tlds[0].AsFunc()
		f.AsNode().AsRaw().SetPackage(tm, t.IDBase)

		if callback != nil {
			if err := callback(f); err != nil {
				return err
			}
		}
	}
	return nil
}
