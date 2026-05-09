package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/neilfarmer/k8s-health/internal/etcd"
)

// newEtcdProbeCmd is the hidden subcommand the in-cluster Job runs. It
// performs a direct-mode collection using kubeadm-default cert paths and
// prints the resulting Status as one JSON line so the launching khealth
// can read it from the pod log.
func newEtcdProbeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "etcd-probe",
		Short:  "Internal: run as Job to collect etcd status (do not call directly)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			endpoints := []string{"https://127.0.0.1:2379"}
			if v := os.Getenv("KHEALTH_ETCD_ENDPOINTS"); v != "" {
				endpoints = []string{v}
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			st, err := etcd.Collect(ctx, etcd.Options{
				Mode:        etcd.ModeDirect,
				Endpoints:   endpoints,
				CAFile:      "/etc/kubernetes/pki/etcd/ca.crt",
				CertFile:    "/etc/kubernetes/pki/etcd/healthcheck-client.crt",
				KeyFile:     "/etc/kubernetes/pki/etcd/healthcheck-client.key",
				DialTimeout: 10 * time.Second,
			}, etcd.Reachers{})
			if err != nil {
				fmt.Fprintf(os.Stderr, "etcd-probe: %v\n", err)
				if st == nil {
					return err
				}
			}
			b, mErr := json.Marshal(st)
			if mErr != nil {
				return mErr
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		},
	}
	return cmd
}
