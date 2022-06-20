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

package golang

var oneof = `
{{define "Oneof"}}
type {{$.InterfaceName}} interface {
	{{$.InterfaceName}}()
}

{{range $i, $f := .Choices}}
type {{$f.MessageName}}_{{$f.ChoiceName}} struct {
	{{$f.ChoiceName}} {{$f.Type.ResolveName ROOT}}
}
{{end}}

{{range $i, $f := .Choices}}
func (*{{$f.MessageName}}_{{$f.ChoiceName}}) {{$.InterfaceName}}() {}
{{end}}

{{range $i, $f := .Choices}}
func (p *{{$f.MessageName}}) Get{{$f.ChoiceName}}() {{$f.Type.ResolveName ROOT}} {
	if p, ok := p.Get{{$.OneofName}}().(*{{$f.MessageName}}_{{$f.ChoiceName}}); ok {
		return p.{{$f.ChoiceName}}
	}
	return {{$f.Type.ResolveDefaultValue}}
}
{{end}}

{{end}}
`
