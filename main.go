package main

import (
	"encoding/json"
	"fmt"
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
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
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

const (
	npmJSON     = "package.json"
	npmRepoName = "UnityNpm"
	ignoreFIlE  = ".gitignore"
)

func main() {

	//鍵関連
	sshKeyStr := os.Getenv("SSHKEY")
	signer, err := ssh.ParsePrivateKey([]byte(sshKeyStr))
	if err != nil {
		println("ImportKeyError")
		log.Fatal(err)
	}
	auth := &gitssh.PublicKeys{User: "git", Signer: signer}

	npmDir, _ := ioutil.TempDir("", npmRepoName)
	npmRepo, cloneErr := git.PlainClone(npmDir, false, &git.CloneOptions{
		URL:      "git@github.com:KappaBull/" + npmRepoName,
		Progress: os.Stdout,
		Auth:     auth,
	})
	if cloneErr != nil {
		println("CloneError")
		log.Fatal(cloneErr)
	}
	npmRepoWork, _ := npmRepo.Worktree()
	masterCheckOpt := &git.CheckoutOptions{
		Branch: "master",
		Force:  true,
		Create: true,
	}
	err = npmRepoWork.Checkout(masterCheckOpt)
	if err != nil {
		println("CheckOutError")
		log.Fatal(err)
	}

	filePaths, err := filepath.Glob(npmDir + "/*.yaml")
	if err != nil {
		log.Fatal(err)
	}
	confs := make([]Config, len(filePaths))
	for i, filePath := range filePaths {
		buf, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Fatal(err)
			continue
		}
		err = yaml.Unmarshal(buf, &confs[i])
		if err != nil {
			log.Fatal(err)
			continue
		}
	}

	session := sh.NewSession()
	session.ShowCMD = true
	session.SetDir(npmDir)
	for _, conf := range confs {
		splits := strings.Split(conf.Repository, "/")
		repoName := strings.Replace(splits[len(splits)-1], ".git", "", -1)
		dir, _ := ioutil.TempDir("", repoName)
		session.SetDir(dir)
		session.Command("git", "clone", conf.Repository).Run()
		dir = dir + "/" + repoName
		session.SetDir(dir)
		switch conf.Check {
		case "tag":
			out, err := session.Command("git", "tag").Output()
			if err != nil {
				println("GitTagError")
				log.Fatal(err)
				continue
			}

			for _, tag := range strings.Fields(strings.Replace(bstring(out), "\\n", " ", -1)) {
				err = nil
				session.SetDir(dir)
				session.Command("git", "checkout", "-f", tag).Run()

				if copyFileCheck(dir, conf) == false {
					continue
				}

				//Tag名からバージョンを生成
				var version string
				var count int
				for i, ver := range regexp.MustCompile("([0-9]+)").FindAllString(tag, -1) {
					version = version + ver + "."
					count = i
				}
				for i := count; i < 2; i++ {
					version = version + "0."
				}
				version = strings.TrimRight(version, ".")

				//ブランチ作成
				session.SetDir(npmDir)
				branchName := repoName + "/" + version
				ref := plumbing.ReferenceName(branchName)
				if err != nil {
					log.Println(err)
					continue
				}
				err = npmRepoWork.Checkout(&git.CheckoutOptions{
					Branch: ref,
					Force:  true,
					Create: true,
				})
				if err != nil {
					println("CheckOutError")
					log.Fatal(err)
					continue
				}

				ignoreAllRemove(npmDir, ".git")

				//package.json生成
				conf.Pack.Version = version
				if genPackageJSON(conf.Pack, repoName, npmDir) == false {
					continue
				}

				//対象ファイル追加
				if err = os.Rename(dir+conf.License, npmDir+"/"+filepath.Base(conf.License)); err != nil {
					continue
				}
				_, err = npmRepoWork.Add(filepath.Base(conf.License))
				if err != nil {
					log.Println(filepath.Base(conf.License) + " AddError")
					log.Println(err)
				}
				var copyFileErr error
				for _, copyTarget := range conf.Copy {
					if copyFileErr = os.Rename(dir+copyTarget, npmDir+"/"+filepath.Base(copyTarget)); copyFileErr != nil {
						break
					}
					_, err = npmRepoWork.Add(filepath.Base(copyTarget))
					if err != nil {
						log.Println(filepath.Base(copyTarget) + " AddError")
						log.Println(err)
					}
				}
				if copyFileErr != nil {
					continue
				}
				//gitignore生成
				if err := ioutil.WriteFile(npmDir+"/"+ignoreFIlE, []byte(npmJSON+".meta\n"+conf.License+".meta"), 0644); err != nil {
					println("File I/O Error")
					continue
				}
				_, err = npmRepoWork.Add(ignoreFIlE)
				if err != nil {
					log.Println(ignoreFIlE + " AddError")
					log.Println(err)
				}
				commitComment := tag + " " + time.Now().Format("2006/01/02")
				hash, comitErr := npmRepoWork.Commit(commitComment, &git.CommitOptions{
					All: true,
					Author: &object.Signature{
						Name:  "KappaBull",
						Email: "kappa8v11@gmail.com",
						When:  time.Now(),
					},
				})
				if comitErr != nil {
					log.Println("CommitError")
					log.Println(comitErr)
					continue
				}
				obj, err := npmRepo.CommitObject(hash)
				fmt.Println("Commit:" + commitComment + " Hash:" + hash.String())
				fmt.Println(obj)
				// npmRepo.Storer.SetReference(plumbing.NewReferenceFromStrings(branchName, hash.String()))
				err = npmRepo.Push(&git.PushOptions{
					RemoteName: "origin",
					Progress:   os.Stdout,
					RefSpecs: []config.RefSpec{
						config.RefSpec(ref + ":" + plumbing.ReferenceName("refs/heads/"+branchName)),
					},
					Auth: auth,
				})
				if err != nil {
					log.Println("PushError")
					log.Println(err)
				}
			}
		}

		//全部終わったら一旦Masterブランチへ戻す
		err = npmRepoWork.Checkout(masterCheckOpt)
		if err != nil {
			println("CheckOutError")
			log.Fatal(err)
		}
	}
}

func copyFileCheck(dir string, conf Config) bool {
	if exists(dir+conf.License) == false {
		println("Not Found License File")
		return false
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
		return false
	}
	return true
}

func genPackageJSON(pack PackageJSON, repoName string, npmDir string) bool {
	if pack.Version == "" {
		pack.Version = "0.0.0"
	}
	if pack.Dependencis == nil {
		pack.Dependencis = map[string]string{}
	}
	if pack.Unity == "" {
		pack.Unity = "2018.1"
	}
	if pack.Display == "" {
		pack.Display = repoName
	}
	jsonBytes, _ := json.Marshal(pack)
	if err := ioutil.WriteFile(npmDir+"/"+npmJSON, jsonBytes, 0644); err != nil {
		println("File I/O Error")
		return false
	}
	return true
}

func ignoreAllRemove(dir string, ignores ...string) {
	fileinfos, _ := ioutil.ReadDir(dir)
	for _, fileinfo := range fileinfos {
		var isIgnore bool
		for _, ignoreName := range ignores {
			isIgnore = fileinfo.Name() == ignoreName
			if isIgnore {
				break
			}
		}
		if isIgnore {
			continue
		}
		fileFullPath := dir + "/" + fileinfo.Name()
		if fileinfo.IsDir() {
			if err := os.RemoveAll(fileFullPath); err != nil {
				fmt.Println(err)
			}
		} else {
			if err := os.Remove(fileFullPath); err != nil {
				fmt.Println(err)
			}
		}
		fmt.Println("Del:" + fileFullPath)
	}
}

func exists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func bstring(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
