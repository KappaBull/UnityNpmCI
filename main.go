package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unsafe"

	sh "github.com/codeskyblue/go-sh"
	"golang.org/x/crypto/ssh"
	git "gopkg.in/src-d/go-git.v4"
	gitssh "gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
	yaml "gopkg.in/yaml.v2"
)

//Config importConfigData
type Config struct {
	Repository string
	Check      string
	Pack       PackageJSON `yaml:"package"`
	License    string
	Copy       []string
}

//PackageJSON UnityPakageJsonData
type PackageJSON struct {
	Name        string            `json:"name"`
	Display     string            `json:"displayName"`
	Version     string            `json:"version"`
	Unity       string            `json:"unity"`
	Description string            `json:"description"`
	Dependencis map[string]string `json:"dependencis"`
}

func main() {

	//鍵関連
	sshKeyStr := os.Getenv("SSHKEY")
	signer, err := ssh.ParsePrivateKey(sbytes(sshKeyStr))
	if err != nil {
		log.Fatal(err)
	}
	auth := &gitssh.PublicKeys{User: "git", Signer: signer}

	dirName := "UnityNpm"
	npmDir, _ := ioutil.TempDir("", dirName)
	npmRepo, cloneErr := git.PlainClone(npmDir, false, &git.CloneOptions{
		URL:      "git@github.com:KappaBull/UnityNpm",
		Progress: os.Stdout,
		Auth:     auth,
	})
	if cloneErr != nil {
		log.Fatal(err)
	}
	npmRepoWork, _ := npmRepo.Worktree()

	session := sh.NewSession()
	session.ShowCMD = true
	npmDir = npmDir + "/" + dirName
	session.SetDir(npmDir)
	session.Command("git", "config", "--local", "user.name", "KappaBull").Run()
	session.Command("git", "config", "--local", "user.email", "kappa8v11@gmail.com").Run()
	masterCheckOpt := &git.CheckoutOptions{
		Branch: "master",
		Force:  true,
	}
	err = npmRepoWork.Checkout(masterCheckOpt)
	//err = session.Command("git", "checkout", "-f", "master").Run()
	if err != nil {
		log.Fatal(err)
	}
	filePaths, err := filepath.Glob(npmDir + "/*.yaml")
	if err != nil {
		log.Fatal(err)
	}
	for _, filePath := range filePaths {
		session.SetDir(npmDir)
		npmRepoWork.Checkout(masterCheckOpt)
		//session.Command("git", "checkout", "-f", "master").Run()
		var conf Config
		buf, err := ioutil.ReadFile(filePath)
		if err != nil {
			panic(err)
		}
		err = yaml.Unmarshal(buf, &conf)
		if err != nil {
			panic(err)
		}
		splits := strings.Split(conf.Repository, "/")
		repoName := strings.Replace(splits[len(splits)-1], ".git", "", -1)
		dir, _ := ioutil.TempDir("", repoName)
		session.SetDir(dir)
		//対象リポジトリをチェックアウト
		session.Command("git", "clone", conf.Repository).Run()
		dir = dir + "/" + repoName
		session.SetDir(dir)
		if conf.Check == "tag" {
			out, err := session.Command("git", "tag").Output()
			if err != nil {
				println("GitTagError")
				log.Fatal(err)
				continue
			}

			for _, tag := range strings.Fields(strings.Replace(bstring(out), "\\n", " ", -1)) {
				session.SetDir(dir)
				session.Command("git", "checkout", "-f", tag).Run()

				//CopyFileCheck
				if exists(dir+conf.License) == false {
					println("Not Found License File")
					continue
				}
				var allTargetFound bool
				allTargetFound = true
				for _, copyTarget := range conf.Copy {
					allTargetFound = exists(dir + copyTarget)
					if allTargetFound == false {
						println("Not Found:" + copyTarget)
						break
					}
				}
				if allTargetFound == false {
					continue
				}

				session.SetDir(npmDir)
				branchName := repoName + "-" + tag
				session.Command("git", "checkout", "-fb", branchName).Run()
				session.Command("ls").Command("grep", "-v", "-E", "'.git'").Command("xargs", "rm", "-r").Run()

				//package.json生成
				assined := regexp.MustCompile("([0-9]+)")
				group := assined.FindAllString(tag, -1)
				var version string
				for _, ver := range group {
					version = version + ver + "."
				}
				conf.Pack.Version = strings.TrimRight(version, ".")
				if conf.Pack.Dependencis == nil {
					conf.Pack.Dependencis = map[string]string{}
				}
				if conf.Pack.Unity == "" {
					conf.Pack.Unity = "2018.1"
				}
				if conf.Pack.Display == "" {
					conf.Pack.Display = repoName
				}
				jsonBytes, _ := json.Marshal(conf.Pack)
				if err := ioutil.WriteFile(npmDir+"/package.json", jsonBytes, 0644); err != nil {
					println("File I/O Error")
					continue
				}

				//対象ファイル追加
				if err = os.Rename(dir+conf.License, npmDir+"/"+filepath.Base(conf.License)); err != nil {
					continue
				}
				var copyFileErr error
				for _, copyTarget := range conf.Copy {
					if copyFileErr = os.Rename(dir+copyTarget, npmDir+"/"+filepath.Base(copyTarget)); copyFileErr != nil {
						break
					}
				}
				if copyFileErr != nil {
					continue
				}

				err = session.Command("git", "add", "--all").Run()
				if err != nil {
					log.Fatal(err)
					continue
				}

				err = session.Command("git", "commit", "-m", tag+" "+time.Now().Format("2006/01/02")).Run()
				if err != nil {
					if err.Error() == "nothing to commit, working tree clean" {
						println(branchName + " No update")
					} else {
						continue
					}
				}

				//TODO: テストの為、コメントアウト
				// err = session.Command("git", "push", "origin", "HEAD:"+branchName).Run()
				// if err != nil {
				// 	log.Fatal(err)
				// 	continue
				// }
			}
		}
	}
}

func exists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func bstring(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func sbytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&s))
}
