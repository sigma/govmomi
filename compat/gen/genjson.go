// +build ignore

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"

	"github.com/antchfx/htmlquery"
	"github.com/sigma/vmomidb"
	"golang.org/x/net/html"
)

var (
	versionsFile string
	versionsURL  string
	destDir      string
)

func init() {
	const (
		defaultVersionsURL = "https://vdc-download.vmware.com/vmwb-repository/dcr-public/b50dcbbf-051d-4204-a3e7-e1b618c1e384/538cf2ec-b34f-4bae-a332-3820ef9e7773/api_versions_all_index.html"
		defaultDir         = "./data"
	)

	flag.StringVar(&versionsFile, "versions_file", "", "versions reference file")
	flag.StringVar(&versionsURL, "versions_url", defaultVersionsURL, "versions reference URL")
	flag.StringVar(&destDir, "dest_dir", defaultDir, "directory where to dump json files")
}

type api vmomidb.API

func (a *api) getManagedObject(alias string) *vmomidb.ManagedObject {
	o, ok := a.ManagedObjects[alias]
	if !ok {
		o = &vmomidb.ManagedObject{
			Methods:    make(map[string]*vmomidb.Field),
			Properties: make(map[string]*vmomidb.Field),
		}
		a.ManagedObjects[alias] = o
	}
	return o
}

func (a *api) getDataObject(alias string) *vmomidb.DataObject {
	o, ok := a.DataObjects[alias]
	if !ok {
		o = &vmomidb.DataObject{
			Properties: make(map[string]*vmomidb.Field),
		}
		a.DataObjects[alias] = o
	}
	return o
}

func (a *api) getEnum(alias string) *vmomidb.Enum {
	o, ok := a.Enums[alias]
	if !ok {
		o = &vmomidb.Enum{
			Constants: make(map[string]*vmomidb.Field),
		}
		a.Enums[alias] = o
	}
	return o
}

func (a *api) getFault(alias string) *vmomidb.Fault {
	o, ok := a.Faults[alias]
	if !ok {
		o = &vmomidb.Fault{
			Properties: make(map[string]*vmomidb.Field),
		}
		a.Faults[alias] = o
	}
	return o
}

type db struct {
	versions      map[string]*api
	registrations map[string]func(string) map[string]*vmomidb.Field
}

func newDB() *db {
	return &db{
		versions:      make(map[string]*api),
		registrations: make(map[string]func(string) map[string]*vmomidb.Field),
	}
}

func (d *db) getVersionAPI(v string) *api {
	a, ok := d.versions[v]
	if !ok {
		a = &api{
			Version:        v,
			ManagedObjects: make(map[string]*vmomidb.ManagedObject),
			DataObjects:    make(map[string]*vmomidb.DataObject),
			Enums:          make(map[string]*vmomidb.Enum),
			Faults:         make(map[string]*vmomidb.Fault),
		}
		d.versions[v] = a
	}
	return a
}

func (d *db) parseAPIVersions(table *html.Node) []string {
	var versions []string
	for _, v := range htmlquery.Find(table, "//th[@style]/strong/text()") {
		versions = append(versions, v.Data)
	}
	return versions
}

func getAttr(n *html.Node, a string) string {
	for _, attr := range n.Attr {
		if attr.Key == a {
			return attr.Val
		}
	}
	return ""
}

func getFQN(link *html.Node) string {
	cpts := strings.Split(getAttr(link, "href"), "#")
	cpts[0] = strings.TrimSuffix(cpts[0], ".html")
	return strings.Join(cpts, ".")
}

func getDeprecation(texts []*html.Node) []bool {
	var depr []bool
	for _, t := range texts {
		switch t.Data {
		case "":
		case "X":
			depr = append(depr, false)
		case "D":
			depr = append(depr, true)
		}
	}
	return depr
}

type registrarFunc func(string) func(string) map[string]*vmomidb.Field

func (d *db) parseManagedObjects(table *html.Node) error {
	getter := func(version, alias string) *vmomidb.Field {
		return &d.getVersionAPI(version).getManagedObject(alias).Field
	}
	registrars := map[string]registrarFunc{
		"m+": func(alias string) func(string) map[string]*vmomidb.Field {
			return func(version string) map[string]*vmomidb.Field {
				return d.getVersionAPI(version).getManagedObject(alias).Methods
			}
		},
		"p+": func(alias string) func(string) map[string]*vmomidb.Field {
			return func(version string) map[string]*vmomidb.Field {
				return d.getVersionAPI(version).getManagedObject(alias).Properties
			}
		},
	}
	return d.parseMainTable(table, getter, registrars)
}

func (d *db) parseDataObjects(table *html.Node) error {
	getter := func(version, alias string) *vmomidb.Field {
		return &d.getVersionAPI(version).getDataObject(alias).Field
	}
	registrars := map[string]registrarFunc{
		"p+": func(alias string) func(string) map[string]*vmomidb.Field {
			return func(version string) map[string]*vmomidb.Field {
				return d.getVersionAPI(version).getDataObject(alias).Properties
			}
		},
	}
	return d.parseMainTable(table, getter, registrars)
}

func (d *db) parseEnums(table *html.Node) error {
	getter := func(version, alias string) *vmomidb.Field {
		return &d.getVersionAPI(version).getEnum(alias).Field
	}
	registrars := map[string]registrarFunc{
		"c+": func(alias string) func(string) map[string]*vmomidb.Field {
			return func(version string) map[string]*vmomidb.Field {
				return d.getVersionAPI(version).getEnum(alias).Constants
			}
		},
	}
	return d.parseMainTable(table, getter, registrars)
}

func (d *db) parseFaults(table *html.Node) error {
	getter := func(version, alias string) *vmomidb.Field {
		return &d.getVersionAPI(version).getFault(alias).Field
	}
	registrars := map[string]registrarFunc{
		"p+": func(alias string) func(string) map[string]*vmomidb.Field {
			return func(version string) map[string]*vmomidb.Field {
				return d.getVersionAPI(version).getFault(alias).Properties
			}
		},
	}
	return d.parseMainTable(table, getter, registrars)
}

func (d *db) parseMainTable(table *html.Node, getter func(string, string) *vmomidb.Field, registrars map[string]registrarFunc) error {
	versions := d.parseAPIVersions(table)

	rows := htmlquery.Find(table, "//tr[position()>1]")
	for _, r := range rows {
		link := htmlquery.FindOne(r, "//td[@id]/a")
		fqn := getFQN(link)
		alias := htmlquery.InnerText(link)
		depr := getDeprecation(htmlquery.Find(r, "//td[@align]/text()"))

		for i := range depr {
			obj := getter(versions[len(versions)-i-1], alias)
			obj.FQN = fqn
			obj.Deprecated = depr[len(depr)-i-1]
		}

		apis := htmlquery.Find(r, "//td/input")
		for _, api := range apis {
			key := getAttr(api, "value")
			id := strings.Split(getAttr(api, "onclick"), "','")[1]
			d.registrations[id] = registrars[key](alias)
		}
	}
	return nil
}

func (d *db) parseAPITable(table *html.Node) error {
	registrar := d.registrations[getAttr(table, "id")]
	versions := d.parseAPIVersions(table)

	rows := htmlquery.Find(table, "//tr[position()>1]")
	for _, r := range rows {
		link := htmlquery.FindOne(r, "//td/a")
		fqn := getFQN(link)
		alias := htmlquery.InnerText(link)
		depr := getDeprecation(htmlquery.Find(r, "//td[@align]/text()"))

		for i := range depr {
			registrar(versions[len(versions)-i-1])[alias] = &vmomidb.Field{
				FQN:        fqn,
				Deprecated: depr[len(depr)-i-1],
			}
		}
	}
	return nil
}

func (d *db) parseDoc(doc *html.Node) error {
	parsers := []struct {
		name       string
		parserFunc func(*html.Node) error
	}{
		{"managedObjects", d.parseManagedObjects},
		{"dataObjects", d.parseDataObjects},
		{"enums", d.parseEnums},
		{"faults", d.parseFaults},
	}

	for _, p := range parsers {
		query := fmt.Sprintf("//h2[@id='%s']/following-sibling::table[1]", p.name)
		table := htmlquery.FindOne(doc, query)
		if err := p.parserFunc(table); err != nil {
			return err
		}
	}

	apiTables := htmlquery.Find(doc, "//div[@class='apiTable']")
	for _, t := range apiTables {
		if err := d.parseAPITable(t); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	flag.Parse()

	ref := versionsURL
	loader := htmlquery.LoadURL
	if versionsFile != "" {
		ref = versionsFile
		loader = htmlquery.LoadDoc
	}

	doc, err := loader(ref)
	if err != nil {
		log.Fatal(err)
	}

	db := newDB()
	if err := db.parseDoc(doc); err != nil {
		log.Fatal(err)
	}

	for _, api := range db.versions {
		body, err := json.MarshalIndent(api, "", "  ")
		if err != nil {
			log.Fatal(err)
		}

		fname := fmt.Sprintf("vsphere-%s.json", api.Version)
		if err := ioutil.WriteFile(filepath.Join(destDir, fname), body, 0644); err != nil {
			log.Fatal(err)
		}
	}
}
