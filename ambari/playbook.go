// Copyright 2018 Oliver Szabo
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

package ambari

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"strings"
)

const (
	RemoteCommand = "RemoteCommand"
	LocalCommand  = "LocalCommand"
	Download      = "Download"
	Upload        = "Upload"
)

// Playbook contains an array of tasks that will be executed on ambari hosts
type Playbook struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Tasks       []Task `yaml:"tasks"`
}

// Task represent a task that can be executed on an ambari hosts
type Task struct {
	Name                string            `yaml:"name"`
	Type                string            `yaml:"type"`
	Command             string            `yaml:"command"`
	HostComponentFilter string            `yaml:"host_component_filter"`
	AmbariServerFilter  bool              `yaml:"ambari_server"`
	AmbariAgentFilter   bool              `yaml:"ambari_agent"`
	HostFilter          string            `yaml:"hosts"`
	ServiceFilter       string            `yaml:"services"`
	ComponentFilter     string            `yaml:"components"`
	Parameters          map[string]string `yaml:"parameters,omitempty"`
}

// LoadPlaybookFile read a playbook yaml file and transform it to a Playbook object
func LoadPlaybookFile(location string) Playbook {
	data, err := ioutil.ReadFile(location)
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}
	playbook := Playbook{}
	err = yaml.Unmarshal([]byte(data), &playbook)
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}
	fmt.Println(fmt.Sprintf("[Executing playbook: %v, file: %v]", playbook.Name, location))
	return playbook
}

// ExecutePlaybook runs tasks on ambari hosts based on a playbook object
func (a AmbariRegistry) ExecutePlaybook(playbook Playbook) {
	tasks := playbook.Tasks
	for _, task := range tasks {
		if len(task.Type) > 0 {
			filteredHosts := make(map[string]bool)
			if !task.AmbariAgentFilter {
				filter := CreateFilter(task.ServiceFilter, task.ComponentFilter, task.HostFilter, task.AmbariServerFilter)
				filteredHosts = a.GetFilteredHosts(filter)
			}
			if task.Type == RemoteCommand {
				a.ExecuteRemoteCommandTask(task, filteredHosts)
			}
			if task.Type == LocalCommand {
				ExecuteLocalCommandTask(task)
			}
			if task.Type == Download {
				ExecuteDownloadFileTask(task)
			}
			if task.Type == Upload {
				a.ExecuteUploadFileTask(task, filteredHosts)
			}
		} else {
			if len(task.Name) > 0 {
				fmt.Println(fmt.Sprintf("Type field for task '%s' is required!", task.Name))
			} else {
				fmt.Println("Type field for task is required!")
			}
			os.Exit(1)
		}
	}
}

// ExecuteRemoteCommandTask executes a remote command on filtered hosts
func (a AmbariRegistry) ExecuteRemoteCommandTask(task Task, filteredHosts map[string]bool) {
	if len(task.Command) > 0 {
		fmt.Println("Executing remote command:" + task.Command)
		a.RunRemoteHostCommand(task.Command, filteredHosts)
	}
}

// ExecuteUploadFileTask upload a file to specific (filtered) hosts
func (a AmbariRegistry) ExecuteUploadFileTask(task Task, filteredHosts map[string]bool) {
	if task.Parameters != nil {
		haveSourceFile := false
		haveTargetFile := false
		if sourceVal, ok := task.Parameters["source"]; ok {
			haveSourceFile = true
			if targetVal, ok := task.Parameters["target"]; ok {
				haveTargetFile = true
				a.CopyToRemote(sourceVal, targetVal, filteredHosts)
			}
		}
		if !haveSourceFile {
			fmt.Println("'source' parameter is required for 'Upload' task")
			os.Exit(1)
		}
		if !haveTargetFile {
			fmt.Println("'target' parameter is required for 'Upload' task")
			os.Exit(1)
		}

	}
}

// ExecuteLocalCommandTask executes a local shell command
func ExecuteLocalCommandTask(task Task) {
	if len(task.Command) > 0 {
		fmt.Println("Execute local command: " + task.Command)
		splitted := strings.Split(task.Command, " ")
		if len(splitted) == 1 {
			RunLocalCommand(splitted[0])
		} else {
			RunLocalCommand(splitted[0], splitted[1:]...)
		}
	}
}

// ExecuteDownloadFileTask
func ExecuteDownloadFileTask(task Task) {
	if task.Parameters != nil {
		haveUrl := false
		haveFile := false
		if urlVal, ok := task.Parameters["url"]; ok {
			haveUrl = true
			if fileVal, ok := task.Parameters["file"]; ok {
				haveFile = true
				DownloadFile(fileVal, urlVal)
			}
		}
		if !haveFile {
			fmt.Println("'file' parameter is required for 'Download' task")
			os.Exit(1)
		}
		if !haveUrl {
			fmt.Println("'url' parameter is required for 'Download' task")
			os.Exit(1)
		}
	}
}
