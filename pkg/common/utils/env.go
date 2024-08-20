package utils

import (
	"os"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/pkg/common/errors"
)

// Get bool from env
func GetBoolFromEnv(key string) (bool, error) {
	value, isExist := os.LookupEnv(key)
	if !isExist {
		return false, errors.NewPublic("env not exist")
	}

	value = strings.TrimSpace(value)
	return strconv.ParseBool(value)
}
