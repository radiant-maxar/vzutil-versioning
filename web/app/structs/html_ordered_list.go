// Copyright 2019, RadiantBlue Technologies, Inc.
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

package structs

import (
	"bytes"
	"html/template"
)

type HtmlOrderedList struct {
	id    string
	items []template.HTML
}

func NewHtmlOrderedList(id string, items ...template.HTML) *HtmlOrderedList {
	return &HtmlOrderedList{id, items}
}

func (h *HtmlOrderedList) Add(items ...template.HTML) *HtmlOrderedList {
	h.items = append(h.items, items...)
	return h
}

func (h *HtmlOrderedList) Template() template.HTML {
	return template.HTML(h.String())
}

func (h *HtmlOrderedList) String() string {
	buf := bytes.NewBufferString(`<ol id="`)
	buf.WriteString(h.id)
	buf.WriteString(`">`)
	for _, i := range h.items {
		buf.WriteString("<li>")
		buf.WriteString(string(i))
		buf.WriteString("</li>")
	}
	buf.WriteString("</ol>")
	return buf.String()
}
