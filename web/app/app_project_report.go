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
	d "github.com/radiant-maxar/vzutil-versioning/common/dependency"
	s "github.com/radiant-maxar/vzutil-versioning/web/app/structs"
	"github.com/radiant-maxar/vzutil-versioning/web/es/types"
	u "github.com/radiant-maxar/vzutil-versioning/web/util"
)

func (a *Application) reportRefOnProject(c *gin.Context) {
	projId := c.Param("proj")
	var form struct {
		Back       string `form:"button_back"`
		ReportType string `form:"reporttype"`
		Ref        string `form:"button_submit"`
		Download   string `form:"download_csv"`
	}
	if err := c.Bind(&form); err != nil {
		c.String(400, "Unable to bind form: %s", err.Error())
		return
	}
	if form.Back != "" {
		c.Redirect(303, "/project/"+projId)
		return
	}

	if form.Download != "" {
		a.reportRefOnProjectDownloadCSV(c)
		return
	}

	h := gin.H{"report": ""}
	project, err := a.rtrvr.GetProjectById(projId)
	if err != nil {
		h["refs"] = u.Format("Unable to retrieve this projects refs: %s", err.Error())
	} else {
		if refs, err := project.GetAllRefs(); err != nil {
			h["refs"] = u.Format("Unable to retrieve this projects refs: %s", err.Error())
		} else {
			buttons := s.NewHtmlCollection()
			for _, ref := range refs {
				buttons.Add(s.NewHtmlSubmitButton2("button_submit", ref))
				buttons.Add(s.NewHtmlBr())
			}
			h["refs"] = buttons.Template()
		}
		if form.Ref != "" {
			if scans, err := project.ScansByRefInProject(form.Ref); err != nil {
				h["report"] = u.Format("Unable to generate report: %s", err.Error())
			} else {
				report := a.frmttr.formatReportByRef(form.Ref, scans, form.ReportType)
				h["report"] = s.NewHtmlCollection(s.NewHtmlButton("Download CSV", "download_csv", form.Ref, "submit").Style("float:right;"), s.NewHtmlBr(), s.NewHtmlBasic("pre", report)).Template()
			}
		}
	}
	c.HTML(200, "reportref.html", h)
}

func (a *Application) reportRefOnProjectDownloadCSV(c *gin.Context) {
	projId := c.Param("proj")
	var form struct {
		ReportType string `form:"reporttype"`
		Ref        string `form:"download_csv"`
	}
	if err := c.Bind(&form); err != nil {
		c.String(400, "Unable to bind form: %s", err.Error())
		return
	}

	buf := bytes.NewBuffer([]byte{})
	writer := csv.NewWriter(buf)

	if projId == "" || form.Ref == "" {
		c.Header("Content-Disposition", "attachment; filename=\"report_invalid_ref.csv\"")
		writer.Write([]string{"ERROR", "Invalid project/ref", projId, form.Ref})
		writer.Flush()
		c.Data(404, "text/csv", buf.Bytes())
		return
	}
	projName := "unknown"
	project, err := a.rtrvr.GetProjectById(projId)
	if err == nil {
		projName = project.EscapedName
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"report_%s_%s.csv\"", projName, form.Ref))

	if err != nil {
		writer.Write([]string{"ERROR", "Unable to retrieve this project", err.Error()})
		writer.Flush()
		c.Data(404, "text/csv", buf.Bytes())
		return
	}

	if scans, err := project.ScansByRefInProject(form.Ref); err != nil {
		writer.Write([]string{"ERROR", "Unable to generate report", err.Error()})
		writer.Flush()
		c.Data(500, "text/csv", buf.Bytes())
	} else {
		a.reportAtRefWrkCSV(writer, form.Ref, scans, form.ReportType)
		writer.Flush()
		c.Data(200, "text/csv", buf.Bytes())
		return
	}
}

func (a *Application) reportAtRefWrkCSV(w *csv.Writer, ref string, deps map[string]*types.Scan, typ string) {
	switch typ {
	case "seperate":
		for name, depss := range deps {
			w.Write([]string{
				fmt.Sprintf("%s at %s in %s", name, ref, depss.ProjectId),
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
