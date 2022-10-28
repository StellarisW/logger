/*
 * Copyright 2022 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * The MIT License (MIT)
 *
 * Copyright (c) 2016 Aliaksandr Valialkin, VertaMedia
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.

 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2022 CloudWeGo Authors.
 */

package fasttemplate

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"unsafe"

	"github.com/cloudwego/hertz/pkg/common/bytebufferpool"
)

// ExecuteFunc calls f on each template tag (placeholder) occurrence.
//
// Returns the number of bytes written to w.
//
// This function is optimized for constantly changing templates.
// Use Template.ExecuteFunc for frozen templates.
func ExecuteFunc(template, startTag, endTag string, w io.Writer, f TagFunc) (int64, error) {
	s := s2b(template)
	a := s2b(startTag)
	b := s2b(endTag)

	var nn int64
	var ni int
	var err error
	for {
		n := bytes.Index(s, a)
		if n < 0 {
			break
		}
		ni, err = w.Write(s[:n])
		nn += int64(ni)
		if err != nil {
			return nn, err
		}

		s = s[n+len(a):]
		n = bytes.Index(s, b)
		if n < 0 {
			// cannot find end tag - just write it to the output.
			ni, _ = w.Write(a)
			nn += int64(ni)
			break
		}

		ni, err = f(w, b2s(s[:n]))
		nn += int64(ni)
		if err != nil {
			return nn, err
		}
		s = s[n+len(b):]
	}
	ni, err = w.Write(s)
	nn += int64(ni)

	return nn, err
}

// Execute substitutes template tags (placeholders) with the corresponding
// values from the map m and writes the result to the given writer w.
//
// Substitution map m may contain values with the following types:
//   - []byte - the fastest value type
//   - string - convenient value type
//   - TagFunc - flexible value type
//
// Returns the number of bytes written to w.
//
// This function is optimized for constantly changing templates.
// Use Template.Execute for frozen templates.
func Execute(template, startTag, endTag string, w io.Writer, m map[string]interface{}) (int64, error) {
	return ExecuteFunc(template, startTag, endTag, w, func(w io.Writer, tag string) (int, error) { return stdTagFunc(w, tag, m) })
}

// ExecuteStd works the same way as Execute, but keeps the unknown placeholders.
// This can be used as a drop-in replacement for strings.Replacer
//
// Substitution map m may contain values with the following types:
//   - []byte - the fastest value type
//   - string - convenient value type
//   - TagFunc - flexible value type
//
// Returns the number of bytes written to w.
//
// This function is optimized for constantly changing templates.
// Use Template.ExecuteStd for frozen templates.
func ExecuteStd(template, startTag, endTag string, w io.Writer, m map[string]interface{}) (int64, error) {
	return ExecuteFunc(template, startTag, endTag, w, func(w io.Writer, tag string) (int, error) { return keepUnknownTagFunc(w, startTag, endTag, tag, m) })
}

// ExecuteFuncString calls f on each template tag (placeholder) occurrence
// and substitutes it with the data written to TagFunc's w.
//
// Returns the resulting string.
//
// This function is optimized for constantly changing templates.
// Use Template.ExecuteFuncString for frozen templates.
func ExecuteFuncString(template, startTag, endTag string, f TagFunc) string {
	s, err := ExecuteFuncStringWithErr(template, startTag, endTag, f)
	if err != nil {
		panic(fmt.Sprintf("unexpected error: %s", err))
	}
	return s
}

// ExecuteFuncStringWithErr is nearly the same as ExecuteFuncString
// but when f returns an error, ExecuteFuncStringWithErr won't panic like ExecuteFuncString
// it just returns an empty string and the error f returned
func ExecuteFuncStringWithErr(template, startTag, endTag string, f TagFunc) (string, error) {
	tagsCount := bytes.Count(s2b(template), s2b(startTag))
	if tagsCount == 0 {
		return template, nil
	}

	bb := byteBufferPool.Get()
	if _, err := ExecuteFunc(template, startTag, endTag, bb, f); err != nil {
		bb.Reset()
		byteBufferPool.Put(bb)
		return "", err
	}
	s := string(bb.B)
	bb.Reset()
	byteBufferPool.Put(bb)
	return s, nil
}

var byteBufferPool bytebufferpool.Pool

// ExecuteString substitutes template tags (placeholders) with the corresponding
// values from the map m and returns the result.
//
// Substitution map m may contain values with the following types:
//   - []byte - the fastest value type
//   - string - convenient value type
//   - TagFunc - flexible value type
//
// This function is optimized for constantly changing templates.
// Use Template.ExecuteString for frozen templates.
func ExecuteString(template, startTag, endTag string, m map[string]interface{}) string {
	return ExecuteFuncString(template, startTag, endTag, func(w io.Writer, tag string) (int, error) { return stdTagFunc(w, tag, m) })
}

// ExecuteStringStd works the same way as ExecuteString, but keeps the unknown placeholders.
// This can be used as a drop-in replacement for strings.Replacer
//
// Substitution map m may contain values with the following types:
//   - []byte - the fastest value type
//   - string - convenient value type
//   - TagFunc - flexible value type
//
// This function is optimized for constantly changing templates.
// Use Template.ExecuteStringStd for frozen templates.
func ExecuteStringStd(template, startTag, endTag string, m map[string]interface{}) string {
	return ExecuteFuncString(template, startTag, endTag, func(w io.Writer, tag string) (int, error) { return keepUnknownTagFunc(w, startTag, endTag, tag, m) })
}

// Template implements simple template engine, which can be used for fast
// tags' (aka placeholders) substitution.
type Template struct {
	template string
	startTag string
	endTag   string

	texts          [][]byte
	tags           []string
	byteBufferPool bytebufferpool.Pool
}

// New parses the given template using the given startTag and endTag
// as tag start and tag end.
//
// The returned template can be executed by concurrently running goroutines
// using Execute* methods.
//
// New panics if the given template cannot be parsed. Use NewTemplate instead
// if template may contain errors.
func New(template, startTag, endTag string) *Template {
	t, err := NewTemplate(template, startTag, endTag)
	if err != nil {
		panic(err)
	}
	return t
}

// NewTemplate parses the given template using the given startTag and endTag
// as tag start and tag end.
//
// The returned template can be executed by concurrently running goroutines
// using Execute* methods.
func NewTemplate(template, startTag, endTag string) (*Template, error) {
	var t Template
	err := t.Reset(template, startTag, endTag)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// TagFunc can be used as a substitution value in the map passed to Execute*.
// Execute* functions pass tag (placeholder) name in 'tag' argument.
//
// TagFunc must be safe to call from concurrently running goroutines.
//
// TagFunc must write contents to w and return the number of bytes written.
type TagFunc func(w io.Writer, tag string) (int, error)

// Reset resets the template t to new one defined by
// template, startTag and endTag.
//
// Reset allows Template object re-use.
//
// Reset may be called only if no other goroutines call t methods at the moment.
func (t *Template) Reset(template, startTag, endTag string) error {
	// Keep these vars in t, so GC won't collect them and won't break
	// vars derived via unsafe*
	t.template = template
	t.startTag = startTag
	t.endTag = endTag
	t.texts = t.texts[:0]
	t.tags = t.tags[:0]

	if len(startTag) == 0 {
		panic("startTag cannot be empty")
	}
	if len(endTag) == 0 {
		panic("endTag cannot be empty")
	}

	s := s2b(template)
	a := s2b(startTag)
	b := s2b(endTag)

	tagsCount := bytes.Count(s, a)
	if tagsCount == 0 {
		return nil
	}

	if tagsCount+1 > cap(t.texts) {
		t.texts = make([][]byte, 0, tagsCount+1)
	}
	if tagsCount > cap(t.tags) {
		t.tags = make([]string, 0, tagsCount)
	}

	for {
		n := bytes.Index(s, a)
		if n < 0 {
			t.texts = append(t.texts, s)
			break
		}
		t.texts = append(t.texts, s[:n])

		s = s[n+len(a):]
		n = bytes.Index(s, b)
		if n < 0 {
			return fmt.Errorf("cannot find end tag=%q in the template=%q starting from %q", endTag, template, s)
		}

		t.tags = append(t.tags, b2s(s[:n]))
		s = s[n+len(b):]
	}

	return nil
}

// ExecuteFunc calls f on each template tag (placeholder) occurrence.
//
// Returns the number of bytes written to w.
//
// This function is optimized for frozen templates.
// Use ExecuteFunc for constantly changing templates.
func (t *Template) ExecuteFunc(w io.Writer, f TagFunc) (int64, error) {
	var nn int64

	n := len(t.texts) - 1
	if n == -1 {
		ni, err := w.Write(s2b(t.template))
		return int64(ni), err
	}

	for i := 0; i < n; i++ {
		ni, err := w.Write(t.texts[i])
		nn += int64(ni)
		if err != nil {
			return nn, err
		}

		ni, err = f(w, t.tags[i])
		nn += int64(ni)
		if err != nil {
			return nn, err
		}
	}
	ni, err := w.Write(t.texts[n])
	nn += int64(ni)
	return nn, err
}

// Execute substitutes template tags (placeholders) with the corresponding
// values from the map m and writes the result to the given writer w.
//
// Substitution map m may contain values with the following types:
//   - []byte - the fastest value type
//   - string - convenient value type
//   - TagFunc - flexible value type
//
// Returns the number of bytes written to w.
func (t *Template) Execute(w io.Writer, m map[string]interface{}) (int64, error) {
	return t.ExecuteFunc(w, func(w io.Writer, tag string) (int, error) { return stdTagFunc(w, tag, m) })
}

// ExecuteStd works the same way as Execute, but keeps the unknown placeholders.
// This can be used as a drop-in replacement for strings.Replacer
//
// Substitution map m may contain values with the following types:
//   - []byte - the fastest value type
//   - string - convenient value type
//   - TagFunc - flexible value type
//
// Returns the number of bytes written to w.
func (t *Template) ExecuteStd(w io.Writer, m map[string]interface{}) (int64, error) {
	return t.ExecuteFunc(w, func(w io.Writer, tag string) (int, error) { return keepUnknownTagFunc(w, t.startTag, t.endTag, tag, m) })
}

// ExecuteFuncString calls f on each template tag (placeholder) occurrence
// and substitutes it with the data written to TagFunc's w.
//
// Returns the resulting string.
//
// This function is optimized for frozen templates.
// Use ExecuteFuncString for constantly changing templates.
func (t *Template) ExecuteFuncString(f TagFunc) string {
	s, err := t.ExecuteFuncStringWithErr(f)
	if err != nil {
		panic(fmt.Sprintf("unexpected error: %s", err))
	}
	return s
}

// ExecuteFuncStringWithErr calls f on each template tag (placeholder) occurrence
// and substitutes it with the data written to TagFunc's w.
//
// Returns the resulting string.
//
// This function is optimized for frozen templates.
// Use ExecuteFuncString for constantly changing templates.
func (t *Template) ExecuteFuncStringWithErr(f TagFunc) (string, error) {
	bb := t.byteBufferPool.Get()
	if _, err := t.ExecuteFunc(bb, f); err != nil {
		bb.Reset()
		t.byteBufferPool.Put(bb)
		return "", err
	}
	s := string(bb.Bytes())
	bb.Reset()
	t.byteBufferPool.Put(bb)
	return s, nil
}

// ExecuteString substitutes template tags (placeholders) with the corresponding
// values from the map m and returns the result.
//
// Substitution map m may contain values with the following types:
//   - []byte - the fastest value type
//   - string - convenient value type
//   - TagFunc - flexible value type
//
// This function is optimized for frozen templates.
// Use ExecuteString for constantly changing templates.
func (t *Template) ExecuteString(m map[string]interface{}) string {
	return t.ExecuteFuncString(func(w io.Writer, tag string) (int, error) { return stdTagFunc(w, tag, m) })
}

// ExecuteStringStd works the same way as ExecuteString, but keeps the unknown placeholders.
// This can be used as a drop-in replacement for strings.Replacer
//
// Substitution map m may contain values with the following types:
//   - []byte - the fastest value type
//   - string - convenient value type
//   - TagFunc - flexible value type
//
// This function is optimized for frozen templates.
// Use ExecuteStringStd for constantly changing templates.
func (t *Template) ExecuteStringStd(m map[string]interface{}) string {
	return t.ExecuteFuncString(func(w io.Writer, tag string) (int, error) { return keepUnknownTagFunc(w, t.startTag, t.endTag, tag, m) })
}

func stdTagFunc(w io.Writer, tag string, m map[string]interface{}) (int, error) {
	v := m[tag]
	if v == nil {
		return 0, nil
	}
	switch value := v.(type) {
	case []byte:
		return w.Write(value)
	case string:
		return w.Write([]byte(value))
	case TagFunc:
		return value(w, tag)
	default:
		panic(fmt.Sprintf("tag=%q contains unexpected value type=%#v. Expected []byte, string or TagFunc", tag, v))
	}
}

func keepUnknownTagFunc(w io.Writer, startTag, endTag, tag string, m map[string]interface{}) (int, error) {
	v, ok := m[tag]
	if !ok {
		if _, err := w.Write(s2b(startTag)); err != nil {
			return 0, err
		}
		if _, err := w.Write(s2b(tag)); err != nil {
			return 0, err
		}
		if _, err := w.Write(s2b(endTag)); err != nil {
			return 0, err
		}
		return len(startTag) + len(tag) + len(endTag), nil
	}
	if v == nil {
		return 0, nil
	}
	switch value := v.(type) {
	case []byte:
		return w.Write(value)
	case string:
		return w.Write([]byte(value))
	case TagFunc:
		return value(w, tag)
	default:
		panic(fmt.Sprintf("tag=%q contains unexpected value type=%#v. Expected []byte, string or TagFunc", tag, v))
	}
}

func b2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func s2b(s string) (b []byte) {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	bh.Data = sh.Data
	bh.Cap = sh.Len
	bh.Len = sh.Len
	return b
}