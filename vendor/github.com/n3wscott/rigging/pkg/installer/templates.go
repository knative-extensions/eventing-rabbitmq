/*
Copyright 2019 The Rigging Authors

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

package installer

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

func ParseTemplates(path string, config map[string]interface{}) string {
	dir, err := ioutil.TempDir("", "processed_yaml")
	if err != nil {
		panic(err)
	}

	err = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if info == nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), "yaml") {
			t, err := template.ParseFiles(path)
			if err != nil {
				return err
			}
			tmpfile, err := ioutil.TempFile(dir, strings.Replace(info.Name(), ".yaml", "-*.yaml", 1))
			if err != nil {
				log.Fatal(err)
			}
			err = t.Execute(tmpfile, config)
			if err != nil {
				log.Print("execute: ", err)
				return err
			}
			_ = tmpfile.Close()
		}
		return nil
	})
	log.Print("new files in ", dir)
	if err != nil {
		panic(err)
	}
	return dir
}
