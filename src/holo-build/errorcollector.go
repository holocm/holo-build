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
)

//ErrorCollector is a wrapper around []error that simplifies code where
//multiple errors can happen and need to be aggregated for collective display
//in an error display.
type ErrorCollector struct {
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
func (c *ErrorCollector) Add(err error) {
	if err != nil {
		c.Errors = append(c.Errors, err)
	}
}

//Addf adds an error to this collector by passing the arguments into
//fmt.Errorf(). If only one argument is given, it is used as error string
//verbatim.
func (c *ErrorCollector) Addf(format string, args ...interface{}) {
	if len(args) > 0 {
		c.Errors = append(c.Errors, fmt.Errorf(format, args...))
	} else {
		c.Errors = append(c.Errors, errors.New(format))
	}
}
