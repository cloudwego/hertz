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
 * Copyright (c) 2021 LabStack
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

package route

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route/param"
)

type router struct {
	method        string
	root          *node
	hasTsrHandler map[string]bool
}

type MethodTrees []*router

func (trees MethodTrees) get(method string) *router {
	for _, tree := range trees {
		if tree.method == method {
			return tree
		}
	}
	return nil
}

func countParams(path string) uint16 {
	var n uint16
	s := bytesconv.S2b(path)
	n += uint16(bytes.Count(s, bytestr.StrColon))
	n += uint16(bytes.Count(s, bytestr.StrStar))
	return n
}

type (
	node struct {
		kind     kind
		label    byte
		prefix   string
		parent   *node
		children children
		// original path
		ppath string
		// param names
		pnames     []string
		handlers   app.HandlersChain
		paramChild *node
		anyChild   *node
		// isLeaf indicates that node does not have child routes
		isLeaf bool
	}
	kind     uint8
	children []*node
)

const (
	// static kind
	skind kind = iota
	// param kind
	pkind
	// all kind
	akind
	paramLabel = byte(':')
	anyLabel   = byte('*')
	slash      = "/"
	nilString  = ""
)

func checkPathValid(path string) {
	if path == nilString {
		panic("empty path")
	}
	if path[0] != '/' {
		panic("path must begin with '/'")
	}
	for i, c := range []byte(path) {
		switch c {
		case ':':
			if (i < len(path)-1 && path[i+1] == '/') || i == (len(path)-1) {
				panic("wildcards must be named with a non-empty name in path '" + path + "'")
			}
			i++
			for ; i < len(path) && path[i] != '/'; i++ {
				if path[i] == ':' || path[i] == '*' {
					panic("only one wildcard per path segment is allowed, find multi in path '" + path + "'")
				}
			}
		case '*':
			if i == len(path)-1 {
				panic("wildcards must be named with a non-empty name in path '" + path + "'")
			}
			if i > 0 && path[i-1] != '/' {
				panic(" no / before wildcards in path " + path)
			}
			for ; i < len(path); i++ {
				if path[i] == '/' {
					panic("catch-all routes are only allowed at the end of the path in path '" + path + "'")
				}
			}
		}
	}
}

// addRoute adds a node with the given handle to the path.
func (r *router) addRoute(path string, h app.HandlersChain) {
	checkPathValid(path)

	var (
		pnames []string // Param names
		ppath  = path   // Pristine path
	)

	if h == nil {
		panic(fmt.Sprintf("Adding route without handler function: %v", path))
	}

	// Add the front static route part of a non-static route
	for i, lcpIndex := 0, len(path); i < lcpIndex; i++ {
		// param route
		if path[i] == paramLabel {
			j := i + 1

			r.insert(path[:i], nil, skind, nilString, nil)
			for ; i < lcpIndex && path[i] != '/'; i++ {
			}

			pnames = append(pnames, path[j:i])
			path = path[:j] + path[i:]
			i, lcpIndex = j, len(path)

			if i == lcpIndex {
				// path node is last fragment of route path. ie. `/users/:id`
				r.insert(path[:i], h, pkind, ppath, pnames)
				return
			} else {
				r.insert(path[:i], nil, pkind, nilString, pnames)
			}
		} else if path[i] == anyLabel {
			r.insert(path[:i], nil, skind, nilString, nil)
			pnames = append(pnames, path[i+1:])
			r.insert(path[:i+1], h, akind, ppath, pnames)
			return
		}
	}

	r.insert(path, h, skind, ppath, pnames)
}

func (r *router) insert(path string, h app.HandlersChain, t kind, ppath string, pnames []string) {
	currentNode := r.root
	if currentNode == nil {
		panic("hertz: invalid node")
	}
	search := path

	for {
		searchLen := len(search)
		prefixLen := len(currentNode.prefix)
		lcpLen := 0

		max := prefixLen
		if searchLen < max {
			max = searchLen
		}
		for ; lcpLen < max && search[lcpLen] == currentNode.prefix[lcpLen]; lcpLen++ {
		}

		if lcpLen == 0 {
			// At root node
			currentNode.label = search[0]
			currentNode.prefix = search
			if h != nil {
				currentNode.kind = t
				currentNode.handlers = h
				currentNode.ppath = ppath
				currentNode.pnames = pnames
			}
			currentNode.isLeaf = currentNode.children == nil && currentNode.paramChild == nil && currentNode.anyChild == nil
		} else if lcpLen < prefixLen {
			// Split node
			n := newNode(
				currentNode.kind,
				currentNode.prefix[lcpLen:],
				currentNode,
				currentNode.children,
				currentNode.handlers,
				currentNode.ppath,
				currentNode.pnames,
				currentNode.paramChild,
				currentNode.anyChild,
			)
			// Update parent path for all children to new node
			for _, child := range currentNode.children {
				child.parent = n
			}
			if currentNode.paramChild != nil {
				currentNode.paramChild.parent = n
			}
			if currentNode.anyChild != nil {
				currentNode.anyChild.parent = n
			}

			// Reset parent node
			currentNode.kind = skind
			currentNode.label = currentNode.prefix[0]
			currentNode.prefix = currentNode.prefix[:lcpLen]
			currentNode.children = nil
			currentNode.handlers = nil
			currentNode.ppath = nilString
			currentNode.pnames = nil
			currentNode.paramChild = nil
			currentNode.anyChild = nil
			currentNode.isLeaf = false

			// Only Static children could reach here
			currentNode.children = append(currentNode.children, n)

			if lcpLen == searchLen {
				// At parent node
				currentNode.kind = t
				currentNode.handlers = h
				currentNode.ppath = ppath
				currentNode.pnames = pnames
			} else {
				// Create child node
				n = newNode(t, search[lcpLen:], currentNode, nil, h, ppath, pnames, nil, nil)
				// Only Static children could reach here
				currentNode.children = append(currentNode.children, n)
			}
			currentNode.isLeaf = currentNode.children == nil && currentNode.paramChild == nil && currentNode.anyChild == nil
		} else if lcpLen < searchLen {
			search = search[lcpLen:]
			c := currentNode.findChildWithLabel(search[0])
			if c != nil {
				// Go deeper
				currentNode = c
				continue
			}
			// Create child node
			n := newNode(t, search, currentNode, nil, h, ppath, pnames, nil, nil)
			switch t {
			case skind:
				currentNode.children = append(currentNode.children, n)
			case pkind:
				currentNode.paramChild = n
			case akind:
				currentNode.anyChild = n
			}
			currentNode.isLeaf = currentNode.children == nil && currentNode.paramChild == nil && currentNode.anyChild == nil
		} else {
			// Node already exists
			if currentNode.handlers != nil && h != nil {
				panic("handlers are already registered for path '" + ppath + "'")
			}

			if h != nil {
				currentNode.handlers = h
				currentNode.ppath = ppath
				currentNode.pnames = pnames
			}
		}
		return
	}
}

// find finds registered handler by method and path, parses URL params and puts params to context
func (r *router) find(path string, paramsPointer *param.Params, unescape bool) (res nodeValue) {
	var (
		cn          = r.root // current node
		search      = path   // current path
		searchIndex = 0
		buf         []byte
		paramIndex  int
	)

	backtrackToNextNodeKind := func(fromKind kind) (nextNodeKind kind, valid bool) {
		previous := cn
		cn = previous.parent
		valid = cn != nil

		// Next node type by priority
		if previous.kind == akind {
			nextNodeKind = skind
		} else {
			nextNodeKind = previous.kind + 1
		}

		if fromKind == skind {
			// when backtracking is done from static kind block we did not change search so nothing to restore
			return
		}

		// restore search to value it was before we move to current node we are backtracking from.
		if previous.kind == skind {
			searchIndex -= len(previous.prefix)
		} else {
			paramIndex--
			// for param/any node.prefix value is always `:` so we can not deduce searchIndex from that and must use pValue
			// for that index as it would also contain part of path we cut off before moving into node we are backtracking from
			searchIndex -= len((*paramsPointer)[paramIndex].Value)
			(*paramsPointer) = (*paramsPointer)[:paramIndex]
		}
		search = path[searchIndex:]
		return
	}

	// search order: static > param > any
	for {
		if cn.kind == skind {
			if len(search) >= len(cn.prefix) && cn.prefix == search[:len(cn.prefix)] {
				// Continue search
				search = search[len(cn.prefix):]
				searchIndex = searchIndex + len(cn.prefix)
			} else {
				// not equal
				if (len(cn.prefix) == len(search)+1) &&
					(cn.prefix[len(search)]) == '/' && cn.prefix[:len(search)] == search && (cn.handlers != nil || cn.anyChild != nil) {
					res.tsr = true
				}
				// No matching prefix, let's backtrack to the first possible alternative node of the decision path
				nk, ok := backtrackToNextNodeKind(skind)
				if !ok {
					return // No other possibilities on the decision path
				} else if nk == pkind {
					goto Param
				} else {
					// Not found (this should never be possible for static node we are looking currently)
					break
				}
			}
		}
		if search == nilString && len(cn.handlers) != 0 {
			res.handlers = cn.handlers
			break
		}

		// Static node
		if search != nilString {
			// If it can execute that logic, there is handler registered on the current node and search is `/`.
			if search == "/" && cn.handlers != nil {
				res.tsr = true
			}
			if child := cn.findChild(search[0]); child != nil {
				cn = child
				continue
			}
		}

		if search == nilString {
			if cd := cn.findChild('/'); cd != nil && (cd.handlers != nil || cd.anyChild != nil) {
				res.tsr = true
			}
		}

	Param:
		// Param node
		if child := cn.paramChild; search != nilString && child != nil {
			cn = child
			i := strings.Index(search, slash)
			if i == -1 {
				i = len(search)
			}
			(*paramsPointer) = (*paramsPointer)[:(paramIndex + 1)]
			val := search[:i]
			if unescape {
				if v, err := url.QueryUnescape(search[:i]); err == nil {
					val = v
				}
			}
			(*paramsPointer)[paramIndex].Value = val
			paramIndex++
			search = search[i:]
			searchIndex = searchIndex + i
			if search == nilString {
				if cd := cn.findChild('/'); cd != nil && (cd.handlers != nil || cd.anyChild != nil) {
					res.tsr = true
				}
			}
			continue
		}
	Any:
		// Any node
		if child := cn.anyChild; child != nil {
			// If any node is found, use remaining path for paramValues
			cn = child
			(*paramsPointer) = (*paramsPointer)[:(paramIndex + 1)]
			index := len(cn.pnames) - 1
			val := search
			if unescape {
				if v, err := url.QueryUnescape(search); err == nil {
					val = v
				}
			}

			(*paramsPointer)[index].Value = bytesconv.B2s(append(buf, val...))
			// update indexes/search in case we need to backtrack when no handler match is found
			paramIndex++
			searchIndex += len(search)
			search = nilString
			res.handlers = cn.handlers
			break
		}

		// Let's backtrack to the first possible alternative node of the decision path
		nk, ok := backtrackToNextNodeKind(akind)
		if !ok {
			break // No other possibilities on the decision path
		} else if nk == pkind {
			goto Param
		} else if nk == akind {
			goto Any
		} else {
			// Not found
			break
		}
	}

	if cn != nil {
		res.fullPath = cn.ppath
		for i, name := range cn.pnames {
			(*paramsPointer)[i].Key = name
		}
	}

	return
}

func (n *node) findChild(l byte) *node {
	for _, c := range n.children {
		if c.label == l {
			return c
		}
	}
	return nil
}

func (n *node) findChildWithLabel(l byte) *node {
	for _, c := range n.children {
		if c.label == l {
			return c
		}
	}
	if l == paramLabel {
		return n.paramChild
	}
	if l == anyLabel {
		return n.anyChild
	}
	return nil
}

func newNode(t kind, pre string, p *node, child children, mh app.HandlersChain, ppath string, pnames []string, paramChildren, anyChildren *node) *node {
	return &node{
		kind:       t,
		label:      pre[0],
		prefix:     pre,
		parent:     p,
		children:   child,
		ppath:      ppath,
		pnames:     pnames,
		handlers:   mh,
		paramChild: paramChildren,
		anyChild:   anyChildren,
		isLeaf:     child == nil && paramChildren == nil && anyChildren == nil,
	}
}

// nodeValue holds return values of (*Node).getValue method
type nodeValue struct {
	handlers app.HandlersChain
	tsr      bool
	fullPath string
}

// Makes a case-insensitive lookup of the given path and tries to find a handler.
// It returns the case-corrected path and a bool indicating whether the lookup
// was successful.
func (n *node) findCaseInsensitivePath(path string, fixTrailingSlash bool) (ciPath []byte, found bool) {
	ciPath = make([]byte, 0, len(path)+1) // preallocate enough memory
	// Match paramKind.
	if n.label == paramLabel {
		end := 0
		for end < len(path) && path[end] != '/' {
			end++
		}
		ciPath = append(ciPath, path[:end]...)
		if end < len(path) {
			if len(n.children) > 0 {
				path = path[end:]

				goto loop
			}

			if fixTrailingSlash && len(path) == end+1 {
				return ciPath, true
			}
			return
		}

		if n.handlers != nil {
			return ciPath, true
		}
		if fixTrailingSlash && len(n.children) == 1 {
			// No handle found. Check if a handle for this path with(without) a trailing slash exists
			n = n.children[0]
			if n.prefix == "/" && n.handlers != nil {
				return append(ciPath, '/'), true
			}
		}
		return
	}

	// Match allKind.
	if n.label == anyLabel {
		return append(ciPath, path...), true
	}

	// Match static kind.
	if len(path) >= len(n.prefix) && strings.EqualFold(path[:len(n.prefix)], n.prefix) {
		path = path[len(n.prefix):]
		ciPath = append(ciPath, n.prefix...)

		if len(path) == 0 {
			if n.handlers != nil {
				return ciPath, true
			}
			// No handle found.
			// Try to fix the path by adding a trailing slash.
			if fixTrailingSlash {
				for i := 0; i < len(n.children); i++ {
					if n.children[i].label == '/' {
						n = n.children[i]
						if (len(n.prefix) == 1 && n.handlers != nil) ||
							(n.prefix == "*" && n.children[0].handlers != nil) {
							return append(ciPath, '/'), true
						}
						return
					}
				}
			}
			return
		}
	} else if fixTrailingSlash {
		// Nothing found.
		// Try to fix the path by adding / removing a trailing slash.
		if path == "/" {
			return ciPath, true
		}
		if len(path)+1 == len(n.prefix) && n.prefix[len(path)] == '/' &&
			strings.EqualFold(path, n.prefix[:len(path)]) &&
			n.handlers != nil {
			return append(ciPath, n.prefix...), true
		}
	}

loop:
	// First match static kind.
	for _, node := range n.children {
		if unicode.ToLower(rune(path[0])) == unicode.ToLower(rune(node.label)) {
			out, found := node.findCaseInsensitivePath(path, fixTrailingSlash)
			if found {
				return append(ciPath, out...), true
			}
		}
	}

	if n.paramChild != nil {
		out, found := n.paramChild.findCaseInsensitivePath(path, fixTrailingSlash)
		if found {
			return append(ciPath, out...), true
		}
	}

	if n.anyChild != nil {
		out, found := n.anyChild.findCaseInsensitivePath(path, fixTrailingSlash)
		if found {
			return append(ciPath, out...), true
		}
	}

	// Nothing found. We can recommend to redirect to the same URL
	// without a trailing slash if a leaf exists for that path
	found = fixTrailingSlash && path == "/" && n.handlers != nil
	return
}
