/*
Copyright 2018, RadiantBlue Technologies, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package resolve

import (
	"regexp"

	"github.com/venicegeo/vzutil-versioning/single/util"
)

type MvnDependency struct {
	GroupId    string `json:"groupId"`
	ArtifactId string `json:"artifactId"`
	Packaging  string `json:"packaging"`
	Version    string `json:"version,omitempty"`
}

var re = regexp.MustCompile(`Download(?:(?:ing)|(?:ed)): .+(?:\n|\r)`)

func GenerateMvnReport(location string) util.CmdRet {
	cmd := util.RunCommand("mvn", "--file", location+"pom.xml", "dependency:resolve")
	if cmd.IsError() {
		cmd.Stdout = re.ReplaceAllString(cmd.Stdout, "")
	}
	return cmd
}
