package checks

import "fmt"

// Distro identifies a Kubernetes distribution. Some checks only make
// sense on a specific distro (e.g. RKE2 ships helm-install Jobs); they
// gate themselves by implementing DistroAware.
type Distro string

// Defined distros. DistroAuto is the sentinel passed by the CLI when the
// user has not forced a value; the runner is expected to resolve it via
// detection before filtering.
const (
	DistroAuto    Distro = "auto"
	DistroRKE2    Distro = "rke2"
	DistroK3s     Distro = "k3s"
	DistroKubeadm Distro = "kubeadm"
	DistroEKS     Distro = "eks"
)

// AllDistros lists every concrete (non-auto) distro. Useful for flag
// validation and detector fallthrough.
var AllDistros = []Distro{DistroRKE2, DistroK3s, DistroKubeadm, DistroEKS}

// ParseDistro validates and normalizes a user-supplied distro string.
func ParseDistro(s string) (Distro, error) {
	switch d := Distro(s); d {
	case DistroAuto, DistroRKE2, DistroK3s, DistroKubeadm, DistroEKS:
		return d, nil
	case "":
		return DistroAuto, nil
	default:
		return "", fmt.Errorf("unknown distro %q (want auto|rke2|k3s|kubeadm|eks)", s)
	}
}

// DistroAware is implemented by checks that only apply on specific
// distributions. Checks that omit this method run on every distro.
type DistroAware interface {
	Distros() []Distro
}

// AppliesToDistro reports whether c should run on distro. Checks that do
// not implement DistroAware always apply.
func AppliesToDistro(c Check, distro Distro) bool {
	da, ok := c.(DistroAware)
	if !ok {
		return true
	}
	for _, d := range da.Distros() {
		if d == distro {
			return true
		}
	}
	return false
}
