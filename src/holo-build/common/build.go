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
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

//Build builds the package using the given Generator.
func (pkg *Package) Build(generator Generator, printToStdout bool) error {
	//do magical Holo integration tasks
	pkg.doMagicalHoloIntegration()
	//move unmaterializable filesystem metadata into the setupScript
	pkg.postponeUnmaterializableFSMetadata()

	//build package
	pkgBytes, err := generator.Build(pkg)
	if err != nil {
		return err
	}

	//write package, either to stdout or to the working directory
	if printToStdout {
		_, err := os.Stdout.Write(pkgBytes)
		if err != nil {
			return err
		}
	} else {
		pkgFile := generator.RecommendedFileName(pkg)
		if strings.ContainsAny(pkgFile, "/ \t\r\n") {
			return fmt.Errorf("Unexpected filename generated: \"%s\"", pkgFile)
		}
		err := ioutil.WriteFile(pkgFile, pkgBytes, 0666)
		if err != nil {
			return err
		}
	}

	return nil
}

func (pkg *Package) doMagicalHoloIntegration() {
	//does this package need to provision stuff with Holo plugins?
	plugins := make(map[string]bool)
	pkg.WalkFSWithAbsolutePaths(func(path string, node FSNode) error {
		if strings.HasPrefix(path, "/usr/share/holo/") {
			//extract the plugin ID from the path
			pathParts := strings.Split(path, "/")
			if len(pathParts) > 4 {
				plugins[pathParts[4]] = true
			}
		}
		return nil
	})
	if len(plugins) == 0 {
		return
	}

	//it does -> add all these Holo plugins to the list of requirements...
	for pluginID := range plugins {
		depName := "holo-" + pluginID
		hasDep := false
		for _, rel := range pkg.Requires {
			if rel.RelatedPackage == depName {
				hasDep = true
				break
			}
		}
		if !hasDep {
			pkg.Requires = append(pkg.Requires, PackageRelation{RelatedPackage: depName})
		}
	}

	//...and run `holo apply` during setup/cleanup
	pkg.PrependActions(
		PackageAction{Type: SetupAction, Content: "holo apply"},
		PackageAction{Type: CleanupAction, Content: "holo apply"},
	)
}

func (pkg *Package) postponeUnmaterializableFSMetadata() {
	//When an FSEntry identifies its Owner/Group by name, we cannot materialize
	//that at build time since we don't know the UID/GID to write into the
	//archive. Therefore, remove the Owner/Group from the FS entry and add a
	//chown(1)/chgrp(1) call to the setupScript to apply ownership at install
	//time.
	pkg.WalkFSWithAbsolutePaths(func(path string, node FSNode) error {
		var script string
		switch n := node.(type) {
		case *FSDirectory:
			script = n.Metadata.PostponeUnmaterializable(path)
		case *FSRegularFile:
			script = n.Metadata.PostponeUnmaterializable(path)
		default:
			//don't do anything for FSNodes that don't have metadata
		}
		//ensure that ownership is correct before running the actual setup script
		if script != "" {
			pkg.PrependActions(PackageAction{Type: SetupAction, Content: script})
		}
		return nil
	})

}
