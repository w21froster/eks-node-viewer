package model

import (
	"fmt"
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

type OTelModel struct {
	cluster     *Cluster
	extraLabels []string
}

func NewOTelModel(extraLabels []string) *OTelModel {
	return &OTelModel{
		// red to green
		cluster:     NewCluster(),
		extraLabels: extraLabels,
	}
}

func (u *OTelModel) Cluster() *Cluster {
	return u.cluster
}

func (u *OTelModel) Run() {
	stats := u.cluster.Stats()

	u.writeClusterSummary(u.cluster.resources, stats)

	//if start >= 0 && end >= start {
	//	for _, n := range stats.Nodes[start:end] {
	//		u.writeNodeInfo(n, ctw, u.cluster.resources)
	//	}
	//}
}

//func (u *OTelModel) writeNodeInfo(n *Node, w io.Writer, resources []v1.ResourceName) {
//	allocatable := n.Allocatable()
//	used := n.Used()
//	firstLine := true
//	resNameLen := 0
//	for _, res := range resources {
//		if len(res) > resNameLen {
//			resNameLen = len(res)
//		}
//	}
//	for _, res := range resources {
//		usedRes := used[res]
//		allocatableRes := allocatable[res]
//		pct := usedRes.AsApproximateFloat64() / allocatableRes.AsApproximateFloat64()
//		if allocatableRes.AsApproximateFloat64() == 0 {
//			pct = 0
//		}
//
//		if firstLine {
//			priceLabel := fmt.Sprintf("/$%0.4f", n.Price)
//			if !n.HasPrice() {
//				priceLabel = ""
//			}
//			fmt.Fprintf(w, "%s\t%s\t%s\t(%d pods)\t%s%s", n.Name(), res, u.progress.ViewAs(pct), n.NumPods(), n.InstanceType(), priceLabel)
//
//			// node compute type
//			if n.IsOnDemand() {
//				fmt.Fprintf(w, "\tOn-Demand")
//			} else if n.IsSpot() {
//				fmt.Fprintf(w, "\tSpot")
//			} else if n.IsFargate() {
//				fmt.Fprintf(w, "\tFargate")
//			} else {
//				fmt.Fprintf(w, "\t-")
//			}
//
//			// node status
//			if n.Cordoned() && n.Deleting() {
//				fmt.Fprintf(w, "\tCordoned/Deleting")
//			} else if n.Deleting() {
//				fmt.Fprintf(w, "\tDeleting")
//			} else if n.Cordoned() {
//				fmt.Fprintf(w, "\tCordoned")
//			} else {
//				fmt.Fprintf(w, "\t-")
//			}
//
//			// node readiness or time we've been waiting for it to be ready
//			if n.Ready() {
//				fmt.Fprintf(w, "\tReady")
//			} else {
//				fmt.Fprintf(w, "\tNotReady/%s", duration.HumanDuration(time.Since(n.NotReadyTime())))
//			}
//
//			for _, label := range u.extraLabels {
//				labelValue, ok := n.node.Labels[label]
//				if !ok {
//					// support computed label values
//					labelValue = n.ComputeLabel(label)
//				}
//				fmt.Fprintf(w, "\t%s", labelValue)
//			}
//
//		} else {
//			fmt.Fprintf(w, " \t%s\t%s\t\t\t\t\t", res, u.progress.ViewAs(pct))
//			for range u.extraLabels {
//				fmt.Fprintf(w, "\t")
//			}
//		}
//		fmt.Fprintln(w)
//		firstLine = false
//	}
//}

func (u *OTelModel) writeClusterSummary(resources []v1.ResourceName, stats Stats) {
	for _, res := range resources {
		allocatable := stats.AllocatableResources[res]
		used := stats.UsedResources[res]
		pctUsed := 0.0
		if allocatable.AsApproximateFloat64() != 0 {
			pctUsed = 100 * (used.AsApproximateFloat64() / allocatable.AsApproximateFloat64())
		}
		pctUsedStr := fmt.Sprintf("%0.1f%%", pctUsed)

		monthlyPrice := stats.TotalPrice * (365 * 24) / 12 // average hours per month
		// stats.TotalPrice / hour
		fmt.Printf("Cluster CPU Usage: %s, Hourly Price: $%0.3f, Monthly Price: $%0.3f\n", pctUsedStr, stats.TotalPrice, monthlyPrice)
	}
}

func (u *OTelModel) SetResources(resources []string) {
	u.cluster.resources = nil
	for _, r := range resources {
		u.cluster.resources = append(u.cluster.resources, v1.ResourceName(r))
	}
}
