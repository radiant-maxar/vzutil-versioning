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
	"encoding/json"
	"log"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/venicegeo/pz-gocommon/elasticsearch/elastic-5-api"
	nt "github.com/venicegeo/pz-gocommon/gocommon"
	s "github.com/venicegeo/vzutil-versioning/web/app/structs"
	"github.com/venicegeo/vzutil-versioning/web/es"
	u "github.com/venicegeo/vzutil-versioning/web/util"
)

func (a *Application) viewProject(c *gin.Context) {
	proj := c.Param("proj")
	project, err := a.rtrvr.GetProject(proj)
	if err != nil {
		c.String(400, "Error getting this project: %s", err.Error())
		return
	}

	var form struct {
		Back   string `form:"button_back"`
		Util   string `form:"button_util"`
		Sha    string `form:"button_sha"`
		Gen    string `form:"button_gen"`
		Diff   string `form:"button_diff"`
		Reload string `form:"button_reload"`
	}
	if err := c.Bind(&form); err != nil {
		c.String(400, "Unable to bind form: %s", err.Error())
		return
	}
	depsStr := "Result info will appear here"
	if form.Back != "" {
		c.Redirect(303, "/ui")
		return
	} else if form.Reload != "" {
		c.Redirect(303, "/project/"+proj)
		return
	} else if form.Util != "" {
		switch form.Util {
		case "Report By Ref":
			c.Redirect(303, "/reportref/"+proj)
			return
		case "Generate All Tags":
			str, err := a.genTagsWrk(proj)
			if err != nil {
				u.Format("Unable to generate all tags: %s", err.Error())
			} else {
				depsStr = str
			}
		case "Add Repository":
			c.Redirect(303, "/addrepo/"+proj)
			return
		case "Remove Repository":
			c.Redirect(303, "/removerepo/"+proj)
			return
		case "Dependency Search":
			c.Redirect(303, "/depsearch/"+proj)
			return
		case "Delete Project":
			c.Redirect(303, "/delproj/"+proj)
			return
		}
	} else if form.Sha != "" {
		scan, found, err := project.ScanBySha(form.Sha)
		if !found && err != nil {
			c.String(400, "Unable to find this sha: %s", err.Error())
			return
		} else if found && err != nil {
			c.String(500, "Unable to obtain the results: %s", err.Error())
			return
		}
		depsStr = a.reportAtShaWrk(scan)
	} else if form.Gen != "" {
		repoFullName := strings.TrimPrefix(form.Gen, "Generate Branch - ")
		c.Redirect(303, u.Format("/genbranch/%s/%s", proj, repoFullName))
		return
	} else if form.Diff != "" {
		c.Redirect(303, "/diff/"+proj)
		return
	}
	accord := s.NewHtmlAccordion()
	repos, err := project.GetAllRepositories()
	if err != nil {
		c.String(500, "Unable to retrieve repository list: %s", err.Error())
		return
	}
	mux := sync.Mutex{}
	errs := make(chan error, len(repos))
	for _, repo := range repos {
		go a.generateAccordion(accord, repo, errs, mux)
	}
	err = nil
	for i := 0; i < len(repos); i++ {
		e := <-errs
		if e != nil {
			err = e
		}
	}
	if err != nil {
		c.String(500, "Error retrieving data: %s", err.Error())
		return
	}
	accord.Sort()
	h := gin.H{}
	h["accordion"] = accord.Template()
	h["deps"] = depsStr
	{
		diffs, err := a.diffMan.GetAllDiffsInProject(proj)
		if err != nil {
			h["diff"] = ""
		} else {
			h["diff"] = u.Format(" (%d)", len(*diffs))
		}
	}
	c.HTML(200, "project.html", h)
}

func (a *Application) generateAccordion(accord *s.HtmlAccordion, repo *Repository, errs chan error, mux sync.Mutex) {
	refs, err := repo.GetAllRefs()
	if err != nil {
		errs <- err
		return
	}
	tempAccord := s.NewHtmlAccordion()
	shas, _, err := repo.MapRefToShas()
	if err != nil {
		errs <- err
		return
	}
	for _, ref := range refs {
		c := s.NewHtmlCollection()
		correctShas := shas["refs/"+ref]
		for i, sha := range correctShas {
			c.Add(s.NewHtmlButton2("button_sha", sha))
			if i < len(correctShas)-1 {
				c.Add(s.NewHtmlBr())
			}
		}
		tempAccord.AddItem(ref, s.NewHtmlForm(c).Post())
	}
	mux.Lock()
	accord.AddItem(repo.RepoFullname, s.NewHtmlCollection(s.NewHtmlForm(s.NewHtmlButton2("button_gen", "Generate Branch - "+repo.RepoFullname)).Post(), tempAccord.Sort()))
	mux.Unlock()
	errs <- nil
}

func (a *Application) addRepoToProject(c *gin.Context) {
	var form struct {
		Back string `form:"button_back"`

		Org         string `form:"org"`
		Repo        string `form:"repo"`
		PrimaryType string `form:"primtype"`

		AltOrg        string `form:"altorg"`
		AltRepo       string `form:"altrepo"`
		SecondaryType string `form:"sectype"`
		TextRef       string `form:"text_ref"`
		TextSha       string `form:"text_sha"`

		Scan string `form:"button_scan"`

		Files  []string `form:"files[]"`
		Submit string   `form:"button_submit"`
	}
	proj := c.Param("proj")
	if err := c.Bind(&form); err != nil {
		c.String(400, "Error binding form: %s", err.Error())
		return
	}
	if form.Back != "" {
		c.Redirect(303, "/project/"+proj)
		return
	}
	form.Org = strings.TrimSpace(form.Org)
	form.Repo = strings.TrimSpace(form.Repo)
	form.AltOrg = strings.TrimSpace(form.AltOrg)
	form.AltRepo = strings.TrimSpace(form.AltRepo)
	h := gin.H{
		"org":      form.Org,
		"repo":     form.Repo,
		"altorg":   form.AltOrg,
		"altrepo":  form.AltRepo,
		"text_ref": form.TextRef,
		"text_sha": form.TextSha,
		"hidescan": true,
	}
	depinfo := es.ProjectEntryDependencyInfo{FilesToScan: form.Files}
	isThis := false
	switch form.PrimaryType {
	case "radio_other":
		h["radio_other_checked"] = "checked"
	case "radio_this":
		fallthrough
	default:
		isThis = true
		depinfo.RepoFullname = form.Org + "/" + form.Repo
		depinfo.CheckoutType = es.IncomingSha
		h["radio_this_checked"] = "checked"
	}
	switch form.SecondaryType {
	case "radio_ref":
		h["radio_ref_checked"] = "checked"
		if !isThis {
			depinfo.CheckoutType = es.CustomRef
			depinfo.CustomField = form.TextRef
		}
	case "radio_sha":
		h["radio_sha_checked"] = "checked"
		if !isThis {
			depinfo.CheckoutType = es.ExactSha
			depinfo.CustomField = form.TextSha
		}
	case "radio_same":
		fallthrough
	default:
		h["radio_same_checked"] = "checked"
		if !isThis {
			depinfo.CheckoutType = es.SameRef
		}
	}
	if !isThis {
		depinfo.RepoFullname = form.AltOrg + "/" + form.AltRepo
	}
	repoName := u.Format("%s/%s", form.Org, form.Repo)
	var altRepoName string
	if form.AltOrg == "" {
		altRepoName = repoName
	} else {
		altRepoName = u.Format("%s/%s", form.AltOrg, form.AltRepo)
	}

	setScan := func(i interface{}) {
		h["hidescan"] = false
		switch i.(type) {
		case string:
			h["scan"] = s.NewHtmlString(i.(string)).Template()
		case []string:
			check := s.NewHtmlCheckbox("files[]")
			for _, file := range i.([]string) {
				check.Add(file, file, true)
			}
			h["scan"] = s.NewHtmlCollection(check, s.NewHtmlButton2("button_submit", "Submit")).Template()
		default:
			panic("Youre doing this wrong")
		}
	}

	primaryScan := func() {
		if !a.checkRepoIsReal(form.Org, form.Repo) {
			setScan("This isnt a real repo")
		} else {
			if files, err := a.wrkr.snglRnnr.ScanWithSingle(repoName); err != nil {
				setScan(err.Error())
			} else {
				setScan(files)
			}
		}
	}

	secondaryScan := func() {
		if !a.checkRepoIsReal(form.AltOrg, form.AltRepo) {
			setScan("This isnt a real repo")
		} else {
			if files, err := a.wrkr.snglRnnr.ScanWithSingle(altRepoName); err != nil {
				setScan(err.Error())
			} else {
				setScan(files)
			}
		}
	}

	submit := func() {
		entry := es.ProjectEntry{
			ProjectName:    proj,
			RepoFullname:   repoName,
			DependencyInfo: depinfo,
		}
		boolq := es.NewBool().
			SetMust(es.NewBoolQ(
				es.NewTerm(es.ProjectEntryNameField, entry.ProjectName),
				es.NewTerm(es.ProjectEntryRepositoryField, entry.RepoFullname)))
		resp, err := a.index.SearchByJSON(ProjectEntryType, map[string]interface{}{
			"query": map[string]interface{}{"bool": boolq},
			"size":  1,
		})
		if err != nil {
			c.String(500, "Error checking database for existing repo: %s", err.Error())
			return
		}
		if resp.Hits.TotalHits == 1 {
			c.String(400, "This repo already exists under this project")
			return
		}
		iresp, err := a.index.PostData(ProjectEntryType, "", entry)
		if err != nil || !iresp.Created {
			c.String(500, "Error adding entry to database: ", err)
			return
		}
		c.Redirect(303, "/project/"+proj)
	}

	if form.Scan != "" && form.PrimaryType == "radio_this" {
		primaryScan()
	} else if form.Scan != "" && form.PrimaryType == "radio_other" {
		secondaryScan()
	} else if form.Submit != "" {
		submit()
		return
	}

	c.HTML(200, "addrepo.html", h)
}

// Checks to see if a repo name is an actual repo on github
func (a *Application) checkRepoIsReal(name ...string) bool {
	var fullname string
	switch len(name) {
	case 1:
		fullname = strings.TrimSpace(name[0])
		if fullname == "" || fullname == "/" {
			return false
		}
	case 2:
		org := strings.TrimSpace(name[0])
		repo := strings.TrimSpace(name[1])
		if org == "" || repo == "" {
			return false
		}
		fullname = u.Format("%s/%s", name[0], name[1])
	default:
		panic("Youre doing this wrong")
	}
	url := u.Format("https://github.com/%s", fullname)
	if code, _, _, e := nt.HTTP(nt.HEAD, url, nt.NewHeaderBuilder().GetHeader(), nil); e != nil || code != 200 {
		return false
	} else {
		return true
	}
}

func (a *Application) removeReposFromProject(c *gin.Context) {
	proj := c.Param("proj")
	var form struct {
		Back string `form:"button_back"`
		Repo string `form:"button_submit"`
	}
	if err := c.Bind(&form); err != nil {
		c.String(400, "Unable to bind form: %s", err.Error())
		return
	}
	if form.Back != "" {
		c.Redirect(303, "/project/"+proj)
		return
	}
	if form.Repo != "" {
		boolq := es.NewBool().
			SetMust(es.NewBoolQ(
				es.NewTerm(es.ProjectEntryNameField, proj),
				es.NewTerm(es.ProjectEntryRepositoryField, form.Repo)))
		resp, err := a.index.SearchByJSON(ProjectEntryType, map[string]interface{}{
			"query": map[string]interface{}{"bool": boolq},
			"size":  1,
		})
		if err != nil {
			c.String(400, "Unable to find the project entry: %s", err.Error())
			return
		}
		if resp.Hits.TotalHits != 1 {
			c.String(400, "Could not find the project entry")
			return
		}
		entry := new(es.ProjectEntry)
		if err = json.Unmarshal(*resp.Hits.Hits[0].Source, entry); err != nil {
			c.String(500, "Unable to read project entry: %s", err.Error())
			return
		}
		_, err = a.index.DeleteByIDWait(ProjectEntryType, resp.Hits.Hits[0].Id)
		if err != nil {
			c.String(500, "Unable to delete project entry: %s", err.Error())
			return
		}
		func() {
			hits, err := es.GetAll(a.index, RepositoryEntryType, map[string]interface{}{
				"bool": es.NewBool().
					SetMust(es.NewBoolQ(
						es.NewTerm(Scan_FullnameField, entry.RepoFullname),
						es.NewTerm(Scan_ProjectField, entry.ProjectName))),
			})
			if err != nil {
				log.Println("Unable to cleanup after repo", entry.RepoFullname, "in", entry.ProjectName, ":", err.Error())
				return
			}
			wg := sync.WaitGroup{}
			wg.Add(len(hits.Hits))
			for _, hit := range hits.Hits {
				go func(hit *elastic.SearchHit) {
					a.index.DeleteByID(RepositoryEntryType, hit.Id)
					wg.Done()
				}(hit)
			}
			wg.Wait()
		}()
		c.Redirect(303, "/removerepo/"+proj)
		return
	}
	project, err := a.rtrvr.GetProject(proj)
	if err != nil {
		c.String(500, "Unable to get the project: %s", err)
		return
	}
	repos, err := project.GetAllRepositories()
	if err != nil {
		c.String(500, "Unable to get the repos: %s", err)
		return
	}
	h := gin.H{}
	buttons := s.NewHtmlCollection()
	for _, repo := range repos {
		buttons.Add(s.NewHtmlButton2("button_submit", repo.RepoFullname))
		buttons.Add(s.NewHtmlBr())
	}
	h["repos"] = buttons.Template()
	c.HTML(200, "removerepo.html", h)
}
