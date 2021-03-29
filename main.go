package main

import (
	"flag"
	"time"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	"github.com/zduymz/tag-to-label/pkg/apis/tag-to-label"
	"github.com/zduymz/tag-to-label/pkg/controller"
	"github.com/zduymz/tag-to-label/pkg/signals"
)

var config tag_to_label.Config

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(config.Master, config.KubeConfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	// (client kubernetes.Interface, defaultResync time.Duration)
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)

	controller, err := controller.NewController(kubeInformerFactory.Core().V1().Nodes(), kubeInformerFactory.Core().V1().Pods(), kubeClient, &config)
	if err != nil {
		klog.Fatalf("Error building kubernetes controller: %s", err.Error())
	}

	kubeInformerFactory.Start(stopCh)

	if err = controller.Run(stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&config.KubeConfig, "kubeconfig", "", "kubeconfig")
	flag.StringVar(&config.Master, "master", "", "master url")
	flag.StringVar(&config.AWSRegion, "aws.region", "us-west-2", "aws region")
	flag.IntVar(&config.APIRetries, "aws.retries", 3, "aws api call retries")
	flag.StringVar(&config.AWSAssumeRole, "aws.role", "", "aws assume role")
	flag.StringVar(&config.AWSCredsFile, "aws.creds", "", "aws creds")
}
