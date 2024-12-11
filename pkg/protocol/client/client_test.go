/*
 * Copyright 2024 CloudWeGo Authors
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
package client

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var firstTime = true

type MockDoer struct {
	mock.Mock
}

func (m *MockDoer) Do(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {

	// this is the real logic in (c *HostClient) doNonNilReqResp method
	if len(req.Header.Host()) == 0 {
		req.Header.SetHostBytes(req.URI().Host())
	}

	if firstTime {
		// req.Header.Host() is the real host writing to the wire
		if string(req.Header.Host()) != "example.com" {
			return errors.New("host not match")
		}
		// this is the real logic in (c *HostClient) doNonNilReqResp method
		if len(req.Header.Host()) == 0 {
			req.Header.SetHostBytes(req.URI().Host())
		}
		resp.Header.SetCanonical(bytestr.StrLocation, []byte("https://a.b.c/foo"))
		resp.SetStatusCode(301)
		firstTime = false
		return nil
	}

	if string(req.Header.Host()) != "a.b.c" {
		resp.SetStatusCode(400)
		return errors.New("host not match")
	}

	resp.SetStatusCode(200)

	return nil
}

func TestDoRequestFollowRedirects(t *testing.T) {
	mockDoer := new(MockDoer)
	mockDoer.On("Do", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	statusCode, _, err := DoRequestFollowRedirects(context.Background(), &protocol.Request{}, &protocol.Response{}, "https://example.com", defaultMaxRedirectsCount, mockDoer)
	assert.NoError(t, err)
	assert.Equal(t, 200, statusCode)
}
