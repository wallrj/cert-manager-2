/*
Copyright 2020 The cert-manager Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package acmechallenges

import (
	"context"
	"errors"
	"fmt"

	acmecl "github.com/cert-manager/cert-manager/pkg/acme/client"
	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	acmeapi "golang.org/x/crypto/acme"
)

// syncChallengeStatus will communicate with the ACME server to retrieve the current
// state of the Challenge. It will then update the Challenge's status block with the new
// state of the Challenge.
func syncChallengeStatus(ctx context.Context, cl acmecl.Interface, ch *cmacme.Challenge) error {
	if ch.Spec.URL == "" {
		return fmt.Errorf("challenge URL is blank - challenge has not been created yet")
	}

	// Here we GetAuthorization and prune out the Challenge we are concerned with
	// to gather the current state of the Challenge. In older versions of
	// cert-manager we called the Challenge endpoint directly using a POST-as-GET
	// request (GetChallenge). This caused issues with some ACME server
	// implementations whereby they either interpreted this call as an Accept
	// which would invalidate the Order as Challenge resources were not ready yet
	// to complete the Challenge, or otherwise bork their state machines.
	// While the ACME RFC[1] is left ambiguous as to whether this call is indeed
	// supported, it is the general consensus by the cert-manager team that it
	// should be. In any case, in an effort to support as many current and future
	// ACME server implementations as possible, we have decided to use a
	// POST-as-GET to the Authorization endpoint instead which unequivocally is
	// part of the RFC explicitly.
	// This issue was brought to the RFC mailing list[2].
	// [1] - https://datatracker.ietf.org/doc/html/rfc8555#section-7.5.1
	// [2] - https://mailarchive.ietf.org/arch/msg/acme/NknXHBXl3aRG0nBmgsFH-SP90A4/
	acmeAuthorization, err := cl.GetAuthorization(ctx, ch.Spec.AuthorizationURL)
	if err != nil {
		return err
	}
	acmeAuthorization.Challenges
	var acmeChallenge *acmeapi.Challenge
	for _, challenge := range acmeAuthorization.Challenges {
		if challenge.URI == ch.Spec.URL {
			acmeChallenge = challenge
			break
		}
	}

	if acmeChallenge == nil {
		return errors.New("challenge was not present in authorization")
	}

	// TODO: should we validate the State returned by the ACME server here?
	cmState := cmacme.State(acmeChallenge.Status)
	// be nice to our users and check if there is an error that we
	// can tell them about in the reason field
	// TODO(dmo): problems may be compound and they may be tagged with
	// a type field that suggests changes we should make (like provisioning
	// an account). We might be able to handle errors more gracefully using
	// this info
	ch.Status.Reason = ""
	if acmeChallenge.Error != nil {
		if acmeErr, ok := acmeChallenge.Error.(*acmeapi.Error); ok {
			ch.Status.Reason = acmeErr.Detail
		} else {
			ch.Status.Reason = acmeChallenge.Error.Error()
		}
	}
	ch.Status.State = cmState

	return nil
}
