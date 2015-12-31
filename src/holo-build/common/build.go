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

//#include <unistd.h>
import "C"
import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

//Build builds the package using the given Generator.
func (pkg *Package) Build(generator Generator, printToStdout bool, buildReproducibly bool) error {
	//do magical Holo integration tasks
	pkg.doMagicalHoloIntegration()
	//move unmaterializable filesystem metadata into the setupScript
	pkg.postponeUnmaterializableFSMetadata()

	//try to build package in memory
	pkgBytes, err := generator.BuildInMemory(pkg, buildReproducibly)
	if err != nil && err != UnsupportedBuildMethodError {
		return err
	}

	//if this is not supported, build package from a materialized filesystem tree
	if err == UnsupportedBuildMethodError {
		//choose root directory in such a way that the user can easily find and
		//inspect it in the case that an error occurs
		rootPath := fmt.Sprintf("./holo-build-%s-%s", pkg.Name, pkg.Version)

		//if the root directory exists from a previous run, remove it recursively
		err = os.RemoveAll(rootPath)
		if err != nil {
			return err
		}

		//create the root directory
		err = os.MkdirAll(rootPath, 0755)
		if err != nil {
			return err
		}

		//materialize FS entries in the root directory
		err = pkg.materializeFSEntries(rootPath, buildReproducibly)
		if err != nil {
			return err
		}

		//build package
		pkgBytes, err = generator.Build(pkg, rootPath, buildReproducibly)
		if err != nil {
			return err
		}

		//if requested, cleanup the target directory
		err = os.RemoveAll(rootPath)
		if err != nil {
			return err
		}
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
	for _, entry := range pkg.FSEntries {
		if strings.HasPrefix(entry.Path, "/usr/share/holo/") {
			//extract the plugin ID from the path
			pathParts := strings.Split(entry.Path, "/")
			if len(pathParts) > 4 {
				plugins[pathParts[4]] = true
			}
			break
		}
	}
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
	pkg.SetupScript = "holo apply\n" + pkg.SetupScript
	pkg.CleanupScript = "holo apply\n" + pkg.CleanupScript
}

func (pkg *Package) postponeUnmaterializableFSMetadata() {
	//When an FSEntry identifies its Owner/Group by name, we cannot materialize
	//that at build time since we don't know the UID/GID to write into the archive.
	//Therefore, remove the Owner/Group from the FS entry and add a chown(1) call
	//to the setupScript to apply ownership at install time.
	var additionalSetupScript string

	for _, entry := range pkg.FSEntries {
		var ownerStr, groupStr string
		if entry.Owner != nil && entry.Owner.Str != "" {
			ownerStr = entry.Owner.Str
			entry.Owner = nil
		}
		if entry.Group != nil && entry.Group.Str != "" {
			groupStr = entry.Group.Str
			entry.Group = nil
		}

		if ownerStr != "" {
			if groupStr != "" {
				additionalSetupScript += fmt.Sprintf("chown %s:%s %s\n", ownerStr, groupStr, entry.Path)
			} else {
				additionalSetupScript += fmt.Sprintf("chown %s %s\n", ownerStr, entry.Path)
			}
		} else {
			if groupStr != "" {
				additionalSetupScript += fmt.Sprintf("chgrp %s %s\n", groupStr, entry.Path)
			}
		}
	}

	//ensure that ownership is correct before running the actual setup script
	pkg.SetupScript = additionalSetupScript + pkg.SetupScript
}

func (pkg *Package) materializeFSEntries(rootPath string, buildReproducibly bool) error {
	for _, entry := range pkg.FSEntries {
		//find the path within the rootPath for this entry
		path, _ := filepath.Rel("/", entry.Path)
		path = filepath.Join(rootPath, path)

		//mkdir -p $(dirname $entry_path)
		err := os.MkdirAll(filepath.Dir(path), 0755)
		if err != nil {
			return err
		}

		//write entry
		switch entry.Type {
		case FSEntryTypeRegular:
			err = ioutil.WriteFile(path, []byte(entry.Content), entry.Mode)
		case FSEntryTypeDirectory:
			err = os.Mkdir(path, entry.Mode)
		case FSEntryTypeSymlink:
			err = os.Symlink(entry.Content, path)
		}
		if err != nil {
			return err
		}

		//ownership only applies to files and directories
		if entry.Type == FSEntryTypeSymlink {
			continue
		}

		//apply ownership (numeric ownership can be written into the package directly; ownership by name will be applied in the setupScript)
		var uid C.__uid_t
		var gid C.__gid_t
		if entry.Owner != nil {
			uid = C.__uid_t(entry.Owner.Int)
		}
		if entry.Group != nil {
			gid = C.__gid_t(entry.Group.Int)
		}
		if uid != 0 || gid != 0 {
			//cannot use os.Chown(); os.Chown calls into syscall.Chown and thus
			//does a direct syscall which cannot be intercepted by fakeroot; I
			//need to call chown(2) via cgo
			result, err := C.chown(C.CString(path), uid, gid)
			if result != 0 && err != nil {
				return err
			}
		}
	}

	//if a reproducible build has been requested, set all timestamps for all FS
	//entries to 0 (i.e. 1970-01-01T00:00:00Z)
	if buildReproducibly {
		err := filepath.Walk(rootPath, func(path string, fileInfo os.FileInfo, err error) error {
			//skip over unaccessible stuff
			if err != nil {
				return err
			}
			return ResetTimestamp(path)
		})
		if err != nil {
			return err
		}
	}

	return nil
}
