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
	"errors"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/venicegeo/pz-gocommon/elasticsearch"
	s "github.com/venicegeo/vzutil-versioning/web/app/structs"
	u "github.com/venicegeo/vzutil-versioning/web/util"
)

type Application struct {
	indexName       string
	searchSize      int
	singleLocation  string
	compareLocation string
	debugMode       bool

	wrkr     *Worker
	rtrvr    *Retriever
	diffMan  *DifferenceManager
	wbhkRnnr *WebhookRunner
	plrlRnnr *PluralRunner
	cmprRnnr *CompareRunner

	killChan chan bool

	index *elasticsearch.Index
}

type Back struct {
	BackButton string `form:"button_back"`
}

func NewApplication(indexName, singleLocation, compareLocation string, debugMode bool) *Application {
	return &Application{
		indexName:       indexName,
		searchSize:      250,
		singleLocation:  singleLocation,
		compareLocation: compareLocation,
		debugMode:       debugMode,
		killChan:        make(chan bool),
	}
}

func (a *Application) Start() chan error {
	done := make(chan error)
	log.Println("Starting up...")

	if err := a.handleMaven(); err != nil {
		log.Fatal(err)
	}

	url, user, pass, err := s.GetVcapES()
	log.Printf("The elasticsearch url has been found to be [%s]\n", url)
	if err != nil {
		log.Fatal(err)
	}
	i, err := elasticsearch.NewIndex2(url, user, pass, a.indexName, `
{
	"mappings": {
		"repository_entry":{
			"dynamic":"strict",
			"properties":{
				"repo_fullname":{"type":"keyword"},
				"repo_name":{"type":"keyword"},
				"ref_name":{"type":"keyword"},
				"sha":{"type":"keyword"},
				"timestamp":{"type":"long"},
				"dependencies":{"type":"keyword"}
			}
		},
		"dependency":{
			"dynamic":"strict",
			"properties":{
				"hashsum":{"type":"keyword"},
				"name":{"type":"keyword"},
				"version":{"type":"keyword"},
				"language":{"type":"keyword"}
			}
		},
		"difference":{
			"dynamic":"strict",
			"properties":{
				"id":{"type":"keyword"},
				"full_name":{"type":"keyword"},
				"ref":{"type":"keyword"},
				"old_sha":{"type":"keyword"},
				"new_sha":{"type":"keyword"},
				"removed":{"type":"keyword"},
				"added":{"type":"keyword"},
				"time":{"type":"long"}
			}
		}
	}
}`)
	if err != nil {
		log.Fatal(err.Error())
	} else {
		log.Println(i.GetVersion())
	}

	a.index = i

	a.diffMan = NewDifferenceManager(a)
	a.wrkr = NewWorker(a, 2)
	a.rtrvr = NewRetriever(a)
	a.wbhkRnnr = NewWebhookRunner(a)
	a.plrlRnnr = NewPluralRunner(a)
	a.cmprRnnr = NewCompareRunner(a)

	a.wrkr.Start()

	port := os.Getenv("PORT")
	if port == "" {
		port = "20012"
	}

	log.Println("Starting on port", port)
	server := u.Server{}
	server.Configure([]u.RouteData{
		u.RouteData{"GET", "/", a.defaultPath},
		u.RouteData{"POST", "/webhook", a.webhookPath},
		u.RouteData{"GET", "/generate/tags/:org/:repo", a.updateAllTags},
		u.RouteData{"GET", "/generate/tags/:org", a.updateAllTagsOrg},
		u.RouteData{"GET", "/generate/branch/:org/:repo/:branch", a.generateBranch},

		u.RouteData{"GET", "/report/sha/:shaorg", a.reportSha},
		u.RouteData{"GET", "/report/sha/:shaorg/:repo/:sha", a.reportSha},
		u.RouteData{"GET", "/report/ref/:reforg", a.reportRef},
		u.RouteData{"GET", "/report/ref/:reforg/:refrepo", a.reportRef},
		u.RouteData{"GET", "/report/ref/:reforg/:refrepo/:ref", a.reportRef},

		u.RouteData{"GET", "/list/shas/:org/:repo", a.listShas},
		u.RouteData{"GET", "/list/refs/:org/:repo", a.listRefsRepo},
		u.RouteData{"GET", "/list/refs/:org", a.listRefs},
		u.RouteData{"GET", "/list/repositories", a.listRepositories},
		u.RouteData{"GET", "/list/repositories/:org", a.listRepositoriesOrg},

		u.RouteData{"GET", "/search", a.uiSearchForDep},
		u.RouteData{"GET", "/search/:dep", a.searchForDep},
		u.RouteData{"GET", "/search/:dep/:version", a.searchForDep},

		u.RouteData{"GET", "/ui", a.formPath},
		u.RouteData{"GET", "/test", a.test},
		u.RouteData{"GET", "/project/test", a.testProject},

		u.RouteData{"GET", "/diff", a.diffPath},
		u.RouteData{"GET", "/cdiff", a.customDiffPath},
		u.RouteData{"GET", "/tdiff", a.textDiffPath},
		u.RouteData{"POST", "/tdiff", a.textDiffPath},
	})
	select {
	case err = <-server.Start(":" + port):
		done <- err
	case <-a.killChan:
		done <- errors.New(u.Format("was stopped: %s", server.Stop()))
	}
	return done
}
func (a *Application) Stop() {
	a.killChan <- true
}

func (a *Application) defaultPath(c *gin.Context) {
	c.String(200, "Welcome to the dependency service!")
}

func (a *Application) test(c *gin.Context) {
	type form struct {
		Project string `form:"button_project"`
	}
	var f form
	_ = c.Bind(&f)
	if f.Project == "" {
		table := s.NewHtmlTable()
		makeButton := func(name string) string {
			return s.NewHtmlButton3("button_project", name, "button").String()
		}
		table.AddRow().AddItem(0, makeButton("Piazza")).AddItem(0, makeButton("Beachfront")).AddItem(0, makeButton("Eventkit"))
		table.AddRow().AddItem(1, makeButton("Add New"))
		h := gin.H{}
		h["table"] = table.Template()
		c.HTML(200, "test.html", h)
		return
	} else if f.Project == "Add New" {
		c.String(200, "Still working on this")
	} else {
		c.Redirect(303, "/project/test")
	}
}

func (a *Application) testProject(c *gin.Context) {
	c.HTML(200, "test2.html", nil)
}

func (a *Application) formPath(c *gin.Context) {
	var form s.Form
	if err := c.Bind(&form); err != nil {
		c.String(400, err.Error())
		return
	}
	if form.IsEmpty() {
		repos, err := a.rtrvr.ListRepositories()
		h := gin.H{}
		if err != nil {
			h["repositories"] = "Sorry... could not\nload this.\n" + err.Error()
		} else {
			res := ""
			for i, p := range repos {
				if i > 0 {
					res += "\n"
				}
				res += p
			}
			h["repositories"] = res
		}
		diffs, err := a.diffMan.DiffList()
		if err != nil {
			h["differences"] = "Sorry... could not\nload this.\n" + err.Error()
		} else {
			res := ""
			for i, d := range diffs {
				if i > 0 {
					res += "\n"
				}
				res += d
			}
			h["differences"] = res
		}
		c.HTML(200, "form.html", h)
		return
	}
	buttonPress := form.FindButtonPress()
	switch buttonPress {
	case s.DepSearch:
		c.Redirect(303, "/search")
	case s.ReportRef:
		if form.ReportRefRepo != "" && form.ReportRefOrg == "" {
			a.displayFailure(c, "Must specify an org if you specify a repo")
		} else {
			url := "/report/ref"
			if form.ReportRefOrg != "" {
				url += "/" + form.ReportRefOrg
			}
			if form.ReportRefRepo != "" {
				url += "/" + form.ReportRefRepo
			}
			if form.ReportRefRef != "" {
				url += "/" + strings.Replace(form.ReportRefRef, `/`, "_", -1)
			}
			c.Redirect(303, url)
		}
	case s.ReportSha:
		c.Redirect(303, u.Format("/report/sha/%s/%s/%s", form.ReportShaOrg, form.ReportShaRepo, form.ReportShaSha))
	case s.ListRefs:
		if form.RefsRepo != "" {
			c.Redirect(303, u.Format("/list/refs/%s/%s", form.RefsOrg, form.RefsRepo))
		} else {
			c.Redirect(303, "/list/refs/"+form.RefsOrg)
		}
	case s.ListShas:
		c.Redirect(303, u.Format("/list/shas/%s/%s", form.ShasOrg, form.ShasRepo))
	case s.GenerateTag:
		if form.AllTagRepo != "" {
			c.Redirect(303, u.Format("/generate/tags/%s/%s", form.AllTagOrg, form.AllTagRepo))
		} else {
			c.Redirect(303, "/generate/tags/"+form.AllTagOrg)
		}
	case s.GenerateBranch:
		c.Redirect(303, u.Format("/generate/branch/%s/%s/%s", form.BranchOrg, form.BranchRepo, form.BranchBranch))
	case s.Differences:
		c.Redirect(303, "/diff")
	case s.CustomDifference:
		c.Redirect(303, "/cdiff")
	case s.TextDifference:
		c.Redirect(303, "/tdiff")
	default:
		c.String(400, "What did you do? :(")
	}
}

func (a *Application) checkBack(c *gin.Context) (wasHandled bool) {
	var back Back
	if err := c.Bind(&back); err != nil {
		c.String(500, err.Error())
		return true
	}
	if back.BackButton != "" {
		c.Redirect(303, "/ui")
		return true
	}
	return false
}

func (a *Application) displaySuccess(c *gin.Context, data string) {
	if !a.checkForRedirect(c) {
		c.String(200, data)
	} else {
		c.HTML(200, "back.html", gin.H{"data": data})
	}
}
func (a *Application) displayFailure(c *gin.Context, data string) {
	//TODO assuming 400
	if !a.checkForRedirect(c) {
		c.String(400, data)
	} else {
		c.HTML(400, "back.html", gin.H{"data": data})
	}
}

func (a *Application) checkForRedirect(c *gin.Context) bool {
	return c.Request.Header.Get("Referer") != ""
}

func (a *Application) handleMaven() error {
	_, err := os.Stat("settings.xml")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		} else {
			return err
		}
	}
	dat, _ := exec.Command("mvn", "-X").Output()
	re := regexp.MustCompile(`Reading user settings from (.+)\/`)
	finds := re.FindStringSubmatch(string(dat))
	if len(finds) != 2 {
		return errors.New("Couldnt find maven settings location")
	}

	return exec.Command("mv", "settings.xml", finds[1]).Run()
}
