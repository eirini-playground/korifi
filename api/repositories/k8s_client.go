package repositories

import (
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	k8sclient "k8s.io/client-go/kubernetes"

	"code.cloudfoundry.org/cf-k8s-controllers/api/apierrors"
	"code.cloudfoundry.org/cf-k8s-controllers/api/authorization"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type UserK8sClientFactory interface {
	BuildClient(authorization.Info) (client.WithWatch, error)
	BuildK8sClient(info authorization.Info) (k8sclient.Interface, error)
}

type UnprivilegedClientFactory struct {
	config *rest.Config
	mapper meta.RESTMapper
}

func NewUnprivilegedClientFactory(config *rest.Config, mapper meta.RESTMapper) UnprivilegedClientFactory {
	return UnprivilegedClientFactory{
		config: rest.AnonymousClientConfig(rest.CopyConfig(config)),
		mapper: mapper,
	}
}

func (f UnprivilegedClientFactory) BuildClient(authInfo authorization.Info) (client.WithWatch, error) {
	config := rest.CopyConfig(f.config)

	switch strings.ToLower(authInfo.Scheme()) {
	case authorization.BearerScheme:
		config.BearerToken = authInfo.Token

	case authorization.CertScheme:
		certBlock, rst := pem.Decode(authInfo.CertData)
		if certBlock == nil {
			return nil, fmt.Errorf("failed to decode cert PEM")
		}

		keyBlock, _ := pem.Decode(rst)
		if keyBlock == nil {
			return nil, fmt.Errorf("failed to decode key PEM")
		}

		config.CertData = pem.EncodeToMemory(certBlock)
		config.KeyData = pem.EncodeToMemory(keyBlock)

	default:
		return nil, apierrors.NewNotAuthenticatedError(errors.New("unsupported Authorization header scheme"))
	}

	userClient, err := client.NewWithWatch(config, client.Options{
		Scheme: scheme.Scheme,
		Mapper: f.mapper,
	})
	if err != nil {
		return nil, apierrors.FromK8sError(err, "")
	}

	return userClient, nil
}

func (f UnprivilegedClientFactory) BuildK8sClient(authInfo authorization.Info) (k8sclient.Interface, error) {
	config := rest.CopyConfig(f.config)

	switch strings.ToLower(authInfo.Scheme()) {
	case authorization.BearerScheme:
		config.BearerToken = authInfo.Token

	case authorization.CertScheme:
		certBlock, rst := pem.Decode(authInfo.CertData)
		if certBlock == nil {
			return nil, fmt.Errorf("failed to decode cert PEM")
		}

		keyBlock, _ := pem.Decode(rst)
		if keyBlock == nil {
			return nil, fmt.Errorf("failed to decode key PEM")
		}

		config.CertData = pem.EncodeToMemory(certBlock)
		config.KeyData = pem.EncodeToMemory(keyBlock)

	default:
		return nil, apierrors.NewNotAuthenticatedError(errors.New("unsupported Authorization header scheme"))
	}

	userK8sClient, err := k8sclient.NewForConfig(config)
	if err != nil {
		return nil, apierrors.FromK8sError(err, "")
	}

	return userK8sClient, nil
}

func NewPrivilegedClientFactory(config *rest.Config, mapper meta.RESTMapper) PrivilegedClientFactory {
	return PrivilegedClientFactory{
		config: config,
		mapper: mapper,
	}
}

type PrivilegedClientFactory struct {
	config *rest.Config
	mapper meta.RESTMapper
}

func (f PrivilegedClientFactory) BuildClient(_ authorization.Info) (client.WithWatch, error) {
	return client.NewWithWatch(f.config, client.Options{
		Scheme: scheme.Scheme,
		Mapper: f.mapper,
	})
}
