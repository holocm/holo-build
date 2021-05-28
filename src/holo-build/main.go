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
	"fmt"
	"io"
	"os"
	"path/filepath"

	build "github.com/holocm/libpackagebuild"
	"github.com/holocm/libpackagebuild/debian"
	"github.com/holocm/libpackagebuild/pacman"
	"github.com/holocm/libpackagebuild/rpm"
	"github.com/ogier/pflag"
)

type options struct {
	generatorFactory build.GeneratorFactory
	inputFileName    string //or "" for stdin
	outputFileName   string //or "" for automatic or "-" for stdout
	filenameOnly     bool
	withForce        bool
}

var opts = parseArgs()

func main() {
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
	pkg, errs := ParsePackageDefinition(input, baseDirectory)

	//initialize generator
	generator := opts.generatorFactory(pkg)

	//try to validate package
	var validateErrs []error
	if pkg != nil {
		validateErrs = generator.Validate()
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
	pkgFile := generator.RecommendedFileName()
	if opts.filenameOnly {
		fmt.Println(pkgFile)
		return
	}
	if opts.outputFileName != "" {
		if opts.outputFileName == "-" {
			pkgFile = "-"
		} else {
			//use opts.outputFileName directly if a file, or choose it inside there if a directory
			fi, err := os.Stat(opts.outputFileName)
			if err == nil && fi.Mode().IsDir() {
				pkgFile = filepath.Join(opts.outputFileName, pkgFile)
			} else {
				pkgFile = opts.outputFileName
			}
		}
	}

	//build package
	DoMagicalHoloIntegration(pkg)
	pkgBytes, err := generator.Build()
	if err != nil {
		showErrorMsg("cannot build %s: %s", pkgFile, err.Error())
		os.Exit(2)
	}

	wasWritten, err := WriteOutput(pkgBytes, pkgFile, opts.withForce)
	if err != nil {
		showErrorMsg("cannot write %s: %s", pkgFile, err.Error())
		os.Exit(2)
	}

	if !wasWritten {
		os.Exit(0)
	}

	//TODO: more stuff coming
}

func parseArgs() options {
	//TODO: remove everything that is flagged as deprecated
	withForce := pflag.BoolP("force", "f", false, "Overwrite existing output file")
	formatString := pflag.String("format", "", "Output file format (\"debian\", \"pacman\" or \"rpm\")")
	formatDebian := pflag.Bool("debian", false, "Generate Debian package (deprecated, use \"--format=debian\" instead)")
	formatPacman := pflag.Bool("pacman", false, "Generate Pacman package (deprecated, use \"--format=pacman\" instead)")
	formatRPM := pflag.Bool("rpm", false, "Generate RPM package (deprecated, use \"--format=rpm\" instead)")
	outputFileName := pflag.StringP("output", "o", "", "Output file name (or \"-\" for standard output)")
	outputStdout := pflag.Bool("stdout", false, "Write package to standard output (deprecated, use \"-o -\" instead)")
	noOutputStdout := pflag.Bool("no-stdout", false, "Revert --stdout (deprecated, use \"-o\" instead)")
	reproducible := pflag.Bool("reproducible", false, "Deprecated, no effect")
	noReproducible := pflag.Bool("no-reproducible", false, "Deprecated, no effect")
	suggestFileName := pflag.Bool("suggest-filename", false, "Only print the suggested filename for this package")
	showVersion := pflag.BoolP("version", "V", false, "Show program version")

	pflag.Parse()

	if *noOutputStdout {
		showErrorMsg("--no-stdout is deprecated - use \"--output=''\" instead")
		*outputStdout = false
	}
	if *noReproducible {
		showErrorMsg("--no-reproducible is deprecated and can safely be removed")
		*reproducible = false
	}

	if *showVersion {
		fmt.Println(VersionString())
		os.Exit(0)
	}

	if *reproducible {
		showErrorMsg("--reproducible is deprecated and can safely be removed")
	}

	var hasArgsError bool
	if *outputStdout {
		showErrorMsg("--stdout is deprecated - use \"--output=-\" instead")
		if *outputFileName != "" {
			showErrorMsg("--output and --stdout may not be used at the same time")
			hasArgsError = true
		}
		*outputFileName = "-"
	}

	switch {
	case *formatDebian:
		showErrorMsg("--debian is deprecated - use \"--format=debian\" instead")
		if *formatString != "" {
			showErrorMsg("--debian and --format may not be used at the same time")
			hasArgsError = true
		}
		*formatString = "debian"
	case *formatPacman:
		showErrorMsg("--pacman is deprecated - use \"--format=pacman\" instead")
		if *formatString != "" {
			showErrorMsg("--pacman and --format may not be used at the same time")
			hasArgsError = true
		}
		*formatString = "pacman"
	case *formatRPM:
		showErrorMsg("--rpm is deprecated - use \"--format=rpm\" instead")
		if *formatString != "" {
			showErrorMsg("--rpm and --format may not be used at the same time")
			hasArgsError = true
		}
		*formatString = "rpm"
	}

	var generatorFactory build.GeneratorFactory
	switch *formatString {
	case "debian":
		generatorFactory = debian.GeneratorFactory
	case "pacman":
		generatorFactory = pacman.GeneratorFactory
	case "rpm":
		generatorFactory = rpm.GeneratorFactory
	case "":
		showErrorMsg("No package format specified. Use the wrapper script at /usr/bin/holo-build to autoselect a package format.")
		hasArgsError = true
	default:
		showErrorMsg("Invalid package format: '%s'", *formatString)
		hasArgsError = true
	}

	var inputFileName string
	switch len(pflag.Args()) {
	case 0:
		inputFileName = "" //use stdin
	case 1:
		inputFileName = pflag.Arg(0)
	default:
		showErrorMsg("Multiple input files specified.")
		hasArgsError = true
	}

	if hasArgsError {
		os.Exit(1)
	}
	return options{
		generatorFactory: generatorFactory,
		inputFileName:    inputFileName,
		outputFileName:   *outputFileName,
		filenameOnly:     *suggestFileName,
		withForce:        *withForce,
	}
}

func showError(err error) {
	showErrorMsg(err.Error())
}

func showErrorMsg(msg string, args ...interface{}) {
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	fmt.Fprintf(os.Stderr, "\x1b[31m\x1b[1m!!\x1b[0m %s\n", msg)
}
