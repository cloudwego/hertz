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

package protobuf

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/cmd/hz/internal/config"
	"github.com/cloudwego/hertz/cmd/hz/internal/generator"
	"github.com/cloudwego/hertz/cmd/hz/internal/generator/model"
	"github.com/cloudwego/hertz/cmd/hz/internal/protobuf/api"
	"github.com/cloudwego/hertz/cmd/hz/internal/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoimpl"
	"google.golang.org/protobuf/types/descriptorpb"
)

var (
	jsonSnakeName  = false
	unsetOmitempty = false
)

func CheckTagOption(args *config.Argument) (ret []generator.Option) {
	if args == nil {
		return
	}
	if args.SnakeName {
		jsonSnakeName = true
	}
	if args.UnsetOmitempty {
		unsetOmitempty = true
	}
	if args.JSONEnumStr {
		ret = append(ret, generator.OptionMarshalEnumToText)
	}
	return ret
}

func checkSnakeName(name string) string {
	if jsonSnakeName {
		name = util.ToSnakeCase(name)
	}
	return name
}

var (
	HttpMethodOptions = map[*protoimpl.ExtensionInfo]string{
		api.E_Get:     "GET",
		api.E_Post:    "POST",
		api.E_Put:     "PUT",
		api.E_Patch:   "PATCH",
		api.E_Delete:  "DELETE",
		api.E_Options: "OPTIONS",
		api.E_Head:    "HEAD",
		api.E_Any:     "Any",
	}

	BindingTags = map[*protoimpl.ExtensionInfo]string{
		api.E_Path:    "path",
		api.E_Query:   "query",
		api.E_Form:    "form",
		api.E_Header:  "header",
		api.E_Cookie:  "cookie",
		api.E_Body:    "json",
		api.E_RawBody: "raw_body",
	}

	ValidatorTags = map[*protoimpl.ExtensionInfo]string{api.E_Vd: "vd"}

	SerializerOptions = map[*protoimpl.ExtensionInfo]string{api.E_Serializer: "serializer"}
)

func checkFirstOptions(extensions map[*protoimpl.ExtensionInfo]string, opts ...protoreflect.ProtoMessage) (string, interface{}) {
	for _, opt := range opts {
		for e, t := range extensions {
			if proto.HasExtension(opt, e) {
				v := proto.GetExtension(opt, e)
				return t, v
			}
		}
	}
	return "", nil
}

func checkFirstOption(ext *protoimpl.ExtensionInfo, opts ...protoreflect.ProtoMessage) interface{} {
	for _, opt := range opts {
		if proto.HasExtension(opt, ext) {
			v := proto.GetExtension(opt, ext)
			return v
		}
	}
	return nil
}

func checkOption(ext *protoimpl.ExtensionInfo, opts ...protoreflect.ProtoMessage) (ret []interface{}) {
	for _, opt := range opts {
		if proto.HasExtension(opt, ext) {
			v := proto.GetExtension(opt, ext)
			ret = append(ret, v)
		}
	}
	return
}

func tag(k string, v interface{}) model.Tag {
	return model.Tag{
		Key:   k,
		Value: fmt.Sprintf("%v", v),
	}
}

//-----------------------------------For Compiler---------------------------

func defaultBindingTags(f *descriptorpb.FieldDescriptorProto) []model.Tag {
	opts := f.GetOptions()
	out := make([]model.Tag, 3)
	if v := checkFirstOption(api.E_Body, opts); v != nil {
		val := getJsonValue(f, v.(string))
		out[0] = tag("json", val)
	} else {
		out[0] = jsonTag(f)
	}
	if v := checkFirstOption(api.E_Query, opts); v != nil {
		val := checkRequire(f, v.(string))
		out[1] = tag(BindingTags[api.E_Query], val)
	} else {
		val := checkRequire(f, checkSnakeName(f.GetName()))
		out[1] = tag(BindingTags[api.E_Query], val)
	}
	if v := checkFirstOption(api.E_Form, opts); v != nil {
		val := checkRequire(f, v.(string))
		out[2] = tag(BindingTags[api.E_Form], val)
	} else {
		val := checkRequire(f, checkSnakeName(f.GetName()))
		out[2] = tag(BindingTags[api.E_Form], val)
	}
	return out
}

func jsonTag(f *descriptorpb.FieldDescriptorProto) (ret model.Tag) {
	ret.Key = "json"
	ret.Value = checkSnakeName(f.GetJsonName())
	if v := checkFirstOption(api.E_JsConv, f.GetOptions()); v != nil {
		ret.Value += ",string"
	}
	if !unsetOmitempty && f.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL {
		ret.Value += ",omitempty"
	} else if f.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REQUIRED {
		ret.Value += ",required"
	}
	return
}

func injectTagsToModel(f *descriptorpb.FieldDescriptorProto, gf *model.Field, needDefault bool) error {
	as := f.GetOptions()

	tags := gf.Tags
	if tags == nil {
		tags = make([]model.Tag, 0, 4)
	}

	// binding tags
	if needDefault {
		tags = append(tags, defaultBindingTags(f)...)
	}
	for k, v := range BindingTags {
		if vv := checkFirstOption(k, as); vv != nil {
			tags.Remove(v)
			if v == "json" {
				vv = getJsonValue(f, vv.(string))
			} else {
				vv = checkRequire(f, vv.(string))
			}
			tags = append(tags, tag(v, vv))
		}
	}

	// validator tags
	for k, v := range ValidatorTags {
		for _, vv := range checkOption(k, as) {
			tags = append(tags, tag(v, vv))
		}
	}

	// go.tags
	for _, v := range checkOption(api.E_GoTag, as) {
		gts := util.SplitGoTags(v.(string))
		for _, gt := range gts {
			sp := strings.SplitN(gt, ":", 2)
			if len(sp) != 2 {
				return fmt.Errorf("invalid go tag: %s", v)
			}
			vv, err := strconv.Unquote(sp[1])
			if err != nil {
				return fmt.Errorf("invalid go.tag value: %s, err: %v", sp[1], err.Error())
			}
			key := sp[0]
			tags.Remove(key)
			tags = append(tags, model.Tag{
				Key:   key,
				Value: vv,
			})
		}
	}

	sort.Sort(tags)
	gf.Tags = tags
	return nil
}

func getJsonValue(f *descriptorpb.FieldDescriptorProto, val string) string {
	if v := checkFirstOption(api.E_JsConv, f.GetOptions()); v != nil {
		val += ",string"
	}
	if !unsetOmitempty && f.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL {
		val += ",omitempty"
	} else if f.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REQUIRED {
		val += ",required"
	}

	return val
}

func checkRequire(f *descriptorpb.FieldDescriptorProto, val string) string {
	if f.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REQUIRED {
		val += ",required"
	}

	return val
}

//-------------------------For plugin---------------------------------

func m2s(mt model.Tag) (ret [2]string) {
	ret[0] = mt.Key
	ret[1] = mt.Value
	return ret
}

func reflectJsonTag(f protoreflect.FieldDescriptor) (ret model.Tag) {
	ret.Key = "json"
	ret.Value = checkSnakeName(f.JSONName())
	if v := checkFirstOption(api.E_Body, f.Options()); v != nil {
		ret.Value += ",string"
	}
	if !unsetOmitempty && descriptorpb.FieldDescriptorProto_Label(f.Cardinality()) == descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL {
		ret.Value += ",omitempty"
	} else if descriptorpb.FieldDescriptorProto_Label(f.Cardinality()) == descriptorpb.FieldDescriptorProto_LABEL_REQUIRED {
		ret.Value += ",required"
	}
	return
}

func defaultBindingStructTags(f protoreflect.FieldDescriptor) []model.Tag {
	opts := f.Options()
	out := make([]model.Tag, 3)
	if v := checkFirstOption(api.E_Body, opts); v != nil {
		val := getStructJsonValue(f, v.(string))
		out[0] = tag("json", val)
	} else {
		out[0] = reflectJsonTag(f)
	}
	if v := checkFirstOption(api.E_Query, opts); v != nil {
		val := checkStructRequire(f, v.(string))
		out[1] = tag(BindingTags[api.E_Query], val)
	} else {
		val := checkStructRequire(f, checkSnakeName(string(f.Name())))
		out[1] = tag(BindingTags[api.E_Query], val)
	}
	if v := checkFirstOption(api.E_Form, opts); v != nil {
		val := checkStructRequire(f, v.(string))
		out[2] = tag(BindingTags[api.E_Form], val)
	} else {
		val := checkStructRequire(f, checkSnakeName(string(f.Name())))
		out[2] = tag(BindingTags[api.E_Form], val)
	}
	return out
}

func injectTagsToStructTags(f protoreflect.FieldDescriptor, out *structTags, needDefault bool) error {
	as := f.Options()
	// binding tags
	tags := model.Tags(make([]model.Tag, 0, 6))

	if needDefault {
		tags = append(tags, defaultBindingStructTags(f)...)
	}
	for k, v := range BindingTags {
		if vv := checkFirstOption(k, as); vv != nil {
			tags.Remove(v)
			if v == "json" {
				vv = getStructJsonValue(f, vv.(string))
			} else {
				vv = checkStructRequire(f, vv.(string))
			}
			tags = append(tags, tag(v, vv))
		}
	}

	// validator tags
	for k, v := range ValidatorTags {
		if vv := checkFirstOption(k, as); vv != nil {
			tags = append(tags, tag(v, vv))
		}
	}

	if v := checkFirstOption(api.E_GoTag, as); v != nil {
		gts := util.SplitGoTags(v.(string))
		for _, gt := range gts {
			sp := strings.SplitN(gt, ":", 2)
			if len(sp) != 2 {
				return fmt.Errorf("invalid go tag: %s", v)
			}
			vv, err := strconv.Unquote(sp[1])
			if err != nil {
				return fmt.Errorf("invalid go.tag value: %s, err: %v", sp[1], err.Error())
			}
			key := sp[0]
			tags.Remove(key)
			tags = append(tags, model.Tag{
				Key:   key,
				Value: vv,
			})
		}
	}

	sort.Sort(tags)
	for _, t := range tags {
		*out = append(*out, m2s(t))
	}
	return nil
}

func getStructJsonValue(f protoreflect.FieldDescriptor, val string) string {
	if v := checkFirstOption(api.E_JsConv, f.Options()); v != nil {
		val += ",string"
	}
	if !unsetOmitempty && descriptorpb.FieldDescriptorProto_Label(f.Cardinality()) == descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL {
		val += ",omitempty"
	} else if descriptorpb.FieldDescriptorProto_Label(f.Cardinality()) == descriptorpb.FieldDescriptorProto_LABEL_REQUIRED {
		val += ",required"
	}

	return val
}

func checkStructRequire(f protoreflect.FieldDescriptor, val string) string {
	if descriptorpb.FieldDescriptorProto_Label(f.Cardinality()) == descriptorpb.FieldDescriptorProto_LABEL_REQUIRED {
		val += ",required"
	}

	return val
}
