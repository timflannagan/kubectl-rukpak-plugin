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
		bundleName      string
		systemNamespace string
	)

	cmd := &cobra.Command{
		Use:  "evaluate",
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if bundleName == "" {
				return fmt.Errorf("--bundle cannot be empty")
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

	cmd.Flags().StringVar(&bundleName, "bundle", "", "Configures which Bundle resources to unpack")
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
