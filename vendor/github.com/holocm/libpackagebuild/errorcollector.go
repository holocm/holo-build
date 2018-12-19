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

import (
	"errors"
	"fmt"
)

//errorCollector is a wrapper around []error that simplifies code where
//multiple errors can happen and need to be aggregated for collective display
//in an error display.
type errorCollector struct {
	Errors []error
}

//Add adds an error to this collector. If nil is given, nothing happens, so you
//can safely write
//
//    ec.Add(OperationThatMightFail())
//
//instead of
//
//    err := OperationThatMightFail()
//    if err != nil {
//        ec.Add(err)
//    }
//
func (c *errorCollector) Add(err error) {
	if err != nil {
		c.Errors = append(c.Errors, err)
	}
}

//Addf adds an error to this collector by passing the arguments into
//fmt.Errorf(). If only one argument is given, it is used as error string
//verbatim.
func (c *errorCollector) Addf(format string, args ...interface{}) {
	if len(args) > 0 {
		c.Errors = append(c.Errors, fmt.Errorf(format, args...))
	} else {
		c.Errors = append(c.Errors, errors.New(format))
	}
}
