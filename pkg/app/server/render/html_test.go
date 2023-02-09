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

package render

import (
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestHTMLDebug_StartChecker_timer(t *testing.T) {
	render := &HTMLDebug{RefreshInterval: time.Second}
	select {
	case <-render.reloadCh:
		t.Fatalf("should not be triggered")
	default:
	}
	render.startChecker()
	select {
	case <-time.After(render.RefreshInterval + 100*time.Millisecond):
		t.Fatalf("should be triggered in 1 second")
	case <-render.reloadCh:
	}
}

func TestHTMLDebug_StartChecker_fs_watcher(t *testing.T) {
	f, _ := ioutil.TempFile("./", "test.tmpl")
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()
	render := &HTMLDebug{Files: []string{f.Name()}}
	select {
	case <-render.reloadCh:
		t.Fatalf("should not be triggered")
	default:
	}
	render.startChecker()
	f.Write([]byte("hello"))
	f.Sync()
	select {
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("should be triggered immediately")
	case <-render.reloadCh:
	}
	select {
	case <-render.reloadCh:
		t.Fatalf("should not be triggered")
	default:
	}
}
