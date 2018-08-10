package main

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	sh "github.com/codeskyblue/go-sh"
	yaml "gopkg.in/yaml.v2"
)

//Config importConfigData
type Config struct {
	Repository string
	Check      string
	Pack       PackageJSON
	License    string
	Copy       []string
}

//PackageJSON UnityPakageJsonData
type PackageJSON struct {
	Name        string
	Display     string
	Version     string
	Unity       string
	Description string
	Dependencis string
}

func main() {

	filePaths, err := filepath.Glob("*.yaml")
	if err != nil {
		log.Fatal(err)
	}
	dirName := "UnityNpm"
	session := sh.NewSession()
	session.ShowCMD = true
	npmDir, _ := ioutil.TempDir("", dirName)
	session.SetDir(npmDir)
	session.Command("git", "clone", "git@github.com:KappaBull/"+dirName+".git").Run()
	npmDir = npmDir + "/" + dirName

	for _, filePath := range filePaths {
		var conf Config
		buf, err := ioutil.ReadFile(filePath)
		if err != nil {
			panic(err)
		}
		err = yaml.Unmarshal(buf, &conf)
		if err != nil {
			panic(err)
		}

		dir, _ := ioutil.TempDir("", conf.Pack.Name)
		session.SetDir(dir)
		//対象リポジトリをチェックアウト
		session.Command("git", "clone", conf.Repository).Run()
		dir = dir + "/" + conf.Pack.Name
		session.SetDir(dir)
		if conf.Check == "tag" {
			out, err := session.Command("git", "tag").Output()
			if err != nil {
				println("GitTagError")
				log.Fatal(err)
				continue
			}
			for _, tag := range strings.Fields(strings.Replace(Bstring(out), "\\n", " ", -1)) {
				session.SetDir(dir)
				session.Command("git", "checkout", tag).Run()
				session.SetDir(npmDir)
				branchName := conf.Pack.Name + "-" + tag
				session.Command("git", "checkout", "-b", branchName)
				session.Command("rm", "-rf", "*").Run()
				FileMove(dir+conf.License, npmDir+conf.License)
				for _, copyTarget := range conf.Copy {
					FileMove(dir+copyTarget, npmDir+copyTarget)
				}
				session.Command("git", "add", "--all").Run()
				err = session.Command("git", "commit", "-m", tag+" "+time.Now().Format("2006/01/02")).Run()
				if err != nil {
					log.Fatal(err)
					continue
				}
				err = session.Command("git", "push", "origin", "HEAD:"+branchName).Run()
				if err != nil {
					log.Fatal(err)
					continue
				}
			}
		}
	}
}

func FileMove(target string, destination string) error {
	if fileInfo, _ := os.Stat(target); fileInfo.IsDir() {
		files, err := ioutil.ReadDir(target)
		if err != nil {
			return err
		}

		for _, f := range files {
			exportPath := filepath.Join(destination, f.Name())
			if Exists(exportPath) {
				if err = os.Remove(exportPath); err != nil {
					return err
				}
			}
			if err := os.Rename(filepath.Join(target, f.Name()), exportPath); err != nil {
				return err
			}
		}
		return nil
	}
	err := os.Rename(target, destination)
	return err
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err) == false
}

//bstring byteをStringへキャストする
func Bstring(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

//sbytes stringをbyteへキャストする
func Sbytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&s))
}
