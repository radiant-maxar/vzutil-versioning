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
	"encoding/csv"
	"fmt"
	"sort"

	"github.com/gin-gonic/gin"
	d "github.com/venicegeo/vzutil-versioning/common/dependency"
	"github.com/venicegeo/vzutil-versioning/common/table"
	s "github.com/venicegeo/vzutil-versioning/web/app/structs"
	u "github.com/venicegeo/vzutil-versioning/web/util"
)

func (a *Application) reportRefOnProject(c *gin.Context) {
	proj := c.Param("proj")
	var form struct {
		Back       string `form:"button_back"`
		ReportType string `form:"reporttype"`
		Ref        string `form:"button_submit"`
		Download   string `form:"downloadCSV"`
	}
	if err := c.Bind(&form); err != nil {
		c.String(400, "Unable to bind form: %s", err.Error())
		return
	}
	if form.Back != "" {
		c.Redirect(303, "/project/"+proj)
		return
	}

	if form.Download != "" {
		a.reportRefOnProjectDownloadCSV(c)
		return
	}

	h := gin.H{"report": ""}
	project, err := a.rtrvr.GetProject(proj)
	if err != nil {
		h["refs"] = u.Format("Unable to retrieve this projects refs: %s", err.Error())
	} else {
		if refs, err := project.GetAllRefs(); err != nil {
			h["refs"] = u.Format("Unable to retrieve this projects refs: %s", err.Error())
		} else {
			buttons := s.NewHtmlCollection()
			for _, ref := range refs {
				buttons.Add(s.NewHtmlButton2("button_submit", ref))
				buttons.Add(s.NewHtmlBr())
			}
			h["refs"] = buttons.Template()
		}
		if form.Ref != "" {
			if scans, err := project.ScansByRefInProject(form.Ref); err != nil {
				h["report"] = u.Format("Unable to generate report: %s", err.Error())
			} else {
				h["report"] = a.reportAtRefWrk(form.Ref, scans, form.ReportType)
			}
		}
	}
	c.HTML(200, "reportref.html", h)
}

func (a *Application) reportRefOnProjectDownloadCSV(c *gin.Context) {
	proj := c.Param("proj")
	var form struct {
		ReportType string `form:"reporttype"`
		Ref        string `form:"button_download"`
	}
	if err := c.Bind(&form); err != nil {
		c.String(400, "Unable to bind form: %s", err.Error())
		return
	}

	buf := bytes.NewBuffer([]byte{})
	writer := csv.NewWriter(buf)

	if proj == "" || form.Ref == "" {
		c.Header("Content-Disposition", "attachment; filename=\"report_invalid_ref.csv\"")
		writer.Write([]string{"ERROR", "Invalid project/ref", proj, form.Ref})
		c.Data(404, "text/csv", buf.Bytes())
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"report_%s_%s.csv\"", proj, form.Ref))

	project, err := a.rtrvr.GetProject(proj)

	if err != nil {
		writer.Write([]string{"ERROR", "Unable to retrieve this project", err.Error()})
		c.Data(404, "text/csv", buf.Bytes())
		return
	}

	if scans, err := project.ScansByRefInProject(form.Ref); err != nil {
		writer.Write([]string{"ERROR", "Unable to generate report", err.Error()})
		c.Data(500, "text/csv", buf.Bytes())
	} else {
		a.reportAtRefWrkCSV(writer, form.Ref, scans, typ)
		c.Data(200, "text/csv", buf.Bytes())
		return
	}
}

func (a *Application) reportAtRefWrkCSV(w *csv.Writer, ref string, deps map[string]*RepositoryDependencyScan, typ string) {
	switch typ {
	case "seperate":
		for name, depss := range deps {
			w.Write([]string{
				fmt.Sprintf("%s at %s in %s", name, ref, depss.Project),
				depss.Sha,
				fmt.Sprintf("From %s %s", depss.Scan.Fullname, depss.Scan.Sha),
			})
			w.Write([]string{})
			for _, dep := range depss.Scan.Deps {
				w.Write([]string{dep.Name, dep.Version, dep.Language.String()})
			}
			w.Write([]string{})
		}
	case "grouped":
		w.Write([]string{fmt.Sprintf("All repos at %s", ref)})
		w.Write([]string{})

		noDups := map[string]d.Dependency{}
		for name, depss := range deps {
			w.Write([]string{name})
			for _, dep := range depss.Scan.Deps {
				noDups[dep.String()] = dep
			}
		}
		w.Write([]string{})

		sorted := make(d.Dependencies, 0, len(noDups))
		for _, dep := range noDups {
			sorted = append(sorted, dep)
		}
		sort.Sort(sorted)

		for _, dep := range sorted {
			w.Write([]string{dep.Name, dep.Version, dep.Language.String()})
		}
	default:
		w.Write([]string{"Unknown report type", typ})
	}
}

func (a *Application) reportAtRefWrk(ref string, deps map[string]*RepositoryDependencyScan, typ string) string {
	buf := bytes.NewBufferString("")
	s.NewHtmlButton2("foo", "bar")
	buf.WriteString()
	switch typ {
	case "seperate":
		for name, depss := range deps {
			buf.WriteString(u.Format("%s at %s in %s\n%s\nFrom %s %s", name, ref, depss.Project, depss.Sha, depss.Scan.Fullname, depss.Scan.Sha))
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

func (a *Application) reportAtShaWrk(scan *RepositoryDependencyScan) string {
	buf := bytes.NewBufferString("")
	buf.WriteString(u.Format("%s at %s in %s\n", scan.RepoFullname, scan.Sha, scan.Project))
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
