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
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	nt "github.com/venicegeo/pz-gocommon/gocommon"
	h "github.com/venicegeo/vzutil-versioning/web/app/helpers"
	s "github.com/venicegeo/vzutil-versioning/web/app/structs"
	u "github.com/venicegeo/vzutil-versioning/web/util"
)

func (a *Application) webhookPath(c *gin.Context) {
	var git = new(s.GitWebhook)
	if err := c.BindJSON(git); err != nil {
		log.Println("Unable to bind json:", err.Error())
		c.Status(400)
		return
	}

	if git.Zen != "" {
		_, str, _, _ := nt.HTTP(nt.GET, "https://api.github.com/zen", nt.NewHeaderBuilder().GetHeader(), nil)
		if string(str) == "" {
			str = []byte("Thanks!")
		}
		c.String(200, string(str))
	} else {
		c.String(200, "Thanks!")
		a.ff.FireGit(git)
	}

}

func (a *Application) generateBranch(c *gin.Context) {
	var form struct {
		Back   string `form:"button_back"`
		Gen    string `form:"button_generatebranch"`
		Branch string `form:"branch"`
	}
	if err := c.Bind(&form); err != nil {
		c.String(400, "Could not bind form: %s", err.Error())
	}
	pprojId := c.Param("proj")
	porg := c.Param("org")
	prepo := c.Param("repo")
	branch := form.Branch
	if form.Back != "" {
		c.Redirect(303, "/project/"+pprojId)
		return
	}
	if form.Gen != "" {
		_, err := a.generateBranchWrk(prepo, u.Format("%s/%s", porg, prepo), branch, pprojId)
		if err != nil {
			c.String(400, "Could not generate this sha: %s", err.Error())
			return
		}
		c.Redirect(303, "/project/"+pprojId)
		return
	}
	h := gin.H{}
	h["org"] = porg
	h["repo"] = prepo
	c.HTML(200, "genbranch.html", h)
}

func (a *Application) generateBranchWrk(repoName, fullName, branch, projId string) (string, error) {
	sha, err := h.GetBranchSha(repoName, fullName, branch)
	if err != nil {
		return "", err
	}
	project, err := a.rtrvr.GetProjectById(projId)
	if err != nil {
		return "", err
	}
	repository, err := project.GetRepository(fullName)
	if err != nil {
		return "", err
	}

	go func(repository *Repository, branch, sha string) {
		ref := "refs/heads/" + branch
		request := SingleRunnerRequest{
			repository: repository,
			sha:        sha,
			ref:        ref,
		}
		log.Println(repository.Fullname, sha, ref, projId)
		a.ff.FireRequest(&request)
	}(repository, branch, sha)
	return sha, nil
}

func (a *Application) genTagsWrk(projId string) (string, error) {
	project, err := a.rtrvr.GetProjectById(projId)
	if err != nil {
		return "", err
	}
	repos, err := project.GetAllRepositories()
	if err != nil {
		return "", err
	}
	go func(repos []*Repository, proj string) {
		for _, repo := range repos {
			name := strings.SplitN(repo.Fullname, "/", 2)[1]
			dat, err := h.NewTagsRunner(name, repo.Fullname).Run()
			if err != nil {
				log.Println("[TAG UPDATER] Was unable to run tags against " + repo.Fullname + ": [" + err.Error() + "]")
				continue
			}
			go func(dat map[string]string, repo *Repository) {
				for sha, ref := range dat {
					request := SingleRunnerRequest{
						repository: repo,
						sha:        sha,
						ref:        ref,
					}
					log.Println(repo, sha, ref)
					a.ff.FireRequest(&request)
				}
			}(dat, repo)
		}
	}(repos, projId)

	buf := bytes.NewBufferString("Trying to run against:\n")
	for _, repo := range repos {
		buf.WriteString("\n")
		buf.WriteString(repo.Fullname)
	}
	return buf.String(), nil
}
