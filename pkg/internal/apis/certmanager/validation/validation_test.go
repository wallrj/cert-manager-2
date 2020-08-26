package validation_test

import (
	"fmt"
	"math/rand"
	"testing"

	v1 "gitlab.jetstack.net/portal/aerodrome/api/v1"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	metafuzzer "k8s.io/apimachinery/pkg/apis/meta/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/util/jsonpath"

	internalcmapi "github.com/jetstack/cert-manager/pkg/apis/cert"
	internalcmapi "github.com/jetstack/cert-manager/pkg/internal/apis/certmanager"
	cmfuzzer "github.com/jetstack/cert-manager/pkg/internal/apis/certmanager/fuzzer"
	"github.com/jetstack/cert-manager/pkg/internal/apis/certmanager/install"
	"github.com/jetstack/cert-manager/pkg/internal/apis/certmanager/validation"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	scheme := runtime.NewScheme()
	codecFactory := runtimeserializer.NewCodecFactory(scheme)

	install.Install(scheme)
	f := fuzzer.FuzzerFor(
		fuzzer.MergeFuzzerFuncs(metafuzzer.Funcs, cmfuzzer.Funcs),
		rand.NewSource(rand.Int63()),
		codecFactory,
	)

	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprintf("t-%d", i), func(t *testing.T) {
			var certificate internalcmapi.Certificate
			f.Fuzz(&certificate)
			scheme.ConvertToVersion(&certificate, v1.GroupVersion)
			errors := validation.ValidateCertificate(&certificate)
			p := jsonpath.New("parser")
			for _, e := range errors {
				t.Log(e.Field)
				err := p.Parse(e.Field)
				require.NoError(t, err)
				_, err = p.FindResults(&certificate)
				require.NoError(t, err)
			}
		})
	}

}
