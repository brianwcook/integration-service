/*
Copyright 2023 Red Hat Inc.

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

package testsubject

import (
	"reflect"

	integrationv1alpha1 "github.com/konflux-ci/integration-service/api/integration/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// TestSubjectCreatedPredicate returns a predicate that triggers when a TestSubject is created
func TestSubjectCreatedPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

// TestSubjectTestStatusChangedPredicate returns a predicate that triggers when test status changes
func TestSubjectTestStatusChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldTestSubject, oldOk := e.ObjectOld.(*integrationv1alpha1.TestSubject)
			newTestSubject, newOk := e.ObjectNew.(*integrationv1alpha1.TestSubject)

			if !oldOk || !newOk {
				return false
			}

			// Trigger if test results have changed
			return !reflect.DeepEqual(oldTestSubject.Status.TestResults, newTestSubject.Status.TestResults)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
