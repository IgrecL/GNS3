package utils

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func ToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}
