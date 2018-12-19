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
	"sort"
	"strings"

	build "github.com/holocm/libpackagebuild"
	"github.com/holocm/libpackagebuild/filesystem"
)

//DoMagicalHoloIntegration makes the implicit "holo apply" setup script and the
//implicit "holo-$PLUGIN" dependencies explicit.
func DoMagicalHoloIntegration(pkg *build.Package) {
	//does this package need to provision stuff with Holo plugins?
	plugins := make(map[string]bool)
	pkg.WalkFSWithAbsolutePaths(func(path string, node filesystem.Node) error {
		if strings.HasPrefix(path, "/usr/share/holo/") {
			//extract the plugin ID from the path
			pathParts := strings.Split(path, "/")
			if len(pathParts) > 5 {
				//NOTE: not > 4, but > 5, since we only want entries that are
				//strictly below, rather than at, "/usr/share/holo/$plugin_id"
				plugins[pathParts[4]] = true
			}
		}
		return nil
	})
	if len(plugins) == 0 {
		return
	}

	//it does -> sort list of plugins for reproducibility...
	pluginIDs := make([]string, 0, len(plugins))
	for pluginID := range plugins {
		pluginIDs = append(pluginIDs, pluginID)
	}
	sort.Strings(pluginIDs)

	//add all these Holo plugins to the list of requirements...
	for _, pluginID := range pluginIDs {
		depName := "holo-" + pluginID
		hasDep := false
		for _, rel := range pkg.Requires {
			if rel.RelatedPackage == depName {
				hasDep = true
				break
			}
		}
		if !hasDep {
			pkg.Requires = append(pkg.Requires, build.PackageRelation{RelatedPackage: depName})
		}
	}

	//...and run `holo apply` during setup/cleanup
	pkg.PrependActions(
		build.PackageAction{Type: build.SetupAction, Content: "holo apply"},
		build.PackageAction{Type: build.CleanupAction, Content: "holo apply"},
	)
}
