package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	outputStream io.Writer = os.Stdout
	errStream    io.Writer = os.Stderr
)

type Command struct {
	releaseBranch        string
	gitRoot              string
	release              string
	artifactBucket       string

	makeTarget string
	makeArgs   []string

	dryRun bool
}

func (c *Command) buildProject(projectPath string) error {
	commandArgs := []string{
		"-C",
		filepath.Join(c.gitRoot, "projects", projectPath),
		c.makeTarget,
	}
	commandArgs = append(commandArgs, c.makeArgs...)

	cmd := exec.Command("make", commandArgs...)
	log.Printf("Executing: %s", strings.Join(cmd.Args, " "))
	cmd.Stdout = outputStream
	cmd.Stderr = errStream
	if !c.dryRun {
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("Error running make: %v", err)
		}
	}
	return nil
}

func main() {
	target := flag.String("target", "release", "Make target")
	releaseBranch := flag.String("release-branch", "1-18", "Release branch to test")
	release := flag.String("release", "1", "Release to test")
	development := flag.Bool("development", false, "Build as a development build")
	region := flag.String("region", "us-west-2", "AWS region to use")
	accountId := flag.String("account-id", "", "AWS Account ID to use")
	baseImage := flag.String("base-image", "", "Base container image")
	imageRepo := flag.String("image-repo", "", "Container image repository")
	goRunnerImage := flag.String("go-runner-image", "", "go-runner image")
	kubeProxyBase := flag.String("kube-proxy-base", "", "kube-proxy base image")
	artifactBucket := flag.String("artifact-bucket", "", "S3 bucket for artifacts")
	gitRoot := flag.String("git-root", "", "Git root directory")
	dryRun := flag.Bool("dry-run", false, "Echo out commands, but don't run them")

	flag.Parse()
	log.Printf("Running postsubmit - dry-run: %t", *dryRun)

	c := &Command{
		releaseBranch:        *releaseBranch,
		release:              *release,
		artifactBucket:       *artifactBucket,
		gitRoot:              *gitRoot,
		makeTarget:           *target,
		dryRun:               *dryRun,
	}
	if c.gitRoot == "" {
		gitRootOutput, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
		if err != nil {
			log.Fatalf("Error running finding git root: %v", err)
		}
		c.gitRoot = strings.Fields(string(gitRootOutput))[0]
	}
	c.makeArgs = []string{
		fmt.Sprintf("RELEASE_BRANCH=%s", c.releaseBranch),
		fmt.Sprintf("RELEASE=%s", c.release),
		fmt.Sprintf("DEVELOPMENT=%t", *development),
		fmt.Sprintf("AWS_REGION=%s", *region),
		fmt.Sprintf("AWS_ACCOUNT_ID=%s", *accountId),
		fmt.Sprintf("BASE_IMAGE=%s", *baseImage),
		fmt.Sprintf("IMAGE_REPO=%s", *imageRepo),
		fmt.Sprintf("GO_RUNNER_IMAGE=%s", *goRunnerImage),
		fmt.Sprintf("KUBE_PROXY_BASE_IMAGE=%s", *kubeProxyBase),
		"IMAGE_TAG='$(GIT_TAG)-$(PULL_BASE_SHA)'",
	}

	cmd := exec.Command("git", "-C", *gitRoot, "diff", "--name-only", "HEAD^", "HEAD")
	log.Printf("Executing command: %s", strings.Join(cmd.Args, " "))
	gitDiffOutput, err := cmd.Output()
	if err != nil {
		log.Fatalf("error running git diff: %v\n%s", err, string(gitDiffOutput))
	}
	filesChanged := strings.Fields(string(gitDiffOutput))

	allChanged := false
	projects := map[string]*struct {
		changed         bool
	}{
		"kubernetes/kubernetes": {},
		"kubernetes/release": {},
		"coredns/coredns": {},
		"containernetworking/plugins": {},
		"kubernetes-sigs/aws-iam-authenticator": {},
		"kubernetes-sigs/metrics-server": {},
		"etcd-io/etcd": {},
		"kubernetes-csi/external-attacher": {},
		"kubernetes-csi/external-resizer": {},
		"kubernetes-csi/livenessprobe": {},
		"kubernetes-csi/node-driver-registrar": {},
		"kubernetes-csi/external-snapshotter": {},
		"kubernetes-csi/external-provisioner": {},
	}

	for _, file := range filesChanged {
		for projectPath := range projects {
			if strings.Contains(file, projectPath) {
				projects[projectPath].changed = true
			}
		}
		r := regexp.MustCompile("Makefile|cmd/main_postsubmit.go|release/.*")
		if r.MatchString(file) {
			allChanged = true
		}
	}
	for projectPath, config := range projects {
		if config.changed || allChanged {
			err = c.buildProject(projectPath)
			if err != nil {
				log.Fatalf("error building %s: %v", projectPath, err)
			}
		}
	}
}
