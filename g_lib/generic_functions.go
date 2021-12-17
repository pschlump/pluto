package g_lib

import (
	"fmt"
	"constraints"
)

func Min[T constraints.Ordered](a, b T) T{
  if a < b {
    return a
  }
  return b
}

func Max[T constraints.Ordered](a, b T) T{
  if a > b {
    return a
  }
  return b
}

func MinArray[T constraints.Ordered](a []T) (rv T){
	if len(a) > 0 {
		rv = a[0]
	}
	for _, v := range a {
		if v < rv {
			rv = v
		}
	}
  	return 
}

func MaxArray[T constraints.Ordered](a []T) (rv T){
	if len(a) > 0 {
		rv = a[0]
	}
	for _, v := range a {
		if v > rv {
			rv = v
		}
	}
  	return 
}


func InArray[T comparable](needle T, haystack []T) bool {
	for _, val := range haystack {
		if val == needle {
			return true
		}
	}
	return false
}

func LocationInArray[T comparable](needle T, haystack []T) int {
	for ii, val := range haystack {
		if val == needle {
			return ii
		}
	}
	return -1
}

func KeysForStringMap[T any](aMap map[string]T) (rv []string ) {
	for key := range aMap {
		rv = append ( rv, key )
	}
	return 
}

func callOnce() {
	fmt.Printf ( "MaxArray ( []string{\"a\",\"b\"} ) = %v\n", MaxArray( []string{"a","b"} ) )
}
