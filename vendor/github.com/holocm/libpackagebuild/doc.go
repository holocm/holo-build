/*******************************************************************************
*
* Copyright 2018 Stefan Majewsky <majewsky@gmx.net>
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

//Package build generates packages that can be installed by a system package
//manager. Supported formats include dpkg (used by Debian and Ubuntu), pacman
//(used by Arch Linux), and RPM (used by Suse, Redhat, Fedora, Mageia). RPM
//support is experimental.
//
//This package contains the common API that is shared by all generators. The
//generator implementations are in this package's subdirectories.
package build
