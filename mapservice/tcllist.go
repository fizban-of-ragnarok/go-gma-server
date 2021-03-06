// vi:set ai sm nu ts=4 sw=4 fileencoding=utf-8:
/*
########################################################################################
#  _______  _______  _______                ___       _______     _______              #
# (  ____ \(       )(  ___  )              /   )     / ___   )   / ___   )             #
# | (    \/| () () || (   ) |             / /) |     \/   )  |   \/   )  |             #
# | |      | || || || (___) |            / (_) (_        /   )       /   )             #
# | | ____ | |(_)| ||  ___  |           (____   _)     _/   /      _/   /              #
# | | \_  )| |   | || (   ) | Game           ) (      /   _/      /   _/               #
# | (___) || )   ( || )   ( | Master's       | |   _ (   (__/\ _ (   (__/\             #
# (_______)|/     \||/     \| Assistant      (_)  (_)\_______/(_)\_______/             #
#                                                                                      #
########################################################################################
*/
//
////////////////////////////////////////////////////////////////////////////////////////
//                                                                                    //
//                                     TclList                                        //
//                                                                                    //
// Represents a list of values in such a way that we can interact with them as a      //
// list of values in Go, but can translate them to and from TCL list syntax.          //
//                                                                                    //
////////////////////////////////////////////////////////////////////////////////////////

package mapservice

import (
	"fmt"
	"strings"
)

//
// Some of the older elements of GMA (which used to be entirely written in
// the Tcl language, after it was ported from the even older C++ code) use
// Tcl  list  objects  as  their  data representation. Notably the biggest
// example is the mapper(6) tool (which is still written in  Tcl  itself),
// whose  map  file  format and TCP/IP communications protocl include mar‐
// shalled data structures represented as Tcl lists.
// 
// While this is obviously convenient for Tcl programs in  that  they  can
// take  such strings and natively use them as lists of values, it is also
// useful generally in that it is a simple string representation of a sim‐
// ple  data  structure.  The  actual  definition of this string format is
// included below.
// 
// The TclList Python object provides an easy Python interface to  manipu‐
// late  Tcl  lists  as  Python  lists.   (Courtesy  of  the fact that the
// Python's tkinter standard  library  module  includes  an  embedded  Tcl
// interpreter,  this is done by calling into the Tcl interpreter, so this
// should be very accurate in terms of being genuine Tcl lists and not  an
// approximate emulation.)
//
// In our case, we don't have a Tcl interpreter handy,  so we'll implement
// a simple string scanner in Go which will convert these string represen-
// tations to and from Go slices.
// 
// As of yet, this only converts to and from slices of strings. We will
// extend this to manage mixed-types and nested lists if necessary, but
// that might not be needed for this application. We'll see.
//
// TCL LIST FORMAT
//
// In  a  nutshell,  a  Tcl  list (as a string representation) is a space-
// delimited list of values. Any value which includes spaces  is  enclosed
// in  curly braces.  An empty string (empty list) as an element in a list
// is represented as “{}”.  (E.g., “1 {} 2” is a list of  three  elements,
// the  middle of which is an empty string.) An entirely empty Tcl list is
// represented as an empty string “”.
// 
// A list value must have balanced braces. A balanced pair of braces  that
// happen  to  be  inside a larger string value may be left as-is, since a
// string that happens to contain spaces or braces is  only  distinguished
// from a deeply-nested list value when you attempt to interpret it as one
// or another in the code. Thus, the list
// 	  “a b {this {is a} string}”
// has three elements: “a”, “b”, and “this {is a} string”.   Otherwise,  a
// lone brace that's part of a string value should be escaped with a back‐
// slash:
// 	  “a b {this \{ too}”
// 
// Literal backslashes may be escaped with a backslash as well.
// 
// While extra spaces are ignored when  parsing  lists  into  elements,  a
// properly  formed  string representation of a list will have the miminum
// number of spaces and braces needed to describe the list structure.


//--------------------------------------------------------------------------------------------------------------
//
// Rather than imperfectly try to mimic the behavior of Tcl list
// code (and hope we didn't miss some nuance), the following code is
// our port of the code from the Tcl sources, which is itself released
// under the following terms:
//
//  _______________________________________________________________________
// |  The following terms apply to the all versions of the core Tcl/Tk     |
// |  releases, the Tcl/Tk browser plug-in version 2.0, and TclBlend       |
//    and Jacl version 1.0. Please note that the TclPro tools are under
//    a different license agreement. This agreement is part of the 
//    standard Tcl/Tk distribution as the file named "license.terms".
//
//    Tcl/Tk License Terms
//
//    This software is copyrighted by the Regents of the University of 
//    California, Sun Microsystems, Inc., Scriptics Corporation, and 
//    other parties. The following terms apply to all files associated 
//    with the software unless explicitly disclaimed in individual files.
//
//    The authors hereby grant permission to use, copy, modify, distribute, 
//    and license this software and its documentation for any purpose, 
//    provided that existing copyright notices are retained in all copies 
//    and that this notice is included verbatim in any distributions. No 
//    written agreement, license, or royalty fee is required for any of 
//    the authorized uses. Modifications to this software may be 
//    copyrighted by their authors and need not follow the licensing 
//    terms described here, provided that the new terms are clearly 
//    indicated on the first page of each file where they apply.
//
//    IN NO EVENT SHALL THE AUTHORS OR DISTRIBUTORS BE LIABLE TO ANY 
//    PARTY FOR DIRECT, INDIRECT, SPECIAL, INCIDENTAL, OR CONSEQUENTIAL 
//    DAMAGES ARISING OUT OF THE USE OF THIS SOFTWARE, ITS DOCUMENTATION, 
//    OR ANY DERIVATIVES THEREOF, EVEN IF THE AUTHORS HAVE BEEN ADVISED 
//    OF THE POSSIBILITY OF SUCH DAMAGE.
//
//    THE AUTHORS AND DISTRIBUTORS SPECIFICALLY DISCLAIM ANY WARRANTIES, 
//    INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY, 
//    FITNESS FOR A PARTICULAR PURPOSE, AND NON-INFRINGEMENT. THIS SOFTWARE 
//    IS PROVIDED ON AN "AS IS" BASIS, AND THE AUTHORS AND DISTRIBUTORS HAVE 
//    NO OBLIGATION TO PROVIDE MAINTENANCE, SUPPORT, UPDATES, ENHANCEMENTS, 
//    OR MODIFICATIONS.
//
//    GOVERNMENT USE: If you are acquiring this software on behalf of the 
//    U.S. government, the Government shall have only "Restricted Rights" 
//    in the software and related documentation as defined in the Federal 
//    Acquisition Regulations (FARs) in Clause 52.227.19 (c) (2). If you 
//    are acquiring the software on behalf of the Department of Defense, 
//    the software shall be classified as "Commercial Computer Software" 
//    and the Government shall have only "Restricted Rights" as defined 
//    in Clause 252.227-7013 (c) (1) of DFARs. Notwithstanding the foregoing, 
//    the authors grant the U.S. Government and others acting in its behalf 
//    permission to use and distribute the software in accordance with the 
//  | terms specified in this license.                                    |
//  |_____________________________________________________________________|
//
//
// The following code was written for GMA by Steven Willoughby, based on the
// original C code distirbuted in the Tcl core source code files "tclUtil.c",
// "tclParse.c", "tclUtf.c", as a direct port of that original code to Go.
// 
const t_CONVERT_NONE        = 0
const t_TCL_DONT_USE_BRACES = 1
const t_CONVERT_BRACE       = 2
const t_CONVERT_ESCAPE      = 4
const t_CONVERT_MASK        = (t_CONVERT_BRACE | t_CONVERT_ESCAPE)
const t_TCL_DONT_QUOTE_HASH = 8
const t_CONVERT_ANY         = 16
//
// Tcl_ScanElement(string, flagPtr) -> len
// scans the input string, setting flags based on what's in that element
// and returns the string length needed to hold the string representation
// of that element (an overestimation for allocation purposes)
//
func tcl_scan_element(element string, flags int) (int, int, error) {
	length := len(element)
	nesting_level := 0
	forbid_none := false
	require_escape := false
	extra := 0
	bytes_needed := 0
	prefer_escape := false
	prefer_brace := false
	brace_count := 0
	after_backslash := false

	if length == 0 {
		return 2, t_CONVERT_BRACE, nil
	}

	if element[0] == '#' && (flags & t_TCL_DONT_QUOTE_HASH) == 0 {
		prefer_brace = true
	}

	if element[0] == '{' || element[0] == '"' {
		forbid_none = true
		prefer_brace = true
	}

	for i, r := range element {
		if after_backslash {
			if r == '{' || r == '}' || r == '\\' {
				extra++
			} else if r == '\n' {
				extra++
				require_escape = true
			}
			after_backslash = false
			continue
		}

		switch r {
			case '{':
				brace_count++
				extra++
				nesting_level++
			case '}':
				extra++
				nesting_level--
				if nesting_level < 0 {
					require_escape = true
				}
			case ']', '"':
				forbid_none = true
				extra++
				prefer_escape = true
			case '[', '$', ';', ' ', '\f', '\n', '\r', '\t', '\v':
				forbid_none = true
				extra++
				prefer_brace = true
			case '\\':
				extra++
				if i+1 >= length {
					require_escape = true
					break
				}
				after_backslash = true
				forbid_none = true
				prefer_brace = true
		}
	}
	if nesting_level != 0 {
		require_escape = true
	}
	bytes_needed = length
	if require_escape {
		bytes_needed += extra
		if element[0] == '#' && (flags & t_TCL_DONT_QUOTE_HASH) == 0 {
			bytes_needed++
		}
		flags = t_CONVERT_ESCAPE
		goto overflow_check
	}

	if (flags & t_CONVERT_ANY) != 0 {
		if extra < 2 {
			extra = 2
		}
		flags &= ^t_CONVERT_ANY
		flags |= t_TCL_DONT_USE_BRACES
	}

	if forbid_none {
		if prefer_escape && !prefer_brace {
			bytes_needed += (extra - brace_count)
			if element[0] == '#' && (flags & t_TCL_DONT_QUOTE_HASH) == 0 {
				bytes_needed++
			}
			if (flags & t_TCL_DONT_USE_BRACES) != 0 {
				bytes_needed += brace_count
			}
			flags = t_CONVERT_MASK
			goto overflow_check
		}
		if (flags & t_TCL_DONT_USE_BRACES) != 0 {
			bytes_needed += extra
			if element[0] == '#' && (flags & t_TCL_DONT_QUOTE_HASH) == 0 {
				bytes_needed++
			}
		} else {
			bytes_needed += 2
		}
		flags = t_CONVERT_BRACE
		goto overflow_check
	}

	if element[0] == '#' && (flags & t_TCL_DONT_QUOTE_HASH) == 0 {
		bytes_needed += 2
	}
	flags = t_CONVERT_NONE

overflow_check:
	if bytes_needed < 0 {
		return 0, 0, fmt.Errorf("string length overflow")
	}
	return bytes_needed, flags, nil
}

func tcl_convert_element(src string, flags int) string {
	conversion := flags & t_CONVERT_MASK
	var p strings.Builder

	if (flags & t_TCL_DONT_USE_BRACES) != 0 && (conversion & t_CONVERT_BRACE) != 0{
		conversion = t_CONVERT_ESCAPE
	}
	if len(src) == 0 {
		conversion = t_CONVERT_BRACE
	} else {
		if src[0] == '#' && (flags & t_TCL_DONT_QUOTE_HASH) == 0 {
			if conversion == t_CONVERT_ESCAPE {
				p.WriteRune('\\')
				//p.WriteRune('#') we'll write this later
			} else {
				conversion = t_CONVERT_BRACE
			}
		}
	}

	if conversion == t_CONVERT_NONE {
		p.WriteString(src)
		return p.String()
	}

	if conversion == t_CONVERT_BRACE {
		p.WriteRune('{')
		p.WriteString(src)
		p.WriteRune('}')
		return p.String()
	}

	for _, r := range src {
		switch r {
			case ']', '[', '$', ';', ' ', '\\', '"':
				p.WriteRune('\\')
			case '{', '}':
				if conversion == t_CONVERT_ESCAPE {
					p.WriteRune('\\')
				}
			case '\f':
				p.WriteRune('\\')
				p.WriteRune('f')
			case '\n':
				p.WriteRune('\\')
				p.WriteRune('n')
			case '\r':
				p.WriteRune('\\')
				p.WriteRune('r')
			case '\t':
				p.WriteRune('\\')
				p.WriteRune('t')
			case '\v':
				p.WriteRune('\\')
				p.WriteRune('v')
		}
		p.WriteRune(r)
	}
	return p.String()
}
//
// END of ported Tcl core code.____________________________________________________________
//

// By contrast, the code to go the other direction (parsing Tcl strings as slices)
// is all original but seems to work fine. 
//
// rules about braces:
//	{ inside a string doesn't count (but still needs to be balanced)
//	} that ends a braced list can't be followed by trailing characters
//
// more formally:
// If the first character of an element is {/", then the element ends with the matching }/"
// The ending } MUST be present and MUST not be followed by any non-space runes.
// 
// TCL strings generated here SHOULD also conform to the following:
//  no newlines between elements
//  no unescaped ;,] except in quotes/braces
//  no unescaped $,[,\ except in braces
//  no unescaped # as first character of first element except in quotes/braces
//  don't put \<newline> in the string. Use \\\<newline>.

// ToTclString takes a slice of strings and outputs a single string value
// which represents that slice as a valid Tcl list. This function may return
// an error, but as currently implemented that should rarely happen (it is
// triggered by a string whose length is too long to fit in an integer),
// but there may be other error conditions added in the future, so check it
// anyway).
// It returns the Tcl string and the error, if any.
func ToTclString(listval []string) (string, error) {
	var s strings.Builder

	for element_idx, element := range listval {
		if element_idx > 0 {
			s.WriteRune(' ')
		}
		flags := 0
		if element_idx > 0 {
			flags = t_TCL_DONT_QUOTE_HASH
		}
		_, flags, err := tcl_scan_element(element, flags)
		if err != nil {
			return "", err
		}
		s.WriteString(tcl_convert_element(element, flags))
	}
	return s.String(), nil
}

// ParseTclList takes a properly-formatted Tcl list string
// and returns a slice of the list's elements as string values
// as well as an error (if something went wrong).
// Note that this only parses a single nesting level of elements,
// since with Tcl lists it is impossible to distintuish an element
// which happens to contain spaces from a nested list of values. It
// is simply up to the program to use the element as a string or as
// a sublist, so in the latter case you'll need to call ParseTclList
// on that element.
func ParseTclList(tcl_string string) ([]string, error) {
	l := make([]string, 0, 10)
	level := 0
	var s strings.Builder

	between_elements := true
	end_of_element := false
	braced_string := false
	literal_next := false
	quoted_string := false

	for _, r := range tcl_string {
		// step through the string representation of the list,
		// handling \{, \}, and \\ escapes as well as multiple
		// spaces between elements
		// We check for this specific set of whitespace characters
		// instead of unicode.IsSpace because the Tcl spec says so.
		if literal_next {
			if r != '{' && r != '}' && r != '\\' && r != '"' && r != ' ' && r != '#' {
				_,_= s.WriteRune('\\')
			}
			_,_ = s.WriteRune(r)
			literal_next = false
			continue
		}
		if r == '\\' {
			literal_next = true
			continue
		}
		if !braced_string && !quoted_string && strings.ContainsRune(" \t\n\v\f\r", r) {
			if between_elements {
				continue	// skip over multiple (superfluous) spaces
			}
			if end_of_element {
				end_of_element = false	// past that now
			}
			// not between elements? ship out what we were collecting
			// and start a new one
			l = append(l, s.String())
			s.Reset()
			between_elements = true
			braced_string = false
			quoted_string = false
		} else {
			if end_of_element {
				// we got superfluous text after a closing brace
				return l, fmt.Errorf("list element in braces (\"%s\") followed by '%c' instead of space", s.String(), r)
			}
			if r == '"' {
				if between_elements {
					// Quotes are just like braces except that they
					// can't really nest.
					between_elements = false
					quoted_string = true
					continue
				} else if quoted_string {
					quoted_string = false
					end_of_element = true
					continue
				}
			} else if r == '{' {
				level += 1
				if between_elements {
					// this is the brace that starts a string which
					// means it may allow leading spaces in the value
					// so let's not retain the brace but stop looking
					// for the next element
					between_elements = false
					braced_string = true
					continue
				}
			} else if r == '}' {
				level -= 1
				if level == 0 {
					if braced_string {
						// this should be the end of the string
						end_of_element = true
						braced_string = false
						continue // and don't keep the brace
					}
				}
				if level < 0 {
					return l, fmt.Errorf("too many right braces after \"%s\"", s.String())
				}
			}
			if between_elements {
				between_elements = false		// we're in an element now
			}
			_,_ = s.WriteRune(r)
		}
	}
	if !between_elements {
		l = append(l, s.String())
	}
	if level != 0 {
		return l, fmt.Errorf("unterminated brace at end of string")
	}
	if quoted_string {
		return l, fmt.Errorf("unterminated quote at end of string")
	}
	if literal_next {
		return l, fmt.Errorf("trailing backslash at end of string")
	}
	return l, nil
}
// @[00]@| GMA 4.2.2
// @[01]@|
// @[10]@| Copyright © 1992–2020 by Steven L. Willoughby
// @[11]@| (AKA Software Alchemy), Aloha, Oregon, USA. All Rights Reserved.
// @[12]@| Distributed under the terms and conditions of the BSD-3-Clause
// @[13]@| License as described in the accompanying LICENSE file distributed
// @[14]@| with GMA.
// @[15]@|
// @[20]@| Redistribution and use in source and binary forms, with or without
// @[21]@| modification, are permitted provided that the following conditions
// @[22]@| are met:
// @[23]@| 1. Redistributions of source code must retain the above copyright
// @[24]@|    notice, this list of conditions and the following disclaimer.
// @[25]@| 2. Redistributions in binary form must reproduce the above copy-
// @[26]@|    right notice, this list of conditions and the following dis-
// @[27]@|    claimer in the documentation and/or other materials provided
// @[28]@|    with the distribution.
// @[29]@| 3. Neither the name of the copyright holder nor the names of its
// @[30]@|    contributors may be used to endorse or promote products derived
// @[31]@|    from this software without specific prior written permission.
// @[32]@|
// @[33]@| THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND
// @[34]@| CONTRIBUTORS “AS IS” AND ANY EXPRESS OR IMPLIED WARRANTIES,
// @[35]@| INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
// @[36]@| MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// @[37]@| DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS
// @[38]@| BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY,
// @[39]@| OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
// @[40]@| PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR
// @[41]@| PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// @[42]@| THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR
// @[43]@| TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF
// @[44]@| THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
// @[45]@| SUCH DAMAGE.
// @[46]@|
// @[50]@| This software is not intended for any use or application in which
// @[51]@| the safety of lives or property would be at risk due to failure or
// @[52]@| defect of the software.
