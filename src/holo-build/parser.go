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

package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	build "github.com/holocm/libpackagebuild"
	"github.com/holocm/libpackagebuild/filesystem"
)

//PackageDefinition only needs a nice exported name for the TOML parser to
//produce more meaningful error messages on malformed input data.
type PackageDefinition struct {
	Package   PackageSection
	File      []FileSection
	Directory []DirectorySection
	Symlink   []SymlinkSection
	Action    []ActionSection
	User      []UserSection  //see common/entities.go
	Group     []GroupSection //see common/entities.go
}

//PackageSection only needs a nice exported name for the TOML parser to produce
//more meaningful error messages on malformed input data.
type PackageSection struct {
	Name           string
	Version        string
	PrereleaseType string
	PrereleaseNo   uint
	Release        uint
	Epoch          uint
	Description    string
	Author         string
	Architecture   string
	Requires       []string
	Provides       []string
	Conflicts      []string
	Replaces       []string
	SetupScript    string
	CleanupScript  string
	DefinitionFile string //see compileEntityDefinitions
}

//FileSection only needs a nice exported name for the TOML parser to produce
//more meaningful error messages on malformed input data.
type FileSection struct {
	Path        string
	Content     string
	ContentFrom string
	Raw         bool
	Mode        string      //TOML does not support octal number literals, so we have to write: mode = "0666"
	Owner       interface{} //either string (name) or integer (ID)
	Group       interface{} //same
	//NOTE: We could use custom types implementing TextUnmarshaler for Mode,
	//Owner and Group, but then toml.Decode would accept any primitive type.
	//But for Mode, we need the type enforcement to prevent the "mode = 0666"
	//error (which would be 666 in decimal = something else in octal). And for
	//Owner and Group, we need to distinguish IDs from names using the type.
}

//DirectorySection only needs a nice exported name for the TOML parser to
//produce more meaningful error messages on malformed input data.
type DirectorySection struct {
	Path  string
	Mode  string      //see above
	Owner interface{} //see above
	Group interface{} //see above
}

//SymlinkSection only needs a nice exported name for the TOML parser to produce
//more meaningful error messages on malformed input data.
type SymlinkSection struct {
	Path   string
	Target string
}

//ActionSection only needs a nice exported name for the TOML parser to produce
//more meaningful error messages on malformed input data.
type ActionSection struct {
	On     string
	Script string
}

//versions are dot-separated numbers like (0|[1-9][0-9]*) (this enforces no
//trailing zeros)
var versionRx = regexp.MustCompile(`^(?:0|[1-9][0-9]*)(?:\.(?:0|[1-9][0-9]*))*$`)

//the author information should be in the form "Firstname Lastname <email.address@server.tld>"
var authorRx = regexp.MustCompile(`^[^<>]+\s+<[^<>\s]+>$`)

//map supported input strings for architecture to internal architecture enum;
//the "BEGIN ARCH" and "END ARCH" comments are used by test/generate-architecture-tests.sh
var archMap = map[string]build.Architecture{
	//BEGIN ARCH
	"aarch64": build.ArchitectureAArch64,
	"all":     build.ArchitectureAny, //from Debian
	"amd64":   build.ArchitectureX86_64,
	"any":     build.ArchitectureAny,     //from Arch Linux
	"arm":     build.ArchitectureARMv5,   //from Arch Linux
	"arm64":   build.ArchitectureAArch64, //from Debian
	"armel":   build.ArchitectureARMv5,   //from Debian
	"armhf":   build.ArchitectureARMv7h,  //from Debian
	"armv5tl": build.ArchitectureARMv5,   //from Mageia
	"armv6h":  build.ArchitectureARMv6h,  //from Arch Linux
	"armv6hl": build.ArchitectureARMv6h,  //from OpenSuse
	"armv7h":  build.ArchitectureARMv7h,  //from Arch Linux
	"armv7hl": build.ArchitectureARMv7h,  //from OpenSuse
	"i386":    build.ArchitectureI386,
	"i686":    build.ArchitectureI386,
	"noarch":  build.ArchitectureAny, //from RPM
	"x86_64":  build.ArchitectureX86_64,
	//END ARCH
}

//map supported input strings for prerelease types to internal prerelease type enum
var prerelTypeMap = map[string]build.PrereleaseType{
	"none":  build.PrereleaseTypeNone,
	"alpha": build.PrereleaseTypeAlpha,
	"beta":  build.PrereleaseTypeBeta,
}

//ParsePackageDefinition parses a package definition from the given input.
//The operation is successful if the returned []error is empty.
func ParsePackageDefinition(input io.Reader, baseDirectory string) (*build.Package, []error) {
	//read from input
	blob, err := ioutil.ReadAll(input)
	if err != nil {
		return nil, []error{err}
	}
	var p PackageDefinition
	_, err = toml.Decode(string(blob), &p)
	if err != nil {
		return nil, []error{err}
	}

	//restructure the parsed data into a common.Package struct
	pkg := build.Package{
		Name:              strings.TrimSpace(p.Package.Name),
		Version:           strings.TrimSpace(p.Package.Version),
		PrereleaseVersion: p.Package.PrereleaseNo,
		Release:           p.Package.Release,
		Epoch:             p.Package.Epoch,
		Description:       strings.TrimSpace(p.Package.Description),
		Author:            strings.TrimSpace(p.Package.Author),
		ArchitectureInput: p.Package.Architecture,
		Actions:           []build.PackageAction{},
		FSRoot:            filesystem.NewDirectory(),
	}
	pkg.FSRoot.Implicit = true

	//parse prerelease type string
	if p.Package.PrereleaseType != "" {
		var ok bool
		pkg.PrereleaseType, ok = prerelTypeMap[p.Package.PrereleaseType]
		if !ok {
			err := fmt.Errorf("Invalid prereleaseType \"%s\"", p.Package.PrereleaseType)
			return nil, []error{err}
		}
	}

	if pkg.PrereleaseType == build.PrereleaseTypeNone && pkg.PrereleaseVersion != 0 {
		err := fmt.Errorf("Invalid (nonzero) prereleaseNo (%d) for prereleaseType \"none\"", p.Package.PrereleaseNo)
		return nil, []error{err}
	}

	if pkg.PrereleaseType != build.PrereleaseTypeNone && pkg.PrereleaseVersion == 0 {
		err := fmt.Errorf("Invalid prereleaseNo (0) for prereleaseType \"%s\" (not \"none\")", p.Package.PrereleaseType)
		return nil, []error{err}
	}

	if script := strings.TrimSpace(p.Package.SetupScript); script != "" {
		WarnDeprecatedKey("package.setupScript")
		pkg.AppendActions(build.PackageAction{
			Type:    build.SetupAction,
			Content: script,
		})
	}

	if script := strings.TrimSpace(p.Package.CleanupScript); script != "" {
		WarnDeprecatedKey("package.cleanupScript")
		pkg.AppendActions(build.PackageAction{
			Type:    build.CleanupAction,
			Content: script,
		})
	}

	//default value for Release is 1
	if pkg.Release == 0 {
		pkg.Release = 1
	}

	//do some basic validation on the package name and version since we're
	//going to use these to construct a path
	ec := &ErrorCollector{}
	switch {
	case pkg.Name == "":
		ec.Addf("Missing package name")
	case strings.ContainsAny(pkg.Name, "/\r\n"):
		ec.Addf("Invalid package name \"%s\" (may not contain slashes or newlines)", pkg.Name)
		pkg.Name = "" // don't complain about the broken value again in generator.Validate()
	}
	switch {
	case pkg.Version == "":
		ec.Addf("Missing package version")
	case !versionRx.MatchString(pkg.Version):
		ec.Addf("Invalid package version \"%s\" (must be a chain of numbers like \"1.2.0\" or \"20151104\")", pkg.Version)
		pkg.Version = "" // don't complain about the broken value again in generator.Validate()
	}
	if strings.ContainsAny(pkg.Description, "\r\n") {
		ec.Addf("Invalid package description \"%s\" (may not contain newlines)", pkg.Name)
		pkg.Description = "" // don't complain about the broken value again in generator.Validate()
	}
	//the author field is not required (except for --debian), but if it is
	//given, check the format
	if pkg.Author != "" && !authorRx.MatchString(pkg.Author) {
		ec.Addf("Invalid package author \"%s\" (should look like \"Jane Doe <jane.doe@example.org>\")", pkg.Author)
	}

	//parse architecture string
	if p.Package.Architecture != "" {
		var ok bool
		pkg.Architecture, ok = archMap[p.Package.Architecture]
		if !ok {
			ec.Addf("Invalid package architecture \"%s\"", p.Package.Architecture)
		}
	}

	//parse relations to other packages
	pkg.Requires = parseRelatedPackages("requires", p.Package.Requires, ec)
	pkg.Provides = parseRelatedPackages("provides", p.Package.Provides, ec)
	pkg.Conflicts = parseRelatedPackages("conflicts", p.Package.Conflicts, ec)
	pkg.Replaces = parseRelatedPackages("replaces", p.Package.Replaces, ec)

	//compile entity definition file
	entityNode, entityPath := compileEntityDefinitions(p.Package, p.Group, p.User, ec)
	if entityNode != nil && entityPath != "" {
		ec.Add(pkg.InsertFSNode(entityPath, entityNode))
	}

	//parse and validate actions
	for idx, actSection := range p.Action {
		action, isValid := parseAction(actSection, ec, idx)
		if isValid {
			pkg.AppendActions(action)
		}
	}

	//parse and validate FS entries
	for idx, dirSection := range p.Directory {
		path := dirSection.Path
		isPathValid := validatePath(path, ec, "directory", idx)

		entryDesc := fmt.Sprintf("directory \"%s\"", path)
		dirNode := filesystem.NewDirectory()
		dirNode.Metadata = filesystem.NodeMetadata{
			Mode:  parseFileMode(dirSection.Mode, 0755, ec, entryDesc),
			Owner: parseUserOrGroupRef(dirSection.Owner, ec, entryDesc),
			Group: parseUserOrGroupRef(dirSection.Group, ec, entryDesc),
		}
		if isPathValid {
			ec.Add(pkg.InsertFSNode(path, dirNode))
		}
	}

	for idx, fileSection := range p.File {
		path := fileSection.Path
		isPathValid := validatePath(path, ec, "file", idx)

		entryDesc := fmt.Sprintf("file \"%s\"", path)
		node := &filesystem.RegularFile{
			Content: parseFileContent(fileSection.Content, fileSection.ContentFrom, fileSection.Raw, baseDirectory, ec, entryDesc),
			Metadata: filesystem.NodeMetadata{
				Mode:  parseFileMode(fileSection.Mode, 0644, ec, entryDesc),
				Owner: parseUserOrGroupRef(fileSection.Owner, ec, entryDesc),
				Group: parseUserOrGroupRef(fileSection.Group, ec, entryDesc),
			},
		}
		if isPathValid {
			ec.Add(pkg.InsertFSNode(path, node))
		}
	}

	for idx, symlinkSection := range p.Symlink {
		path := symlinkSection.Path
		isPathValid := validatePath(path, ec, "symlink", idx)

		if symlinkSection.Target == "" {
			ec.Addf("symlink \"%s\" is invalid: missing target", path)
		}

		node := &filesystem.Symlink{Target: symlinkSection.Target}
		if isPathValid {
			ec.Add(pkg.InsertFSNode(path, node))
		}
	}

	return &pkg, ec.Errors
}

//relatedPackageRx and providesPackageRx are nearly identical, except that for a "provides" relation, only the operator "=" is acceptable
var relatedPackageRx = regexp.MustCompile(`^([^\s<=>]+)\s*(?:(<=?|>=?|=)\s*([^\s<=>]+))?$`)
var providesPackageRx = regexp.MustCompile(`^([^\s<=>]+)\s*(?:(=)\s*([^\s<=>]+))?$`)

func parseRelatedPackages(relType string, specs []string, ec *ErrorCollector) []build.PackageRelation {
	rels := make([]build.PackageRelation, 0, len(specs))
	idxByName := make(map[string]int, len(specs))

	for _, spec := range specs {
		//which format to use?
		rx := relatedPackageRx
		if relType == "provides" {
			rx = providesPackageRx
		}

		//check format of spec
		match := rx.FindStringSubmatch(spec)
		if match == nil {
			ec.Addf("Invalid package reference in %s: \"%s\"", relType, spec)
			continue
		}

		//do we have a relation to this package already?
		name := match[1]
		idx, exists := idxByName[name]
		if !exists {
			//no, add a new one and remember it for later additional constraints
			idx = len(rels)
			idxByName[name] = idx
			rels = append(rels, build.PackageRelation{RelatedPackage: name})
		}

		//add version constraint if one was specified
		if match[2] != "" {
			constraint := build.VersionConstraint{Relation: match[2], Version: match[3]}
			rels[idx].Constraints = append(rels[idx].Constraints, constraint)
		}
	}

	return rels
}

//maps string values of "action.on" to internal enum values
var actionTypeMap = map[string]uint{
	"setup":   build.SetupAction,
	"cleanup": build.CleanupAction,
}

func parseAction(data ActionSection, ec *ErrorCollector, entryIdx int) (action build.PackageAction, isValid bool) {
	action.Type, isValid = actionTypeMap[data.On]
	if !isValid {
		if data.On == "" {
			ec.Addf("action %d is invalid: missing or empty \"on\" attribute", entryIdx)
		} else {
			ec.Addf("action %d is invalid: unacceptable value \"%s\" for \"on\" attribute", entryIdx, data.On)
		}
	}

	action.Content = strings.TrimSpace(data.Script)
	if action.Content == "" {
		ec.Addf("action %d is invalid: missing or empty \"script\" attribute", entryIdx)
		isValid = false
	}
	return
}

//path is the path to be validated.
//entryType and entryIdx are used for error messages and describe the entry.
func validatePath(path string, ec *ErrorCollector, entryType string, entryIdx int) bool {
	if path == "" {
		ec.Addf("%s %d is invalid: missing \"path\" attribute", entryType, entryIdx)
		return false
	}
	if !strings.HasPrefix(path, "/") {
		ec.Addf("%s \"%s\" is invalid: must be an absolute path", entryType, path)
		return false
	}
	if strings.HasSuffix(path, "/") {
		ec.Addf("%s \"%s\" is invalid: trailing slash(es)", entryType, path)
		return false
	}
	return true
}

func parseFileMode(modeStr string, defaultMode os.FileMode, ec *ErrorCollector, entryDesc string) os.FileMode {
	//default value
	if modeStr == "" {
		return defaultMode
	}

	//parse modeStr as uint in base 8 to uint32 (== os.FileMode)
	value, err := strconv.ParseUint(modeStr, 8, 32)
	if err != nil {
		ec.Addf("%s is invalid: cannot parse mode \"%s\" (%s)", entryDesc, modeStr, err.Error())
	}
	return os.FileMode(value)
}

func parseFileContent(content string, contentFrom string, dontPruneIndent bool, baseDirectory string, ec *ErrorCollector, entryDesc string) string {
	//option 1: content given verbatim in "content" field
	if content != "" {
		if contentFrom != "" {
			ec.Addf("%s is invalid: cannot use both `content` and `contentFrom`", entryDesc)
		}
		if dontPruneIndent {
			return content
		}
		return string(pruneIndentation([]byte(content)))
	}

	//option 2: content referenced in "contentFrom" field
	if contentFrom == "" {
		ec.Addf("%s is invalid: missing content", entryDesc)
		return ""
	}
	if !strings.HasPrefix(contentFrom, "/") {
		//resolve relative paths
		contentFrom = filepath.Join(baseDirectory, contentFrom)
	}
	if opts.filenameOnly {
		return string("")
	}
	bytes, err := ioutil.ReadFile(contentFrom)
	ec.Add(err)
	return string(bytes)
}

func pruneIndentation(text []byte) []byte {
	//split into lines for analysis
	lines := bytes.Split(text, []byte{'\n'})

	//use the indentation of the first non-empty line as a starting point for the longest common prefix
	var prefix []byte
	for _, line := range lines {
		if len(line) != 0 {
			lineWithoutIndentation := bytes.TrimLeft(line, "\t ")
			prefix = line[:len(line)-len(lineWithoutIndentation)]
			break
		}
	}

	//find the longest common prefix (from the starting point, remove trailing
	//characters until it *is* the longest common prefix)
	for len(prefix) > 0 {
		found := true
		for _, line := range lines {
			if len(line) == 0 {
				continue
			}
			if !bytes.HasPrefix(line, prefix) {
				//not the longest common prefix yet -> chop off one byte and retry
				prefix = prefix[:len(prefix)-1]
				found = false
				break
			}
		}
		if found {
			break
		}
	}

	//remove the longest common prefix from all non-empty lines
	if len(prefix) == 0 {
		return text //fast exit
	}
	for idx, line := range lines {
		if len(line) > 0 {
			lines[idx] = line[len(prefix):]
		}
	}
	return bytes.Join(lines, []byte{'\n'})
}
