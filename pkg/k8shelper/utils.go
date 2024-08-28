package k8shelper

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func UnstructuredToResource(obj interface{}, resource interface{}) error {
	unstructuredObj := obj.(*unstructured.Unstructured)
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), resource)
	if err != nil {
		return fmt.Errorf("UnstructuredToResource: %w", err)
	}
	return nil
}
