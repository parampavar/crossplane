/*
Copyright 2020 The Crossplane Authors.

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

package revision

import (
	"context"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	admv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

var _ Establisher = &APIEstablisher{}

func TestAPIEstablisherEstablish(t *testing.T) {
	errBoom := errors.New("boom")
	webhookTLSSecretName := "webhook-tls"
	caBundle := []byte("CABUNDLE")

	type args struct {
		est     *APIEstablisher
		objs    []runtime.Object
		parent  v1.PackageRevision
		control bool
	}

	type want struct {
		err  error
		refs []xpv1.TypedReference
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessfulExistsEstablishControl": {
			reason: "Establishment should be successful if we can establish control for a parent of existing objects.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
				},
				objs: []runtime.Object{
					&extv1.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ref-me",
						},
					},
				},
				parent: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "provider-name",
								UID:  "some-unique-uid-2312",
							},
						},
						Labels: map[string]string{
							v1.LabelParentPackage: "provider-name",
						},
					},
				},
				control: true,
			},
			want: want{
				refs: []xpv1.TypedReference{{Name: "ref-me"}},
			},
		},
		"SuccessfulNotExistsEstablishControl": {
			reason: "Establishment should be successful if we can establish control for a parent of new objects.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
						MockCreate: test.NewMockCreateFn(nil),
					},
				},
				objs: []runtime.Object{
					&extv1.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ref-me",
						},
					},
				},
				parent: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "provider-name",
								UID:  "some-unique-uid-2312",
							},
						},
						Labels: map[string]string{
							v1.LabelParentPackage: "provider-name",
						},
					},
				},
				control: true,
			},
			want: want{
				refs: []xpv1.TypedReference{{Name: "ref-me"}},
			},
		},
		"SuccessfulNotExistsEstablishControlWebhookEnabled": {
			reason: "Establishment should be successful if we can establish control for a parent of new objects in case webhooks are enabled.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
							if s, ok := obj.(*corev1.Secret); ok {
								(&corev1.Secret{
									Data: map[string][]byte{
										"tls.crt": caBundle,
									},
								}).DeepCopyInto(s)
								return nil
							}
							return kerrors.NewNotFound(schema.GroupResource{}, "")
						},
						MockCreate: test.NewMockCreateFn(nil),
					},
				},
				objs: []runtime.Object{
					&extv1.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ref-me",
						},
						Spec: extv1.CustomResourceDefinitionSpec{
							Conversion: &extv1.CustomResourceConversion{
								Strategy: extv1.WebhookConverter,
							},
						},
					},
					&admv1.MutatingWebhookConfiguration{
						ObjectMeta: metav1.ObjectMeta{
							Name: "crossplane-providerrevision-provider-name",
						},
						Webhooks: []admv1.MutatingWebhook{
							{
								Name: "some-webhook",
							},
						},
					},
					&admv1.ValidatingWebhookConfiguration{
						ObjectMeta: metav1.ObjectMeta{
							Name: "crossplane-providerrevision-provider-name",
						},
						Webhooks: []admv1.ValidatingWebhook{
							{
								Name: "some-webhook",
							},
						},
					},
				},
				parent: &v1.ProviderRevision{
					TypeMeta: metav1.TypeMeta{
						Kind: "ProviderRevision",
					},
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "Provider",
								Name: "provider-name",
								UID:  "some-unique-uid-2312",
							},
						},
						Labels: map[string]string{
							v1.LabelParentPackage: "provider-name",
						},
					},
					Spec: v1.PackageRevisionSpec{
						WebhookTLSSecretName: &webhookTLSSecretName,
					},
				},
				control: true,
			},
			want: want{
				refs: []xpv1.TypedReference{
					{Name: "ref-me"},
					{Name: "crossplane-provider-provider-name"},
					{Name: "crossplane-provider-provider-name"},
				},
			},
		},
		"SuccessfulExistsEstablishOwnership": {
			reason: "Establishment should be successful if we can establish ownership for a parent of existing objects.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
				},
				objs: []runtime.Object{
					&extv1.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ref-me",
						},
					},
				},
				parent:  &v1.ProviderRevision{},
				control: false,
			},
			want: want{
				refs: []xpv1.TypedReference{{Name: "ref-me"}},
			},
		},
		"SuccessfulNotExistsDoNotCreate": {
			reason: "Establishment should be successful if we skip creating a resource we do not want to control.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
						MockCreate: test.NewMockCreateFn(errBoom),
					},
				},
				objs: []runtime.Object{
					&extv1.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ref-me",
						},
					},
				},
				parent:  &v1.ProviderRevision{},
				control: false,
			},
			want: want{
				refs: []xpv1.TypedReference{{Name: "ref-me"}},
			},
		},
		"FailedCreationWebhookDisabledConversionRequested": {
			reason: "Establishment should fail if the CRD requires conversion webhook and Crossplane does not have the webhooks enabled.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
						MockCreate: test.NewMockCreateFn(nil),
					},
				},
				objs: []runtime.Object{
					&extv1.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ref-me",
						},
						Spec: extv1.CustomResourceDefinitionSpec{
							Conversion: &extv1.CustomResourceConversion{
								Strategy: extv1.WebhookConverter,
							},
						},
					},
				},
				parent: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Name: "provider-name",
								UID:  "some-unique-uid-2312",
							},
						},
						Labels: map[string]string{
							v1.LabelParentPackage: "provider-name",
						},
					},
				},
				control: true,
			},
			want: want{
				err: errors.New(errConversionWithNoWebhookCA),
			},
		},
		"FailedGettingWebhookTLSSecret": {
			reason: "Establishment should fail if a webhook TLS secret is given but cannot be fetched",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(errBoom),
					},
				},
				parent: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						WebhookTLSSecretName: &webhookTLSSecretName,
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetWebhookTLSSecret),
			},
		},
		"FailedEmptyWebhookTLSSecret": {
			reason: "Establishment should fail if a webhook TLS secret is given but empty",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
							s := &corev1.Secret{}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						},
					},
				},
				parent: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						WebhookTLSSecretName: &webhookTLSSecretName,
					},
				},
			},
			want: want{
				err: errors.New(errWebhookSecretWithoutCABundle),
			},
		},
		"FailedCreate": {
			reason: "Cannot establish control of object if we cannot create it.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
						MockCreate: test.NewMockCreateFn(errBoom),
					},
				},
				objs: []runtime.Object{
					&extv1.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ref-me",
						},
					},
				},
				parent: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				control: true,
			},
			want: want{
				err: errBoom,
			},
		},
		"FailedUpdate": {
			reason: "Cannot establish control of object if we cannot update it.",
			args: args{
				est: &APIEstablisher{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockUpdate: test.NewMockUpdateFn(errBoom),
					},
				},
				objs: []runtime.Object{
					&extv1.CustomResourceDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ref-me",
						},
					},
				},
				parent: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				control: true,
			},
			want: want{
				err: errBoom,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			refs, err := tc.args.est.Establish(context.TODO(), tc.args.objs, tc.args.parent, tc.args.control)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Check(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			trans := cmp.Transformer("Sort", func(refs []xpv1.TypedReference) []xpv1.TypedReference {
				out := append([]xpv1.TypedReference(nil), refs...) // Copy input to avoid mutating it
				sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
				return out
			})
			if diff := cmp.Diff(tc.want.refs, refs, test.EquateErrors(), trans); diff != "" {
				t.Errorf("\n%s\ne.Check(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetPackageOwnerReference(t *testing.T) {
	type args struct {
		revision resource.Object
	}
	type want struct {
		ref metav1.OwnerReference
		ok  bool
	}
	ref := metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "Provider",
		Name:       "provider-name",
		UID:        types.UID("some-random-uid"),
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Found": {
			reason: "We need to correctly find the owner reference of the parent package",
			args: args{
				revision: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{},
							ref,
							{
								Name: "something-else",
							},
						},
						Labels: map[string]string{
							v1.LabelParentPackage: "provider-name",
						},
					},
				},
			},
			want: want{
				ref: ref,
				ok:  true,
			},
		},
		"NotFound": {
			args: args{
				revision: &v1.ProviderRevision{},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result, ok := GetPackageOwnerReference(tc.args.revision)

			if diff := cmp.Diff(tc.want.ref, result); diff != "" {
				t.Errorf("\n%s\ne.GetPackageOwnerReference(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.ok, ok); diff != "" {
				t.Errorf("\n%s\ne.GetPackageOwnerReference(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
