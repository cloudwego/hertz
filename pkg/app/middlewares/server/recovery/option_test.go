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

package recovery

import (
	"context"
	"fmt"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestDefaultOption(t *testing.T) {
	opts := newOptions()
	assert.DeepEqual(t, fmt.Sprintf("%p", defaultRecoveryHandler), fmt.Sprintf("%p", opts.recoveryHandler))
}

func newRecoveryHandler(c context.Context, ctx *app.RequestContext, err interface{}, stack []byte) {
	hlog.SystemLogger().CtxErrorf(c, "[New Recovery] panic recovered:\n%s\n%s\n",
		err, stack)
	ctx.JSON(consts.StatusNotImplemented, utils.H{"msg": err.(string)})
}

func TestOption(t *testing.T) {
	opts := newOptions(WithRecoveryHandler(newRecoveryHandler))
	assert.DeepEqual(t, fmt.Sprintf("%p", newRecoveryHandler), fmt.Sprintf("%p", opts.recoveryHandler))
}
