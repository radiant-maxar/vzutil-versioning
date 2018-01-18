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

package es

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/venicegeo/pz-gocommon/elasticsearch"
	"github.com/venicegeo/vzutil-versioning/web/f"
)

func GetProjectById(index *elasticsearch.Index, fullName string) (*Project, error) {
	docName := strings.Replace(fullName, "/", "_", -1)
	resp, err := index.GetByID("project", docName)
	if err != nil {
		return nil, err
	}
	if !resp.Found {
		return nil, errors.New("Could not find this document: [" + docName + "]")
	}
	project := &Project{}
	if err = json.Unmarshal([]byte(*resp.Source), project); err != nil {
		return nil, err
	}
	return project, nil
}

func CheckShaExists(index *elasticsearch.Index, fullName string, sha string) (bool, error) {
	exists, err := index.ItemExists("project", fullName)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	project, err := GetProjectById(index, fullName)
	if err != nil {
		return false, err
	}
	entries, err := project.GetEntries()
	if err != nil {
		return false, err
	}
	_, exists = (*entries)[sha]
	return exists, nil
}

func MatchAllSize(index *elasticsearch.Index, typ string, size int) (*elasticsearch.SearchResult, error) {
	return index.SearchByJSON(typ, f.Format(`
{
	"size": %d,
	"query":{}
}	
	`, size))
}
