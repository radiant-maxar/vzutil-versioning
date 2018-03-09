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
	"github.com/gin-gonic/gin"
	"github.com/venicegeo/vzutil-versioning/common/table"
	u "github.com/venicegeo/vzutil-versioning/web/util"
)

func (a *Application) listShas(c *gin.Context) {
	if a.checkBack(c) {
		return
	}
	fullName := u.Format("%s/%s", c.Param("org"), c.Param("repo"))
	refShas, count, err := a.rprtr.ListShas(fullName)
	if err != nil {
		a.displayFailure(c, err.Error())
		return
	}
	header := "List of Shas for " + fullName + "\n"
	t := table.NewTable(2, count+len(refShas))
	for refName, shas := range refShas {
		t.Fill(refName, "")
		for _, sha := range shas {
			t.Fill("", sha)
		}
	}
	a.displaySuccess(c, header+t.NoRowBorders().Format().String())
}

func (a *Application) listRefsRepo(c *gin.Context) {
	if a.checkBack(c) {
		return
	}
	fullName := u.Format("%s/%s", c.Param("org"), c.Param("repo"))
	refs, err := a.rprtr.ListRefsRepo(fullName)
	if err != nil {
		a.displayFailure(c, err.Error())
		return
	}
	header := "List of refs for " + fullName + "\n"
	t := table.NewTable(1, len(*refs))
	for _, r := range *refs {
		t.Fill(r)
	}
	a.displaySuccess(c, header+t.NoRowBorders().NoColumnBorders().Format().String())
}
func (a *Application) listRefs(c *gin.Context) {
	if a.checkBack(c) {
		return
	}
	org := c.Param("org")
	tags, num, err := a.rprtr.ListRefs(org)
	if err != nil {
		a.displayFailure(c, err.Error())
		return
	}
	header := "List of refs for " + org + "\n"
	t := table.NewTable(2, num+len(*tags))
	for k, v := range *tags {
		if len(v) == 0 {
			continue
		}
		t.Fill("")
		t.Fill("")
		t.Fill(k)
		for i, vv := range v {
			t.Fill(vv)
			if i != len(v)-1 {
				t.Fill(" ")
			}
		}
	}
	a.displaySuccess(c, header+t.SpaceColumn(1).NoRowBorders().NoColumnBorders().Format().String())
}

func (a *Application) listProjects(c *gin.Context) {
	if a.checkBack(c) {
		return
	}
	ps, err := a.rprtr.ListProjects()
	header := "List of projects\n"
	a.listProjectsWrk(ps, err, header, c)
}
func (a *Application) listProjectsOrg(c *gin.Context) {
	if a.checkBack(c) {
		return
	}
	org := c.Param("org")
	ps, err := a.rprtr.ListProjectsByOrg(org)
	header := "List of projects for " + org + "\n"
	a.listProjectsWrk(ps, err, header, c)
}
func (a *Application) listProjectsWrk(ps []string, err error, header string, c *gin.Context) {
	if err != nil {
		a.displayFailure(c, err.Error())
		return
	}
	t := table.NewTable(1, len(ps))
	for _, v := range ps {
		t.Fill(v)
	}
	a.displaySuccess(c, header+t.NoRowBorders().NoColumnBorders().Format().String())
}
