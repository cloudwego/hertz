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
 */

package utils

import (
	"bytes"
	"fmt"
	"io"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/network"
)

var errBrokenChunk = errors.NewPublic("cannot find crlf at the end of chunk")

func ParseChunkSize(r network.Reader) (int, error) {
	n, err := bytesconv.ReadHexInt(r)
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return -1, err
	}
	for {
		c, err := r.ReadByte()
		if err != nil {
			return -1, errors.NewPublic(fmt.Sprintf("cannot read '\r' char at the end of chunk size: %s", err))
		}
		// Skip any trailing whitespace after chunk size.
		if c == ' ' {
			continue
		}
		if c != '\r' {
			return -1, errors.NewPublic(
				fmt.Sprintf("unexpected char %q at the end of chunk size. Expected %q", c, '\r'),
			)
		}
		break
	}
	c, err := r.ReadByte()
	if err != nil {
		return -1, errors.NewPublic(fmt.Sprintf("cannot read '\n' char at the end of chunk size: %s", err))
	}
	if c != '\n' {
		return -1, errors.NewPublic(
			fmt.Sprintf("unexpected char %q at the end of chunk size. Expected %q", c, '\n'),
		)
	}
	return n, nil
}

// SkipCRLF will only skip the next CRLF("\r\n"), otherwise, error will be returned.
func SkipCRLF(reader network.Reader) error {
	p, err := reader.Peek(len(bytestr.StrCRLF))
	if err != nil {
		return err
	}
	if !bytes.Equal(p, bytestr.StrCRLF) {
		return errBrokenChunk
	}

	reader.Skip(len(p)) // nolint: errcheck
	return nil
}
