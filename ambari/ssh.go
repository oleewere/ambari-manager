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
	"os"
	"strconv"
	"sync"
	"time"
)

// RemoteResponse represents an ssh command output
type RemoteResponse struct {
	StdOut string
	StdErr string
	Done   bool
}

// RunRemoteHostCommand executes bash commands on ambari agent hosts
func (a AmbariRegistry) RunRemoteHostCommand(command string, filteredHosts map[string]bool) map[string]RemoteResponse {
	connectionProfileId := a.ConnectionProfile
	if len(connectionProfileId) == 0 {
		fmt.Println("No connection profile is attached for the active ambari server entry!")
		os.Exit(1)
	}
	connectionProfile := GetConnectionProfileById(connectionProfileId)
	var hosts map[string]bool
	if len(filteredHosts) > 0 {
		hosts = filteredHosts
	} else {
		hosts = a.GetFilteredHosts(Filter{})
	}
	response := make(map[string]RemoteResponse)
	var wg sync.WaitGroup
	wg.Add(len(hosts))
	for host := range hosts {
		ssh := &MakeConfig{
			User:    connectionProfile.Username,
			Server:  host,
			KeyPath: connectionProfile.KeyPath,
			Port:    strconv.Itoa(connectionProfile.Port),
			Timeout: 60 * time.Second,
		}
		go func(ssh *MakeConfig, command string, host string, response map[string]RemoteResponse) {
			defer wg.Done()
			stdout, stderr, done, err := ssh.Run(command, 60)
			// Handle errors
			msgHeader := fmt.Sprintf("%v (done: %v) - output:", host, done)
			fmt.Println(msgHeader)
			if err != nil {
				panic("Can't run remote command: " + err.Error())
			} else {
				if len(stdout) > 0 {
					fmt.Println(stdout)
				}
				if len(stderr) > 0 {
					fmt.Println("std error:")
					fmt.Println(stderr)
				}
				response[host] = RemoteResponse{StdOut: stdout, StdErr: stderr, Done: done}
			}
		}(ssh, command, host, response)
	}
	wg.Wait()
	return response
}

// CopyFromRemote copy files locally from remote location
func (a AmbariRegistry) CopyFromRemote(dest string, filteredHosts map[string]bool) {
	connectionProfileId := a.ConnectionProfile
	if len(connectionProfileId) == 0 {
		fmt.Println("No connection profile is attached for the active ambari server entry!")
		os.Exit(1)
	}
	connectionProfile := GetConnectionProfileById(connectionProfileId)
	var hosts map[string]bool
	if len(filteredHosts) > 0 {
		hosts = filteredHosts
	} else {
		hosts = a.GetFilteredHosts(Filter{})
	}

	var wg sync.WaitGroup
	wg.Add(len(hosts))
	for host := range hosts {
		ssh := &MakeConfig{
			User:    connectionProfile.Username,
			Server:  host,
			KeyPath: connectionProfile.KeyPath,
			Port:    strconv.Itoa(connectionProfile.Port),
			Timeout: 60 * time.Second,
		}
		go func(ssh *MakeConfig, dest string, host string) {
			defer wg.Done()
			//ScpDownload(ssh, "", "")
		}(ssh, dest, host)
	}
	wg.Wait()
}