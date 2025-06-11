/*
 * Copyright 2023 CloudWeGo Authors
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

package decoder

import (
	"fmt"
	"mime/multipart"
	"reflect"

	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
)

type fileTypeDecoder struct {
	*fieldInfo
}

var (
	fileBindingType = reflect.TypeOf((*multipart.FileHeader)(nil)).Elem()
)

func (d *fileTypeDecoder) Decode(req *protocol.Request, params param.Params, rv reflect.Value) (bool, error) {
	if d.IsRepeated() {
		return d.fileSliceDecode(req, params, rv)
	}

	var fileName string
	// file_name > form > fieldName
	for _, ti := range d.tagInfos {
		if ti.Tag == fileNameTag {
			fileName = ti.Name
			break
		}
		if ti.Tag == formTag {
			fileName = ti.Name
		}
	}
	if len(fileName) == 0 {
		fileName = d.fieldName
	}
	file, err := req.FormFile(fileName)
	if err != nil {
		logger().Warnf("can not get file '%s' form request, reason: %v, so skip '%s' field binding", fileName, err, d.fieldName)
		return false, nil
	}
	d.FieldValue(rv).Set(reflect.ValueOf(*file))
	return true, nil
}

func (d *fileTypeDecoder) fileSliceDecode(req *protocol.Request, params param.Params, rv reflect.Value) (bool, error) {
	var fileName string
	// file_name > form > fieldName
	for _, ti := range d.tagInfos {
		if ti.Tag == fileNameTag {
			fileName = ti.Name
			break
		}
		if ti.Tag == formTag {
			fileName = ti.Name
		}
	}
	if len(fileName) == 0 {
		fileName = d.fieldName
	}
	multipartForm, err := req.MultipartForm()
	if err != nil {
		logger().Warnf("can not get MultipartForm from request, reason: %v, so skip '%s' field binding", fileName, err, d.fieldName)
		return false, nil
	}
	files, exist := multipartForm.File[fileName]
	if !exist {
		logger().Warnf("the file '%s' is not existed in request, so skip '%s' field binding", fileName, d.fieldName)
		return false, nil
	}

	if ft := d.fieldType; ft.Kind() == reflect.Array && len(files) != ft.Len() {
		return false, fmt.Errorf("the numbers(%d) of file '%s' does not match the length(%d) of %s",
			len(files), fileName, ft.Len(), ft.String())
	}

	field := d.FieldValue(rv)
	if field.Kind() == reflect.Slice {
		field.Set(reflect.MakeSlice(field.Type(), len(files), len(files)))
	}
	for i, file := range files {
		v := dereference2lvalue(field.Index(i))
		v.Set(reflect.ValueOf(*file))
	}
	return true, nil
}
