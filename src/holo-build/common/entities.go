/*******************************************************************************
*
* Copyright 2015 Stefan Majewsky <majewsky@gmx.net>
*
* This file is part of Holo.
*
* Holo is free software: you can redistribute it and/or modify it under the
* terms of the GNU General Public License as published by the Free Software
* Foundation, either version 3 of the License, or (at your option) any later
* version.
*
* Holo is distributed in the hope that it will be useful, but WITHOUT ANY
* WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR
* A PARTICULAR PURPOSE. See the GNU General Public License for more details.
*
* You should have received a copy of the GNU General Public License along with
* Holo. If not, see <http://www.gnu.org/licenses/>.
*
*******************************************************************************/

package common

import (
	"bytes"
	"regexp"
	"strings"

	"../../internal/toml"
)

//This file contains the parts of parser.go relating to the support for entity
//definitions (users and groups). As part of the initial parsing and validation
//process, these definitions are converted into an file entry in the package
//containing the entity definition file, so that other parts of holo-build do
//not need to know about entity definitions at all.

//UserSection only needs a nice exported name for the TOML parser to
//produce more meaningful error messages on malformed input data.
type UserSection struct {
	Name    string   `toml:"name"`
	Comment string   `toml:"comment"`
	UID     uint32   `toml:"uid"`
	System  bool     `toml:"system"`
	Home    string   `toml:"home"`
	Group   string   `toml:"group"`
	Groups  []string `toml:"groups"`
	Shell   string   `toml:"shell"`
}

//GroupSection only needs a nice exported name for the TOML parser to
//produce more meaningful error messages on malformed input data.
type GroupSection struct {
	Name   string `toml:"name"`
	Gid    uint32 `toml:"gid"`
	System bool   `toml:"system"`
}

//this regexp copied from useradd(8) manpage
var userOrGroupRx = regexp.MustCompile(`^[a-z_][a-z0-9_-]*\$?$`)

//parseUserOrGroupRef is used for references to users/groups in FS entries.
//Those references can either be an integer ID or a string name.
func parseUserOrGroupRef(value interface{}, ec *ErrorCollector, entryDesc string) *IntOrString {
	//default value
	if value == nil {
		return nil
	}

	switch val := value.(type) {
	case int64:
		if val < 0 {
			ec.Addf("%s is invalid: user or group ID \"%d\" may not be negative", entryDesc, val)
		}
		if val >= 1<<32 {
			ec.Addf("%s is invalid: user or group ID \"%d\" does not fit in uint32", entryDesc, val)
		}
		return &IntOrString{Int: uint32(val)}
	case string:
		if !userOrGroupRx.MatchString(val) {
			ec.Addf("%s is invalid: \"%s\" is not an acceptable user or group name", entryDesc, val)
		}
		return &IntOrString{Str: val}
	default:
		ec.Addf("%s is invalid: \"owner\"/\"group\" attributes must be strings or integers, found type %T", entryDesc, value)
		return nil
	}
}

var definitionFileRx = regexp.MustCompile(`^/usr/share/holo/users-groups/[^/]+.toml$`)

func compileEntityDefinitions(pkg PackageSection, groups []GroupSection, users []UserSection, ec *ErrorCollector) (node FSNode, path string) {
	//only add an entity definition file if it is required
	if len(groups) == 0 && len(users) == 0 {
		return nil, ""
	}

	//needs a valid definition file name
	switch {
	case pkg.DefinitionFile == "":
		ec.Addf("Cannot declare users/groups when package.definitionFile field is missing")
	case !definitionFileRx.MatchString(pkg.DefinitionFile):
		ec.Addf("\"%s\" is not an acceptable definition file (should look like \"/usr/share/holo/users-groups/01-foo.toml\")", pkg.DefinitionFile)
	}

	//validate users/groups
	for idx, group := range groups {
		validateGroup(group, ec, idx)
	}
	for idx, user := range users {
		validateUser(user, ec, idx)
	}

	//encode into a definition file
	s := struct {
		Group []GroupSection `toml:"group"`
		User  []UserSection  `toml:"user"`
	}{groups, users}
	var buf bytes.Buffer
	err := toml.NewEncoder(&buf).Encode(&s)
	if err != nil {
		ec.Addf("encoding of \"%s\" failed: %s", pkg.DefinitionFile, err.Error())
		return nil, ""
	}

	//toml.Encode does not support the omitempty flag yet, so remove unset fields manually
	pruneRx := regexp.MustCompile(`(?m:^\s*[a-z]+ = (?:0|""|false)$)\n`)
	content := pruneRx.ReplaceAllString(string(buf.Bytes()), "")

	return &FSRegularFile{
		Content:  content,
		Metadata: FSNodeMetadata{Mode: 0644},
	}, pkg.DefinitionFile
}

func validateGroup(group GroupSection, ec *ErrorCollector, entryIdx int) {
	//check group name
	switch {
	case group.Name == "":
		ec.Addf("group %d is invalid: missing \"name\" attribute", entryIdx)
	case !userOrGroupRx.MatchString(group.Name):
		ec.Addf("group \"%s\" is invalid: name is not an acceptable group name", group.Name)
	}

	//if GID is given, "system" attribute is useless since it's only used to choose a GID
	if group.System && group.Gid != 0 {
		ec.Addf("group \"%s\" is invalid: if \"gid\" is given, then \"system\" is useless", group.Name)
	}
}

func validateUser(user UserSection, ec *ErrorCollector, entryIdx int) {
	//check user name
	switch {
	case user.Name == "":
		ec.Addf("user %d is invalid: missing \"name\" attribute", entryIdx)
	case !userOrGroupRx.MatchString(user.Name):
		ec.Addf("user \"%s\" is invalid: name is not an acceptable user name", user.Name)
	}

	//if UID is given, "system" attribute is useless since it's only used to choose a UID
	if user.System && user.UID != 0 {
		ec.Addf("user \"%s\" is invalid: if \"uid\" is given, then \"system\" is useless", user.Name)
	}

	//check groups
	if user.Group != "" && !userOrGroupRx.MatchString(user.Group) {
		ec.Addf("user \"%s\" is invalid: \"%s\" is not an acceptable group name", user.Name, user.Group)
	}
	for _, group := range user.Groups {
		if !userOrGroupRx.MatchString(group) {
			ec.Addf("user \"%s\" is invalid: \"%s\" is not an acceptable group name", user.Name, group)
		}
	}

	//check home directory
	if user.Home != "" {
		if !strings.HasPrefix(user.Home, "/") {
			ec.Addf("user \"%s\" is invalid: home directory \"%s\" must be an absolute path", user.Name, user.Home)
		}
		if strings.HasSuffix(user.Home, "/") {
			ec.Addf("user \"%s\" is invalid: home directory \"%s\" has trailing slash(es)", user.Name, user.Home)
		}
	}
}
