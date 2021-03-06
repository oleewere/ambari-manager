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
	"bytes"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"strings"
	"text/template"
)

const (
	// RemoteCommand remote command type for running commands on the agent hosts
	RemoteCommand = "RemoteCommand"
	// LocalCommand local command type for running commands on localhost
	LocalCommand = "LocalCommand"
	// Download command type for downloading a file from an url
	Download = "Download"
	// Upload command type for uploading files to the agent hosts
	Upload = "Upload"
	// Config command type is for managing (update) configuration
	Config = "Config"
	// AmbariCommand runs an ambari command (like START or STOP) against components or services
	AmbariCommand = "AmbariCommand"
)

// Playbook contains an array of tasks that will be executed on ambari hosts
type Playbook struct {
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	Tasks       []Task  `yaml:"tasks"`
	Inputs      []Input `yaml:"inputs"`
}

// Task represents a task that can be executed on an ambari hosts
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

// Input represents a variable that needs to be provided by users (if default value is empty)
type Input struct {
	Name    string `yaml:"name"`
	Default string `yaml:"default,omitempty"`
}

// LoadPlaybookFile read a playbook yaml file and transform it to a Playbook object
func LoadPlaybookFile(location string, varsInput string) Playbook {
	varInputMap := createVarMap(varsInput)
	data, err := ioutil.ReadFile(location)
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}
	playbookTempl := Playbook{}
	err = yaml.Unmarshal([]byte(data), &playbookTempl)
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}
	if len(playbookTempl.Inputs) > 0 {
		for _, input := range playbookTempl.Inputs {
			if varVal, ok := varInputMap[input.Name]; ok {
				fmt.Println(fmt.Sprintf("Found input: %v - %v", input.Name, varVal))
				continue
			}
			if len(input.Default) == 0 {
				varSetByUser := GetStringFlag("", "", fmt.Sprintf("Enter %v", input.Name))
				varInputMap[input.Name] = varSetByUser
				continue
			}
			varInputMap[input.Name] = input.Default
		}
	}
	templ := template.New("playbook template")
	textTemplate, _ := templ.Parse(fmt.Sprintf("%s", data))
	var tpl bytes.Buffer
	textTemplate.Execute(&tpl, varInputMap)

	playbook := Playbook{}
	err = yaml.Unmarshal(tpl.Bytes(), &playbook)
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
			if task.Type == Config {
				a.ExecuteConfigCommand(task)
			}
			if task.Type == AmbariCommand {
				a.ExecuteAmbariCommand(task)
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

// ExecuteAmbariCommand executes an ambari command against services or components
func (a AmbariRegistry) ExecuteAmbariCommand(task Task) {
	if len(task.Command) > 0 {
		useComponentFilter := false
		useServiceFilter := false
		if len(task.ComponentFilter) > 0 {
			useComponentFilter = true
		} else if len(task.ServiceFilter) > 0 {
			useServiceFilter = true
		}

		if useComponentFilter {
			filter := CreateFilter("", task.ComponentFilter, "", false)
			a.RunAmbariServiceCommand(task.Command, filter, useServiceFilter, useComponentFilter)
		}
		if useServiceFilter {
			filter := CreateFilter(task.ServiceFilter, "", "", false)
			a.RunAmbariServiceCommand(task.Command, filter, useServiceFilter, useComponentFilter)
		}
	}
}

// ExecuteConfigCommand executes a configuration upgrade
func (a AmbariRegistry) ExecuteConfigCommand(task Task) {
	if task.Parameters != nil {
		haveConfigType := false
		haveConfigKey := false
		haveConfigValue := false
		if configType, ok := task.Parameters["config_type"]; ok {
			haveConfigType = true
			if configKey, ok := task.Parameters["config_key"]; ok {
				haveConfigKey = true
				if configValue, ok := task.Parameters["config_value"]; ok {
					haveConfigValue = true
					a.SetConfig(configType, configKey, configValue)
				}
			}
		}
		if !haveConfigType {
			fmt.Println("'config_type' parameter is required for 'Upload' task")
			os.Exit(1)
		}
		if !haveConfigKey {
			fmt.Println("'config_key' parameter is required for 'Upload' task")
			os.Exit(1)
		}
		if !haveConfigValue {
			fmt.Println("'config_value' parameter is required for 'Upload' task")
			os.Exit(1)
		}
	}
}

// ExecuteRemoteCommandTask executes a remote command on filtered hosts
func (a AmbariRegistry) ExecuteRemoteCommandTask(task Task, filteredHosts map[string]bool) {
	if len(task.Command) > 0 {
		fmt.Println("Execute remote command: " + task.Command)
		a.RunRemoteHostCommand(task.Command, filteredHosts, task.AmbariServerFilter)
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
				fmt.Println(fmt.Sprintf("Execute upload file command - source: %s, target: %s",
					task.Parameters["source"], task.Parameters["target"]))
				a.CopyToRemote(sourceVal, targetVal, filteredHosts, task.AmbariServerFilter)
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

// ExecuteDownloadFileTask download a file from an url to the local filesystem
func ExecuteDownloadFileTask(task Task) {
	if task.Parameters != nil {
		haveUrl := false
		haveFile := false
		if urlVal, ok := task.Parameters["url"]; ok {
			haveUrl = true
			if fileVal, ok := task.Parameters["file"]; ok {
				haveFile = true
				fmt.Println(fmt.Sprintf("Execute download file command - url: %s, location: %s",
					task.Parameters["url"], task.Parameters["file"]))
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

func createVarMap(varMapStr string) map[string]interface{} {
	resultMap := make(map[string]interface{})
	if len(varMapStr) > 0 {
		var ss []string
		ss = strings.Split(varMapStr, " ")
		for _, pair := range ss {
			z := strings.Split(pair, "=")
			resultMap[z[0]] = z[1]
		}
	}
	return resultMap
}
