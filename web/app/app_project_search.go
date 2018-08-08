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

package app

import (
	"bytes"
	"encoding/json"

	"github.com/gin-gonic/gin"
	c "github.com/venicegeo/vzutil-versioning/common"
	"github.com/venicegeo/vzutil-versioning/web/es"
	u "github.com/venicegeo/vzutil-versioning/web/util"
)

func (a *Application) searchForDep(c *gin.Context) {
	var form struct {
		Back         string `form:"button_back"`
		DepName      string `form:"depsearchname"`
		DepVersion   string `form:"depsearchversion"`
		ButtonSearch string `form:"button_depsearch"`
	}
	if err := c.Bind(&form); err != nil {
		c.String(400, "Unable to bind form: %s", err.Error())
		return
	}
	h := gin.H{
		"data":             "Search Results will appear here",
		"depsearchname":    form.DepName,
		"depsearchversion": form.DepVersion,
	}
	if form.Back != "" {
		c.Redirect(303, "ui")
	} else if form.ButtonSearch != "" {
		repos, err := a.rtrvr.ListRepositories()
		if err != nil {
			c.String(400, "Unable to retrieve the projects repositories: %s", err.Error())
			return
		}
		code, dat := a.searchForDepWrk(form.DepName, form.DepVersion, repos)
		h["data"] = dat
		c.HTML(code, "depsearch.html", h)
	} else {
		c.HTML(200, "depsearch.html", h)
	}
}

func (a *Application) searchForDepInProject(c *gin.Context) {
	proj := c.Param("proj")
	var form struct {
		Back         string `form:"button_back"`
		DepName      string `form:"depsearchname"`
		DepVersion   string `form:"depsearchversion"`
		ButtonSearch string `form:"button_depsearch"`
	}
	if err := c.Bind(&form); err != nil {
		c.String(400, "Unable to bind form: %s", err.Error())
		return
	}
	h := gin.H{
		"data":             "Search Results will appear here",
		"depsearchname":    form.DepName,
		"depsearchversion": form.DepVersion,
	}
	if form.Back != "" {
		c.Redirect(303, "/project/"+proj)
	} else if form.ButtonSearch != "" {
		repos, err := a.rtrvr.ListRepositoriesByProj(proj)
		if err != nil {
			c.String(400, "Unable to retrieve the projects repositories: %s", err.Error())
			return
		}
		code, dat := a.searchForDepWrk(form.DepName, form.DepVersion, repos)
		h["data"] = dat
		c.HTML(code, "depsearch.html", h)
	} else {
		c.HTML(200, "depsearch.html", h)
	}
}

func (a *Application) getDepsMatching(depName, depVersion string) ([]es.Dependency, error) {
	rawDat, err := es.GetAll(a.index, "dependency", u.Format(`
{
	"bool":{
	"must":[
		{
			"term":{
				"name":"%s"
			}
		},{
			"wildcard":{
				"version":"%s*"
			}
		}
	]
	}
}`, depName, depVersion))
	if err != nil {
		return nil, err
	}

	containingDeps := make([]es.Dependency, len(rawDat), len(rawDat))
	for i, b := range rawDat {
		var dep es.Dependency
		if err = json.Unmarshal(b.Dat, &dep); err != nil {
			containingDeps[i] = es.Dependency{"", u.Format("\tError decoding %s\n", b.Id), ""}
		} else {
			containingDeps[i] = dep
		}
	}
	return containingDeps, nil
}

func (a *Application) searchForDepWrk(depName, depVersion string, repos []string) (int, string) {
	boool := es.NewBool()
	must := es.NewBoolQ(es.NewTerm("name", depName), es.NewWildcard("version", depVersion+"*"))
	boolDat, err := json.Marshal(boool.SetMust(must))
	if err != nil {
		return 400, "Unable to create bool query: " + err.Error()
	}
	rawDat, err := es.GetAll(a.index, "dependency", u.Format(`{"bool":%s}`, string(boolDat)))
	if err != nil {
		return 500, "Error querying database: " + err.Error()
	}

	buf := bytes.NewBufferString("Searching for:\n")
	containingDeps, err := a.getDepsMatching(depName, depVersion)
	if err != nil {
		return 500, "Error querying database: " + err.Error()
	}
	hashes := make([]string, len(containingDeps), len(containingDeps))
	for i, dep := range containingDeps {
		hashes[i] = dep.GetHashSum() // u.Format(`"%s"`, dep.GetHashSum())
		buf.WriteString("\t")
		buf.WriteString(dep.String())
		buf.WriteString("\n")
	}
	buf.WriteString("\n\n\n")

	boool = es.NewBool().SetMust(es.NewBoolQ(es.NewTerms("dependencies", hashes...), es.NewTerms("repo_fullname", repos...)))
	s := `{"timestamp":"desc"}`
	if boolDat, err = json.Marshal(boool); err != nil {
		return 500, "Unable to create repo bool query: " + err.Error()
	}
	rawDat, err = es.GetAll(a.index, "repository_entry", u.Format(`{"bool":%s}`, string(boolDat)), s)
	if err != nil {
		return 500, "Unable to query repos: " + err.Error()
	}

	test := map[string]map[string][]string{}
	for _, b := range rawDat {
		var entry c.DependencyScan
		if err = json.Unmarshal(b.Dat, &entry); err != nil {
			return 500, "Error getting entry: " + err.Error()
		}
		if _, ok := test[entry.Fullname]; !ok {
			test[entry.Fullname] = map[string][]string{}
		}
		for _, refName := range entry.Refs {
			if _, ok := test[entry.Fullname][refName]; !ok {
				test[entry.Fullname][refName] = []string{}
			}
			test[entry.Fullname][refName] = append(test[entry.Fullname][refName], entry.Sha)
		}
	}
	for repoName, refs := range test {
		buf.WriteString(repoName)
		buf.WriteString("\n")
		for refName, shas := range refs {
			buf.WriteString(u.Format("\t%s\n", refName))
			for _, sha := range shas {
				buf.WriteString(u.Format("\t\t %s \n", sha))
			}
		}
	}

	return 200, buf.String()
}
