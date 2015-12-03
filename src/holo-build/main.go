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
	"os"
	"syscall"

	"./common"
	"./debian"
	"./pacman"
)

func main() {
	//holo-build needs to run in a fakeroot(1)
	if os.Getenv("FAKEROOTKEY") != "" {
		//already running in fakeroot, commence normal operation
		actualMain()
		return
	}

	//not running in fakeroot -> exec self with fakeroot
	args := append([]string{"/usr/bin/fakeroot"}, os.Args...)
	syscall.Exec(args[0], args, os.Environ())
}

type options struct {
	generator     common.Generator
	printToStdout bool
	reproducible  bool
	filenameOnly  bool
}

func actualMain() {
	opts, earlyExit := parseArgs()
	if earlyExit {
		return
	}
	generator := opts.generator

	//read package definition from stdin
	pkg, errs := common.ParsePackageDefinition(os.Stdin)

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
	if opts.filenameOnly {
		fmt.Println(generator.RecommendedFileName(pkg))
		return
	}

	//build package
	err := pkg.Build(generator, opts.printToStdout, opts.reproducible)
	if err != nil {
		showError(fmt.Errorf("cannot build %s: %s",
			generator.RecommendedFileName(pkg), err.Error(),
		))
		os.Exit(2)
	}
}

func parseArgs() (result options, exit bool) {
	//default settings
	opts := options{
		generator:     nil,
		printToStdout: false,
		reproducible:  false,
	}

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
		case "--stdout":
			opts.printToStdout = true
		case "--no-stdout":
			opts.printToStdout = false
		case "--suggest-filename":
			opts.filenameOnly = true
		case "--reproducible":
			opts.reproducible = true
		case "--no-reproducible":
			opts.reproducible = false
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
		//NOTE: When adding new package formats here, don't forget to update
		//holo-build.sh accordingly!
		default:
			showError(fmt.Errorf("Unrecognized argument: '%s'", arg))
			hasArgsError = true
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
	fmt.Printf("Usage: %s <options> < definitionfile > packagefile\n\nOptions:\n", program)
	fmt.Println("  --stdout\t\tPrint resulting package on stdout")
	fmt.Println("  --no-stdout\t\tWrite resulting package to the working directory (default)")
	fmt.Println("  --reproducible\tBuild a reproducible package with bogus timestamps etc.")
	fmt.Println("  --no-reproducible\tBuild a non-reproducible package with actual timestamps etc. (default)")
	fmt.Println("  --debian\t\tBuild a debian package\n")
	fmt.Println("  --pacman\t\tBuild a pacman package\n")
	fmt.Println("If no options are given, the package format for the current distribution is selected.\n")
}

func showError(err error) {
	fmt.Fprintf(os.Stderr, "\x1b[31m\x1b[1m!!\x1b[0m %s\n", err.Error())
}
