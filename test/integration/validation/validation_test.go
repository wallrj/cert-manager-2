package validation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	cmapi "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cmclient "github.com/jetstack/cert-manager/pkg/client/clientset/versioned"
	"github.com/jetstack/cert-manager/test/unit/gen"
)

func TestValidation(t *testing.T) {
	t.Log("validate this")
	env := &envtest.Environment{
		CRDDirectoryPaths: []string{
			"../../../bazel-bin/deploy/crds/crds.regular.yaml",
		},
	}
	config, err := env.Start()
	require.NoError(t, err)
	defer func() {
		require.NoError(t, env.Stop())
	}()
	client := cmclient.NewForConfigOrDie(config)
	certificate := gen.Certificate("certificate1", gen.SetCertificateNamespace("default"))
	certificate.Spec.Usages = []cmapi.KeyUsage{
		"foo",
	}
	ctx := context.Background()
	_, err = client.CertmanagerV1().Certificates("default").Create(ctx, certificate, metav1.CreateOptions{})
	require.NoError(t, err)
}
