package binding_v2

import (
	"fmt"
)

func ExampleValidateStruct() {
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
	//Key: 'User.LastName' Error:Field validation for 'LastName' failed on the 'required' tag
	//Key: 'User.Age' Error:Field validation for 'Age' failed on the 'lte' tag
	//Key: 'User.Email' Error:Field validation for 'Email' failed on the 'email' tag
	//Key: 'User.FavouriteColor' Error:Field validation for 'FavouriteColor' failed on the 'iscolor' tag
}
