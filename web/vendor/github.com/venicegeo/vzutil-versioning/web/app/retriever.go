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
	"errors"
	"sort"
	"strings"
	"sync"

	nt "github.com/venicegeo/pz-gocommon/gocommon"
	s "github.com/venicegeo/vzutil-versioning/web/app/structs"
	"github.com/venicegeo/vzutil-versioning/web/es"
	u "github.com/venicegeo/vzutil-versioning/web/util"
)

type Retriever struct {
	app *Application
}

func NewRetriever(app *Application) *Retriever {
	return &Retriever{app}
}

func (r *Retriever) DepsByShaName(fullName, sha string) ([]es.Dependency, bool, error) {
	var project *es.Project
	var err error
	var found bool

	if project, found, err = es.GetProjectById(r.app.index, fullName); err != nil {
		return nil, found, err
	} else if !found {
		return nil, false, nil
	}
	return r.DepsByShaProject(project, sha)
}
func (r *Retriever) DepsByShaNameGen(fullName, sha string) ([]es.Dependency, error) {
	deps, found, err := r.app.rtrvr.DepsByShaName(fullName, sha)
	if err != nil || !found {
		{
			code, _, _, err := nt.HTTP(nt.HEAD, u.Format("https://github.com/%s/commit/%s", fullName, sha), nt.NewHeaderBuilder().GetHeader(), nil)
			if err != nil {
				return nil, u.Error("Could not verify this sha: %s", err.Error())
			}
			if code != 200 {
				return nil, u.Error("Could not verify this sha, head code: %d", code)
			}
		}
		res := r.app.snglRnnr.RunAgainstSingle(&s.GitWebhook{AfterSha: sha, Repository: s.GitRepository{FullName: fullName}})
		if res == nil {
			return nil, u.Error("Sha [%s] did not previously exist and could not be generated", sha)
		}
		deps = res.Deps
		sort.Sort(es.DependencySort(deps))
	}
	return deps, nil
}
func (r *Retriever) DepsByShaProject(project *es.Project, sha string) (res []es.Dependency, exists bool, err error) {
	ref, entry, exists := project.GetEntry(sha)
	if !exists {
		return nil, false, nil
	}
	if entry.EntryReference != "" {
		if entry, exists = ref.GetEntry(entry.EntryReference); !exists {
			return nil, true, errors.New("The database is corrupted, this sha points to a sha that doesnt exist: " + entry.EntryReference)
		}
	}
	mux := &sync.Mutex{}
	done := make(chan bool, len(entry.Dependencies))
	work := func(dep string) {
		if resp, err := r.app.index.GetByID("dependency", dep); err != nil || !resp.Found {
			name := u.Format("Cound not find [%s]", dep)
			tmp := es.Dependency{name, "", ""}
			mux.Lock()
			res = append(res, tmp)
			mux.Unlock()
		} else {
			var depen es.Dependency
			if err = json.Unmarshal([]byte(*resp.Source), &depen); err != nil {
				tmp := es.Dependency{u.Format("Error getting [%s]: [%s]", dep, err.Error()), "", ""}
				mux.Lock()
				res = append(res, tmp)
				mux.Unlock()
			} else {
				mux.Lock()
				res = append(res, depen)
				mux.Unlock()
			}
		}
		done <- true
	}

	for _, d := range entry.Dependencies {
		go work(d)
	}
	for i := 0; i < len(entry.Dependencies); i++ {
		<-done
	}
	sort.Sort(es.DependencySort(res))
	return res, true, nil
}

func (r *Retriever) DepsByRef(info ...string) (map[string][]es.Dependency, error) {
	switch len(info) {
	case 1: //just a ref
		ref := info[0]
		p, e := es.GetAllProjects(r.app.index, r.app.searchSize)
		return r.byRefWork(p, e, ref)
	case 2: //org and ref
		org := info[0]
		ref := info[1]
		p, e := es.GetProjectsOrg(r.app.index, org, r.app.searchSize)
		return r.byRefWork(p, e, ref)
	case 3: //org repo and ref
		org := info[0]
		repo := info[1]
		ref := info[2]
		p, o, e := es.GetProjectById(r.app.index, org+"_"+repo)
		if !o {
			return nil, u.Error("Unable to find doc [%s_%s]", org, repo)
		}
		return r.byRefWork(&[]*es.Project{p}, e, ref)
	}
	return nil, errors.New("Sorry, something is wrong with the code..")
}

func (r *Retriever) byRefWork(projects *[]*es.Project, err error, ref string) (map[string][]es.Dependency, error) {
	if err != nil {
		return nil, err
	}
	res := map[string][]es.Dependency{}
	mux := &sync.Mutex{}
	errs := make(chan error, len(*projects))
	work := func(project *es.Project, e chan error) {
		var refp *es.Ref = nil
		for _, r := range project.Refs {
			if r.Name == `refs/`+ref {
				refp = r
				break
			}
		}
		if refp == nil {
			e <- nil
			return
		}
		if len(refp.WebhookOrder) == 0 {
			e <- nil
			return
		}
		sha := refp.WebhookOrder[0]
		deps, found, err := r.DepsByShaProject(project, sha)
		if err != nil {
			e <- err
			return
		} else if !found {
			e <- u.Error("Could not find sha [%s]", sha)
			return
		}
		sort.Sort(es.DependencySort(deps))
		mux.Lock()
		{
			res[project.FullName] = deps
		}
		mux.Unlock()
		e <- nil
	}
	for _, project := range *projects {
		go work(project, errs)
	}
	for i := 0; i < len(*projects); i++ {
		err := <-errs
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

func (r *Retriever) byRef1(ref string) (map[string][]es.Dependency, error) {
	projects, err := es.GetAllProjects(r.app.index, r.app.searchSize)
	if err != nil {
		return nil, err
	}

	res := map[string][]es.Dependency{}
	mux := &sync.Mutex{}
	errs := make(chan error, len(*projects))
	work := func(project *es.Project) {
		var refp *es.Ref = nil
		for _, r := range project.Refs {
			if r.Name == ref {
				refp = r
				break
			}
		}
		if refp == nil {
			return
		}
		if len(refp.WebhookOrder) == 0 {
			return
		}
		sha := refp.WebhookOrder[0]
		deps, found, err := r.DepsByShaProject(project, sha)
		if err != nil {
			errs <- err
			return
		} else if !found {
			errs <- u.Error("Could not find sha [%s]", sha)
		}
		mux.Lock()
		{
			res[project.FullName] = deps
			errs <- nil
		}
		mux.Unlock()
	}
	for _, project := range *projects {
		go work(project)
	}
	for i := 0; i < len(*projects); i++ {
		err := <-errs
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

func (r *Retriever) byRef2(org, tag string) (map[string][]es.Dependency, error) {
	projects, err := es.GetProjectsOrg(r.app.index, org, r.app.searchSize)
	if err != nil {
		return nil, err
	}
	res := map[string][]es.Dependency{}
	mux := &sync.Mutex{}
	errs := make(chan error, len(*projects))
	work := func(project *es.Project) {
		sha, exists := project.GetTagFromSha(tag)
		if !exists {
			errs <- errors.New("Could not find sha for tag " + tag)
			return
		}
		deps, found, err := r.DepsByShaProject(project, sha)
		if err != nil {
			errs <- err
			return
		} else if !found {
			errs <- u.Error("Project [%s] not found", project.FullName)
			return
		}
		mux.Lock()
		res[project.FullName] = deps
		mux.Unlock()
		errs <- nil
	}
	for _, project := range *projects {
		go work(project)
	}
	for i := 0; i < len(*projects); i++ {
		err := <-errs
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

func (r *Retriever) byRef3(docName, tag string) (map[string][]es.Dependency, error) {
	var project *es.Project
	var err error
	var sha string
	var ok bool

	if project, ok, err = es.GetProjectById(r.app.index, docName); err != nil {
		return nil, err
	} else if !ok {
		return nil, u.Error("Could not find [%s]", docName)
	}
	ok = false

	for _, ts := range project.TagShas {
		if ts.Tag == tag {
			sha = ts.Sha
			ok = true
			break
		}
	}
	if !ok {
		return nil, errors.New("Could not find this tag: [" + tag + "]")
	}
	deps, found, err := r.DepsByShaProject(project, sha)
	if err != nil {
		return nil, err
	} else if !found {
		return nil, u.Error("Could not find p")
	}
	return map[string][]es.Dependency{strings.Replace(docName, "_", "/", 1): deps}, nil
}

//

func (r *Retriever) ListShas(fullName string) (map[string][]string, int, error) {
	var project *es.Project
	res := map[string][]string{}
	count := 0
	var err error
	var found bool

	if project, found, err = es.GetProjectById(r.app.index, fullName); err != nil {
		return nil, 0, err
	} else if !found {
		return nil, 0, u.Error("Could not find project [%s]", fullName)
	}

	for _, ref := range project.Refs {
		res[ref.Name] = make([]string, len(ref.Entries), len(ref.Entries))
		for i, s := range ref.Entries {
			res[ref.Name][i] = s.Sha
		}
		count += len(ref.Entries)
	}
	return res, count, nil
}

//

func (r *Retriever) ListRefsRepo(fullName string) (*[]string, error) {
	project, found, err := es.GetProjectById(r.app.index, fullName)
	if err != nil {
		return nil, err
	} else if !found {
		return nil, u.Error("Could not find project [%s]", fullName)
	}
	res := make([]string, len(project.Refs), len(project.Refs))
	for i, r := range project.Refs {
		res[i] = strings.TrimPrefix(r.Name, `refs/`)
	}
	return &res, nil
}
func (r *Retriever) ListRefs(org string) (*map[string][]string, int, error) {
	projects, err := es.GetProjectsOrg(r.app.index, org, r.app.searchSize)
	if err != nil {
		return nil, 0, err
	}
	res := map[string][]string{}
	numTags := 0
	errs := make(chan error, len(*projects))
	mux := &sync.Mutex{}

	work := func(project *es.Project) {
		num := len(project.Refs)
		temp := make([]string, num, num)
		for i, r := range project.Refs {
			temp[i] = strings.TrimPrefix(r.Name, `refs/`)
		}
		sort.Strings(temp)
		mux.Lock()
		{
			numTags += num
			res[project.FullName] = temp
			errs <- nil
		}
		mux.Unlock()
	}

	for _, p := range *projects {
		go work(p)
	}
	for i := 0; i < len(*projects); i++ {
		err := <-errs
		if err != nil {
			return nil, 0, err
		}
	}
	return &res, numTags, err
}

//

func (r *Retriever) ListProjects() ([]string, error) {
	return r.listProjectsWrk(es.GetAllProjects(r.app.index, r.app.searchSize))

}
func (r *Retriever) ListProjectsByOrg(org string) ([]string, error) {
	return r.listProjectsWrk(es.GetProjectsOrg(r.app.index, org, r.app.searchSize))
}
func (r *Retriever) listProjectsWrk(projects *[]*es.Project, err error) ([]string, error) {
	if err != nil {
		return nil, err
	}
	res := make([]string, len(*projects))
	for i, project := range *projects {
		res[i] = project.FullName
	}
	sort.Strings(res)
	return res, nil
}
