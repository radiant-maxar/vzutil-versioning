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
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	h "github.com/venicegeo/vzutil-versioning/web/app/helpers"
	s "github.com/venicegeo/vzutil-versioning/web/app/structs"
	u "github.com/venicegeo/vzutil-versioning/web/util"
)

func (a *Application) webhookPath(c *gin.Context) {
	git := s.GitWebhook{}
	if err := c.BindJSON(&git); err != nil {
		log.Println("Unable to bind json:", err.Error())
		c.Status(400)
		return
	}

	c.String(200, "Thanks!")
	log.Println("[RECIEVED WEBHOOK]", git.Repository.FullName, git.AfterSha, git.Ref)

	a.wrkr.AddTask(&git)
}

func (a *Application) updateAllTags(c *gin.Context) {
	if a.checkBack(c) {
		return
	}
	name := c.Param("repo")
	fullName := u.Format("%s/%s", c.Param("org"), name)
	runner := h.NewTagsRunner(name, fullName)
	canDo, err := runner.CanDo()
	if err != nil {
		a.displayFailure(c, "Sorry, no can do. Problem: ["+err.Error()+"]")
		return
	} else if !canDo {
		a.displayFailure(c, "That repo doesnt appear to exist")
		return
	}
	go func(name, fullName string, runner *h.TagsRunner) {
		dat, err := h.NewTagsRunner(name, fullName).Run()
		if err != nil {
			log.Println("Error running tags on", fullName, ":", err.Error())
			return
		}
		for sha, ref := range dat {
			git := s.GitWebhook{
				Ref:      ref,
				AfterSha: sha,
				Repository: s.GitRepository{
					Name:     name,
					FullName: fullName,
				},
			}
			log.Println(fullName, sha, ref)
			a.wrkr.AddTask(&git)
		}
	}(name, fullName, runner)
	a.displaySuccess(c, "Yeah, I can do that. Check back in a minute")
}

func (a *Application) updateAllTagsOrg(c *gin.Context) {
	if a.checkBack(c) {
		return
	}
	org := c.Param("org")
	projects, err := a.rtrvr.ListProjectsByOrg(org)
	if err != nil {
		a.displayFailure(c, "Problemo: ["+err.Error()+"]")
		return
	}
	go func(projects []string) {
		for _, project := range projects {
			name := strings.SplitN(project, "/", 2)[1]
			dat, err := h.NewTagsRunner(name, project).Run()
			if err != nil {
				log.Println("[TAG UPDATER] Was unable to run tags against " + project + ": [" + err.Error() + "]")
				continue
			}
			go func(dat map[string]string, name string, project string) {
				for sha, ref := range dat {
					git := s.GitWebhook{
						Ref:      ref,
						AfterSha: sha,
						Repository: s.GitRepository{
							Name:     name,
							FullName: project,
						},
					}
					log.Println(project, sha, ref)
					a.wrkr.AddTask(&git)
				}
			}(dat, name, project)
		}
	}(projects)

	res := "Trying to run against:\n"
	for _, project := range projects {
		res += "\n" + project
	}

	a.displaySuccess(c, res)
}

func (a *Application) generateBranch(c *gin.Context) {
	if a.checkBack(c) {
		return
	}
	org := c.Param("org")
	repo := c.Param("repo")
	branch := c.Param("branch")
	fullName := u.Format("%s/%s", org, repo)
	sha, err := h.GetBranchSha(repo, fullName, branch)
	if err != nil {
		a.displayFailure(c, "Could not generate this sha: "+err.Error())
		return
	}

	go func(name, fullName, branch, sha string) {
		ref := "refs/heads/" + branch
		git := s.GitWebhook{
			Ref:      ref,
			AfterSha: sha,
			Repository: s.GitRepository{
				Name:     name,
				FullName: fullName,
			},
		}
		log.Println(fullName, sha, ref)
		a.wrkr.AddTask(&git)
	}(repo, fullName, branch, sha)

	a.displaySuccess(c, "Going to run against sha "+sha)
}