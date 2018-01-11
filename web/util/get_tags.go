// Copyright 2018, RadiantBlue Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type tagsRunner struct {
	name     string
	fullName string
}

func newTagsRunner(name, fullName string) *tagsRunner {
	return &tagsRunner{name, fullName}
}

func (tr *tagsRunner) run() (res map[string]string, err error) {
	res = map[string]string{}
	tempFolder := fmt.Sprint(time.Now().Unix())
	defer func() { exec.Command("rm", "-rf", tempFolder).Run() }()
	targetFolder := fmt.Sprintf("%s/%s", tempFolder, tr.name)
	if err = exec.Command("mkdir", tempFolder).Run(); err != nil {
		return res, err
	}
	if err = exec.Command("git", "clone", "https://github.com/"+tr.fullName, targetFolder).Run(); err != nil {
		return res, err
	}
	var dat []byte
	if dat, err = exec.Command("git", "-C", targetFolder, "show-ref", "--tags").Output(); err != nil {
		return res, err
	}
	lines := strings.Split(string(dat), "\n")
	for _, l := range lines {
		if l == "" {
			continue
		}
		shaRef := strings.Split(l, " ")
		if len(shaRef) != 2 {
			return res, fmt.Errorf("Problem parsing this line [%s]", l)
		}
		res[shaRef[0]] = shaRef[1]
	}
	return res, nil

}
