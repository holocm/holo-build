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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/holocm/holo-build/src/holo-build/common"
	"github.com/holocm/holo-build/src/holo-build/debian"
	"github.com/holocm/holo-build/src/holo-build/pacman"
	"github.com/holocm/holo-build/src/holo-build/rpm"
)

type options struct {
	generator     common.Generator
	inputFileName string
	printToStdout bool
	filenameOnly  bool
	withForce     bool
}

func main() {
	opts, earlyExit := parseArgs()
	if earlyExit {
		return
	}
	generator := opts.generator

	//read package definition from stdin
	input := io.Reader(os.Stdin)
	baseDirectory := "."
	if opts.inputFileName != "" {
		var err error
		input, err = os.Open(opts.inputFileName)
		if err != nil {
			showError(err)
			os.Exit(1)
		}
		baseDirectory = filepath.Dir(opts.inputFileName)
	}
	pkg, errs := common.ParsePackageDefinition(input, baseDirectory)

	//try to validate package
	var validateErrs []error
	if pkg != nil {
		validateErrs = generator.Validate(pkg)
	}
	errs = append(errs, validateErrs...)

	//did that go wrong?
	if len(errs) > 0 {
		for _, err := range errs {
			showError(err)
		}
		os.Exit(1)
	}

	//print filename instead of building package, if requested
	pkgFile := generator.RecommendedFileName(pkg)
	if opts.filenameOnly {
		fmt.Println(pkgFile)
		return
	}

	//build package
	pkgBytes, err := pkg.Build(generator)
	if err != nil {
		showError(fmt.Errorf("cannot build %s: %s", pkgFile, err.Error()))
		os.Exit(2)
	}

	wasWritten, err := pkg.WriteOutput(generator, pkgBytes, opts.printToStdout, opts.withForce)
	if err != nil {
		showError(fmt.Errorf("cannot write %s: %s", pkgFile, err.Error()))
		os.Exit(2)
	}

	if wasWritten {
		os.Exit(0)
	}

	//TODO: more stuff coming
}

func parseArgs() (result options, exit bool) {
	//default settings
	var opts options

	//parse arguments
	args := os.Args[1:]
	hasArgsError := false
	for _, arg := range args {
		switch arg {
		case "--help":
			printHelp()
			return opts, true
		case "--version":
			fmt.Println(common.VersionString())
			return opts, true
		case "--force":
			opts.withForce = true
		case "--no-force":
			opts.withForce = false
		case "--stdout":
			opts.printToStdout = true
		case "--no-stdout":
			opts.printToStdout = false
		case "--suggest-filename":
			opts.filenameOnly = true
		case "--reproducible", "--no-reproducible":
			//no effect anymore - TODO: remove in 2.0
		case "--pacman":
			if opts.generator != nil {
				showError(errors.New("Multiple package formats specified."))
				hasArgsError = true
			}
			opts.generator = &pacman.Generator{}
		case "--debian":
			if opts.generator != nil {
				showError(errors.New("Multiple package formats specified."))
				hasArgsError = true
			}
			opts.generator = &debian.Generator{}
		case "--rpm":
			if opts.generator != nil {
				showError(errors.New("Multiple package formats specified."))
				hasArgsError = true
			}
			opts.generator = &rpm.Generator{}
		//NOTE: When adding new package formats here, don't forget to update
		//holo-build.sh accordingly!
		default:
			//the first positional argument is used as input file name
			if opts.inputFileName == "" && !strings.HasPrefix(arg, "-") {
				opts.inputFileName = arg
			} else {
				showError(fmt.Errorf("Unrecognized argument: '%s'", arg))
				hasArgsError = true
			}
		}
	}
	if hasArgsError {
		printHelp()
		os.Exit(1)
	}
	if opts.generator == nil {
		showError(errors.New("No package format specified. Use the wrapper script at /usr/bin/holo-build to autoselect a package format."))
		os.Exit(1)
	}

	return opts, false
}

func printHelp() {
	program := os.Args[0]
	fmt.Printf("Usage: %s <options> <definitionfile>\n\nOptions:\n", program)
	fmt.Println("  --suggest-filename\tDo not compile package; just print the suggested filename")
	fmt.Println("  --stdout\t\tPrint resulting package on stdout")
	fmt.Println("  --no-stdout\t\tWrite resulting package to the working directory (default)")
	fmt.Println("  --force\t\tOverwrite target file if it exists")
	fmt.Println("  --no-force\t\tFail if target file exists (default)\n")
	fmt.Println("  --debian\t\tBuild a Debian package")
	fmt.Println("  --pacman\t\tBuild a pacman package")
	fmt.Println("  --rpm\t\t\tBuild an RPM package\n")
	fmt.Println("If no options are given, the package format for the current distribution is selected.\n")
	fmt.Println("If the definition file is not given as an argument, it will be read from standard input.\n")
}

func showError(err error) {
	fmt.Fprintf(os.Stderr, "\x1b[31m\x1b[1m!!\x1b[0m %s\n", err.Error())
}
