package model

import (
	"fmt"
	"github.com/newrelic/go-agent/v3/newrelic"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

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

type NRModel struct {
	cluster     *Cluster
	extraLabels []string
}

func NewNRModel(extraLabels []string) *NRModel {
	return &NRModel{
		// red to green
		cluster:     NewCluster(),
		extraLabels: extraLabels,
	}
}

func (m *NRModel) Cluster() *Cluster {
	return m.cluster
}

func (m *NRModel) Run(clusterName string, nr *newrelic.Application) {
	stats := m.cluster.Stats()

	//u.writeClusterSummary(u.cluster.resources, stats)

	for _, n := range stats.Nodes {
		event := map[string]interface{}{}
		event["NodeName"] = n.Name()
		event["ClusterName"] = clusterName

		allocatable := n.Allocatable()
		used := n.Used()
		for _, res := range m.cluster.resources {
			usedRes := used[res]
			allocatableRes := allocatable[res]
			pct := usedRes.AsApproximateFloat64() / allocatableRes.AsApproximateFloat64() * 100
			if allocatableRes.AsApproximateFloat64() == 0 {
				pct = 0
			}
			event["PodCount"] = fmt.Sprintf("%d", n.NumPods())
			event["ProcessorUtilization"] = fmt.Sprintf("%.2f", pct)
			event["InstanceType"] = fmt.Sprintf("%s", n.InstanceType())

			priceLabel := fmt.Sprintf("%0.4f", n.Price)
			if !n.HasPrice() {
				priceLabel = ""
			}
			event["PricePerHour"] = priceLabel

			if n.IsOnDemand() {
				event["NodeComputeType"] = "On-Demand"
			} else if n.IsSpot() {
				event["NodeComputeType"] = "Spot"
			} else if n.IsFargate() {
				event["NodeComputeType"] = "Fargate"
			} else {
				// Assume on-demand
				event["NodeComputeType"] = "On-Demand"
			}

			// Find machine pool name for CAPI/CAPA managed pools
			val, ok := n.node.Labels["infrastructureRef"]
			if ok {
				event["PoolName"] = val
			}
			val, ok = n.node.Labels["karpenter.sh/provisioner-name"]
			if ok {
				event["PoolName"] = val
			}
			val, ok = n.node.Annotations["cluster.x-k8s.io/machine"]
			if ok {
				event["PoolName"] = val
			}

			nr.RecordCustomEvent("EKSNodeUtilization", event)
		}
	}
	log.WithFields(log.Fields{"cluster": clusterName, "eventType": "EKSNodeUtilization"}).Info("Finished sending custom events")
}

//func (u *NRModel) writeClusterSummary(resources []v1.ResourceName, stats Stats) {
//	for _, res := range resources {
//		allocatable := stats.AllocatableResources[res]
//		used := stats.UsedResources[res]
//		pctUsed := 0.0
//		if allocatable.AsApproximateFloat64() != 0 {
//			pctUsed = 100 * (used.AsApproximateFloat64() / allocatable.AsApproximateFloat64())
//		}
//		pctUsedStr := fmt.Sprintf("%0.1f%%", pctUsed)
//
//		monthlyPrice := stats.TotalPrice * (365 * 24) / 12 // average hours per month
//		// stats.TotalPrice / hour
//		fmt.Printf("Cluster CPU Usage: %s, Hourly Price: $%0.3f, Monthly Price: $%0.3f\n", pctUsedStr, stats.TotalPrice, monthlyPrice)
//	}
//}

func (u *NRModel) SetResources(resources []string) {
	u.cluster.resources = nil
	for _, r := range resources {
		u.cluster.resources = append(u.cluster.resources, v1.ResourceName(r))
	}
}
