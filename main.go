package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

type dependency struct {
	Vcs  string `json:"vcs"`  // type of the repository
	Repo string `json:"repo"` // repository of the dependency
	Rev  string `json:"rev"`  // revision of the dependency
	Path string `json:"path"` // where we stored the dependency in GOPATH
}

func (d *dependency) install(srcPath string) error {
	log.Printf("Installing %v", d.Repo)
	if err := os.Chdir(srcPath); err != nil {
		return fmt.Errorf("Failed to navigate to srcPath")
	}
	if err := os.MkdirAll(d.Path, 0755); err != nil {
		return fmt.Errorf("Failed to mkdir %s", d.Path)
	}
	switch d.Vcs {
	case "git":
		gitDir := path.Join(srcPath, d.Path, ".git")
		_, err := os.Stat(gitDir)
		if os.IsNotExist(err) {
			d.bootstrap(srcPath)
		}

		if err := os.Chdir(d.Path); err != nil {
			return fmt.Errorf("Failed to change to %s", d.Path)
		}
		out, err := exec.Command("git", "rev-parse", "HEAD").CombinedOutput()
		if err != nil {
			d.bootstrap(srcPath)
		}
		if strings.TrimSpace(string(out)) == d.Rev {
			log.Printf("%s already installed", d.Repo)
			return nil
		}

		if err := exec.Command("git", "reset", "--quiet", "--hard",
			d.Rev).Run(); err != nil {
			if err := exec.Command("git", "fetch").Run(); err != nil {
				return fmt.Errorf("Failed to fetch latest revisions", err)
			}
			if err := exec.Command("git", "reset", "--quiet", "--hard",
				d.Rev).Run(); err != nil {
				return fmt.Errorf("Failed to change to git rev %s: %v", d.Rev, err)
			}
		}

	case "hg":
		if err := exec.Command("hg", "clone", "--quiet", "--updaterev", d.Rev,
			d.Repo, d.Path).Run(); err != nil {
			return fmt.Errorf("Failed to clone hg %s: %v", d.Path, err)
		}
	}
	return nil
}

func (d *dependency) bootstrap(srcPath string) error {
	log.Printf("bootstrapping %v", srcPath)
	path := path.Join(srcPath, d.Path)
	if err := exec.Command("git", "clone", "--quiet", d.Repo, path).Run(); err != nil {
		return fmt.Errorf("Failed to clone git %s: %v", d.Path, err)
	}
	return nil
}

func main() {
	var depPath string
	if len(os.Args) == 1 {
		depPath = "deps.json"
	} else {
		depPath = os.Args[1]
	}

	// read dependency file
	deps, err := readDependencies(depPath)
	if err != nil {
		log.Fatalf("Invalid dependency file %s: %v", depPath, err)
	}

	// write .env file
	writeEnv()

	// create _vendor directory in current dir
	srcPath, err := createVendor()
	if err != nil {
		log.Fatalf("Failed to create _vendor directory: %v", err)
	}

	// start installing dependencies
	for _, dep := range deps {
		if err := dep.install(srcPath); err != nil {
			log.Fatalf("Failed to install %v: %v", dep.Repo, err)
		}
	}
	fmt.Println("Dependencies written into _vendor/src")
}

func createVendor() (string, error) {
	srcPath := "_vendor/src"
	if err := os.MkdirAll(srcPath, 0755); err != nil {
		return "", errors.New("Failed to create _vendor directory")
	}
	return filepath.Abs(srcPath)
}

func readDependencies(depPath string) ([]*dependency, error) {
	depData, err := ioutil.ReadFile(depPath)
	if err != nil {
		return nil, err
	}
	var deps []*dependency
	if err := json.Unmarshal(depData, &deps); err != nil {
		return nil, err
	}
	return deps, nil
}

var envTips = `Written "export GOPATH=$(pwd)/_vendor:$GOPATH" into .env
You can autoload .env file with "https://github.com/kennethreitz/autoenv"
`

func writeEnv() {
	_, err := os.Stat(".env")
	if err == nil {
		log.Println(".env exists. Skipping...")
		return
	}
	if os.IsNotExist(err) {
		err := ioutil.WriteFile(".env",
			[]byte(`export GOPATH=$(pwd)/_vendor:$GOPATH`), 0755)
		if err != nil {
			log.Fatal("Failed to write .env file")
		}
		fmt.Println(envTips)
	}
}
