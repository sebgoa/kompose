/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package openshift

import (
	"os"
	"path/filepath"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

	deployapi "github.com/openshift/origin/pkg/deploy/api"

	"github.com/kubernetes-incubator/kompose/pkg/kobject"
	"github.com/kubernetes-incubator/kompose/pkg/testutils"
)

func newServiceConfig() kobject.ServiceConfig {
	return kobject.ServiceConfig{
		ContainerName: "myfoobarname",
		Image:         "image",
		Environment:   []kobject.EnvVar{kobject.EnvVar{Name: "env", Value: "value"}},
		Port:          []kobject.Ports{kobject.Ports{HostPort: 123, ContainerPort: 456, Protocol: api.ProtocolTCP}},
		Command:       []string{"cmd"},
		WorkingDir:    "dir",
		Args:          []string{"arg1", "arg2"},
		Volumes:       []string{"/tmp/volume"},
		Network:       []string{"network1", "network2"}, // not supported
		Labels:        nil,
		Annotations:   map[string]string{"abc": "def"},
		CPUSet:        "cpu_set",            // not supported
		CPUShares:     1,                    // not supported
		CPUQuota:      1,                    // not supported
		CapAdd:        []string{"cap_add"},  // not supported
		CapDrop:       []string{"cap_drop"}, // not supported
		Expose:        []string{"expose"},   // not supported
		Privileged:    true,
		Restart:       "always",
		User:          "user", // not supported
		Stdin:         true,
		Tty:           true,
	}
}

func TestOpenShiftUpdateKubernetesObjects(t *testing.T) {
	t.Log("Test case: Testing o.UpdateKubernetesObjects()")
	var object []runtime.Object
	o := OpenShift{}
	serviceConfig := newServiceConfig()

	object = append(object, o.initDeploymentConfig("foobar", serviceConfig, 3))
	o.UpdateKubernetesObjects("foobar", serviceConfig, &object)

	for _, obj := range object {
		switch tobj := obj.(type) {
		case *deployapi.DeploymentConfig:
			t.Log("> Testing if stdin is set correctly")
			if tobj.Spec.Template.Spec.Containers[0].Stdin != serviceConfig.Stdin {
				t.Errorf("Expected stdin to be %v, got %v instead", serviceConfig.Stdin, tobj.Spec.Template.Spec.Containers[0].Stdin)
			}
			t.Log("> Testing if TTY is set correctly")
			if tobj.Spec.Template.Spec.Containers[0].TTY != serviceConfig.Tty {
				t.Errorf("Expected TTY to be %v, got %v instead", serviceConfig.Tty, tobj.Spec.Template.Spec.Containers[0].TTY)
			}
		}
	}
}

func TestInitDeploymentConfig(t *testing.T) {
	o := OpenShift{}
	spec := o.initDeploymentConfig("foobar", newServiceConfig(), 1)

	// Check that "foobar" is used correctly as a name
	if spec.Spec.Template.Spec.Containers[0].Name != "foobar" {
		t.Errorf("Expected foobar for name, actual %s", spec.Spec.Template.Spec.Containers[0].Name)
	}

	// Check that "myfoobarname" is used correctly as a ContainerName
	if spec.Spec.Triggers[1].ImageChangeParams.ContainerNames[0] != "myfoobarname" {
		t.Errorf("Expected myfoobarname for name, actual %s", spec.Spec.Triggers[1].ImageChangeParams.ContainerNames[0])
	}
}

func TestKomposeConvertRoute(t *testing.T) {

	o := OpenShift{}
	name := "app"
	sc := newServiceConfig()
	sc.ExposeService = "true"
	var port int32 = 5555
	route := o.initRoute(name, sc, port)

	if route.ObjectMeta.Name != name {
		t.Errorf("Expected %s for name, actual %s", name, route.ObjectMeta.Name)
	}
	if route.Spec.To.Name != name {
		t.Errorf("Expected %s for name, actual %s", name, route.Spec.To.Name)
	}
	if route.Spec.Port.TargetPort.IntVal != port {
		t.Errorf("Expected %d for port, actual %d", port, route.Spec.Port.TargetPort.IntVal)
	}
	if route.Spec.Host != "" {
		t.Errorf("Expected Spec.Host to not be set, got %s instead", route.Spec.Host)
	}

	sc.ExposeService = "example.com"
	route = o.initRoute(name, sc, port)

	if route.Spec.Host != sc.ExposeService {
		t.Errorf("Expected %s for Spec.Host, actual %s", sc.ExposeService, route.Spec.Host)
	}
}

func TestGetGitRemote(t *testing.T) {
	var output string
	var err error

	gitDir := testutils.CreateLocalGitDirectory(t)
	testutils.SetGitRemote(t, gitDir, "newremote", "https://git.test.com/somerepo")
	testutils.CreateGitRemoteBranch(t, gitDir, "newbranch", "newremote")
	dir := testutils.CreateLocalDirectory(t)
	defer os.RemoveAll(gitDir)
	defer os.RemoveAll(dir)

	testCases := map[string]struct {
		expectError bool
		dir         string
		branch      string
		output      string
	}{
		"Get git remote for branch success":   {false, gitDir, "newbranch", "https://git.test.com/somerepo.git"},
		"Get git remote error in non git dir": {true, dir, "", ""},
	}

	for name, test := range testCases {
		t.Log("Test case: ", name)
		output, err = getGitCurrentRemoteUrl(test.dir)

		if test.expectError {
			if err == nil {
				t.Errorf("Expected error, got success instead!")
			}
		} else {
			if err != nil {
				t.Errorf("Expected success, got error: %v", err)
			}
			if output != test.output {
				t.Errorf("Expected: %#v, got: %#v", test.output, output)
			}
		}
	}
}

func TestGitGetCurrentBranch(t *testing.T) {
	var output string
	var err error

	gitDir := testutils.CreateLocalGitDirectory(t)
	testutils.SetGitRemote(t, gitDir, "newremote", "https://git.test.com/somerepo")
	testutils.CreateGitRemoteBranch(t, gitDir, "newbranch", "newremote")
	dir := testutils.CreateLocalDirectory(t)
	defer os.RemoveAll(gitDir)
	defer os.RemoveAll(dir)

	testCases := map[string]struct {
		expectError bool
		dir         string
		output      string
	}{
		"Get git current branch success": {false, gitDir, "newbranch"},
		"Get git current branch error":   {true, dir, ""},
	}

	for name, test := range testCases {
		t.Log("Test case: ", name)
		output, err = getGitCurrentBranch(test.dir)

		if test.expectError {
			if err == nil {
				t.Errorf("Expected error, got success instead!")
			}
		} else {
			if err != nil {
				t.Errorf("Expected success, got error: %v", err)
			}
			if output != test.output {
				t.Errorf("Expected: %#v, got: %#v", test.output, output)
			}
		}
	}
}

func TestGetComposeFileDir(t *testing.T) {
	var output string
	var err error
	wd, _ := os.Getwd()

	testCases := map[string]struct {
		inputFile string
		output    string
	}{
		"Get compose file dir for relative input file path": {"foo/bar.yaml", filepath.Join(wd, "foo")},
		"Get compose file dir for abs input file path":      {"/abs/path/to/compose.yaml", "/abs/path/to"},
	}

	for name, test := range testCases {
		t.Log("Test case: ", name)

		output, err = getComposeFileDir(test.inputFile)

		if err != nil {
			t.Errorf("Expected success, got error: %#v", err)
		}

		if output != test.output {
			t.Errorf("Expected output: %#v, got: %#v", test.output, output)
		}
	}
}

func TestGetAbsBuildContext(t *testing.T) {
	var output string
	var err error

	gitDir := testutils.CreateLocalGitDirectory(t)
	testutils.SetGitRemote(t, gitDir, "newremote", "https://git.test.com/somerepo")
	testutils.CreateGitRemoteBranch(t, gitDir, "newbranch", "newremote")
	testutils.CreateSubdir(t, gitDir, "a/b")
	dir := testutils.CreateLocalDirectory(t)
	defer os.RemoveAll(gitDir)
	defer os.RemoveAll(dir)

	testCases := map[string]struct {
		expectError    bool
		context        string
		composeFileDir string
		output         string
	}{
		"Get abs build context success": {false, "./b/build", filepath.Join(gitDir, "a"), "a/b/build"},
		"Get abs build context error":   {true, "", dir, ""},
	}

	for name, test := range testCases {
		t.Log("Test case: ", name)
		output, err = getAbsBuildContext(test.context, test.composeFileDir)

		if test.expectError {
			if err == nil {
				t.Errorf("Expected error, got success instead!")
			}
		} else {
			if err != nil {
				t.Errorf("Expected success, got error: %v", err)
			}
			if output != test.output {
				t.Errorf("Expected: %#v, got: %#v", test.output, output)
			}
		}
	}
}

func TestInitBuildConfig(t *testing.T) {
	dir := testutils.CreateLocalGitDirectory(t)
	testutils.CreateSubdir(t, dir, "a/build")
	defer os.RemoveAll(dir)

	serviceName := "serviceA"
	composeFileDir := filepath.Join(dir, "a")
	repo := "https://git.test.com/org/repo"
	branch := "somebranch"
	sc := kobject.ServiceConfig{
		Build: "./build",
	}
	bc := initBuildConfig(serviceName, sc, composeFileDir, repo, branch)

	testCases := map[string]struct {
		field string
		value string
	}{
		"Assert buildconfig source git URI":     {bc.Spec.CommonSpec.Source.Git.URI, repo},
		"Assert buildconfig source git Ref":     {bc.Spec.CommonSpec.Source.Git.Ref, branch},
		"Assert buildconfig source context dir": {bc.Spec.CommonSpec.Source.ContextDir, "a/build"},
		"Assert buildconfig output name":        {bc.Spec.CommonSpec.Output.To.Name, serviceName + ":latest"},
	}

	for name, test := range testCases {
		t.Log("Test case: ", name)
		if test.field != test.value {
			t.Errorf("Expected: %#v, got: %#v", test.value, test.field)
		}
	}
}
