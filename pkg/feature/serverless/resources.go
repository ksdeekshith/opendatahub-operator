package serverless

import (
	"context"

	infrav1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/infrastructure/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/feature"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/feature/servicemesh"
)

func ServingCertificateResource(f *feature.Feature) error {
	secretData, err := fetchSecretParams(f)
	if err != nil {
		return err
	}

	return feature.CreateSelfSignedCertificate(
		context.TODO(), f.Client,
		secretData.Name,
		secretData.Type,
		secretData.Domain,
		secretData.Namespace,
	)
}

type secretParams struct {
	Name      string
	Namespace string
	Domain    string
	Type      infrav1.CertType
}

func fetchSecretParams(f *feature.Feature) (*secretParams, error) {
	result := &secretParams{}

	if secret, err := FeatureData.Certificate.From(f); err == nil {
		result.Name = secret
	} else {
		return nil, err
	}

	if domain, err := FeatureData.IngressDomain.From(f); err == nil {
		result.Domain = domain
	} else {
		return nil, err
	}

	if serving, err := FeatureData.Serving.From(f); err == nil {
		result.Type = serving.IngressGateway.Certificate.Type
	} else {
		return nil, err
	}

	if controlPlane, err := servicemesh.FeatureData.ControlPlane.From(f); err == nil {
		result.Namespace = controlPlane.Namespace
	} else {
		return nil, err
	}

	return result, nil
}
