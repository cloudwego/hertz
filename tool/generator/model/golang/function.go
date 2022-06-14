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

var function = `
{{define "Function"}}
func {{template "FuncBody" . -}}
{{end}}{{/* define "Function" */}}

{{define "FuncBody"}}
{{- .Name -}}(
{{- range $i, $arg := .Args -}}
{{- if gt $i 0}}, {{end -}}
{{$arg.Name}} {{$arg.Type.ResolveName ROOT}}
{{- end -}}{{/* range */}})
{{- if gt (len .Rets) 0}} ({{end -}}
{{- range $i, $ret := .Rets -}}
{{- if gt $i 0}}, {{end -}}
{{$ret.Type.ResolveName ROOT}}
{{- end -}}{{/* range */}}
{{- if gt (len .Rets) 0}}) {{end -}}{
{{.Code}}
}
{{end}}{{/* define "FuncBody" */}}
`

var method = `
{{define "Method"}}
func ({{.ReceiverName}} {{.ReceiverType.ResolveName ROOT}}) 
{{- template "FuncBody" .Function -}}
{{end}}
`
