package k8s

import (
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"
)

var (
	resyncPeriod = 30 * time.Second

	builderLabelSelector = labels.Set{
		"buildType": "slugBuilder",
	}.AsSelector()
)

type BuildPodWatcher struct {
	Store      cache.StoreToPodLister
	Controller *framework.Controller
}

func NewBuildPodWatcher(c *client.Client, ns string) *BuildPodWatcher {
	pw := &BuildPodWatcher{}

	pw.Store.Indexer, pw.Controller = framework.NewIndexerInformer(
		&cache.ListWatch{
			ListFunc:  podListFunc(c, ns),
			WatchFunc: podWatchFunc(c, ns),
		},
		&api.Pod{},
		resyncPeriod,
		framework.ResourceEventHandlerFuncs{},
		cache.Indexers{},
	)

	return pw
}

func podListFunc(c *client.Client, ns string) func(options api.ListOptions) (runtime.Object, error) {
	return func(opts api.ListOptions) (runtime.Object, error) {
		return c.Pods(ns).List(api.ListOptions{
			LabelSelector: builderLabelSelector,
		})
	}
}

func podWatchFunc(c *client.Client, ns string) func(options api.ListOptions) (watch.Interface, error) {
	return func(opts api.ListOptions) (watch.Interface, error) {
		return c.Pods(ns).Watch(api.ListOptions{
			LabelSelector: builderLabelSelector,
		})
	}
}
