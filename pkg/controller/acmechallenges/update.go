package acmechallenges

import (
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	internalchallenges "github.com/cert-manager/cert-manager/internal/controller/challenges"
	"github.com/cert-manager/cert-manager/internal/controller/feature"
	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	cmclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	utilfeature "github.com/cert-manager/cert-manager/pkg/util/feature"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type updater struct {
	cmClient     cmclient.Interface
	fieldManager string
	err          error
	original     *cmacme.Challenge
	modified     *cmacme.Challenge
}

func (o *updater) run(ctx context.Context) {
	errs := []error{o.err}
	if !apiequality.Semantic.DeepEqual(o.original.Status, o.modified.Status) {
		if ch, err := o.updateStatus(ctx, o.modified); err != nil {
			errs = append(errs, err)
		} else {
			o.modified = ch
		}
	}
	if !apiequality.Semantic.DeepEqual(o.original.Finalizers, o.modified.Finalizers) {
		if ch, err := o.update(ctx, o.modified); err != nil {
			errs = append(errs, err)
		} else {
			o.modified = ch
		}
	}
	o.err = utilerrors.NewAggregate(errs)
}

func (o *updater) update(ctx context.Context, challenge *cmacme.Challenge) (*cmacme.Challenge, error) {
	if utilfeature.DefaultFeatureGate.Enabled(feature.ServerSideApply) {
		return internalchallenges.Apply(ctx, o.cmClient, o.fieldManager, challenge)
	} else {
		return o.cmClient.AcmeV1().Challenges(challenge.Namespace).Update(ctx, challenge, metav1.UpdateOptions{})
	}
}

func (o *updater) updateStatus(ctx context.Context, challenge *cmacme.Challenge) (*cmacme.Challenge, error) {
	if utilfeature.DefaultFeatureGate.Enabled(feature.ServerSideApply) {
		return internalchallenges.ApplyStatus(ctx, o.cmClient, o.fieldManager, challenge)
	} else {
		return o.cmClient.AcmeV1().Challenges(challenge.Namespace).UpdateStatus(ctx, challenge, metav1.UpdateOptions{})
	}
}
