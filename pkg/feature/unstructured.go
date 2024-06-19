package feature

import (
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/annotations"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"regexp"
	"strings"
)

func MarkAsManaged(objs []*unstructured.Unstructured) {
	for _, obj := range objs {
		objAnnotations := obj.GetAnnotations()
		if objAnnotations == nil {
			objAnnotations = make(map[string]string)
		}

		objAnnotations[annotations.ManagedByODHOperator] = "true"
		obj.SetAnnotations(objAnnotations)
	}
}

func ConvertToUnstructured(resources string) ([]*unstructured.Unstructured, error) {
	splitter := regexp.MustCompile(resourceSeparator)
	objectStrings := splitter.Split(resources, -1)
	objs := make([]*unstructured.Unstructured, 0, len(objectStrings))
	for _, str := range objectStrings {
		if strings.TrimSpace(str) == "" {
			continue
		}
		u := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(str), u); err != nil {
			return nil, err
		}

		objs = append(objs, u)
	}
	return objs, nil
}
