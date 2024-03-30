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
package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"github.com/awslabs/eks-node-viewer/pkg/agent"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/session"
	tea "github.com/charmbracelet/bubbletea"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"sigs.k8s.io/karpenter/pkg/apis/v1beta1"

	"github.com/awslabs/eks-node-viewer/pkg/client"
	"github.com/awslabs/eks-node-viewer/pkg/model"
	"github.com/awslabs/eks-node-viewer/pkg/pricing"
)

//go:generate cp -r ../../ATTRIBUTION.md ./
//go:embed ATTRIBUTION.md
var attribution string

func main() {
	flags, err := ParseFlags()
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		log.Fatalf("cannot parse flags: %v", err)
	}

	if flags.ShowAttribution {
		fmt.Println(attribution)
		os.Exit(0)
	}

	if flags.Version {
		fmt.Printf("eks-node-viewer version %s\n", version)
		fmt.Printf("commit: %s\n", commit)
		fmt.Printf("built at: %s\n", date)
		fmt.Printf("built by: %s\n", builtBy)
		os.Exit(0)
	}

	cs, err := client.Create(flags.Kubeconfig, flags.Context)
	if err != nil {
		log.Fatalf("creating client, %s", err)
	}
	nodeClaimClient, err := client.NodeClaims(flags.Kubeconfig, flags.Context)
	if err != nil {
		log.Fatalf("creating node claim client, %s", err)
	}
	ctx, cancel := context.WithCancel(context.Background())

	defaults.SharedCredentialsFilename()
	pprov := pricing.NewStaticProvider()

	if flags.NRMode {
		startNR(flags, pprov, ctx, cs, nodeClaimClient, cancel)
	} else {
		startUI(flags, pprov, ctx, cs, nodeClaimClient, cancel)
	}

}

func startUI(flags Flags, pprov *pricing.Provider, ctx context.Context, cs *kubernetes.Clientset, nodeClaimClient *rest.RESTClient, cancel context.CancelFunc) {
	style, err := model.ParseStyle(flags.Style)
	if err != nil {
		log.Fatalf("creating style, %s", err)
	}
	m := model.NewUIModel(strings.Split(flags.ExtraLabels, ","), flags.NodeSort, style)
	m.SetResources(strings.FieldsFunc(flags.Resources, func(r rune) bool { return r == ',' }))

	if !flags.DisablePricing {
		sess := session.Must(session.NewSessionWithOptions(session.Options{SharedConfigState: session.SharedConfigEnable}))
		updateAllPrices := func() {
			m.Cluster().ForEachNode(func(n *model.Node) {
				n.UpdatePrice(pprov)
			})
		}
		pprov = pricing.NewProvider(ctx, sess, updateAllPrices)
	}

	var nodeSelector labels.Selector
	if ns, err := labels.Parse(flags.NodeSelector); err != nil {
		log.Fatalf("parsing node selector: %s", err)
	} else {
		nodeSelector = ns
	}

	monitorSettings := &monitorSettingsUI{
		clientset:       cs,
		nodeClaimClient: nodeClaimClient,
		model:           m,
		nodeSelector:    nodeSelector,
		pricing:         pprov,
	}
	startMonitor(ctx, monitorSettings)
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		log.Fatalf("error running tea: %s", err)
	}
	cancel()
}

func startNR(flags Flags, pprov *pricing.Provider, ctx context.Context, cs *kubernetes.Clientset, nodeClaimClient *rest.RESTClient, cancel context.CancelFunc) {
	m := model.NewNRModel(strings.Split(flags.ExtraLabels, ","))
	m.SetResources(strings.FieldsFunc(flags.Resources, func(r rune) bool { return r == ',' }))

	if !flags.DisablePricing {
		sess := session.Must(session.NewSessionWithOptions(session.Options{SharedConfigState: session.SharedConfigEnable}))
		updateAllPrices := func() {
			m.Cluster().ForEachNode(func(n *model.Node) {
				n.UpdatePrice(pprov)
			})
		}
		pprov = pricing.NewProvider(ctx, sess, updateAllPrices)
	}

	var nodeSelector labels.Selector
	if ns, err := labels.Parse(flags.NodeSelector); err != nil {
		log.Fatalf("parsing node selector: %s", err)
	} else {
		nodeSelector = ns
	}

	monitorSettings := &monitorSettingsNR{
		clientset:       cs,
		nodeClaimClient: nodeClaimClient,
		model:           m,
		nodeSelector:    nodeSelector,
		pricing:         pprov,
	}
	startMonitor(ctx, monitorSettings)
	nr, err := agent.SetupApp(flags.NRAppName, flags.NRHostName, flags.NRLicenseKey)
	if err != nil {
		log.Fatalf("error setting up NewRelic agent: %s", err)
	}
	for {
		m.Run(flags.Context, nr)
		time.Sleep(1 * time.Minute)
	}
}

type Settings interface {
	ClientSet() *kubernetes.Clientset
	Cluster() *model.Cluster
	NodeClaimClient() *rest.RESTClient
	NodeSelector() labels.Selector
	Pricing() *pricing.Provider
}

type monitorSettingsNR struct {
	clientset       *kubernetes.Clientset
	nodeClaimClient *rest.RESTClient
	model           *model.NRModel
	nodeSelector    labels.Selector
	pricing         *pricing.Provider
}

func (s *monitorSettingsNR) ClientSet() *kubernetes.Clientset  { return s.clientset }
func (s *monitorSettingsNR) Cluster() *model.Cluster           { return s.model.Cluster() }
func (s *monitorSettingsNR) NodeClaimClient() *rest.RESTClient { return s.nodeClaimClient }
func (s *monitorSettingsNR) NodeSelector() labels.Selector     { return s.nodeSelector }
func (s *monitorSettingsNR) Pricing() *pricing.Provider        { return s.pricing }

type monitorSettingsUI struct {
	clientset       *kubernetes.Clientset
	nodeClaimClient *rest.RESTClient
	model           *model.UIModel
	nodeSelector    labels.Selector
	pricing         *pricing.Provider
}

func (s *monitorSettingsUI) ClientSet() *kubernetes.Clientset  { return s.clientset }
func (s *monitorSettingsUI) Cluster() *model.Cluster           { return s.model.Cluster() }
func (s *monitorSettingsUI) NodeClaimClient() *rest.RESTClient { return s.nodeClaimClient }
func (s *monitorSettingsUI) NodeSelector() labels.Selector     { return s.nodeSelector }
func (s *monitorSettingsUI) Pricing() *pricing.Provider        { return s.pricing }

func startMonitor(ctx context.Context, s Settings) {
	podWatchList := cache.NewListWatchFromClient(s.ClientSet().CoreV1().RESTClient(), "pods",
		v1.NamespaceAll, fields.Everything())

	cluster := s.Cluster()
	_, podController := cache.NewInformer(
		podWatchList,
		&v1.Pod{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				p := obj.(*v1.Pod)
				if !isTerminalPod(p) {
					cluster.AddPod(model.NewPod(p))
					node, ok := cluster.GetNodeByName(p.Spec.NodeName)
					// need to potentially update node price as we need the fargate pod in order to figure out the cost
					if ok && node.IsFargate() && !node.HasPrice() {
						node.UpdatePrice(s.Pricing())
					}
				}
			},
			DeleteFunc: func(obj interface{}) {
				p := obj.(*v1.Pod)
				cluster.DeletePod(p.Namespace, p.Name)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				p := newObj.(*v1.Pod)
				if isTerminalPod(p) {
					cluster.DeletePod(p.Namespace, p.Name)
				} else {
					pod, ok := cluster.GetPod(p.Namespace, p.Name)
					if !ok {
						cluster.AddPod(model.NewPod(p))
					} else {
						pod.Update(p)
						cluster.AddPod(pod)
					}
				}
			},
		},
	)
	go podController.Run(ctx.Done())

	nodeWatchList := cache.NewFilteredListWatchFromClient(s.ClientSet().CoreV1().RESTClient(), "nodes",
		v1.NamespaceAll, func(options *metav1.ListOptions) {
			options.LabelSelector = s.NodeSelector().String()
		})
	_, nodeController := cache.NewInformer(
		nodeWatchList,
		&v1.Node{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				node := model.NewNode(obj.(*v1.Node))
				node.UpdatePrice(s.Pricing())
				n := cluster.AddNode(node)
				n.Show()
			},
			DeleteFunc: func(obj interface{}) {
				cluster.DeleteNode(obj.(*v1.Node).Spec.ProviderID)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				n := newObj.(*v1.Node)
				if !n.DeletionTimestamp.IsZero() && len(n.Finalizers) == 0 {
					cluster.DeleteNode(n.Spec.ProviderID)
				} else {
					node, ok := cluster.GetNode(n.Spec.ProviderID)
					if !ok {
						log.Println("unable to find node", n.Name)
					} else {
						node.Update(n)
						node.UpdatePrice(s.Pricing())
					}
					node.Show()
				}
			},
		},
	)
	go nodeController.Run(ctx.Done())

	// If a NodeClaims Get returns an error, then don't startup the nodeclaims controller since the CRD is not registered
	if err := s.NodeClaimClient().Get().Do(ctx).Error(); err == nil {
		nodeClaimWatchList := cache.NewFilteredListWatchFromClient(s.NodeClaimClient(), "nodeclaims",
			v1.NamespaceAll, func(options *metav1.ListOptions) {
				options.LabelSelector = s.NodeSelector().String()
			})
		_, nodeClaimController := cache.NewInformer(
			nodeClaimWatchList,
			&v1beta1.NodeClaim{},
			time.Second*0,
			cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					nc := obj.(*v1beta1.NodeClaim)
					if nc.Status.ProviderID == "" {
						return
					}
					if _, ok := cluster.GetNode(nc.Status.ProviderID); ok {
						return
					}
					node := model.NewNodeFromNodeClaim(nc)
					node.UpdatePrice(s.Pricing())
					n := cluster.AddNode(node)
					n.Show()
				},
				DeleteFunc: func(obj interface{}) {
					cluster.DeleteNode(obj.(*v1beta1.NodeClaim).Status.ProviderID)
				},
				UpdateFunc: func(oldObj, newObj interface{}) {
					nc := newObj.(*v1beta1.NodeClaim)
					if nc.Status.ProviderID == "" {
						return
					}
					if _, ok := cluster.GetNode(nc.Status.ProviderID); ok {
						return
					}
					node := model.NewNodeFromNodeClaim(nc)
					node.UpdatePrice(s.Pricing())
					n := cluster.AddNode(node)
					n.Show()
				},
			},
		)
		go nodeClaimController.Run(ctx.Done())
	}
}

// isTerminalPod returns true if the pod is deleting or in a terminal state
func isTerminalPod(p *v1.Pod) bool {
	if !p.DeletionTimestamp.IsZero() {
		return true
	}
	switch p.Status.Phase {
	case v1.PodSucceeded, v1.PodFailed:
		return true
	}
	return false
}
