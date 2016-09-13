/*******************************************************************************
*
* Copyright 2016 Stefan Majewsky <majewsky@gmx.net>
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

import "regexp"

//RegexSet is a collection of regular expressions for validating a package.
//A RegexSet is typically constructed by a common.Generator for calling
//common.Package.ValidateWith() inside its Validate() method.
type RegexSet struct {
	PackageName    string
	PackageVersion string
	RelatedName    string
	RelatedVersion string
	FormatName     string //used for error messages only
}

type compiledRegexSet struct {
	PackageName    *regexp.Regexp
	PackageVersion *regexp.Regexp
	RelatedName    *regexp.Regexp
	RelatedVersion *regexp.Regexp
	FormatName     string
}

//ValidateWith validates the package name, version and related packages with
//the given set of regexes, and returns a non-empty list of errors if
//validation fails.
func (pkg *Package) ValidateWith(r RegexSet, archMap map[Architecture]string) []error {
	ec := ErrorCollector{}

	cr := &compiledRegexSet{
		PackageName:    regexp.MustCompile("^" + r.PackageName + "$"),
		PackageVersion: regexp.MustCompile("^" + r.PackageVersion + "$"),
		RelatedName:    regexp.MustCompile("^" + r.RelatedName + "$"),
		RelatedVersion: regexp.MustCompile("^" + r.RelatedVersion + "$"),
		FormatName:     r.FormatName,
	}

	//if name or version is empty, it was already rejected by the parser and we
	//don't need to complain about it again
	if pkg.Name != "" && !cr.PackageName.MatchString(pkg.Name) {
		ec.Addf("Package name \"%s\" is not acceptable for %s packages", pkg.Name, cr.FormatName)
	}
	if pkg.Version != "" && !cr.PackageVersion.MatchString(pkg.Version) {
		//this check is only some Defense in Depth; a stricter version format
		//is already enforced by the generator-independent validation
		ec.Addf("Package version \"%s\" is not acceptable for %s packages", pkg.Version, cr.FormatName)
	}

	//check if architecture is supported by this generator
	if _, ok := archMap[pkg.Architecture]; !ok {
		ec.Addf("Architecture \"%s\" is not acceptable for %s packages", pkg.ArchitectureInput, cr.FormatName)
	}

	validatePackageRelations(cr, "requires", pkg.Requires, &ec)
	validatePackageRelations(cr, "provides", pkg.Provides, &ec)
	validatePackageRelations(cr, "conflicts", pkg.Conflicts, &ec)
	validatePackageRelations(cr, "replaces", pkg.Replaces, &ec)

	return ec.Errors
}

func validatePackageRelations(r *compiledRegexSet, relType string, rels []PackageRelation, ec *ErrorCollector) {
	for _, rel := range rels {
		if !r.RelatedName.MatchString(rel.RelatedPackage) {
			ec.Addf("Package name \"%s\" is not acceptable for %s packages (found in %s)", rel.RelatedPackage, r.FormatName, relType)
		}
		for _, constraint := range rel.Constraints {
			if !r.RelatedVersion.MatchString(constraint.Version) {
				ec.Addf("Version in \"%s %s %s\" is not acceptable for %s packages (found in %s)",
					rel.RelatedPackage, constraint.Relation, constraint.Version, r.FormatName, relType,
				)
			}
		}
	}
}
