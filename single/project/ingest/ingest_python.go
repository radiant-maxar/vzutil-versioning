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
package ingest

import (
	"errors"
	"regexp"
	"strings"

	"github.com/venicegeo/vzutil-versioning/common/dependency"
	lan "github.com/venicegeo/vzutil-versioning/common/language"
	"github.com/venicegeo/vzutil-versioning/single/project/issue"
	"gopkg.in/yaml.v2"
)

type PipProjectWrapper struct {
	Filedat    []byte
	DevFileDat []byte
	ProjectWrapper
}
type CondaProjectWrapper struct {
	Filedat []byte
	ProjectWrapper
}

func (pw *PipProjectWrapper) compileCheck() {
	var _ IProjectWrapper = (*PipProjectWrapper)(nil)
}
func (pw *CondaProjectWrapper) compileCheck() {
	var _ IProjectWrapper = (*CondaProjectWrapper)(nil)
}

func (pw *PipProjectWrapper) GetResults() ([]*dependency.GenericDependency, []*issue.Issue, error) {
	deps := []*dependency.GenericDependency{}
	gitRE := regexp.MustCompile(`^git(?:(?:\+https)|(?:\+ssh)|(?:\+git))*:\/\/(?:git\.)*github\.com\/.+\/([^@.]+)()(?:(?:.git)?@([^#]+))?`)
	elseRE := regexp.MustCompile(`^([^>=<]+)((?:(?:<=)|(?:>=))|(?:==))?(.+)?$`)
	get := func(str string) {
		for _, line := range strings.Split(str, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.Contains(line, "lib/python") || strings.HasPrefix(line, "-r") || strings.HasPrefix(line, "#") {
				continue
			}
			parts := []string{}
			if gitRE.MatchString(line) {
				parts = gitRE.FindStringSubmatch(line)[1:]
			} else {
				parts = elseRE.FindStringSubmatch(line)[1:]
				if parts[1] != "==" {
					pw.addIssue(issue.NewWeakVersion(parts[0], parts[2], parts[1]))
				}
			}
			//parts = append(parts, []string{"Unknown", "Unknown"}...)
			//			if len(parts) < 3 {
			//				parts = append(parts, "Unknown")
			//			}
			deps = append(deps, dependency.NewGenericDependency(parts[0], parts[2], lan.Python))
		}
	}
	get(string(pw.Filedat))
	get(string(pw.DevFileDat))
	return deps, pw.issues, nil
}

func (pw *CondaProjectWrapper) GetResults() ([]*dependency.GenericDependency, []*issue.Issue, error) {
	envw := CondaEnvironmentWrapper{}
	if err := yaml.Unmarshal(pw.Filedat, &envw); err != nil {
		return nil, nil, err
	}
	env, err := envw.Convert()
	if err != nil {
		return nil, nil, err
	}
	deps := make([]*dependency.GenericDependency, len(env.Dependencies), len(env.Dependencies))
	splitRE := regexp.MustCompile(`^([^>=<]+)((?:(?:<=)|(?:>=))|(?:=))?(.+)?$`)
	for i, dep := range env.Dependencies {
		parts := splitRE.FindStringSubmatch(dep)[1:]
		if parts[1] != "=" {
			pw.addIssue(issue.NewWeakVersion(parts[0], parts[2], parts[1]))
		}
		deps[i] = dependency.NewGenericDependency(parts[0], parts[2], lan.Python)
	}
	return deps, pw.issues, nil
}

type CondaEnvironmentWrapper struct {
	Name         string        `yaml:"name"`
	Channels     []string      `yaml:"channels"`
	Dependencies []interface{} `yaml:"dependencies"`
}
type CondaEnvironment struct {
	Name         string   `yaml:"name"`
	Channels     []string `yaml:"channels"`
	Dependencies []string `yaml:"dependencies"`
}

func (c *CondaEnvironmentWrapper) Convert() (*CondaEnvironment, error) {
	ret := &CondaEnvironment{c.Name, c.Channels, []string{}}
	for _, d := range c.Dependencies {
		switch d.(type) {
		case string:
			ret.Dependencies = append(ret.Dependencies, d.(string))
		case map[interface{}]interface{}:
			pip, ok := d.(map[interface{}]interface{})["pip"]
			if !ok {
				return nil, errors.New("Map found in yml not containing pip key")
			}
			pipDeps, ok := pip.([]interface{})
			if !ok {
				return nil, errors.New("Pip entry not []interface{}")
			}
			for _, dep := range pipDeps {
				if str, ok := dep.(string); !ok {
					return nil, errors.New("Pip dependency non type string")
				} else {
					ret.Dependencies = append(ret.Dependencies, str)
				}
			}
		default:
			return nil, errors.New("Unknown type found in yml")
		}
	}
	return ret, nil
}
