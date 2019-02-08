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
	"sort"

	d "github.com/radiant-maxar/vzutil-versioning/common/dependency"
	"github.com/radiant-maxar/vzutil-versioning/common/table"
	"github.com/radiant-maxar/vzutil-versioning/web/es/types"
	u "github.com/radiant-maxar/vzutil-versioning/web/util"
)

type Formatter struct {
	app *Application
}

func NewFormatter(app *Application) *Formatter {
	return &Formatter{app}
}

func (f *Formatter) formatReportByRef(ref string, deps map[string]*types.Scan, typ string) string {
	buf := bytes.NewBufferString("")
	switch typ {
	case "seperate":
		for name, depss := range deps {
			projName := "[Error finding project name]"
			if proj, err := f.app.rtrvr.GetProjectById(depss.ProjectId); err == nil {
				projName = proj.DisplayName
			}
			buf.WriteString(u.Format("%s at %s in %s\n%s\nFrom %s %s", name, ref, projName, depss.Sha, depss.Scan.Fullname, depss.Scan.Sha))
			t := table.NewTable(3, len(depss.Scan.Deps))
			for _, dep := range depss.Scan.Deps {
				t.Fill(dep.Name, dep.Version, dep.Language.String())
			}
			buf.WriteString(u.Format("\n%s\n\n", t.NoRowBorders().SpaceColumn(1).Format().String()))
		}
	case "grouped":
		buf.WriteString(u.Format("All repos at %s\n", ref))
		noDups := map[string]d.Dependency{}
		for name, depss := range deps {
			buf.WriteString(name)
			buf.WriteString("\n")
			for _, dep := range depss.Scan.Deps {
				noDups[dep.String()] = dep
			}
		}
		sorted := make(d.Dependencies, 0, len(noDups))
		for _, dep := range noDups {
			sorted = append(sorted, dep)
		}
		sort.Sort(sorted)
		t := table.NewTable(3, len(sorted))
		for _, dep := range sorted {
			t.Fill(dep.Name, dep.Version, dep.Language.String())
		}
		buf.WriteString(u.Format("\n%s", t.NoRowBorders().SpaceColumn(1).Format().String()))
	default:
	}
	return buf.String()
}

func (f *Formatter) formatReportBySha(scan *types.Scan) string {
	buf := bytes.NewBufferString("")
	projName := "[Error finding project name]"
	if proj, err := f.app.rtrvr.GetProjectById(scan.ProjectId); err == nil {
		projName = proj.DisplayName
	}
	buf.WriteString(u.Format("%s at %s in %s\n", scan.RepoFullname, scan.Sha, projName))
	buf.WriteString(u.Format("Dependencies from %s at %s\n", scan.Scan.Fullname, scan.Scan.Sha))
	buf.WriteString("Files scanned:\n")
	for _, f := range scan.Scan.Files {
		buf.WriteString(f)
		buf.WriteString("\n")
	}
	t := table.NewTable(3, len(scan.Scan.Deps))
	for _, dep := range scan.Scan.Deps {
		t.Fill(dep.Name, dep.Version, dep.Language.String())
	}
	buf.WriteString(t.NoRowBorders().SpaceColumn(1).Format().String())
	return buf.String()
}
