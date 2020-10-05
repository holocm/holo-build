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

package pacman

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	build "github.com/holocm/libpackagebuild"
)

//Renders package relations into .PKGINFO.
func compilePackageRelations(relType string, rels []build.PackageRelation) string {
	if len(rels) == 0 {
		return ""
	}

	lines := make([]string, 0, len(rels)) //only a lower boundary on the final size, but usually a good guess
	for _, rel := range rels {
		if len(rel.Constraints) == 0 {
			//simple relation without constraint, e.g. "depend = linux"
			lines = append(lines, fmt.Sprintf("%s = %s", relType, rel.RelatedPackage))
		} else {
			for _, c := range rel.Constraints {
				//relation with constraint, e.g. "conflict = holo<0.5"
				lines = append(lines, fmt.Sprintf("%s = %s%s%s", relType, rel.RelatedPackage, c.Relation, c.Version))
			}
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

//Like compilePackageRelations, but resolve special syntax for requirements
//(references to groups, exclusion of packages and groups).
func compilePackageRequirements(relType string, rels []build.PackageRelation) (string, error) {
	//acceptRel marks which packages will be included in the result
	//(e.g. "not:foo" sets acceptPkg["foo"] = false)
	acceptPkg := make(map[string]bool, len(rels))

	//read all input relations, and filter plain package relations (those that
	//are not groups or negations)
	actualRels := make([]build.PackageRelation, len(rels))
	for _, rel := range rels {
		name := rel.RelatedPackage
		isNegated := strings.HasPrefix(name, "except:")
		name = strings.TrimPrefix(name, "except:")
		isGroup := strings.HasPrefix(name, "group:")
		name = strings.TrimPrefix(name, "group:")

		if isGroup {
			//resolve groups
			pkgs, err := resolvePackageGroup(name)
			if err != nil {
				return "", err
			}

			//accept packages in this group if not negated
			for _, pkgName := range pkgs {
				acceptPkg[pkgName] = !isNegated
			}
		} else {
			acceptPkg[name] = !isNegated
			if !isNegated {
				actualRels = append(actualRels, rel)
			}
		}
	}

	//prune all not-accepted packages from actualRels
	prunedRels := make([]build.PackageRelation, 0, len(actualRels))
	for _, rel := range actualRels {
		if acceptPkg[rel.RelatedPackage] {
			prunedRels = append(prunedRels, rel)
		}
		delete(acceptPkg, rel.RelatedPackage)
	}

	//add all missing relations (these are all pkgName with acceptPkg[pkgName]
	//= true since we removed existing rels from acceptPkg in the last step)
	additionalRels := make([]build.PackageRelation, 0, len(acceptPkg))
	for pkgName, accepted := range acceptPkg {
		if accepted {
			additionalRels = append(additionalRels, build.PackageRelation{RelatedPackage: pkgName})
		}
	}
	sort.Sort(byRelatedPackage(additionalRels))
	prunedRels = append(prunedRels, additionalRels...)

	return compilePackageRelations(relType, prunedRels), nil
}

func resolvePackageGroup(groupName string) ([]string, error) {
	//mock implementation (for unit tests): read package names from group name
	//(e.g. "group:foo-bar-baz" contains packages "foo", "bar", "baz")
	if value := os.Getenv("HOLO_MOCK"); value == "1" {
		return strings.Split(groupName, "-"), nil
	}

	//actual implementation: call pacman to resolve package groups
	cmd := exec.Command("pacman", "-Sqg", groupName)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Error resolving package group %q: %s", groupName, err.Error())
	}

	return strings.Fields(string(out)), nil
}

//implement sort.Sort interface for package relations
type byRelatedPackage []build.PackageRelation

func (b byRelatedPackage) Len() int           { return len(b) }
func (b byRelatedPackage) Less(i, j int) bool { return b[i].RelatedPackage < b[j].RelatedPackage }
func (b byRelatedPackage) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
