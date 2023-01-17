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

package factory

import (
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/http1"
	"github.com/cloudwego/hertz/pkg/protocol/suite"
)

var _ suite.ServerFactory = (*serverFactory)(nil)

type serverFactory struct {
	option *http1.Option
}

// New is called by Hertz during engine.Run()
func (s *serverFactory) New(core suite.Core) (server protocol.Server, err error) {
	serv := http1.NewServer()
	serv.Option = *s.option
	serv.Core = core
	return serv, nil
}

func NewServerFactory(option *http1.Option) suite.ServerFactory {
	return &serverFactory{
		option: option,
	}
}
