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

//Package contains all information about a single package. This representation
//will be passed into the generator backends.
type Package struct {
	//Name is the package name.
	Name string
	//Version is the version for the package contents. While many package
	//formats are more or less liberal about the format of version strings,
	//holo-build requires versions to adhere to the Semantic Version format
	//(semver.org). Build metadata is not supported, while as an extension, we
	//support an arbitrary number of segments in the initial
	//"MAJOR.MINOR.PATCH" part.
	Version string
	//Release is a counter that can be increased when the same version of one
	//hologram needs to be rebuilt. The default value is 1.
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
	//SetupScript contains a shell script that is executed when the package is
	//installed or upgraded.
	SetupScript string
	//CleanupScript contains a shell script that is executed when the package is
	//installed or upgraded.
	CleanupScript string
	//Entries lists the files and directories contained within this package.
	FSEntries []FSEntry
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

const (
	//FSEntryTypeRegular is the FSEntry.Type for regular files.
	FSEntryTypeRegular = iota
	//FSEntryTypeSymlink is the FSEntry.Type for symlinks.
	FSEntryTypeSymlink
	//FSEntryTypeDirectory is the FSEntry.Type for directories.
	FSEntryTypeDirectory
)

//FSEntry represents a file, directory or symlink in the package.
type FSEntry struct {
	Type     int
	Path     string
	Content  string          //except directories (has content for regular files, target for symlinks)
	Metadata *FSNodeMetadata //except symlinks
}
