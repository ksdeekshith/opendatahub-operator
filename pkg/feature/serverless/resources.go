package serverless

import (
	"context"

	infrav1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/infrastructure/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/feature"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/feature/servicemesh"
)

func ServingCertificateResource(f *feature.Feature) error {
	secretData, err := fetchSecretParams(f)
	if err != nil {
		return err
	}

	switch secretData.Type {
	case infrav1.SelfSigned:
		return cluster.CreateSelfSignedCertificate(
			context.TODO(), f.Client,
			secretData.Name,
			secretData.Domain,
			secretData.Namespace,
			feature.OwnedBy(f))
	case infrav1.Provided:
		return nil
	default:
		return cluster.PropagateDefaultIngressCertificate(context.TODO(), f.Client, secretData.Name, secretData.Namespace)
	}
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
