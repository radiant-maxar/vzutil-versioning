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
	d "github.com/venicegeo/vzutil-versioning/common/dependency"
	"github.com/venicegeo/vzutil-versioning/web/es"
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
		repos, err := a.rtrvr.ListRepositoriesInProject(proj)
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

func (a *Application) searchForDepWrk(depName, depVersion string, repos []string) (int, string) {
	buf := bytes.NewBufferString("Searching for:\n")
	nested := es.NewNestedQuery(c.DependenciesField)
	must := es.NewBoolQ(
		es.NewTerm(c.DependenciesField+"."+d.NameField, depName),
		es.NewWildcard(c.DependenciesField+"."+d.VersionField, depVersion+"*"))

	terms := es.NewTerms("full_name", repos...)

	nested.SetInnerQuery(map[string]interface{}{"bool": es.NewBool().SetMust(must)})
	query := map[string]interface{}{"bool": es.NewBool().SetMust(es.NewBoolQ(nested)).SetFilter(es.NewBoolQ(terms))}

	queryDat, err := json.MarshalIndent(query, " ", "   ")
	if err != nil {
		return 500, "Unable to create bool query: " + err.Error()
	}
	hits, err := es.GetAllSource(a.index, "repository_entry", string(queryDat), []string{c.FullNameField, c.RefsField})
	if err != nil {
		return 500, "Failure executing bool query: " + err.Error()
	}
	deps := d.Dependencies{}
	shas := map[string]map[string]map[string]struct{}{}

	for _, hit := range hits.Hits {
		var scan c.DependencyScan
		if err = json.Unmarshal(*hit.Source, &scan); err != nil {
			return 500, "Failure retrieving source: " + err.Error()
		}
		if _, ok := shas[scan.Fullname]; !ok {
			shas[scan.Fullname] = map[string]map[string]struct{}{}
		}
		for _, ref := range scan.Refs {
			if _, ok := shas[scan.Fullname][ref]; !ok {
				shas[scan.Fullname][ref] = map[string]struct{}{}
			}
			shas[scan.Fullname][ref][hit.Id] = struct{}{}
		}
		for _, innerHit := range hit.InnerHits[c.DependenciesField].Hits.Hits {
			dep := new(d.Dependency)
			if err = json.Unmarshal(*innerHit.Source, dep); err != nil {
				return 500, "Error retrieving dependencies: " + err.Error()
			}
			deps = append(deps, *dep)
		}
	}
	d.RemoveExactDuplicates(&deps)
	for _, dep := range deps {
		buf.WriteString("\t")
		buf.WriteString(dep.String())
		buf.WriteString("\n")
	}
	buf.WriteString("\n\n\n")
	for repo, refs := range shas {
		buf.WriteString(repo)
		buf.WriteString("\n")
		for ref, shas := range refs {
			buf.WriteString("\t")
			buf.WriteString(ref)
			buf.WriteString("\n")
			for sha, _ := range shas {
				buf.WriteString("\t\t")
				buf.WriteString(sha[:40])
				buf.WriteString("\n")
			}
		}
	}

	return 200, buf.String()
}
