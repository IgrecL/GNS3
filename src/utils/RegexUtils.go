package utils

import (
	"fmt"
	"regexp"
)

func RegReplace(input, regex, text string) string {
	regexp, err := regexp.Compile("{" + regex + "}")
	if err != nil {
		fmt.Println(err)
		return ""
	}
	return regexp.ReplaceAllString(input, text)
}

func RegCarriage(input string) string {
	regexp, err := regexp.Compile("\n")
	if err != nil {
		fmt.Println(err)
		return ""
	}
	return regexp.ReplaceAllString(input, "\r\n")
}
