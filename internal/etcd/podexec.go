package etcd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// certCandidate is one combination of CA / client cert / key paths to try
// when running etcdctl inside an etcd pod. Order matters: more specific
// distributions first so the lookup short-circuits on a match.
type certCandidate struct {
	ca, cert, key string
}

var etcdCertCandidates = []certCandidate{
	// RKE2
	{
		ca:   "/var/lib/rancher/rke2/server/tls/etcd/server-ca.crt",
		cert: "/var/lib/rancher/rke2/server/tls/etcd/server-client.crt",
		key:  "/var/lib/rancher/rke2/server/tls/etcd/server-client.key",
	},
	// k3s
	{
		ca:   "/var/lib/rancher/k3s/server/tls/etcd/server-ca.crt",
		cert: "/var/lib/rancher/k3s/server/tls/etcd/server-client.crt",
		key:  "/var/lib/rancher/k3s/server/tls/etcd/server-client.key",
	},
	// kubeadm (server-side cert reused for client auth)
	{
		ca:   "/etc/kubernetes/pki/etcd/ca.crt",
		cert: "/etc/kubernetes/pki/etcd/server.crt",
		key:  "/etc/kubernetes/pki/etcd/server.key",
	},
	// kubeadm (peer cert as client)
	{
		ca:   "/etc/kubernetes/pki/etcd/ca.crt",
		cert: "/etc/kubernetes/pki/etcd/peer.crt",
		key:  "/etc/kubernetes/pki/etcd/peer.key",
	},
}

// statusJSON is the shape returned by `etcdctl endpoint status -w json`.
type statusJSON []struct {
	Endpoint string `json:"Endpoint"`
	Status   struct {
		Header struct {
			MemberID uint64 `json:"member_id"`
			RaftTerm uint64 `json:"raft_term"`
		} `json:"header"`
		Version        string `json:"version"`
		DBSize         int64  `json:"dbSize"`
		DBSizeInUse    int64  `json:"dbSizeInUse"`
		DBSizeQuota    int64  `json:"dbSizeQuota"`
		Leader         uint64 `json:"leader"`
		RaftIndex      uint64 `json:"raftIndex"`
		RaftAppliedIdx uint64 `json:"raftAppliedIndex"`
	} `json:"Status"`
}

// memberListJSON is the shape returned by `etcdctl member list -w json`.
type memberListJSON struct {
	Members []struct {
		ID         uint64   `json:"ID"`
		Name       string   `json:"name"`
		PeerURLs   []string `json:"peerURLs"`
		ClientURLs []string `json:"clientURLs"`
	} `json:"members"`
}

// alarmListJSON is the shape returned by `etcdctl alarm list -w json`.
type alarmListJSON struct {
	Alarms []struct {
		MemberID uint64 `json:"memberID"`
		Alarm    string `json:"alarm"`
	} `json:"alarms"`
}

// PodExec returns a Reacher that fetches etcd state by execing into the
// etcd pods themselves and running `etcdctl`. Works on RKE2/k3s/kubeadm
// out-of-cluster — no host mounts, no extra image, no listener changes.
// Requires `pods/exec` permission in kube-system.
func PodExec(clientset kubernetes.Interface, restConfig *rest.Config) func(ctx context.Context, opts Options) (*Status, error) {
	return func(ctx context.Context, opts Options) (*Status, error) {
		if restConfig == nil {
			return nil, fmt.Errorf("%w: pod-exec needs a rest.Config", ErrUnavailable)
		}
		pods, err := clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
			LabelSelector: "component=etcd",
		})
		if err != nil {
			return nil, fmt.Errorf("%w: list etcd pods: %w", ErrUnavailable, err)
		}
		if len(pods.Items) == 0 {
			return nil, fmt.Errorf("%w: no pods component=etcd in kube-system", ErrUnavailable)
		}

		st := &Status{Mode: ModePodExec, CollectedAt: time.Now()}

		// Per-pod: run endpoint status; first pod that succeeds also gives
		// us alarms + members for the cluster as a whole.
		var pickedCerts *certCandidate
		clusterDataFetched := false
		for i := range pods.Items {
			pod := &pods.Items[i]
			certs, perPod, err := podStatus(ctx, restConfig, clientset, pod)
			if err != nil {
				st.Members = append(st.Members, MemberStatus{
					Endpoint: pod.Name,
					Errors:   []string{err.Error()},
				})
				continue
			}
			pickedCerts = certs
			st.Members = append(st.Members, perPod)
			st.SizeBytes += perPod.DBSize
			st.SizeInUseBytes += perPod.DBSizeInUse
			st.Reachable = true
			if perPod.LeaderID != 0 {
				st.HasLeader = true
			}

			if !clusterDataFetched && pickedCerts != nil {
				st.QuotaBytes = fetchQuota(ctx, restConfig, clientset, pod, *pickedCerts)
				st.Alarms = fetchAlarms(ctx, restConfig, clientset, pod, *pickedCerts)
				clusterDataFetched = true
			}
		}

		if !st.Reachable {
			return st, fmt.Errorf("%w: no etcd pod responded to endpoint status", ErrUnavailable)
		}
		if st.QuotaBytes == 0 {
			st.QuotaBytes = opts.QuotaBytes
		}
		return st, nil
	}
}

// podStatus tries each certCandidate against pod, returning the first
// successful (certs, member-status) pair.
func podStatus(ctx context.Context, cfg *rest.Config, cs kubernetes.Interface, pod *corev1.Pod) (*certCandidate, MemberStatus, error) {
	for _, c := range etcdCertCandidates {
		out, err := execEtcdctl(ctx, cfg, cs, pod, c, "endpoint", "status", "-w", "json")
		if err != nil {
			continue
		}
		var parsed statusJSON
		if err := json.Unmarshal(out, &parsed); err != nil || len(parsed) == 0 {
			continue
		}
		s := parsed[0]
		ms := MemberStatus{
			Endpoint:    pod.Name,
			Reachable:   true,
			Version:     s.Status.Version,
			DBSize:      s.Status.DBSize,
			DBSizeInUse: s.Status.DBSizeInUse,
			LeaderID:    s.Status.Leader,
			RaftIndex:   s.Status.RaftIndex,
		}
		// dbSizeQuota lives on Status; surface it via a stash on the
		// MemberStatus.Errors slice would be wrong, so we let the caller
		// fetch it once via fetchQuota when needed.
		_ = s.Status.DBSizeQuota
		return &c, ms, nil
	}
	return nil, MemberStatus{}, fmt.Errorf("no cert candidate worked for pod %s", pod.Name)
}

func fetchQuota(ctx context.Context, cfg *rest.Config, cs kubernetes.Interface, pod *corev1.Pod, c certCandidate) int64 {
	out, err := execEtcdctl(ctx, cfg, cs, pod, c, "endpoint", "status", "-w", "json")
	if err != nil {
		return 0
	}
	var parsed statusJSON
	if err := json.Unmarshal(out, &parsed); err != nil || len(parsed) == 0 {
		return 0
	}
	return parsed[0].Status.DBSizeQuota
}

func fetchAlarms(ctx context.Context, cfg *rest.Config, cs kubernetes.Interface, pod *corev1.Pod, c certCandidate) []Alarm {
	out, err := execEtcdctl(ctx, cfg, cs, pod, c, "alarm", "list", "-w", "json")
	if err != nil {
		return nil
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return nil
	}
	var parsed alarmListJSON
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil
	}
	out2 := make([]Alarm, 0, len(parsed.Alarms))
	for _, a := range parsed.Alarms {
		out2 = append(out2, Alarm{MemberID: a.MemberID, Type: a.Alarm})
	}
	return out2
}

// FetchMemberCount returns the cluster member count by execing one of the
// reachable etcd pods. Returns 0 if no pod responds.
func FetchMemberCount(ctx context.Context, cs kubernetes.Interface, cfg *rest.Config) int {
	pods, err := cs.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
		LabelSelector: "component=etcd",
	})
	if err != nil || len(pods.Items) == 0 {
		return 0
	}
	for i := range pods.Items {
		pod := &pods.Items[i]
		for _, c := range etcdCertCandidates {
			out, err := execEtcdctl(ctx, cfg, cs, pod, c, "member", "list", "-w", "json")
			if err != nil {
				continue
			}
			var parsed memberListJSON
			if err := json.Unmarshal(out, &parsed); err == nil && len(parsed.Members) > 0 {
				return len(parsed.Members)
			}
		}
	}
	return 0
}

func execEtcdctl(ctx context.Context, cfg *rest.Config, cs kubernetes.Interface, pod *corev1.Pod, c certCandidate, args ...string) ([]byte, error) {
	full := append([]string{
		"etcdctl",
		"--endpoints=https://127.0.0.1:2379",
		"--cacert=" + c.ca,
		"--cert=" + c.cert,
		"--key=" + c.key,
	}, args...)

	req := cs.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: full,
			Stdout:  true,
			Stderr:  true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		return nil, fmt.Errorf("new executor: %w", err)
	}
	var stdout, stderr bytes.Buffer
	if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	}); err != nil {
		return nil, fmt.Errorf("exec etcdctl %v: %w (stderr: %s)", args, err, stderr.String())
	}
	return stdout.Bytes(), nil
}
