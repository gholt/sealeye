package main

import "fmt"

type sprinkles struct {
	SprinkleType  int `option:"sprinkle-type" help:"The type of sprinkles to output."`
	SprinkleCount int `option:"sprinkle-count" help:"The number of sprinkles to output." default:"10"`
}

func (s sprinkles) sprinkle() {
	switch s.SprinkleType {
	case 1:
		for i := 0; i < s.SprinkleCount; i++ {
			fmt.Print("* + x ")
		}
		fmt.Println("*")
	default:
	}
}
