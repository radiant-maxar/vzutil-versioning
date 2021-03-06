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

package structs

import (
	"fmt"
	"html/template"
)

type HtmlForm struct {
	dat    string
	method string
}

func NewHtmlForm(elem fmt.Stringer) *HtmlForm {
	return &HtmlForm{elem.String(), "get"}
}
func (h *HtmlForm) Get() *HtmlForm {
	h.method = "get"
	return h
}
func (h *HtmlForm) Post() *HtmlForm {
	h.method = "post"
	return h
}

func (h *HtmlForm) Template() template.HTML {
	return template.HTML(h.String())
}

func (h *HtmlForm) String() string {
	return fmt.Sprintf("<form method=\"%s\">\n%s\n</form>", h.method, h.dat)
}
