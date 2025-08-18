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

package generator

import "testing"

func Test_checkDupRegister(t *testing.T) {
	type args struct {
		file      []byte
		insertReg string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "dup tab",
			args: args{
				file:      []byte("package main\n\nimport (\n\t\"hertz.io/hertz/pkg/app\"\n)\n\nfunc register() {\n\tapp.Register(r)\n}"),
				insertReg: "app.Register(r)\n",
			},
			want: true,
		},
		{
			name: "dup space",
			args: args{
				file:      []byte("package main\n\nimport (\n\t\"hertz.io/hertz/pkg/app\"\n)\n\nfunc register() {\n   app.Register(r)\n}"),
				insertReg: "app.Register(r)\n",
			},
			want: true,
		},
		{
			name: "not dup prefix",
			args: args{
				file:      []byte("package main\n\nimport (\n\t\"hertz.io/hertz/pkg/app_2\"\n)\n\nfunc register() {\n\tapp_2.Register(r)\n}"),
				insertReg: "app.Register(r)\n",
			},
			want: false,
		},
		{
			name: "not dup subfix",
			args: args{
				file:      []byte("package main\n\nimport (\n\t\"hertz.io/hertz/pkg/xapp\"\n)\n\nfunc register() {\n xapp.Register(r)\n}"),
				insertReg: "app.Register(r)\n",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkDupRegister(tt.args.file, tt.args.insertReg); got != tt.want {
				t.Errorf("checkDupRegister() = %v, want %v", got, tt.want)
			}
		})
	}
}
