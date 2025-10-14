/*
Copyright 2025.

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

package controller

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"bennsimon.me/configmirror-operator/api/v1alpha1"
	"bennsimon.me/configmirror-operator/pkg/utils"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type testClient struct {
	client.Client
	mock.Mock
}

func (t *testClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	args := t.Called(ctx, key, obj, opts)
	return args.Error(0)
}

// default namespace used to return a matching configmap
func (t *testClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	for _, opt := range opts {
		if client.InNamespace("default") == opt {
			configmaps := list.(*v1.ConfigMapList)
			configmaps.Items = append(list.(*v1.ConfigMapList).Items, v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
				Labels: map[string]string{
					"app": "test",
				},
			}})
			break
		}
	}

	args := t.Called(ctx, list, opts)
	return args.Error(0)
}

func Test_updateConfigMapAnnotations(t *testing.T) {
	cfgMirrorReconciler := &ConfigMirrorReconciler{Client: &testClient{}}
	type args struct {
		_configMap   *v1.ConfigMap
		configMap    v1.ConfigMap
		configMirror *v1alpha1.ConfigMirror
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{{name: "should update annotations for new configmap", args: args{
		_configMap:   &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}},
		configMap:    v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{UID: types.UID("id"), Namespace: "default"}},
		configMirror: &v1alpha1.ConfigMirror{ObjectMeta: metav1.ObjectMeta{Name: "test-configmirror", Namespace: "default"}}},
		want: map[string]string{
			utils.GetAnnotationKey(utils.ManagedBy):       fmt.Sprintf("%s/%s", "default", "test-configmirror"),
			utils.GetAnnotationKey(utils.SourceNamespace): "default",
			utils.GetAnnotationKey(utils.MirrorKey):       "id",
		},
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgMirrorReconciler.updateConfigMapAnnotations(tt.args._configMap, tt.args.configMap, tt.args.configMirror)
			if !reflect.DeepEqual(tt.args._configMap.Annotations, tt.want) {
				t.Errorf("updateConfigMapAnnotations() = %v, want %v", tt.args._configMap.Annotations, tt.want)
			}

		})
	}

}

func Test_fetchMatchingConfigMaps(t *testing.T) {

	cfgMirrorReconciler := &ConfigMirrorReconciler{}

	type args struct {
		ctx          context.Context
		configMirror *v1alpha1.ConfigMirror
	}
	tests := []struct {
		name      string
		args      args
		want      *v1.ConfigMapList
		wantError bool
		initFunc  func()
	}{{
		name: "should return list of matching config maps",
		args: args{
			ctx: context.TODO(),
			configMirror: &v1alpha1.ConfigMirror{Spec: v1alpha1.ConfigMirrorSpec{
				SourceNamespace:  &[]string{"default"}[0],
				TargetNamespaces: &[]string{"test"},
				Selector: v1alpha1.ConfigMirrorLabelSelector{
					MatchLabels: map[string]string{
						"api": "test",
					},
				},
			},
				ObjectMeta: metav1.ObjectMeta{Name: "test-configmirror", Namespace: "default"}},
		},
		initFunc: func() {
			_testClient := &testClient{}
			selector, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: map[string]string{}})
			var listOptions []client.ListOption
			listOptions = append(listOptions, client.InNamespace("default"), client.MatchingLabelsSelector{Selector: selector})
			_testClient.On("List",
				mock.Anything,
				mock.AnythingOfType("*v1.ConfigMapList"),
				mock.IsType(listOptions)).Return(nil)
			cfgMirrorReconciler.Client = _testClient
		},
		want: &v1.ConfigMapList{
			Items: []v1.ConfigMap{
				{ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
					Labels: map[string]string{
						"app": "test",
					},
				}},
			},
		},
		wantError: false,
	},
		{
			name: "should return no config maps",
			args: args{
				ctx: context.TODO(),
				configMirror: &v1alpha1.ConfigMirror{Spec: v1alpha1.ConfigMirrorSpec{
					SourceNamespace:  &[]string{"test"}[0],
					TargetNamespaces: &[]string{"test"},
					Selector: v1alpha1.ConfigMirrorLabelSelector{
						MatchLabels: map[string]string{
							"api": "test",
						},
					},
				},
					ObjectMeta: metav1.ObjectMeta{Name: "test-configmirror", Namespace: "default"}},
			},
			initFunc: func() {
				_testClient := &testClient{}
				selector, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: map[string]string{}})
				var listOptions []client.ListOption
				listOptions = append(listOptions, client.InNamespace("test"), client.MatchingLabelsSelector{Selector: selector})
				_testClient.On("List",
					mock.Anything,
					mock.AnythingOfType("*v1.ConfigMapList"),
					mock.IsType(listOptions)).Return(errors.New("not found"))
				cfgMirrorReconciler.Client = _testClient
			},
			want:      nil,
			wantError: true,
		}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.initFunc()
			configMapList, err := cfgMirrorReconciler.fetchMatchingConfigMaps(tt.args.ctx, tt.args.configMirror)
			if (err == nil) == tt.wantError {
				t.Errorf("fetchMatchingConfigMaps() = %v, want %v", err == nil, tt.wantError)
			}

			if !reflect.DeepEqual(configMapList, tt.want) {
				t.Errorf("fetchMatchingConfigMaps() = %v, want %v", configMapList, tt.want)
			}

		})
	}
}
