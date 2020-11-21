//go:generate go-bindata -pkg internal -o ./internal/bindata.go ./data

package compat

import (
	"encoding/json"
	"log"
	"sort"
	"strings"

	"github.com/vmware/govmomi/compat/internal"
	"golang.org/x/mod/semver"
)

type db map[string]*API

var data db

func init() {
	data = make(map[string]*API)
	for _, n := range internal.AssetNames() {
		var api API
		if err := json.Unmarshal(internal.MustAsset(n), &api); err != nil {
			log.Fatal(err)
		}
		data[api.Version] = &api
	}
}

func versionLess(a, b string) bool {
	if !strings.HasPrefix(a, "v") {
		a = "v" + a
	}
	if !strings.HasPrefix(b, "v") {
		b = "v" + b
	}
	return semver.Compare(a, b) == -1
}

func APIVersions() []string {
	var versions []string
	for v := range data {
		versions = append(versions, v)
	}

	sort.Slice(versions, func(i, j int) bool {
		return versionLess(versions[i], versions[j])
	})
	return versions
}

func GetAPI(v string) *API {
	return data[v]
}
