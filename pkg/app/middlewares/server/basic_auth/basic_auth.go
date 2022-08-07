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
 * The MIT License (MIT)
 *
 * Copyright (c) 2014 Manuel Martínez-Almeida
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
 * Modifications are Copyright 2022 CloudWeGo Authors
 */

package basic_auth

import (
	"context"
	"encoding/base64"
	"net/http"
	"strconv"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/pkg/app"
)

// Accounts is an alias to map[string]string, construct with {"username":"password"}
type Accounts map[string]string

// pairs is an alias to map[string]string, which mean {"header":"username"}
type pairs map[string]string

func (p pairs) findValue(needle string) (v string, ok bool) {
	v, ok = p[needle]
	return
}

func constructPairs(accounts Accounts) pairs {
	length := len(accounts)
	p := make(pairs, length)
	for user, password := range accounts {
		value := "Basic " + base64.StdEncoding.EncodeToString(bytesconv.S2b(user+":"+password))
		p[value] = user
	}
	return p
}

// BasicAuthForRealm returns a Basic HTTP Authorization middleware. It takes as arguments a map[string]string where
// the key is the username and the value is the password, as well as the name of the Realm.
// If the realm is empty, "Authorization Required" will be used by default.
// (see http://tools.ietf.org/html/rfc2617#section-1.2)
func BasicAuthForRealm(accounts Accounts, realm, userKey string) app.HandlerFunc {
	realm = "Basic realm=" + strconv.Quote(realm)
	p := constructPairs(accounts)
	return func(ctx context.Context, c *app.RequestContext) {
		// Search user in the slice of allowed credentials
		user, found := p.findValue(c.Request.Header.Get("Authorization"))
		if !found {
			// Credentials doesn't match, we return 401 and abort handlers chain.
			c.Header("WWW-Authenticate", realm)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// The user credentials was found, set user's id to key AuthUserKey in this context, the user's id can be read later using
		c.Set(userKey, user)
	}
}

// BasicAuth is a constructor of BasicAuth verifier to hertz middleware
// It returns a Basic HTTP Authorization middleware. It takes as argument a map[string]string where
// the key is the username and the value is the password.
func BasicAuth(accounts Accounts) app.HandlerFunc {
	return BasicAuthForRealm(accounts, "Authorization Required", "user")
}
