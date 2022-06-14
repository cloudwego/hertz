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

package util

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
)

func CopyStringSlice(from, to *[]string) {
	n := len(*from)
	m := len(*to)
	if n > m {
		n = m
	}
	for i := 0; i < n; i++ {
		(*to)[i] = (*from)[i]
	}
	*to = (*to)[:n]
}

func CopyString2StringMap(from, to map[string]string) {
	for k := range to {
		delete(to, k)
	}
	for k, v := range from {
		to[k] = v
	}
}

func PackArgs(c interface{}) (res []string, err error) {
	t := reflect.TypeOf(c)
	v := reflect.ValueOf(c)
	if reflect.TypeOf(c).Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, errors.New("passed c must be struct or pointer of struct")
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		x := v.Field(i)
		n := f.Name

		if x.IsZero() {
			continue
		}

		switch x.Kind() {
		case reflect.Bool:
			if x.Bool() == false {
				continue
			}
			res = append(res, n+"="+fmt.Sprint(x.Bool()))
		case reflect.String:
			if x.String() == "" {
				continue
			}
			res = append(res, n+"="+x.String())
		case reflect.Slice:
			if x.Len() == 0 {
				continue
			}
			ft := f.Type.Elem()
			if ft.Kind() != reflect.String {
				return nil, fmt.Errorf("slice field %v must be '[]string', err: %v", f.Name, err.Error())
			}
			var ss []string
			for i := 0; i < x.Len(); i++ {
				ss = append(ss, x.Index(i).String())
			}
			res = append(res, n+"="+strings.Join(ss, ";"))
		case reflect.Map:
			if x.Len() == 0 {
				continue
			}
			fk := f.Type.Key()
			if fk.Kind() != reflect.String {
				return nil, fmt.Errorf("map field %v must be 'map[string]string', err: %v", f.Name, err.Error())
			}
			fv := f.Type.Elem()
			if fv.Kind() != reflect.String {
				return nil, fmt.Errorf("map field %v must be 'map[string]string', err: %v", f.Name, err.Error())
			}
			var sk []string
			it := x.MapRange()
			for it.Next() {
				sk = append(sk, it.Key().String()+"="+it.Value().String())
			}
			res = append(res, n+"="+strings.Join(sk, ";"))
		default:
			return nil, fmt.Errorf("unsupported field type: %+v, err: %v", f, err.Error())
		}
	}
	return res, nil
}

func UnpackArgs(args []string, c interface{}) error {
	m, err := MapForm(args)
	if err != nil {
		return fmt.Errorf("unmarshal args failed, err: %v", err.Error())
	}

	t := reflect.TypeOf(c).Elem()
	v := reflect.ValueOf(c).Elem()
	if t.Kind() != reflect.Struct {
		return errors.New("passed c must be struct or pointer of struct")
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		x := v.Field(i)
		n := f.Name
		values, ok := m[n]
		if !ok || len(values) == 0 || values[0] == "" {
			continue
		}
		switch x.Kind() {
		case reflect.Bool:
			if len(values) != 1 {
				return fmt.Errorf("field %s can't be assgined multi values: %v", n, values)
			}
			x.SetBool(values[0] == "true")
		case reflect.String:
			if len(values) != 1 {
				return fmt.Errorf("field %s can't be assgined multi values: %v", n, values)
			}
			x.SetString(values[0])
		case reflect.Slice:

		case reflect.Map:
			if len(values) != 1 {
				return fmt.Errorf("field %s can't be assgined multi values: %v", n, values)
			}
			ss := strings.Split(values[0], ";")
			out := make(map[string]string, len(ss))
			for _, s := range ss {
				sk := strings.SplitN(s, "=", 2)
				if len(sk) != 2 {
					return fmt.Errorf("map filed %v invalid key-value pair '%v'", n, s)
				}
				out[sk[0]] = sk[1]
			}
			x.Set(reflect.ValueOf(out))
		default:
			return fmt.Errorf("field %s has unsupported type %+v", n, f.Type)
		}
	}
	return nil
}

func MapForm(input []string) (map[string][]string, error) {
	out := make(map[string][]string, len(input))

	for _, str := range input {
		parts := strings.SplitN(str, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid argument: '%s'", str)
		}
		key, val := parts[0], parts[1]
		out[key] = append(out[key], val)
	}

	return out, nil
}

func GetFirstKV(m map[string][]string) (string, []string) {
	for k, v := range m {
		return k, v
	}
	return "", nil
}

func GetColonPair(str string) (string, string) {
	ps := strings.Split(str, ":")
	if len(ps) != 2 {
		return "", ""
	}
	return ps[0], ps[1]
}

func ToCamelCase(name string) string {
	return CamelString(name)
}

func ToSnakeCase(name string) string {
	return SnakeString(name)
}

// BaseName get base name for path. ex: "github.com/p.s.m" => "p.s.m"
func BaseName(include, subFixToTrim string) string {
	last := include
	if id := strings.LastIndex(last, "/"); id >= 0 && id < len(last)-1 {
		last = last[id+1:]
	}
	if !strings.HasSuffix(last, subFixToTrim) {
		return last
	}
	return last[:len(last)-len(subFixToTrim)]
}

func BaseNameAndTrim(include string) string {
	last := include
	if id := strings.LastIndex(last, "/"); id >= 0 && id < len(last)-1 {
		last = last[id+1:]
	}

	if id := strings.LastIndex(last, "."); id != -1 {
		last = last[:id]
	}
	return last
}

func SplitPackageName(pkg, subFixToTrim string) string {
	last := SplitPackage(pkg, subFixToTrim)
	if id := strings.LastIndex(last, "/"); id >= 0 && id < len(last)-1 {
		last = last[id+1:]
	}
	return last
}

func SplitPackage(pkg, subFixToTrim string) string {
	last := strings.TrimSuffix(pkg, subFixToTrim)
	if id := strings.LastIndex(last, "/"); id >= 0 && id < len(last)-1 {
		last = last[id+1:]
	}
	return strings.ReplaceAll(last, ".", "/")
}

func PathToImport(path, subFix string) string {
	path = strings.TrimSuffix(path, subFix)
	// path = RelativePath(path)
	return strings.ReplaceAll(path, string(filepath.Separator), "/")
}

func ImportToPath(path, subFix string) string {
	// path = RelativePath(path)
	return strings.ReplaceAll(path, "/", string(filepath.Separator)) + subFix
}

func ImportToPathAndConcat(path, subFix string) string {
	path = strings.TrimSuffix(path, subFix)
	path = strings.ReplaceAll(path, "/", string(filepath.Separator))
	if i := strings.LastIndex(path, string(filepath.Separator)); i >= 0 && i < len(path)-1 && strings.Contains(path[i+1:], ".") {
		base := strings.ReplaceAll(path[i+1:], ".", "_")
		dir := path[:i]
		return dir + string(filepath.Separator) + base
	}
	return path
}

func ToVarName(paths []string) string {
	ps := strings.Join(paths, "__")
	input := []byte(url.PathEscape(ps))
	out := make([]byte, 0, len(input))
	for i := 0; i < len(input); i++ {
		c := input[i]
		if c == ':' || c == '*' {
			continue
		}
		if (c >= '0' && c <= '9' && i != 0) || (c >= 'a' && c <= 'z') || (c > 'A' && c <= 'Z') || (c == '_') {
			out = append(out, c)
		} else {
			out = append(out, '_')
		}
	}

	return string(out)
}

func SplitGoTags(input string) []string {
	out := make([]string, 0, 4)
	ns := len(input)

	flag := false
	prev := 0
	i := 0
	for i = 0; i < ns; i++ {
		c := input[i]
		if c == '"' {
			flag = !flag
		}
		if !flag && c == ' ' {
			if prev < i {
				out = append(out, input[prev:i])
			}
			prev = i + 1
		}
	}
	if i != 0 && prev < i {
		out = append(out, input[prev:i])
	}

	return out
}

func SubPackage(mod, dir string) string {
	if dir == "" {
		return mod
	}
	return mod + "/" + PathToImport(dir, "")
}

func SubDir(root, subPkg string) string {
	if root == "" {
		return ImportToPath(subPkg, "")
	}
	return filepath.Join(root, ImportToPath(subPkg, ""))
}
