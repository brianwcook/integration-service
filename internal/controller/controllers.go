/*
Copyright 2022 Red Hat Inc.

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

package controllers

import (
	"github.com/go-logr/logr"
	"github.com/konflux-ci/integration-service/internal/controller/scenario"
	"github.com/konflux-ci/integration-service/internal/controller/testsubject"
	"github.com/konflux-ci/integration-service/internal/controller/testsubjectconstructor"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// setupFunctions is a list of register functions to be invoked so all controllers are added to the Manager
var setupFunctions = []func(manager.Manager, *logr.Logger) error{
	// Old controllers commented out for ADR-0033 testing - these depend on old AppStudio CRDs
	// integrationpipeline.SetupController,
	// buildpipeline.SetupController,
	// snapshot.SetupController,
	// statusreport.SetupController,
	// component.SetupController,

	// New ADR-0033 controllers
	scenario.SetupController,
	testsubject.SetupController,
	testsubjectconstructor.SetupController,
}

// SetupControllers invoke all SetupController functions defined in setupFunctions, setting all controllers up and
// adding them to the Manager.
func SetupControllers(manager manager.Manager) error {
	log := logf.Log.WithName("controllers")

	for _, function := range setupFunctions {
		if err := function(manager, &log); err != nil {
			return err
		}
	}
	return nil
}
