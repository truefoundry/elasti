package k8sHelper

import "errors"

var ErrNoPodFound = errors.New("no pod found")
var ErrNoActivePodFound = errors.New("no active pod found")
