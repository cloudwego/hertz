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
 * Copyright (c) 2015-present Aliaksandr Valialkin, VertaMedia, Kirill Danshin, Erik Dubbelboer, FastHTTP Authors
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2022 CloudWeGo Authors.
 */

package protocol

import (
	"bytes"
	"mime/multipart"
	"net/textproto"
	"os"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestWriteMultipartForm(t *testing.T) {
	t.Parallel()
	var w bytes.Buffer
	s := strings.Replace(`--foo
Content-Disposition: form-data; name="key"

value
--foo
Content-Disposition: form-data; name="file"; filename="test.json"
Content-Type: application/json

{"foo": "bar"}
--foo--
`, "\n", "\r\n", -1)
	mr := multipart.NewReader(strings.NewReader(s), "foo")
	form, err := mr.ReadForm(1024)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// The length of boundary is in the range of [1,70], which can be verified for strings outside this range.
	err = WriteMultipartForm(&w, form, s)
	assert.NotNil(t, err)

	// set Boundary as empty
	assert.Panic(t, func() {
		err = WriteMultipartForm(&w, form, "")
	})

	// call WriteField as twice
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err = mw.WriteField("field1", "value1"); err != nil {
		t.Fatal(err)
	}
	err = WriteMultipartForm(&w, form, s)
	assert.NotNil(t, err)

	// normal test
	err = WriteMultipartForm(&w, form, "foo")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if w.String() != s {
		t.Fatalf("unexpected output %q", w.Bytes())
	}
}

func TestParseMultipartForm(t *testing.T) {
	t.Parallel()
	s := strings.Replace(`--foo
Content-Disposition: form-data; name="key"

value
--foo--
`, "\n", "\r\n", -1)
	req1 := Request{}
	req1.SetMultipartFormBoundary("foo")
	// test size 0
	assert.NotNil(t, ParseMultipartForm(strings.NewReader(s), &req1, 0, 0))

	err := ParseMultipartForm(strings.NewReader(s), &req1, 1024, 1024)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	req2 := Request{}
	mr := multipart.NewReader(strings.NewReader(s), "foo")
	form, err := mr.ReadForm(1024)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	SetMultipartFormWithBoundary(&req2, form, "foo")
	assert.DeepEqual(t, &req1, &req2)

	// set Boundary as " "
	req1.SetMultipartFormBoundary(" ")
	err = ParseMultipartForm(strings.NewReader(s), &req1, 1024, 1024)
	assert.NotNil(t, err)

	// set size 0
	err = ParseMultipartForm(strings.NewReader(s), &req1, 0, 0)
	assert.NotNil(t, err)
}

func TestWriteMultipartFormFile(t *testing.T) {
	t.Parallel()
	bodyBuffer := &bytes.Buffer{}
	w := multipart.NewWriter(bodyBuffer)

	// read multipart.go to buf1
	f1, err := os.Open("./multipart.go")
	if err != nil {
		t.Fatalf("open file %s error: %s", f1.Name(), err)
	}
	defer f1.Close()

	multipartFile := File{
		Name:      f1.Name(),
		ParamName: "multipartCode",
		Reader:    f1,
	}

	err = WriteMultipartFormFile(w, multipartFile.ParamName, f1.Name(), multipartFile.Reader)
	if err != nil {
		t.Fatalf("write multipart error: %s", err)
	}

	fileInfo1, err := f1.Stat()
	if err != nil {
		t.Fatalf("get file state error: %s", err)
	}

	buf1 := make([]byte, fileInfo1.Size())
	_, err = f1.ReadAt(buf1, 0)
	if err != nil {
		t.Fatalf("read file to bytes error: %s", err)
	}
	assert.True(t, strings.Contains(bodyBuffer.String(), string(buf1)))

	// test file not found
	assert.NotNil(t, WriteMultipartFormFile(w, multipartFile.ParamName, "test.go", multipartFile.Reader))

	// Test Add File Function
	err = AddFile(w, "responseCode", "./response.go")
	if err != nil {
		t.Fatalf("add file error: %s", err)
	}

	// read response.go to buf2
	f2, err := os.Open("./response.go")
	if err != nil {
		t.Fatalf("open file %s error: %s", f2.Name(), err)
	}
	defer f2.Close()

	fileInfo2, err := f2.Stat()
	if err != nil {
		t.Fatalf("get file state error: %s", err)
	}
	buf2 := make([]byte, fileInfo2.Size())
	_, err = f2.ReadAt(buf2, 0)
	if err != nil {
		t.Fatalf("read file to bytes error: %s", err)
	}
	assert.True(t, strings.Contains(bodyBuffer.String(), string(buf2)))

	// test file not found
	err = AddFile(w, "responseCode", "./test.go")
	assert.NotNil(t, err)

	// test WriteMultipartFormFile without file name
	bodyBuffer = &bytes.Buffer{}
	w = multipart.NewWriter(bodyBuffer)
	// read multipart.go to buf1
	f3, err := os.Open("./multipart.go")
	if err != nil {
		t.Fatalf("open file %s error: %s", f3.Name(), err)
	}
	defer f3.Close()
	err = WriteMultipartFormFile(w, "multipart", " ", f3)
	if err != nil {
		t.Fatalf("write multipart error: %s", err)
	}
	assert.False(t, strings.Contains(bodyBuffer.String(), f3.Name()))

	// test empty file
	assert.Nil(t, WriteMultipartFormFile(w, "empty_test", "test.data", bytes.NewBuffer(nil)))
}

func TestMarshalMultipartForm(t *testing.T) {
	s := strings.Replace(`--foo
Content-Disposition: form-data; name="key"

value
--foo
Content-Disposition: form-data; name="file"; filename="test.json"
Content-Type: application/json

{"foo": "bar"}
--foo--
`, "\n", "\r\n", -1)
	mr := multipart.NewReader(strings.NewReader(s), "foo")
	form, err := mr.ReadForm(1024)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	bufs, err := MarshalMultipartForm(form, "foo")
	assert.Nil(t, err)
	assert.DeepEqual(t, s, string(bufs))

	// set boundary invalid
	_, err = MarshalMultipartForm(form, " ")
	assert.NotNil(t, err)
}

func TestAddFile(t *testing.T) {
	t.Parallel()
	bodyBuffer := &bytes.Buffer{}
	w := multipart.NewWriter(bodyBuffer)
	// add null file
	err := AddFile(w, "test", "/test")
	assert.NotNil(t, err)
}

func TestCreateMultipartHeader(t *testing.T) {
	t.Parallel()

	// filename == Null
	hdr1 := make(textproto.MIMEHeader)
	hdr1.Set("Content-Disposition", `form-data; name="test"`)
	hdr1.Set("Content-Type", "application/json")
	assert.DeepEqual(t, hdr1, CreateMultipartHeader("test", "", "application/json"))

	// normal test
	hdr2 := make(textproto.MIMEHeader)
	hdr2.Set("Content-Disposition", `form-data; name="test"; filename="/test.go"`)
	hdr2.Set("Content-Type", "application/json")
	assert.DeepEqual(t, hdr2, CreateMultipartHeader("test", "/test.go", "application/json"))
}
