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

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/venicegeo/pz-gocommon/elasticsearch"
)

var wrkr *worker

func main() {
	fmt.Println("Starting up...")

	if err := handleMaven(); err != nil {
		log.Fatal(err)
	}

	url, user, pass, err := getVcapES()
	fmt.Printf("The elasticsearch url has been found to be [%s]\n", url)
	if user != "" {
		fmt.Println("There is a username")
	}
	if pass != "" {
		fmt.Println("There is a password")
	}
	if err != nil {
		log.Fatal(err)
	}
	i, err := elasticsearch.NewIndex2(url, user, pass, "versioning_tool", `
{
	"mappings": {
		"project":{
			"dynamic":"strict",
			"properties":{
				"full_name":{"type":"string"},
				"name":{"type":"string"},
				"shas":{"type":"string"}
			}
		},
		"dependency":{
			"dynamic":"strict",
			"properties":{
				"hashsum":{"type":"string"},
				"name":{"type":"string"},
				"version":{"type":"string"},
				"language":{"type":"string"}
			}
		}
	}
}`)
	if err != nil {
		log.Fatal(err.Error())
	} else {
		fmt.Println(i.GetVersion())
	}

	wrkr = NewWorker(i)
	wrkr.Start()

	port := os.Getenv("PORT")
	if port == "" {
		port = "20012"
	}

	fmt.Println("Starting on port", port)
	server := Server{}
	server.Configure([]RouteData{RouteData{"GET", "/", defaultPath},
		RouteData{"POST", "/webhook", webhookPath}})
	err = <-server.Start(":" + port)
	fmt.Println(err)
}

func addFSifMissing(url string) string {
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	return url
}

func getRequiredEnv(env string) string {
	temp := os.Getenv(env)
	if temp == "" {
		log.Fatal("Missing env var", env)
	}
	return temp
}

func defaultPath(c *gin.Context) {
	c.String(200, "Welcome to the dependency service!")
}
func webhookPath(c *gin.Context) {
	var git GitWebhook

	if err := c.BindJSON(&git); err != nil {
		log.Println("Unable to bind json:", err.Error())
		c.Status(400)
		return
	}

	fmt.Println(git.Repository.FullName, git.AfterSha)
	c.Status(200)

	wrkr.AddTask(git)
}

func handleMaven() error {
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
		return fmt.Errorf("Couldnt find maven settings location")
	}
	return exec.Command("mv", "settings.xml", finds[1]).Run()
}
