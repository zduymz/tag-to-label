package utils

import (
	"fmt"
	"k8s.io/klog"
)

// All dummy function should stay here

func DumpObject(obj interface{})  {
	klog.Info(obj)
}

func Log(s string) {
	klog.Infof("%s \n", s)
}

// check the list contain string
func IsInSlice(x string, list []*interface{}) bool {
	for _,v := range list {
		if *v == x {
			return true
		}
	}
	return false
}

func LastinSlice(xs []string) (string, error) {
	if (len(xs) == 0) {
		return "", fmt.Errorf("Empty Slice")
	}
	return xs[len(xs)-1], nil
}