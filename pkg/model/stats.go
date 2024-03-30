/*
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

package model

import v1 "k8s.io/api/core/v1"

type Stats struct {
	NumNodes             int
	AllocatableResources v1.ResourceList
	UsedResources        v1.ResourceList
	PercentUsedResources map[v1.ResourceName]float64
	Nodes                []*Node
	TotalPods            int
	PodsByPhase          map[v1.PodPhase]int
	BoundPodCount        int
	TotalPrice           float64
}
