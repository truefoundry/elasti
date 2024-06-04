package controller

import "sync"

type Directory struct {
	Services sync.Map
}

type CRDDetails struct {
	CRDName        string
	DeploymentName string
}

var ServiceDirectory *Directory

var serviceDirectoryOnce sync.Once

func NewDirectory() {
	serviceDirectoryOnce.Do(func() {
		ServiceDirectory = &Directory{}
	})
}

func (d *Directory) AddService(serviceName string, crdDetails *CRDDetails) {
	d.Services.Store(serviceName, crdDetails)
}

func (d *Directory) GetService(serviceName string) (*CRDDetails, bool) {
	value, ok := d.Services.Load(serviceName)
	if !ok {
		return nil, false
	}
	return value.(*CRDDetails), true
}
