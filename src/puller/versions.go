package main

import (
	"io/ioutil"
	"os"
	"encoding/json"
)

func getDashboardsVersions() (versions map[string]int, err error) {
	versions = make(map[string]int)

	filename := *clonePath + "/versions.json"

	_, err = os.Stat(filename)
	if os.IsNotExist(err) {
		return versions, nil
	}

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}

	err = json.Unmarshal(data, &versions)
	return
}

func writeVersions(versions map[string]int, dv map[string]diffVersion) (err error) {
	for slug, diff := range dv {
		versions[slug] = diff.newVersion
	}

	rawJSON, err := json.Marshal(versions)
	if err != nil {
		return
	}

	indentedJSON, err := indent(rawJSON)
	if err != nil {
		return
	}

	filename := *clonePath + "/versions.json"
	return rewriteFile(filename, indentedJSON)
}
