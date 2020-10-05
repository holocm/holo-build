/*******************************************************************************
*
* Copyright 2015-2018 Stefan Majewsky <majewsky@gmx.net>
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You should have received a copy of the License along with this
* program. If not, you may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*
*******************************************************************************/

package build

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/holocm/libpackagebuild/filesystem"
)

//Architecture is an enum that describes the target architecture for this
//package (or "any" for packages without compiled binaries).
type Architecture uint

const (
	// ArchitectureAny = no compiled binaries (default value!)
	ArchitectureAny Architecture = iota
	// ArchitectureI386 = i386
	ArchitectureI386
	// ArchitectureX86_64 = x86_64
	ArchitectureX86_64
	// ArchitectureARMv5 = ARMv5
	ArchitectureARMv5
	// ArchitectureARMv6h = ARMv6h (hardfloat)
	ArchitectureARMv6h
	// ArchitectureARMv7h = ARMv7h (hardfloat)
	ArchitectureARMv7h
	// ArchitectureAArch64 = AArch64 (ARMv8 64-bit)
	ArchitectureAArch64
)

//PrereleaseType is an enum that describes whether the package is an alpha, beta or final release.
type PrereleaseType uint

const (
	//PrereleaseTypeNone indicates a final release that does not have "alpha" or "beta" added to its version number.
	PrereleaseTypeNone PrereleaseType = iota
	//PrereleaseTypeAlpha indicates an alpha release.
	PrereleaseTypeAlpha
	//PrereleaseTypeBeta indicates a beta release.
	PrereleaseTypeBeta
)

func (pt PrereleaseType) String() string {
	switch pt {
	case PrereleaseTypeNone:
		return "none"
	case PrereleaseTypeAlpha:
		return "alpha"
	case PrereleaseTypeBeta:
		return "beta"
	}
	panic(fmt.Sprintf("unexpected value for PrereleaseType: %d", uint(pt)))
}

//Package contains all information about a single package. This representation
//will be passed into the generator backends.
type Package struct {
	//Name is the package name.
	Name string
	//Version is the version for the package contents. This field shall contain
	//only numbered versions separated by dots (e.g. "1.0" or "0.9.184.5" or
	//"2020.10.05"). Other version strings cannot be guaranteed to generate valid
	//outputs for all package formats. To specify an alpha or beta version, set
	//the PrereleaseType and PrereleaseVersion fields. For instance,
	//"1.2.0-beta.1" shall be encoded as (.Version = "1.2.0",
	//.PrereleaseType = PrereleaseTypeBeta, .PrereleaseVersion = 1).
	Version string
	//PrereleaseType specifies whether this package is an alpha, beta or a final
	//release.
	PrereleaseType PrereleaseType
	//PrereleaseVersion is a counter of prereleases of a given type.
	//(.PrereleaseType = PrereleaseTypeAlpha, .PrereleaseVersion = 5) will append
	//"alpha.5" to the package's version (with a separator appropriate for a
	//given package format). The value in this field is ignored when
	//.PrereleaseType is PrereleaseTypeNone.
	PrereleaseVersion uint
	//Release is a counter that can be increased when the same version of one
	//package needs to be rebuilt. The default value shall be 1.
	Release uint
	//Epoch is a counter that can be increased when the version of a newer
	//package is smaller than the previous version, thus breaking normal
	//version comparison logic. This is usually only necessary when changing to
	//a different version numbering scheme. The default value is 0, which
	//usually results in the epoch not being shown in the combined version
	//string at all.
	Epoch uint
	//Description is the optional package description.
	Description string
	//Author contains the package's author's name and mail address in the form
	//"Firstname Lastname <email.address@server.tld>", if this information is
	//available.
	Author string
	//Architecture specifies the target architecture of this package.
	Architecture Architecture
	//ArchitectureInput contains the raw architecture string specified by the
	//user (used only for error messages).
	ArchitectureInput string
	//Requires contains a list of other packages that are required dependencies
	//for this package and thus must be installed together with this package.
	//This is called "Depends" by some package managers.
	Requires []PackageRelation
	//Provides contains a list of packages that this package provides features
	//of (or virtual packages whose capabilities it implements).
	Provides []PackageRelation
	//Conflicts contains a list of other packages that cannot be installed at
	//the same time as this package.
	Conflicts []PackageRelation
	//Replaces contains a list of obsolete packages that are replaced by this
	//package. Upon performing a system upgrade, the obsolete packages will be
	//automatically replaced by this package.
	Replaces []PackageRelation
	//Actions contains a list of actions that can be executed while the package
	//manager runs.
	Actions []PackageAction
	//FSRoot represents the root directory of the package's file system, and
	//contains all other files and directories recursively.
	FSRoot *filesystem.Directory
}

//PackageRelation declares a relation to another package. For the related
//package, any number of version constraints may be given. For example, the
//following snippet makes a Package require any version of package "foo", and
//at least version 2.1.2 (but less than version 3.0) of package "bar".
//
//    pkg.Requires := []PackageRelation{
//        PackageRelation { "foo", nil },
//        PackageRelation { "bar", []VersionConstraint{
//            VersionConstraint { ">=", "2.1.2" },
//            VersionConstraint { "<",  "3.0"   },
//        }
//    }
type PackageRelation struct {
	RelatedPackage string
	Constraints    []VersionConstraint
}

//VersionConstraint is used by the PackageRelation struct to specify version
//constraints for a related package.
type VersionConstraint struct {
	//Relation is one of "<", "<=", "=", ">=" or ">".
	Relation string
	//Version is the version on the right side of the Relation, e.g. "1.2.3-1"
	//or "2:20151024-1.1".  This field is not structured further in this level
	//since the acceptable version format may depend on the package generator
	//used.
	Version string
}

//PackageAction describes an action that can be executed by the package manager
//at various points during its execution.
type PackageAction struct {
	//Type determines when this action will be run. Acceptable values include
	//`SetupAction` and `CleanupAction`.
	Type uint
	//Content is a shell script that will be executed when the action is run.
	Content string
}

const (
	//SetupAction is an acceptable value for `PackageAction.Type`. Setup
	//actions run immediately after the package has been installed or upgraded
	//on a system.
	SetupAction = iota
	//CleanupAction is an acceptable value for `PackageAction.Type`. Cleanup
	//actions run immediately after the package has been removed from a system.
	CleanupAction
)

//PrepareBuild executes common preparation steps. This should be called by each
//generator's Build() implementation.
func (p *Package) PrepareBuild() {
	script := p.FSRoot.PostponeUnmaterializable("/")
	if script != "" {
		script = strings.TrimSuffix(script, "\n")
		p.PrependActions(PackageAction{Type: SetupAction, Content: script})
	}
}

//PrependActions prepends elements to p.Actions.
func (p *Package) PrependActions(actions ...PackageAction) {
	p.Actions = append(actions, p.Actions...)
}

//AppendActions appends elements to p.Actions.
func (p *Package) AppendActions(actions ...PackageAction) {
	p.Actions = append(p.Actions, actions...)
}

//Script returns the concatenation of the scripts for all actions of the given
//type.
func (p *Package) Script(actionType uint) string {
	var scripts []string
	for _, action := range p.Actions {
		if action.Type == actionType {
			scripts = append(scripts, action.Content)
		}
	}
	return strings.TrimSpace(strings.Join(scripts, "\n"))
}

//InsertFSNode inserts a filesystem.Node into the package's FSRoot at the given
//absolute path.
func (p *Package) InsertFSNode(absolutePath string, entry filesystem.Node) error {
	relPath, err := filepath.Rel("/", absolutePath)
	if err != nil {
		return err
	}
	err = p.FSRoot.Insert(entry, strings.Split(relPath, "/"), "/")
	if err != nil {
		return fmt.Errorf("failed to insert \"%s\" into the package file system: %s", absolutePath, err.Error())
	}
	return nil
}

//WalkFSWithAbsolutePaths wraps the FSRoot.Wrap function, yielding absolute
//paths (with a leading slash) to the callback.
func (p *Package) WalkFSWithAbsolutePaths(callback func(absolutePath string, node filesystem.Node) error) error {
	return p.FSRoot.Walk("/", callback)
}

//WalkFSWithRelativePaths wraps the FSRoot.Wrap function, yielding paths
//relative to the FSRoot (without leading slash) to the callback. The FSRoot
//itself will be visited with `relativePath = ""`.
func (p *Package) WalkFSWithRelativePaths(callback func(relativePath string, node filesystem.Node) error) error {
	return p.FSRoot.Walk("", callback)
}
