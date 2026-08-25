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
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var firstTime = true

type MockDoer struct {
	mock.Mock
}

type doerFunc func(ctx context.Context, req *protocol.Request, resp *protocol.Response) error

func (f doerFunc) Do(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
	return f(ctx, req, resp)
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

func TestDoRequestFollowRedirectsDropsSensitiveHeadersOnCrossHost(t *testing.T) {
	var calls int
	req := &protocol.Request{}
	resp := &protocol.Response{}
	req.Header.Set(consts.HeaderAuthorization, "Bearer secret")
	req.Header.Set(consts.HeaderWWWAuthenticate, "Basic realm=secret")
	req.Header.Set(consts.HeaderProxyAuthorization, "Basic secret")
	req.Header.Set(consts.HeaderProxyAuthenticate, "Basic realm=proxy")
	req.Header.SetCookie("sid", "secret")
	req.Header.Set(consts.HeaderCookie2, "sid=secret")
	req.Header.Set("X-Test", "kept")

	doer := doerFunc(func(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
		if len(req.Header.Host()) == 0 {
			req.Header.SetHostBytes(req.URI().Host())
		}

		calls++
		switch calls {
		case 1:
			assert.Equal(t, "trusted.example", string(req.Header.Host()))
			assert.Equal(t, "Bearer secret", string(req.Header.Peek(consts.HeaderAuthorization)))
			assert.Equal(t, "Basic realm=secret", string(req.Header.Peek(consts.HeaderWWWAuthenticate)))
			assert.Equal(t, "Basic secret", string(req.Header.Peek(consts.HeaderProxyAuthorization)))
			assert.Equal(t, "Basic realm=proxy", string(req.Header.Peek(consts.HeaderProxyAuthenticate)))
			assert.Equal(t, "sid=secret", string(req.Header.Peek(consts.HeaderCookie)))
			assert.Equal(t, "sid=secret", string(req.Header.Peek(consts.HeaderCookie2)))
			resp.Header.SetCanonical(bytestr.StrLocation, []byte("https://attacker.example/next"))
			resp.SetStatusCode(consts.StatusFound)
		case 2:
			assert.Equal(t, "attacker.example", string(req.Header.Host()))
			assert.Empty(t, req.Header.Peek(consts.HeaderAuthorization))
			assert.Empty(t, req.Header.Peek(consts.HeaderWWWAuthenticate))
			assert.Empty(t, req.Header.Peek(consts.HeaderProxyAuthorization))
			assert.Empty(t, req.Header.Peek(consts.HeaderProxyAuthenticate))
			assert.Empty(t, req.Header.Peek(consts.HeaderCookie))
			assert.Empty(t, req.Header.Peek(consts.HeaderCookie2))
			assert.Equal(t, "kept", string(req.Header.Peek("X-Test")))
			resp.SetStatusCode(consts.StatusOK)
		default:
			t.Fatalf("unexpected redirect call %d", calls)
		}
		return nil
	})

	statusCode, _, err := DoRequestFollowRedirects(context.Background(), req, resp, "https://trusted.example/start", defaultMaxRedirectsCount, doer)
	assert.NoError(t, err)
	assert.Equal(t, consts.StatusOK, statusCode)
	assert.Equal(t, 2, calls)
}

func TestDoRequestFollowRedirectsKeepsSensitiveHeadersOnSameHost(t *testing.T) {
	var calls int
	req := &protocol.Request{}
	resp := &protocol.Response{}
	req.Header.Set(consts.HeaderAuthorization, "Bearer secret")
	req.Header.SetCookie("sid", "secret")

	doer := doerFunc(func(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
		if len(req.Header.Host()) == 0 {
			req.Header.SetHostBytes(req.URI().Host())
		}

		calls++
		switch calls {
		case 1:
			resp.Header.SetCanonical(bytestr.StrLocation, []byte("/next"))
			resp.SetStatusCode(consts.StatusFound)
		case 2:
			assert.Equal(t, "trusted.example", string(req.Header.Host()))
			assert.Equal(t, "Bearer secret", string(req.Header.Peek(consts.HeaderAuthorization)))
			assert.Equal(t, "sid=secret", string(req.Header.Peek(consts.HeaderCookie)))
			resp.SetStatusCode(consts.StatusOK)
		default:
			t.Fatalf("unexpected redirect call %d", calls)
		}
		return nil
	})

	statusCode, _, err := DoRequestFollowRedirects(context.Background(), req, resp, "https://trusted.example/start", defaultMaxRedirectsCount, doer)
	assert.NoError(t, err)
	assert.Equal(t, consts.StatusOK, statusCode)
	assert.Equal(t, 2, calls)
}

func TestDoRequestFollowRedirectsDropsSensitiveHeadersOnHTTPSDowngrade(t *testing.T) {
	var calls int
	req := &protocol.Request{}
	resp := &protocol.Response{}
	req.Header.Set(consts.HeaderAuthorization, "Bearer secret")
	req.Header.Set(consts.HeaderWWWAuthenticate, "Basic realm=secret")
	req.Header.Set(consts.HeaderProxyAuthorization, "Basic secret")
	req.Header.Set(consts.HeaderProxyAuthenticate, "Basic realm=proxy")
	req.Header.SetCookie("sid", "secret")
	req.Header.Set(consts.HeaderCookie2, "sid=secret")

	doer := doerFunc(func(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
		if len(req.Header.Host()) == 0 {
			req.Header.SetHostBytes(req.URI().Host())
		}

		calls++
		switch calls {
		case 1:
			assert.Equal(t, "trusted.example", string(req.Header.Host()))
			assert.Equal(t, "Bearer secret", string(req.Header.Peek(consts.HeaderAuthorization)))
			resp.Header.SetCanonical(bytestr.StrLocation, []byte("http://trusted.example/next"))
			resp.SetStatusCode(consts.StatusFound)
		case 2:
			assert.Equal(t, "trusted.example", string(req.Header.Host()))
			assert.Empty(t, req.Header.Peek(consts.HeaderAuthorization))
			assert.Empty(t, req.Header.Peek(consts.HeaderWWWAuthenticate))
			assert.Empty(t, req.Header.Peek(consts.HeaderProxyAuthorization))
			assert.Empty(t, req.Header.Peek(consts.HeaderProxyAuthenticate))
			assert.Empty(t, req.Header.Peek(consts.HeaderCookie))
			assert.Empty(t, req.Header.Peek(consts.HeaderCookie2))
			resp.SetStatusCode(consts.StatusOK)
		default:
			t.Fatalf("unexpected redirect call %d", calls)
		}
		return nil
	})

	statusCode, _, err := DoRequestFollowRedirects(context.Background(), req, resp, "https://trusted.example/start", defaultMaxRedirectsCount, doer)
	assert.NoError(t, err)
	assert.Equal(t, consts.StatusOK, statusCode)
	assert.Equal(t, 2, calls)
}

func TestDoRequestFollowRedirectsMissingLocation(t *testing.T) {
	doer := doerFunc(func(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
		resp.SetStatusCode(consts.StatusFound)
		return nil
	})

	statusCode, _, err := DoRequestFollowRedirects(context.Background(), &protocol.Request{}, &protocol.Response{}, "https://trusted.example/start", defaultMaxRedirectsCount, doer)
	assert.ErrorIs(t, err, errMissingLocation)
	assert.Equal(t, consts.StatusFound, statusCode)
}

func TestDoRequestFollowRedirectsTooManyRedirects(t *testing.T) {
	doer := doerFunc(func(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
		resp.Header.SetCanonical(bytestr.StrLocation, []byte("/next"))
		resp.SetStatusCode(consts.StatusFound)
		return nil
	})

	statusCode, _, err := DoRequestFollowRedirects(context.Background(), &protocol.Request{}, &protocol.Response{}, "https://trusted.example/start", 0, doer)
	assert.ErrorIs(t, err, errTooManyRedirects)
	assert.Equal(t, consts.StatusFound, statusCode)
}

func TestGetRedirectURLWithAbsoluteURLInQuery(t *testing.T) {
	redirectURL := getRedirectURL("https://example.com/", []byte("/login/redirect_to_sso?redirect=https://example.com/"))
	assert.Equal(t, "https://example.com/login/redirect_to_sso?redirect=https://example.com/", redirectURL)
}
