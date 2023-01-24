package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type IP struct {
	Digits [8]int // TODO: uint16
	Mask   int
}

func (subnetwork IP) GetRange() (IP, IP) {
	mask := 128 - subnetwork.Mask
	lowIP := subnetwork
	var highIP IP
	var maskbits [8]uint16
	for i := len(maskbits) - 1; i >= 0; i-- {
		for j := 0; j < Min(mask-16*(7-i), 16); j++ {
			maskbits[i] += (1 << j)
		}
	}

	for i := 0; i < len(maskbits); i++ {
		highIP.Digits[i] = subnetwork.Digits[i] | int(maskbits[i])
	}

	highIP.Mask = subnetwork.Mask

	return lowIP, highIP
}

func (ip IP) ToString(withMask bool) string {
	var str string
	for i := 0; i < 8; i++ {
		str += strconv.FormatInt(int64(ip.Digits[i]), 16) + ":"
	}
	str = str[:len(str)-1]
	if withMask {
		str += "/" + fmt.Sprint(ip.Mask)
	}

	regexp, err := regexp.Compile(":(0:)+")
	if err != nil {
		fmt.Println(err)
		return ""
	}

	// Remplace 0:0:0:0 par :: (max 1 fois)
	flag := false
	str = regexp.ReplaceAllStringFunc(str, func(a string) string {
		if flag {
			return a
		}
		flag = true
		return regexp.ReplaceAllString(a, "::")
	})

	return str
}

func (ip *IP) ToInt(ipString string) {
	split := strings.Split(ipString, ":")
	if len(split) < 8 {
		var splitComplet [8]string
		var stop bool
		var i, j int = 0, 0
		for ; !stop; i++ {
			splitComplet[i] = split[i]
			if split[i] == "" {
				stop = true
			}
		}
		stop = false
		for ; !stop; j++ {
			splitComplet[7-j] = split[len(split)-1-j]
			if split[len(split)-1-j] == "" {
				stop = true
			}
		}
		split = splitComplet[:]
	}
	split2 := strings.Split(split[7], "/")
	split[7] = split2[0]
	ip.Mask, _ = strconv.Atoi(split2[1])
	for i, s := range split {
		numHex, _ := strconv.ParseInt(s, 16, 64)
		ip.Digits[i] = int(numHex)
	}
}

func (ip IP) Increment() IP {
	var new IP
	new.Digits = ip.Digits
	new.Mask = ip.Mask
	for i := 7; i >= 0; i-- {
		if new.Digits[i] == 65535 {
			new.Digits[i] = 0
		} else {
			new.Digits[i]++
			break
		}
	}
	return new
}

func (ip1 IP) Equals(ip2 IP) bool {
	for i := 0; i < 8; i++ {
		if ip1.Digits[i] != ip2.Digits[i] {
			return false
		}
	}
	return true
}

type Interface struct {
	Name     string
	Ip       IP
	RouterId int
	ASBR     bool
	OSPFCost int
}

type Link [2]Interface

type AS struct {
	ASN        int
	IGP        string
	RoutersId  []int
	Adj        [][]*Link
	LocalPrefs [][]int
	Prepends   []int
}
