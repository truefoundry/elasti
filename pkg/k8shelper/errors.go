package k8shelper

import "errors"

var ErrNoPodFound = errors.New("no pod found")
var ErrNoActivePodFound = errors.New("no active pod found")
var ErrNoScaleTargetFound = errors.New("no scale target found")
var ErrNoPublicServiceFound = errors.New("no public service found")
