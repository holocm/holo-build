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

//ValidateWith is a helper function provided for generators.
//
//It validates the package name, version and related packages with
//the given set of regexes, and returns a non-empty list of errors if
//validation fails.
func (pkg *Package) ValidateWith(r RegexSet, archMap map[Architecture]string) []error {
	ec := errorCollector{}

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

	if pkg.Release == 0 {
		ec.Addf("Package release may not be zero (numbering of releases starts at 1)")
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

func validatePackageRelations(r *compiledRegexSet, relType string, rels []PackageRelation, ec *errorCollector) {
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
