package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func main() {
	var (
		systemNamespace string
	)
	cmd := &cobra.Command{
		Use:   "evaluate",
		Args:  cobra.ExactArgs(1),
		Short: "Evaluate an individual rukpak Bundle's unpacked contents to stdout",
		Long: `
A kubectl plugin that's responsible for inspecting an individual plain-v0 rukpak Bundle's unpacked
contents that the core.rukpak.io/plain-v0 provisioner has stored on-cluster.

By default, the core.rukpak.io/plain-v0 provisioner sources and unpacks rukpak Bundle(s) to a series
of compressed ConfigMaps, where each ConfigMap contains a compressed version of a Kubernetes manifest.

This plugin helps simply the process of finding that list of underlying ConfigMap(s), decoding their contents,
and aggregating each of those contents into a single YAML stream to the stdout file descriptor.

Installing this plugin:

$ git clone https://github.com/timflannagan/kubectl-rukpak-plugin
$ cd kubectl-rukpak-plugin
$ make plugin

Example usage:

$ kubectl bundle evaluate combo-v0.0.1
apiVersion: v1
kind: Namespace
metadata:
  name: combo
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: combo-operator
  namespace: combo
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: combo
  name: combo-operator
  namespace: combo
...
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			bundleName := args[0]
			if bundleName == "" {
				return fmt.Errorf("validation error: --bundle cannot be empty")
			}
			if systemNamespace == "" {
				return fmt.Errorf("validation error: --namespace cannot be empty")
			}

			config := ctrl.GetConfigOrDie()
			c, err := client.New(config, client.Options{})
			if err != nil {
				return err
			}

			cmList := &corev1.ConfigMapList{}
			if err := c.List(context.Background(), cmList, &client.ListOptions{
				LabelSelector: newBundleConfigMapLabelSelector(bundleName),
				Namespace:     systemNamespace,
			}); err != nil {
				return err
			}
			if len(cmList.Items) == 0 {
				return nil
			}

			var res []string
			for _, cm := range cmList.Items {
				for _, v := range cm.BinaryData {
					reader, err := gzip.NewReader(bytes.NewReader(v))
					if err != nil {
						return err
					}
					output, err := ioutil.ReadAll(reader)
					if err != nil {
						return err
					}
					res = append(res, string(output))
				}
			}
			_, err = io.Copy(os.Stdout, strings.NewReader(strings.Join(res, "---\n")))
			return err
		},
	}
	cmd.Flags().StringVar(&systemNamespace, "namespace", "rukpak-system", "Configures the namespace to find the Bundle underlying resources")

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newBundleConfigMapLabelSelector(name string) labels.Selector {
	configMapTypeRequirement, err := labels.NewRequirement("core.rukpak.io/configmap-type", selection.Equals, []string{"object"})
	if err != nil {
		return nil
	}
	bundleRequirement, err := labels.NewRequirement("core.rukpak.io/owner-kind", selection.Equals, []string{"Bundle"})
	if err != nil {
		return nil
	}
	bundleNameRequirement, err := labels.NewRequirement("core.rukpak.io/owner-name", selection.Equals, []string{name})
	if err != nil {
		return nil
	}
	return labels.NewSelector().Add(*configMapTypeRequirement, *bundleRequirement, *bundleNameRequirement)
}
