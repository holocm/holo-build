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

//Generator is a generic interface for the package generator implementations.
//One Generator exists for every target package format (e.g. pacman, dpkg, RPM)
//supported by libpackagebuild.
//
//A generator should take the package to be built as a struct field like this:
//
//	type DebianGenerator struct {
//		Package *Package
//		//... other fields ...
//	}
//
//Each generator implementation should also provide a function of type
//GeneratorFactory, see below.
type Generator interface {
	//Validate performs additional validations on the package that are specific
	//to the concrete generator. For example, if the package format imposes
	//restrictions on the format of certain fields (names, versions, etc.), they
	//should be checked here.
	//
	//If the package is valid, an empty slice is to be returned.
	Validate() []error
	//Build produces the final package (usually a compressed tar file) in the
	//return argument. The package must be built reproducibly; such that every
	//run (even across systems) produces an identical result. For example, no
	//timestamps or generator version information may be included.
	//
	//Build should call pkg.PrepareBuild() to execute some common preparation steps.
	Build() ([]byte, error)
	//Generate the recommended file name for this package. Distributions usually
	//have guidelines for this sort of thing. The string returned must be a plain
	//file name without any slashes.
	RecommendedFileName() string
}

//GeneratorFactory is a type of function that creates generators.
type GeneratorFactory func(*Package) Generator
