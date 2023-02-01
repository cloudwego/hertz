/*
 * Copyright 2022 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package binding

import (
	"fmt"
)

func ExampleDefaultValidator_ValidateStruct() {
	type User struct {
		FirstName      string `validate:"required"`
		LastName       string `validate:"required"`
		Age            uint8  `validate:"gte=0,lte=130"`
		Email          string `validate:"required,email"`
		FavouriteColor string `validate:"iscolor"`
	}

	user := &User{
		FirstName:      "Hertz",
		Age:            135,
		Email:          "hertz",
		FavouriteColor: "sad",
	}
	err := DefaultValidator.ValidateStruct(user)
	if err != nil {
		fmt.Println(err)
	}
	// Output:
	// Key: 'User.LastName' Error:Field validation for 'LastName' failed on the 'required' tag
	// Key: 'User.Age' Error:Field validation for 'Age' failed on the 'lte' tag
	// Key: 'User.Email' Error:Field validation for 'Email' failed on the 'email' tag
	// Key: 'User.FavouriteColor' Error:Field validation for 'FavouriteColor' failed on the 'iscolor' tag
}
