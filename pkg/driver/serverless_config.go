/*
 * MIT License
 *
 * Copyright (c) 2023 EASL and the vHive community
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

package driver

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/vhive-serverless/loader/pkg/common"
	"gopkg.in/yaml.v3"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// Serverless describes the serverless.yml contents.
type Serverless struct {
	Service          string                  `yaml:"service"`
	FrameworkVersion string                  `yaml:"frameworkVersion"`
	Provider         slsProvider             `yaml:"provider"`
	Package          slsPackage              `yaml:"package,omitempty"`
	Functions        map[string]*slsFunction `yaml:"functions"`
}

type slsProvider struct {
	Name             string `yaml:"name"`
	Runtime          string `yaml:"runtime"`
	Stage            string `yaml:"stage"`
	Region           string `yaml:"region"`
	VersionFunctions bool   `yaml:"versionFunctions"`
	ECR              slsECR `yaml:"ecr,omitempty"`
}

type slsECR struct {
	ScanOnPush bool                `yaml:"scanOnPush"`
	Images     map[string]slsImage `yaml:"images"`
}

type slsImage struct {
	Path     string `yaml:"path"`
	File     string `yaml:"file"`
	Platform string `yaml:"platform"`
}

type slsPackage struct {
	Patterns []string `yaml:"patterns"`
}

type slsFunction struct {
	Handler     string `yaml:"handler,omitempty"`
	Image       string `yaml:"image,omitempty"`
	Description string `yaml:"description,omitempty"`
	Name        string `yaml:"name"`
	Url         bool   `yaml:"url"`
	Timeout     int32  `yaml:"timeout,omitempty"`
	MemorySize  int32  `yaml:"memorySize,omitempty"`
}

// CreateHeader sets the fields Service, FrameworkVersion, and Provider
func (s *Serverless) CreateHeader(index int, provider string) {
	s.Service = fmt.Sprintf("loader-%d", index)
	s.FrameworkVersion = "3"
	s.Provider = slsProvider{
		Name:             provider,
		Runtime:          "provided.al2023", // Golang runtime deprecated, refer to https://aws.amazon.com/fr/blogs/compute/migrating-aws-lambda-functions-from-the-go1-x-runtime-to-the-custom-runtime-on-amazon-linux-2/
		Stage:            "dev",
		Region:           "us-east-1",
		VersionFunctions: false,
		ECR: slsECR{
			ScanOnPush: false,
			Images:     map[string]slsImage{},
		},
	}
	s.Functions = map[string]*slsFunction{}
}

func stringContains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

// AddPackagePattern adds a string pattern to Package.Pattern as long as such a pattern does not already exist in Package.Pattern
func (s *Serverless) AddPackagePattern(pattern string) {
	if !stringContains(s.Package.Patterns, pattern) {
		s.Package.Patterns = append(s.Package.Patterns, pattern)
	}
}

// AddImageConfig adds the slsImage configuration for container deployment as long as the imageName does not already exist in Provider.ECR.Images
func (s *Serverless) AddImageConfig(imageName string, path string, file string, platform string) {
	if _, ok := s.Provider.ECR.Images[imageName]; !ok {
		s.Provider.ECR.Images[imageName] = slsImage{Path: path, File: file, Platform: platform}
	}
}

// AddFunctionConfig adds the function configuration for serverless.com deployment
func (s *Serverless) AddFunctionConfig(function *common.Function, provider string, imageName string) {

	// Extract 0 from trace-func-0-2642643831809466437 by splitting on "-"
	shortName := strings.Split(function.Name, "-")[2]

	var handler string
	var image string
	var timeout int32
	var memorysize int32
	switch provider {
	case "aws":
		if imageName == "" {
			handler = "server/trace-func-go/aws/trace_func"
		} else {
			image = imageName
		}
		timeout = 900     // Maximum Lambda execution time of 15 min
		memorysize = 1024 // Default value by Serverless.com framework
	default:
		log.Fatalf("AddFunctionConfig could not recognize provider %s", provider)
	}

	f := &slsFunction{
		Handler:     handler,
		Image:       image,
		Description: "",
		Name:        shortName,
		Url:         true,
		Timeout:     timeout,
		MemorySize:  memorysize,
	}
	s.Functions[function.Name] = f
}

// CreateServerlessConfigFile dumps the contents of the Serverless struct into a yml file (serverless-<index>.yml)
func (s *Serverless) CreateServerlessConfigFile(index int) {
	data, err := yaml.Marshal(&s)
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile(fmt.Sprintf("./serverless-%d.yml", index), data, os.FileMode(0644))

	if err != nil {
		log.Fatal(err)
	}
}

// DeployServerless deploys the functions defined in the serverless.com file and returns a map from function name to URL
func DeployServerless(index int) map[int]string {
	slsDeployCmd := exec.Command("sls", "deploy", "--config", fmt.Sprintf("./serverless-%d.yml", index))
	stdoutStderr, err := slsDeployCmd.CombinedOutput()
	log.Debug("CMD response: ", string(stdoutStderr))

	// Extract the URLs from the output
	urlPattern := `https://\S+`
	urlRegex := regexp.MustCompile(urlPattern)
	urlMatches := urlRegex.FindAllStringSubmatch(string(stdoutStderr), -1)

	// Map the function names (endpoints) to the URLs (Serverless.com console outputs in order)
	functionToURL := make(map[int]string)
	for i := 0; i < len(urlMatches); i++ {
		functionToURL[i] = urlMatches[i][0]
	}

	if err != nil {
		log.Fatalf("Failed to deploy serverless-%d.yml: %v\n%s", index, err, stdoutStderr)
		return nil
	}

	log.Debugf("Deployed serverless-%d.yml", index)
	return functionToURL
}

// CleanServerless removes the deployed service and deletes the serverless-<index>.yml file
func CleanServerless(index int) bool {
	slsRemoveCmd := exec.Command("sls", "remove", "--config", fmt.Sprintf("./serverless-%d.yml", index))
	stdoutStderr, err := slsRemoveCmd.CombinedOutput()
	log.Debug("CMD response: ", string(stdoutStderr))

	if err != nil {
		log.Warnf("Failed to undeploy serverless-%d.yml: %v\n%s", index, err, stdoutStderr)
		return false
	}

	slsRemoveCmd = exec.Command("rm", "-f", fmt.Sprintf("./serverless-%d.yml", index))
	stdoutStderr, err = slsRemoveCmd.CombinedOutput()

	if err != nil {
		log.Warnf("Failed to delete serverless-%d.yml: %v\n%s", index, err, stdoutStderr)
		return false
	}

	log.Debugf("Undeployed and deleted serverless-%d.yml", index)
	return true
}
