package binding_v2

import (
	"reflect"
	"strings"
)

const (
	pathTag    = "path"
	formTag    = "form"
	queryTag   = "query"
	cookieTag  = "cookie"
	headerTag  = "header"
	jsonTag    = "json"
	rawBodyTag = "raw_body"
)

const (
	requiredTagOpt = "required"
)

type TagInfo struct {
	Key      string
	Value    string
	Required bool
	Default  string
	Options  []string
	Getter   getter
}

func head(str, sep string) (head, tail string) {
	idx := strings.Index(str, sep)
	if idx < 0 {
		return str, ""
	}
	return str[:idx], str[idx+len(sep):]
}

func lookupFieldTags(field reflect.StructField) []TagInfo {
	var ret []string
	tags := []string{pathTag, formTag, queryTag, cookieTag, headerTag, jsonTag, rawBodyTag}
	for _, tag := range tags {
		if _, ok := field.Tag.Lookup(tag); ok {
			ret = append(ret, tag)
		}
	}
	var tagInfos []TagInfo

	for _, tag := range ret {
		tagContent := field.Tag.Get(tag)
		tagValue, opts := head(tagContent, ",")
		var options []string
		var opt string
		var required bool
		for len(opts) > 0 {
			opt, opts = head(opts, ",")
			options = append(options, opt)
			if opt == requiredTagOpt {
				required = true
			}
		}
		tagInfos = append(tagInfos, TagInfo{Key: tag, Value: tagValue, Options: options, Required: required})
	}

	return tagInfos
}
