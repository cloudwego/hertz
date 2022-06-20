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

package adaptor

import (
	"net/http"

	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type compatResponse struct {
	h           *protocol.Response
	header      http.Header
	writeHeader bool
}

func (c *compatResponse) Header() http.Header {
	if c.header != nil {
		return c.header
	}
	c.header = make(map[string][]string)
	return c.header
}

func (c *compatResponse) Write(b []byte) (int, error) {
	if !c.writeHeader {
		c.WriteHeader(consts.StatusOK)
	}

	return c.h.BodyWriter().Write(b)
}

func (c *compatResponse) WriteHeader(statusCode int) {
	if !c.writeHeader {
		for k, v := range c.header {
			for _, vv := range v {
				if k == consts.HeaderContentLength {
					continue
				}
				if k == consts.HeaderSetCookie {
					cookie := protocol.AcquireCookie()
					cookie.Parse(vv)
					c.h.Header.SetCookie(cookie)
					continue
				}
				c.h.Header.Add(k, vv)
			}
		}
		c.writeHeader = true
	}

	c.h.Header.SetStatusCode(statusCode)
}

// GetCompatResponseWriter only support basic function of ResponseWriter, not for all.
func GetCompatResponseWriter(resp *protocol.Response) http.ResponseWriter {
	c := &compatResponse{
		h: resp,
	}
	c.h.Header.SetNoDefaultContentType(true)

	h := make(map[string][]string)
	tmpKey := make([][]byte, 0, c.h.Header.Len())
	c.h.Header.VisitAll(func(k, v []byte) {
		h[string(k)] = append(h[string(k)], string(v))
		tmpKey = append(tmpKey, k)
	})

	for _, k := range tmpKey {
		c.h.Header.DelBytes(k)
	}

	c.header = h
	return c
}
